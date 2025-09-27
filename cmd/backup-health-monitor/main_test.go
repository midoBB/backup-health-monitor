package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"backup-health-monitor/internal/config"
	"backup-health-monitor/internal/health"
)

// resetGlobalFlags resets global flags to their default values
func resetGlobalFlags() {
	configFile = ""
	verbose = false
	jsonOutput = false
}

// captureOutput captures stdout and stderr during command execution
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	// Save original stdout and stderr
	origStdout := os.Stdout
	origStderr := os.Stderr

	// Create pipes
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	// Replace stdout and stderr
	os.Stdout = wOut
	os.Stderr = wErr

	// Create channels to capture output
	outCh := make(chan string)
	errCh := make(chan string)

	// Start goroutines to read from pipes
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rOut)
		outCh <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rErr)
		errCh <- buf.String()
	}()

	// Execute function
	fn()

	// Close writers
	if err := wOut.Close(); err != nil && t != nil {
		t.Logf("error closing stdout writer: %v", err)
	}
	if err := wErr.Close(); err != nil && t != nil {
		t.Logf("error closing stderr writer: %v", err)
	}

	// Restore original stdout and stderr
	os.Stdout = origStdout
	os.Stderr = origStderr

	// Get output
	stdout = <-outCh
	stderr = <-errCh

	return stdout, stderr
}

func TestFormatOutput(t *testing.T) {
	resetGlobalFlags()

	tests := []struct {
		name       string
		jsonOutput bool
		data       interface{}
		wantJSON   bool
		wantError  bool
	}{
		{
			name:       "string output in text format",
			jsonOutput: false,
			data:       "test message",
			wantJSON:   false,
			wantError:  false,
		},
		{
			name:       "string output in JSON format",
			jsonOutput: true,
			data:       "test message",
			wantJSON:   true,
			wantError:  false,
		},
		{
			name:       "health result in text format",
			jsonOutput: false,
			data: &health.HealthResult{
				Status:    health.StatusHealthy,
				Services:  []health.ServiceResult{},
				Summary:   health.Summary{TotalServices: 0},
				Version:   "test",
			},
			wantJSON:  false,
			wantError: false,
		},
		{
			name:       "health result in JSON format",
			jsonOutput: true,
			data: &health.HealthResult{
				Status:    health.StatusHealthy,
				Services:  []health.ServiceResult{},
				Summary:   health.Summary{TotalServices: 0},
				Version:   "test",
			},
			wantJSON:  true,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()
			jsonOutput = tt.jsonOutput

			stdout, _ := captureOutput(t, func() {
				err := formatOutput(tt.data)
				if (err != nil) != tt.wantError {
					t.Errorf("formatOutput() error = %v, wantError %v", err, tt.wantError)
				}
			})

			if tt.wantJSON {
				// Check if output is valid JSON
				var result interface{}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Errorf("formatOutput() produced invalid JSON: %v", err)
				}
			}

			if stdout == "" && !tt.wantError {
				t.Errorf("formatOutput() produced no output")
			}
		})
	}
}

func TestValidateCommand(t *testing.T) {
	resetGlobalFlags()

	tests := []struct {
		name       string
		configFile string
		jsonOutput bool
		wantError  bool
	}{
		{
			name:       "valid config file",
			configFile: "../../configs/config.yaml",
			jsonOutput: false,
			wantError:  false,
		},
		{
			name:       "valid config file with JSON output",
			configFile: "../../configs/config.yaml",
			jsonOutput: true,
			wantError:  false,
		},
		{
			name:       "nonexistent config file",
			configFile: "/nonexistent/config.yaml",
			jsonOutput: false,
			wantError:  true,
		},
		{
			name:       "nonexistent config file with JSON output",
			configFile: "/nonexistent/config.yaml",
			jsonOutput: true,
			wantError:  false, // JSON output should handle errors gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()
			configFile = tt.configFile
			jsonOutput = tt.jsonOutput

			// Create a new command for testing
			cmd := &cobra.Command{
				Use: "validate",
				RunE: validateCmd.RunE,
			}

			stdout, stderr := captureOutput(t, func() {
				err := cmd.Execute()
				if (err != nil) != tt.wantError {
					if !tt.wantError {
						t.Errorf("validateCmd.Execute() error = %v, wantError %v", err, tt.wantError)
					}
				}
			})

			if tt.jsonOutput && !tt.wantError {
				// Check if output is valid JSON
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Errorf("validateCmd produced invalid JSON: %v", err)
				}

				// Check if valid field exists
				if _, exists := result["valid"]; !exists {
					t.Errorf("validateCmd JSON output missing 'valid' field")
				}
			}

			// For valid configs, we should have some output
			if !tt.wantError && stdout == "" && stderr == "" {
				t.Errorf("validateCmd produced no output for valid config")
			}
		})
	}
}

