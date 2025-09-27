package main

import (
	"backup-health-monitor/internal/config"
	"backup-health-monitor/internal/health"
	"backup-health-monitor/internal/logging"
	"backup-health-monitor/internal/server"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	// Version information (injected at build time via ldflags)
	version = "dev-build"

	// Global flags
	configFile string
	verbose    bool
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "backup-health-monitor",
	Short: "A backup health monitoring service",
	Long: `A Go implementation for monitoring backup services that checks systemd service status
and backup file availability, providing HTTP endpoints for health status monitoring.

Examples:
  backup-health-monitor check --config /path/to/config.yaml
  backup-health-monitor check --service vault-backup.service --json
  backup-health-monitor validate --config /path/to/config.yaml
  backup-health-monitor serve --port 8080 --bind 0.0.0.0`,
	Version: version,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Add global flags
	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file path (default searches ./configs/config.yaml, /etc/backup-health/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "V", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")

	// Add commands
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(serveCmd)
}

// formatOutput formats the output based on the global JSON flag
func formatOutput(data any) error {
	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	}

	// Human-readable format
	switch v := data.(type) {
	case *health.HealthResult:
		return formatHealthResult(v)
	case *health.ServiceResult:
		return formatServiceResult(v)
	case string:
		fmt.Println(v)
		return nil
	default:
		// Fallback to JSON for unknown types
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	}
}

// formatHealthResult formats HealthResult in human-readable format
func formatHealthResult(result *health.HealthResult) error {
	fmt.Printf("=== Backup Health Monitor Status ===\n")
	fmt.Printf("Overall Status: %s\n", strings.ToUpper(string(result.Status)))
	fmt.Printf("Timestamp: %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Version: %s\n\n", result.Version)

	fmt.Printf("Summary: %d total, %d healthy, %d degraded, %d unhealthy\n\n",
		result.Summary.TotalServices,
		result.Summary.HealthyServices,
		result.Summary.DegradedServices,
		result.Summary.UnhealthyServices)

	for i, service := range result.Services {
		if i > 0 {
			fmt.Println()
		}
		if err := formatServiceResult(&service); err != nil {
			return err
		}
	}

	return nil
}

// formatServiceResult formats ServiceResult in human-readable format
func formatServiceResult(service *health.ServiceResult) error {
	statusSymbol := "✓"
	switch service.Status {
	case "warning":
		statusSymbol = "⚠"
	case "critical":
		statusSymbol = "✗"
	}

	fmt.Printf("%s %s [%s]\n", statusSymbol, service.Name, strings.ToUpper(service.Status))

	if verbose {
		// Systemd info
		fmt.Printf("  Systemd:\n")
		fmt.Printf("    Enabled: %t\n", service.Systemd.Enabled)
		fmt.Printf("    Exit Code: %d\n", service.Systemd.ExitCode)
		if !service.Systemd.LastRun.IsZero() {
			fmt.Printf("    Last Run: %s\n", service.Systemd.LastRun.Format("2006-01-02 15:04:05"))
		}

		// Backup info
		fmt.Printf("  Backup:\n")
		fmt.Printf("    Path: %s\n", service.Backup.Path)
		fmt.Printf("    Recent Files: %d\n", service.Backup.RecentFiles)
		if service.Backup.NewestFileAge > 0 {
			fmt.Printf("    Newest File Age: %.1f hours\n", service.Backup.NewestFileAge)
			fmt.Printf("    Newest File Size: %.2f MB\n", service.Backup.NewestFileSizeMB)
		}
		fmt.Printf("    Status: %s\n", service.Backup.Status)
	}

	// Show issues if any
	if len(service.Issues) > 0 {
		fmt.Printf("  Issues:\n")
		for _, issue := range service.Issues {
			fmt.Printf("    - %s\n", issue)
		}
	}

	// Show error if any
	if service.Error != "" {
		fmt.Printf("  Error: %s\n", service.Error)
	}

	return nil
}

// loadConfiguration loads the configuration file and initializes the logger
func loadConfiguration() (*config.Config, error) {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := config.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Initialize logger based on configuration
	if err := logging.InitializeLogger(&cfg.Logging); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Log configuration loading success
	logging.WithFields(map[string]any{
		"config_file":    configFile,
		"services_count": len(cfg.Services),
		"log_level":      cfg.Logging.Level,
		"log_format":     cfg.Logging.Format,
	}).Info("Configuration loaded successfully")

	if verbose {
		fmt.Fprintf(os.Stderr, "Loaded configuration with %d services\n", len(cfg.Services))
	}

	return cfg, nil
}

// getServiceNames extracts service names from service configs for logging
func getServiceNames(services []config.ServiceConfig) []string {
	names := make([]string, len(services))
	for i, service := range services {
		names[i] = service.Name
	}
	return names
}

// Command definitions
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run a one-time health check",
	Long: `Run a health check on all configured backup services or a specific service.
This command checks both systemd service status and backup file availability.

Examples:
  backup-health-monitor check                           # Check all services
  backup-health-monitor check --service vault-backup.service  # Check specific service
  backup-health-monitor check --json                   # Output in JSON format
  backup-health-monitor check --verbose                # Detailed output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := loadConfiguration()
		if err != nil {
			return err
		}

		// Get service flag
		serviceName, _ := cmd.Flags().GetString("service")

		if serviceName != "" {
			// Check specific service
			var serviceConfig *config.ServiceConfig
			for _, svc := range cfg.Services {
				if svc.Name == serviceName {
					serviceConfig = &svc
					break
				}
			}

			if serviceConfig == nil {
				logging.WithFields(map[string]any{
					"service_name":       serviceName,
					"available_services": getServiceNames(cfg.Services),
				}).Error("Service not found in configuration")
				return fmt.Errorf("service '%s' not found in configuration", serviceName)
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "Checking service: %s\n", serviceName)
			}

			logging.WithServiceName(serviceName).Info("Starting service health check")
			result := health.CheckService(*serviceConfig)
			logging.WithFields(map[string]any{
				"service": serviceName,
				"status":  result.Status,
			}).Info("Service health check completed")

			if err := formatOutput(&result); err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			// Set exit code based on service status
			switch result.Status {
			case "healthy":
				return nil
			case "warning":
				os.Exit(1)
			case "critical":
				os.Exit(2)
			default:
				os.Exit(2)
			}
		} else {
			// Check all services
			if verbose {
				fmt.Fprintf(os.Stderr, "Checking all %d services\n", len(cfg.Services))
			}

			logging.WithFields(map[string]any{
				"services_count": len(cfg.Services),
			}).Info("Starting health check for all services")
			result := health.CheckAllServices(cfg, version)
			logging.WithFields(map[string]any{
				"overall_status":     string(result.Status),
				"healthy_services":   result.Summary.HealthyServices,
				"degraded_services":  result.Summary.DegradedServices,
				"unhealthy_services": result.Summary.UnhealthyServices,
			}).Info("Health check for all services completed")

			if err := formatOutput(&result); err != nil {
				return fmt.Errorf("failed to format output: %w", err)
			}

			// Set exit code based on overall status
			switch result.Status {
			case health.StatusHealthy:
				return nil
			case health.StatusWarning:
				os.Exit(1)
			case health.StatusCritical:
				os.Exit(2)
			default:
				os.Exit(2)
			}
		}

		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long: `Validate the configuration file for syntax and logical errors.
This command loads the configuration file and checks for:
- Valid YAML syntax
- Required fields presence
- Logical consistency (e.g., positive numbers, valid paths)
- Duplicate service names