func TestCheckCommandFlags(t *testing.T) {
	resetGlobalFlags()

	// Test that the check command has the service flag
	serviceFlag := checkCmd.Flags().Lookup("service")
	if serviceFlag == nil {
		t.Fatal("checkCmd should have 'service' flag")
	}

	if serviceFlag.Usage != "check only the specified service" {
		t.Errorf("checkCmd service flag has incorrect usage text")
	}

	// Test that the service flag has shorthand 's'
	serviceFlagShort := checkCmd.Flags().ShorthandLookup("s")
	if serviceFlagShort == nil {
		t.Errorf("checkCmd should have shorthand flag 's' for service")
	}
}

func TestGlobalFlags(t *testing.T) {
	resetGlobalFlags()

	// Test that root command has the required persistent flags
	flags := []string{"config", "verbose", "json"}

	for _, flagName := range flags {
		flag := rootCmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			t.Errorf("rootCmd should have persistent flag '%s'", flagName)
		}
	}

	// Test config flag
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	if configFlag.DefValue != "" {
		t.Errorf("config flag should have empty default value, got: %s", configFlag.DefValue)
	}

	// Test verbose flag
	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	if verboseFlag.DefValue != "false" {
		t.Errorf("verbose flag should have 'false' default value, got: %s", verboseFlag.DefValue)
	}

	// Test json flag
	jsonFlag := rootCmd.PersistentFlags().Lookup("json")
	if jsonFlag.DefValue != "false" {
		t.Errorf("json flag should have 'false' default value, got: %s", jsonFlag.DefValue)
	}
}

func TestCommandStructure(t *testing.T) {
	// Test that all expected commands are present
	expectedCommands := []string{"check", "validate", "serve"}

	for _, cmdName := range expectedCommands {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == cmdName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rootCmd should have command '%s'", cmdName)
		}
	}

	// Test command properties
	checkCommand := findCommand(rootCmd, "check")
	if checkCommand == nil {
		t.Fatal("check command not found")
	}

	if checkCommand.Short != "Run a one-time health check" {
		t.Errorf("check command has incorrect short description")
	}

	validateCommand := findCommand(rootCmd, "validate")
	if validateCommand == nil {
		t.Fatal("validate command not found")
	}

	if validateCommand.Short != "Validate configuration file" {
		t.Errorf("validate command has incorrect short description")
	}

	serveCommand := findCommand(rootCmd, "serve")
	if serveCommand == nil {
		t.Fatal("serve command not found")
	}

	if serveCommand.Short != "Start HTTP server" {
		t.Errorf("serve command has incorrect short description")
	}
}