Examples:
  backup-health-monitor validate                        # Use default config locations
  backup-health-monitor validate --config /path/to/config.yaml
  backup-health-monitor validate --json                # JSON output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			fmt.Fprintf(os.Stderr, "Validating configuration...\n")
		}

		// Load and validate configuration
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			if jsonOutput {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				return formatOutput(result)
			}
			return fmt.Errorf("configuration loading failed: %w", err)
		}

		// Validate configuration
		if err := config.ValidateConfig(cfg); err != nil {
			if jsonOutput {
				result := map[string]any{
					"valid": false,
					"error": err.Error(),
				}
				return formatOutput(result)
			}
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		// Configuration is valid
		if jsonOutput {
			result := map[string]any{
				"valid":    true,
				"services": len(cfg.Services),
				"message":  "Configuration is valid",
			}
			return formatOutput(result)
		} else {
			fmt.Printf("✓ Configuration is valid\n")
			fmt.Printf("  Services configured: %d\n", len(cfg.Services))
			if verbose {
				fmt.Printf("  Server port: %d\n", cfg.Server.Port)
				fmt.Printf("  Server bind address: %s\n", cfg.Server.BindAddress)
				fmt.Printf("  Log level: %s\n", cfg.Logging.Level)
				fmt.Printf("  Log format: %s\n", cfg.Logging.Format)
			}
		}

		return nil
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server",
	Long: `Start the HTTP server to provide health check endpoints.
The server provides the following endpoints:
  - GET /health                    - Simple health status (compatible with bash script)
  - GET /api/v1/status            - Detailed JSON status information
  - GET /api/v1/services/{name}   - Individual service details
  - GET /metrics                  - Prometheus metrics

Examples:
  backup-health-monitor serve                          # Use default configuration
  backup-health-monitor serve --config /path/to/config.yaml
  backup-health-monitor serve --port 8080 --bind 0.0.0.0`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := loadConfiguration()
		if err != nil {
			return err
		}

		// Apply command-line flag overrides
		if port, _ := cmd.Flags().GetInt("port"); port != 0 {
			cfg.Server.Port = port
			logging.WithFields(map[string]any{"port_override": port}).
				Info("Port overridden via command line")
		}
		if bind, _ := cmd.Flags().GetString("bind"); bind != "" {
			cfg.Server.BindAddress = bind
			logging.WithFields(map[string]any{"bind_override": bind}).
				Info("Bind address overridden via command line")
		}

		if verbose {
			logging.WithFields(map[string]any{
				"bind_address": cfg.Server.BindAddress,
				"port":         cfg.Server.Port,
			}).Info("Starting HTTP server")
		}

		// Create and configure server
		srv := server.NewServer(cfg)

		// Set up signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Start server in a goroutine
		serverErrChan := make(chan error, 1)
		go func() {
			logging.WithFields(map[string]any{
				"port":         cfg.Server.Port,
				"bind_address": cfg.Server.BindAddress,
			}).Info("Starting HTTP server")

			if err := srv.Start(); err != nil {
				serverErrChan <- err
			}
		}()

		// Wait for either a shutdown signal or server error
		select {
		case sig := <-sigChan:
			logging.WithFields(map[string]any{"signal": sig.String()}).
				Info("Received shutdown signal")
			if verbose {
				logging.WithFields(map[string]any{
					"signal": sig.String(),
				}).Info("Received shutdown signal, shutting down gracefully")
			}
		case err := <-serverErrChan:
			logging.WithError(err).Error("Server failed to start")
			return fmt.Errorf("server failed to start: %w", err)
		}

		// Perform graceful shutdown
		logging.Logger.Info("Initiating graceful server shutdown")
		if err := srv.Stop(); err != nil {
			logging.WithError(err).Error("Error during server shutdown")
			return fmt.Errorf("error during server shutdown: %w", err)
		}

		logging.Logger.Info("Server shutdown completed successfully")
		if verbose {
			logging.Logger.Info("Server shutdown completed successfully")
		}

		return nil
	},
}

func init() {
	// Add service-specific flag to check command
	checkCmd.Flags().StringP("service", "s", "", "check only the specified service")

	// Add server-specific flags to serve command
	serveCmd.Flags().IntP("port", "p", 0, "port to bind the server to (overrides config file)")
	serveCmd.Flags().
		StringP("bind", "b", "", "address to bind the server to (overrides config file)")
}