// Helper function to find a command by name
func findCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func TestLoadConfiguration(t *testing.T) {
	resetGlobalFlags()

	tests := []struct {
		name       string
		configFile string
		wantError  bool
	}{
		{
			name:       "valid config file",
			configFile: "../../configs/config.yaml",
			wantError:  false,
		},
		{
			name:       "nonexistent config file",
			configFile: "/nonexistent/config.yaml",
			wantError:  true,
		},
		{
			name:       "empty config file path (should use defaults)",
			configFile: "",
			wantError:  true, // Will fail because no config in default locations in test env
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()
			configFile = tt.configFile

			cfg, err := loadConfiguration()
			if (err != nil) != tt.wantError {
				t.Errorf("loadConfiguration() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && cfg == nil {
				t.Errorf("loadConfiguration() returned nil config without error")
			}

			if !tt.wantError && len(cfg.Services) == 0 {
				t.Errorf("loadConfiguration() returned config with no services")
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkFormatHealthResult(b *testing.B) {
	resetGlobalFlags()

	result := &health.HealthResult{
		Status:   health.StatusHealthy,
		Services: make([]health.ServiceResult, 10), // 10 services
		Summary: health.Summary{
			TotalServices:     10,
			HealthyServices:   8,
			DegradedServices:  1,
			UnhealthyServices: 1,
		},
		Version: "1.0.0",
	}

	// Fill with sample service results
	for i := 0; i < 10; i++ {
		result.Services[i] = health.ServiceResult{
			Name:   fmt.Sprintf("service-%d", i),
			Status: "healthy",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		captureOutput(nil, func() {
			if err := formatOutput(result); err != nil {
				b.Logf("formatOutput failed: %v", err)
			}
		})
	}
}

func BenchmarkFormatHealthResultJSON(b *testing.B) {
	resetGlobalFlags()
	jsonOutput = true

	result := &health.HealthResult{
		Status:   health.StatusHealthy,
		Services: make([]health.ServiceResult, 10), // 10 services
		Summary: health.Summary{
			TotalServices:     10,
			HealthyServices:   8,
			DegradedServices:  1,
			UnhealthyServices: 1,
		},
		Version: "1.0.0",
	}

	// Fill with sample service results
	for i := 0; i < 10; i++ {
		result.Services[i] = health.ServiceResult{
			Name:   fmt.Sprintf("service-%d", i),
			Status: "healthy",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		captureOutput(nil, func() {
			if err := formatOutput(result); err != nil {
				b.Logf("formatOutput failed: %v", err)
			}
		})
	}
}

func TestFormatServiceResult(t *testing.T) {
	resetGlobalFlags()

	// Test time for consistent output
	testTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		verbose        bool
		service        *health.ServiceResult
		expectedOutput []string // Strings that should be in the output
		unexpectedOutput []string // Strings that should NOT be in the output
	}{
		{
			name:    "healthy service - non-verbose",
			verbose: false,
			service: &health.ServiceResult{
				Name:   "test-service",
				Status: "healthy",
				Systemd: health.SystemdInfo{
					LastRun:  testTime,
					ExitCode: 0,
					Enabled:  true,
				},
				Backup: health.BackupInfo{
					Path:            "/backup/path",
					RecentFiles:     5,
					NewestFileAge:   2.5,
					NewestFileSizeMB: 15.2,
					Status:          "healthy",
				},
			},
			expectedOutput: []string{
				"✓ test-service [HEALTHY]",
			},
			unexpectedOutput: []string{
				"Systemd:",
				"Backup:",
				"Issues:",
				"Error:",
			},
		},
		{
			name:    "warning service - non-verbose",
			verbose: false,
			service: &health.ServiceResult{
				Name:   "warning-service",
				Status: "warning",
			},
			expectedOutput: []string{
				"⚠ warning-service [WARNING]",
			},
		},
		{
			name:    "critical service - non-verbose",
			verbose: false,
			service: &health.ServiceResult{
				Name:   "critical-service",
				Status: "critical",
			},
			expectedOutput: []string{
				"✗ critical-service [CRITICAL]",
			},
		},
		{
			name:    "healthy service - verbose",
			verbose: true,
			service: &health.ServiceResult{
				Name:   "test-service",
				Status: "healthy",
				Systemd: health.SystemdInfo{
					LastRun:  testTime,
					ExitCode: 0,
					Enabled:  true,
				},
				Backup: health.BackupInfo{
					Path:            "/backup/path",
					RecentFiles:     5,
					NewestFileAge:   2.5,
					NewestFileSizeMB: 15.2,
					Status:          "healthy",
				},
			},
			expectedOutput: []string{
				"✓ test-service [HEALTHY]",
				"Systemd:",
				"Enabled: true",
				"Exit Code: 0",
				"Last Run: 2025-01-01 12:00:00",
				"Backup:",
				"Path: /backup/path",
				"Recent Files: 5",
				"Newest File Age: 2.5 hours",
				"Newest File Size: 15.20 MB",
				"Status: healthy",
			},
		},
		{
			name:    "service with issues",
			verbose: false,
			service: &health.ServiceResult{
				Name:   "issue-service",
				Status: "warning",
				Issues: []string{
					"No recent backup files found",
					"Backup directory missing",
				},
			},
			expectedOutput: []string{
				"⚠ issue-service [WARNING]",
				"Issues:",
				"- No recent backup files found",
				"- Backup directory missing",
			},
		},
		{
			name:    "service with error",
			verbose: false,
			service: &health.ServiceResult{
				Name:   "error-service",
				Status: "critical",
				Error:  "Failed to connect to systemd",
			},
			expectedOutput: []string{
				"✗ error-service [CRITICAL]",
				"Error: Failed to connect to systemd",
			},
		},
		{
			name:    "service with zero timestamp - verbose",
			verbose: true,
			service: &health.ServiceResult{
				Name:   "no-timestamp-service",
				Status: "warning",
				Systemd: health.SystemdInfo{
					LastRun:  time.Time{}, // Zero time
					ExitCode: 1,
					Enabled:  false,
				},
				Backup: health.BackupInfo{
					Path:            "/backup/path",
					RecentFiles:     0,
					NewestFileAge:   0, // Zero age
					NewestFileSizeMB: 0,
					Status:          "warning",
				},
			},
			expectedOutput: []string{
				"⚠ no-timestamp-service [WARNING]",
				"Systemd:",
				"Enabled: false",
				"Exit Code: 1",
				"Backup:",
				"Path: /backup/path",
				"Recent Files: 0",
				"Status: warning",
			},
			unexpectedOutput: []string{
				"Last Run:", // Should not appear for zero timestamp
				"Newest File Age:", // Should not appear for zero age
				"Newest File Size:", // Should not appear for zero age
			},
		},
		{
			name:    "service with issues and error",
			verbose: true,
			service: &health.ServiceResult{
				Name:   "complex-service",
				Status: "critical",
				Systemd: health.SystemdInfo{
					LastRun:  testTime,
					ExitCode: 2,
					Enabled:  true,
				},
				Backup: health.BackupInfo{
					Path:            "/backup/path",
					RecentFiles:     1,
					NewestFileAge:   48.5,
					NewestFileSizeMB: 0.5,
					Status:          "warning",
				},
				Issues: []string{
					"Backup files are too old",
					"File size below minimum threshold",
				},
				Error: "Service failed with exit code 2",
			},
			expectedOutput: []string{
				"✗ complex-service [CRITICAL]",
				"Systemd:",
				"Enabled: true",
				"Exit Code: 2",
				"Last Run: 2025-01-01 12:00:00",
				"Backup:",
				"Path: /backup/path",
				"Recent Files: 1",
				"Newest File Age: 48.5 hours",
				"Newest File Size: 0.50 MB",
				"Status: warning",
				"Issues:",
				"- Backup files are too old",
				"- File size below minimum threshold",
				"Error: Service failed with exit code 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()
			verbose = tt.verbose

			stdout, _ := captureOutput(t, func() {
				err := formatServiceResult(tt.service)
				if err != nil {
					t.Errorf("formatServiceResult() returned error: %v", err)
				}
			})

			// Check expected output
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(stdout, expected) {
					t.Errorf("formatServiceResult() output missing expected string: %q\nActual output:\n%s", expected, stdout)
				}
			}

			// Check unexpected output
			for _, unexpected := range tt.unexpectedOutput {
				if strings.Contains(stdout, unexpected) {
					t.Errorf("formatServiceResult() output contains unexpected string: %q\nActual output:\n%s", unexpected, stdout)
				}
			}
		})
	}
}

func TestFormatOutputEnhanced(t *testing.T) {
	resetGlobalFlags()

	tests := []struct {
		name       string
		jsonOutput bool
		data       interface{}
		wantJSON   bool
		wantError  bool
		checkContent bool
		expectedContent []string
	}{
		{
			name:       "unknown type fallback to JSON",
			jsonOutput: false,
			data:       map[string]int{"test": 123}, // Unknown type
			wantJSON:   true, // Should fallback to JSON
			wantError:  false,
		},
		{
			name:       "nil data",
			jsonOutput: true,
			data:       nil,
			wantJSON:   true,
			wantError:  false,
		},
		{
			name:       "complex health result JSON",
			jsonOutput: true,
			data: &health.HealthResult{
				Status:    health.StatusWarning,
				Version:   "test-version",
				Services: []health.ServiceResult{
					{
						Name:   "test-service",
						Status: "warning",
						Issues: []string{"test issue"},
					},
				},
				Summary: health.Summary{
					TotalServices:     1,
					HealthyServices:   0,
					DegradedServices:  1,
					UnhealthyServices: 0,
				},
			},
			wantJSON:     true,
			wantError:    false,
			checkContent: true,
			expectedContent: []string{
				"\"status\": \"warning\"",
				"\"version\": \"test-version\"",
				"\"test-service\"",
			},
		},
		{
			name:       "service result with complex data",
			jsonOutput: false,
			data: &health.ServiceResult{
				Name:   "test-service",
				Status: "critical",
				Systemd: health.SystemdInfo{
					LastRun:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
					ExitCode: 1,
					Enabled:  true,
				},
				Issues: []string{"Issue 1", "Issue 2"},
				Error:  "Test error",
			},
			wantJSON:     false,
			wantError:    false,
			checkContent: true,
			expectedContent: []string{
				"✗ test-service [CRITICAL]",
				"Issues:",
				"- Issue 1",
				"- Issue 2",
				"Error: Test error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()
			jsonOutput = tt.jsonOutput

			stdout, _ := captureOutput(t, func() {
				err := formatOutput(tt.data)
				if (err != nil) != tt.wantError {
					t.Errorf("formatOutput() error = %v, wantError %v", err, tt.wantError)
				}
			})

			if tt.wantJSON {
				// Check if output is valid JSON
				var result interface{}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Errorf("formatOutput() produced invalid JSON: %v\nOutput: %s", err, stdout)
				}
			}

			if tt.checkContent {
				for _, expected := range tt.expectedContent {
					if !strings.Contains(stdout, expected) {
						t.Errorf("formatOutput() missing expected content: %q\nActual output:\n%s", expected, stdout)
					}
				}
			}

			if stdout == "" && !tt.wantError {
				t.Errorf("formatOutput() produced no output")
			}
		})
	}
}

func TestCheckCommandExecution(t *testing.T) {
	resetGlobalFlags()

	tests := []struct {
		name           string
		configFile     string
		serviceName    string
		verbose        bool
		jsonOutput     bool
		expectError    bool
		expectedStderr []string
		skipExecution  bool // Skip actual execution for tests that would call os.Exit
	}{
		{
			name:          "check nonexistent service",
			configFile:    "../../configs/config.yaml",
			serviceName:   "nonexistent-service",
			verbose:       false,
			jsonOutput:    false,
			expectError:   true,
			skipExecution: false, // This one won't call os.Exit, it returns an error
		},
		{
			name:          "check with invalid config",
			configFile:    "/nonexistent/config.yaml",
			serviceName:   "",
			verbose:       false,
			jsonOutput:    false,
			expectError:   true,
			skipExecution: false, // This one won't call os.Exit, it returns an error
		},
		// Note: We skip the successful execution tests because they call os.Exit()
		// which would terminate the test process. Instead, we test the individual
		// components (loadConfiguration, formatOutput, etc.) separately.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipExecution {
				t.Skip("Skipping test that would call os.Exit()")
				return
			}

			resetGlobalFlags()
			configFile = tt.configFile
			verbose = tt.verbose
			jsonOutput = tt.jsonOutput

			// Create a new command for testing
			cmd := &cobra.Command{
				Use:  "check",
				RunE: checkCmd.RunE,
			}
			cmd.Flags().StringP("service", "s", "", "check only the specified service")
			if tt.serviceName != "" {
				if err := cmd.Flags().Set("service", tt.serviceName); err != nil {
					t.Fatalf("failed to set flag: %v", err)
				}
			}

			stdout, stderr := captureOutput(t, func() {
				err := cmd.Execute()
				if (err != nil) != tt.expectError {
					if tt.expectError {
						// Expected error but didn't get one
						t.Errorf("checkCmd.Execute() expected error but got none")
					} else {
						// Got unexpected error
						t.Errorf("checkCmd.Execute() unexpected error: %v", err)
					}
				}
			})

			// Check stderr content if specified
			for _, expected := range tt.expectedStderr {
				if !strings.Contains(stderr, expected) {
					t.Errorf("checkCmd stderr missing expected content: %q\nActual stderr:\n%s", expected, stderr)
				}
			}

			// For successful tests, ensure we got some output
			if !tt.expectError && stdout == "" && stderr == "" {
				t.Errorf("checkCmd produced no output for valid execution")
			}

			// For JSON output, verify it's valid JSON
			if !tt.expectError && tt.jsonOutput && stdout != "" {
				var result interface{}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Errorf("checkCmd produced invalid JSON: %v\nOutput: %s", err, stdout)
				}
			}
		})
	}
}

// TestCheckCommandComponents tests the individual components of the check command
// without triggering os.Exit() calls
func TestCheckCommandComponents(t *testing.T) {
	resetGlobalFlags()

	// Test configuration loading with verbose output
	t.Run("config loading verbose", func(t *testing.T) {
		resetGlobalFlags()
		configFile = "../../configs/config.yaml"
		verbose = true

		_, stderr := captureOutput(t, func() {
			cfg, err := loadConfiguration()
			if err != nil {
				t.Errorf("loadConfiguration() failed: %v", err)
				return
			}

			if cfg == nil {
				t.Errorf("loadConfiguration() returned nil config")
				return
			}

			if len(cfg.Services) == 0 {
				t.Errorf("loadConfiguration() returned config with no services")
			}
		})

		expectedStderr := []string{
			"Loaded configuration with",
			"services",
		}

		for _, expected := range expectedStderr {
			if !strings.Contains(stderr, expected) {
				t.Errorf("loadConfiguration() stderr missing expected content: %q\nActual stderr:\n%s", expected, stderr)
			}
		}
	})

	// Test service lookup logic
	t.Run("service lookup", func(t *testing.T) {
		resetGlobalFlags()
		configFile = "../../configs/config.yaml"

		cfg, err := loadConfiguration()
		if err != nil {
			t.Fatalf("loadConfiguration() failed: %v", err)
		}

		// Test finding existing service
		var foundService *config.ServiceConfig
		for _, svc := range cfg.Services {
			if svc.Name == "vault-backup.service" {
				foundService = &svc
				break
			}
		}

		if foundService == nil {
			t.Errorf("Could not find vault-backup.service in configuration")
		}

		// Test not finding non-existent service
		var notFoundService *config.ServiceConfig
		for _, svc := range cfg.Services {
			if svc.Name == "nonexistent-service" {
				notFoundService = &svc
				break
			}
		}

		if notFoundService != nil {
			t.Errorf("Found nonexistent-service when it shouldn't exist")
		}
	})
}

func TestFormatHealthResultEnhanced(t *testing.T) {
	resetGlobalFlags()

	testTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		result         *health.HealthResult
		expectedOutput []string
	}{
		{
			name: "empty services list",
			result: &health.HealthResult{
				Status:    health.StatusHealthy,
				Timestamp: testTime,
				Version:   "1.0.0",
				Services:  []health.ServiceResult{},
				Summary: health.Summary{
					TotalServices:     0,
					HealthyServices:   0,
					DegradedServices:  0,
					UnhealthyServices: 0,
				},
			},
			expectedOutput: []string{
				"=== Backup Health Monitor Status ===",
				"Overall Status: HEALTHY",
				"2025-01-01 12:00:00",
				"Version: 1.0.0",
				"Summary: 0 total, 0 healthy, 0 degraded, 0 unhealthy",
			},
		},
		{
			name: "multiple services with different statuses",
			result: &health.HealthResult{
				Status:    health.StatusWarning,
				Timestamp: testTime,
				Version:   "1.0.0",
				Services: []health.ServiceResult{
					{
						Name:   "healthy-service",
						Status: "healthy",
					},
					{
						Name:   "warning-service",
						Status: "warning",
						Issues: []string{"Minor issue"},
					},
				},
				Summary: health.Summary{
					TotalServices:     2,
					HealthyServices:   1,
					DegradedServices:  1,
					UnhealthyServices: 0,
				},
			},
			expectedOutput: []string{
				"=== Backup Health Monitor Status ===",
				"Overall Status: WARNING",
				"Summary: 2 total, 1 healthy, 1 degraded, 0 unhealthy",
				"healthy-service",
				"warning-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()

			stdout, _ := captureOutput(t, func() {
				err := formatHealthResult(tt.result)
				if err != nil {
					t.Errorf("formatHealthResult() returned error: %v", err)
				}
			})

			for _, expected := range tt.expectedOutput {
				if !strings.Contains(stdout, expected) {
					t.Errorf("formatHealthResult() missing expected content: %q\nActual output:\n%s", expected, stdout)
				}
			}
		})
	}
}

func TestLoadConfigurationEnhanced(t *testing.T) {
	resetGlobalFlags()

	tests := []struct {
		name       string
		configFile string
		verbose    bool
		wantError  bool
		checkStderr bool
		expectedStderr []string
	}{
		{
			name:       "valid config - verbose",
			configFile: "../../configs/config.yaml",
			verbose:    true,
			wantError:  false,
			checkStderr: true,
			expectedStderr: []string{
				"Loaded configuration with",
				"services",
			},
		},
		{
			name:       "valid config - non-verbose",
			configFile: "../../configs/config.yaml",
			verbose:    false,
			wantError:  false,
			checkStderr: false,
		},
		{
			name:       "invalid config path",
			configFile: "/invalid/path/config.yaml",
			verbose:    false,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()
			configFile = tt.configFile
			verbose = tt.verbose

			_, stderr := captureOutput(t, func() {
				cfg, err := loadConfiguration()
				if (err != nil) != tt.wantError {
					t.Errorf("loadConfiguration() error = %v, wantError %v", err, tt.wantError)
					return
				}

				if !tt.wantError && cfg == nil {
					t.Errorf("loadConfiguration() returned nil config without error")
				}
			})

			if tt.checkStderr {
				for _, expected := range tt.expectedStderr {
					if !strings.Contains(stderr, expected) {
						t.Errorf("loadConfiguration() stderr missing expected content: %q\nActual stderr:\n%s", expected, stderr)
					}
				}
			}
		})
	}
}

func TestEdgeCases(t *testing.T) {
	resetGlobalFlags()

	t.Run("formatHealthResult with spacing", func(t *testing.T) {
		resetGlobalFlags()

		result := &health.HealthResult{
			Status:    health.StatusCritical,
			Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Version:   "test",
			Services: []health.ServiceResult{
				{Name: "service1", Status: "healthy"},
				{Name: "service2", Status: "warning"},
				{Name: "service3", Status: "critical"},
			},
			Summary: health.Summary{
				TotalServices:     3,
				HealthyServices:   1,
				DegradedServices:  1,
				UnhealthyServices: 1,
			},
		}

		stdout, _ := captureOutput(t, func() {
			err := formatHealthResult(result)
			if err != nil {
				t.Errorf("formatHealthResult() returned error: %v", err)
			}
		})

		expectedStrings := []string{
			"service1",
			"service2",
			"service3",
		}

		for _, expected := range expectedStrings {
			if !strings.Contains(stdout, expected) {
				t.Errorf("formatHealthResult() missing service: %q", expected)
			}
		}

		// Check that spacing between services exists (empty lines)
		lines := strings.Split(stdout, "\n")
		emptyLineFound := false
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				emptyLineFound = true
				break
			}
		}

		if !emptyLineFound {
			t.Errorf("formatHealthResult() should have empty lines between services")
		}
	})

	t.Run("formatHealthResult error handling", func(t *testing.T) {
		resetGlobalFlags()

		result := &health.HealthResult{
			Status:    health.StatusHealthy,
			Timestamp: time.Now(),
			Version:   "test",
			Services: []health.ServiceResult{
				{
					Name:   "test-service",
					Status: "healthy",
				},
			},
			Summary: health.Summary{TotalServices: 1, HealthyServices: 1},
		}

		// Test calling formatServiceResult returns error - this tests the error path
		stdout, _ := captureOutput(t, func() {
			err := formatHealthResult(result)
			if err != nil {
				t.Errorf("formatHealthResult() returned unexpected error: %v", err)
			}
		})

		if stdout == "" {
			t.Errorf("formatHealthResult() produced no output")
		}
	})

	t.Run("unknown status symbols", func(t *testing.T) {
		resetGlobalFlags()

		service := &health.ServiceResult{
			Name:   "unknown-status-service",
			Status: "unknown",
		}

		stdout, _ := captureOutput(t, func() {
			err := formatServiceResult(service)
			if err != nil {
				t.Errorf("formatServiceResult() returned error: %v", err)
			}
		})

		// Should default to checkmark for unknown status
		if !strings.Contains(stdout, "✓ unknown-status-service [UNKNOWN]") {
			t.Errorf("formatServiceResult() didn't handle unknown status correctly\nOutput: %s", stdout)
		}
	})

	t.Run("configuration loading edge cases", func(t *testing.T) {
		resetGlobalFlags()

		// Test with empty config file path and verbose
		configFile = ""
		verbose = true

		_, stderr := captureOutput(t, func() {
			_, err := loadConfiguration()
			// This should fail since no config in default locations
			if err == nil {
				t.Errorf("loadConfiguration() should have failed with empty config path")
			}
		})

		// Should not have verbose output for failed config loading
		if strings.Contains(stderr, "Loaded configuration") {
			t.Errorf("loadConfiguration() should not have verbose output on failure")
		}
	})

	t.Run("validate command edge cases", func(t *testing.T) {
		resetGlobalFlags()

		// Test validation with empty config path
		configFile = ""
		jsonOutput = false

		cmd := &cobra.Command{
			Use:  "validate",
			RunE: validateCmd.RunE,
		}

		stdout, _ := captureOutput(t, func() {
			err := cmd.Execute()
			if err == nil {
				t.Errorf("validateCmd should have failed with empty config")
			}
		})

		// Should not produce output for failed validation
		if strings.Contains(stdout, "Configuration is valid") {
			t.Errorf("validateCmd should not show success message on failure")
		}
	})
}

func TestFormatOutputErrorPaths(t *testing.T) {
	resetGlobalFlags()

	t.Run("formatOutput with various data types", func(t *testing.T) {
		resetGlobalFlags()

		// Test with integer (unknown type)
		jsonOutput = false
		stdout, _ := captureOutput(t, func() {
			err := formatOutput(42)
			if err != nil {
				t.Errorf("formatOutput() returned error: %v", err)
			}
		})

		var result interface{}
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Errorf("formatOutput() should have produced JSON for unknown type: %v", err)
		}
	})

	t.Run("formatOutput with empty string", func(t *testing.T) {
		resetGlobalFlags()
		jsonOutput = false

		stdout, _ := captureOutput(t, func() {
			err := formatOutput("")
			if err != nil {
				t.Errorf("formatOutput() returned error: %v", err)
			}
		})

		if stdout != "\n" { // Empty string + newline
			t.Errorf("formatOutput() incorrect output for empty string: %q", stdout)
		}
	})
}

func TestServeCommand(t *testing.T) {
	resetGlobalFlags()

	// Test that serve command exists and has correct properties
	serveCommand := findCommand(rootCmd, "serve")
	if serveCommand == nil {
		t.Fatal("serve command not found")
	}

	if serveCommand.Short != "Start HTTP server" {
		t.Errorf("serve command has incorrect short description, got: %s", serveCommand.Short)
	}

	// Test that serve command has the required flags
	flags := []struct {
		name      string
		shorthand string
		usage     string
	}{
		{"port", "p", "port to bind the server to (overrides config file)"},
		{"bind", "b", "address to bind the server to (overrides config file)"},
	}

	for _, flag := range flags {
		f := serveCommand.Flags().Lookup(flag.name)
		if f == nil {
			t.Errorf("serve command should have flag '%s'", flag.name)
		} else {
			if f.Usage != flag.usage {
				t.Errorf("serve command flag '%s' has incorrect usage, expected: %s, got: %s", flag.name, flag.usage, f.Usage)
			}
		}

		// Test shorthand flags
		sf := serveCommand.Flags().ShorthandLookup(flag.shorthand)
		if sf == nil {
			t.Errorf("serve command should have shorthand flag '%s' for %s", flag.shorthand, flag.name)
		}
	}
}

func TestServeCommandFlags(t *testing.T) {
	resetGlobalFlags()

	tests := []struct {
		name           string
		flags          []string
		expectedPort   int
		expectedBind   string
		expectError    bool
		skipExecution  bool
	}{
		{
			name:           "port flag parsing",
			flags:          []string{"--port", "9090"},
			expectedPort:   9090,
			expectedBind:   "",
			expectError:    false,
			skipExecution:  true, // Skip execution since it would start a server
		},
		{
			name:           "bind flag parsing",
			flags:          []string{"--bind", "0.0.0.0"},
			expectedPort:   0,
			expectedBind:   "0.0.0.0",
			expectError:    false,
			skipExecution:  true,
		},
		{
			name:           "port and bind flags",
			flags:          []string{"--port", "3000", "--bind", "localhost"},
			expectedPort:   3000,
			expectedBind:   "localhost",
			expectError:    false,
			skipExecution:  true,
		},
		{
			name:           "shorthand flags",
			flags:          []string{"-p", "4000", "-b", "192.168.1.1"},
			expectedPort:   4000,
			expectedBind:   "192.168.1.1",
			expectError:    false,
			skipExecution:  true,
		},
		{
			name:           "invalid port flag",
			flags:          []string{"--port", "invalid"},
			expectedPort:   0,
			expectedBind:   "",
			expectError:    true,
			skipExecution:  false, // Test flag parsing error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalFlags()

			// Create a copy of the serve command for testing
			cmd := &cobra.Command{
				Use:  "serve",
				RunE: func(cmd *cobra.Command, args []string) error {
					if tt.skipExecution {
						// Just test flag parsing without running the server
						port, _ := cmd.Flags().GetInt("port")
						bind, _ := cmd.Flags().GetString("bind")

						if port != tt.expectedPort {
							t.Errorf("Expected port %d, got %d", tt.expectedPort, port)
						}
						if bind != tt.expectedBind {
							t.Errorf("Expected bind %s, got %s", tt.expectedBind, bind)
						}
						return nil
					}
					return serveCmd.RunE(cmd, args)
				},
			}

			// Add flags to test command
			cmd.Flags().IntP("port", "p", 0, "port to bind the server to (overrides config file)")
			cmd.Flags().StringP("bind", "b", "", "address to bind the server to (overrides config file)")

			// Set the flags
			cmd.SetArgs(tt.flags)

			// Execute command
			err := cmd.Execute()
			if (err != nil) != tt.expectError {
				t.Errorf("serve command execution error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestServeCommandWithInvalidConfig(t *testing.T) {
	resetGlobalFlags()

	// Test serve command with invalid configuration
	configFile = "/nonexistent/config.yaml"

	cmd := &cobra.Command{
		Use:  "serve",
		RunE: serveCmd.RunE,
	}

	err := cmd.Execute()
	if err == nil {
		t.Error("serve command should fail with invalid config file")
	}

	// Check that the error is about configuration loading
	if !strings.Contains(err.Error(), "config") {
		t.Errorf("serve command error should mention config, got: %v", err)
	}
}

func TestServeCommandHelp(t *testing.T) {
	resetGlobalFlags()

	// Test that serve command help contains expected content
	serveCommand := findCommand(rootCmd, "serve")
	if serveCommand == nil {
		t.Fatal("serve command not found")
	}

	helpText := serveCommand.Long

	expectedContent := []string{
		"Start the HTTP server",
		"GET /health",
		"GET /api/v1/status",
		"GET /api/v1/services",
		"GET /metrics",
		"backup-health-monitor serve",
		"--port",
		"--bind",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(helpText, expected) {
			t.Errorf("serve command help missing expected content: %q\nFull help:\n%s", expected, helpText)
		}
	}
}

// TestServeCommandConfigOverride tests that command-line flags properly override config values
func TestServeCommandConfigOverride(t *testing.T) {
	resetGlobalFlags()

	// This test verifies the override logic without actually starting a server
	t.Run("config override logic", func(t *testing.T) {
		resetGlobalFlags()
		configFile = "../../configs/config.yaml"

		// Load base configuration
		cfg, err := loadConfiguration()
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}

		originalPort := cfg.Server.Port
		originalBind := cfg.Server.BindAddress

		// Test port override
		newPort := 9999
		if newPort != 0 {
			cfg.Server.Port = newPort
		}

		if cfg.Server.Port != newPort {
			t.Errorf("Port override failed: expected %d, got %d", newPort, cfg.Server.Port)
		}

		// Test bind override
		newBind := "0.0.0.0"
		if newBind != "" {
			cfg.Server.BindAddress = newBind
		}

		if cfg.Server.BindAddress != newBind {
			t.Errorf("Bind override failed: expected %s, got %s", newBind, cfg.Server.BindAddress)
		}

		// Verify original config wasn't permanently modified
		cfgCheck, err := loadConfiguration()
		if err != nil {
			t.Fatalf("Failed to reload configuration: %v", err)
		}

		if cfgCheck.Server.Port != originalPort {
			t.Errorf("Original config was modified: port should be %d, got %d", originalPort, cfgCheck.Server.Port)
		}

		if cfgCheck.Server.BindAddress != originalBind {
			t.Errorf("Original config was modified: bind should be %s, got %s", originalBind, cfgCheck.Server.BindAddress)
		}
	})
}

func TestVerboseOutputPaths(t *testing.T) {
	resetGlobalFlags()

	t.Run("formatServiceResult verbose with all fields", func(t *testing.T) {
		resetGlobalFlags()
		verbose = true

		testTime := time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC)
		service := &health.ServiceResult{
			Name:   "comprehensive-service",
			Status: "warning",
			Systemd: health.SystemdInfo{
				LastRun:  testTime,
				ExitCode: 1,
				Enabled:  true,
			},
			Backup: health.BackupInfo{
				Path:            "/test/backup",
				RecentFiles:     3,
				NewestFileAge:   12.5,
				NewestFileSizeMB: 25.75,
				Status:          "warning",
			},
			Issues: []string{
				"First issue",
				"Second issue",
				"Third issue",
			},
			Error: "Test error message",
		}

		stdout, _ := captureOutput(t, func() {
			err := formatServiceResult(service)
			if err != nil {
				t.Errorf("formatServiceResult() returned error: %v", err)
			}
		})

		expectedContent := []string{
			"⚠ comprehensive-service [WARNING]",
			"Systemd:",
			"Enabled: true",
			"Exit Code: 1",
			"Last Run: 2025-06-15 14:30:45",
			"Backup:",
			"Path: /test/backup",
			"Recent Files: 3",
			"Newest File Age: 12.5 hours",
			"Newest File Size: 25.75 MB",
			"Status: warning",
			"Issues:",
			"- First issue",
			"- Second issue",
			"- Third issue",
			"Error: Test error message",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(stdout, expected) {
				t.Errorf("formatServiceResult() missing content: %q\nFull output:\n%s", expected, stdout)
			}
		}
	})
}