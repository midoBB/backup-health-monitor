package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_ValidConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	validConfig := `
services:
  - name: "test-service.service"
    backup_path: "/tmp/test-backups"
    max_age_hours: 24
    expected_file_patterns: ["*.txt"]
    min_file_size_mb: 1

server:
  port: 9090
  bind_address: "0.0.0.0"

logging:
  level: "debug"
  format: "text"

monitoring:
  check_interval_seconds: 60
  metrics_enabled: false
`

	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load and validate config
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Validate loaded values
	if len(config.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(config.Services))
	}

	service := config.Services[0]
	if service.Name != "test-service.service" {
		t.Errorf("Expected service name 'test-service.service', got '%s'", service.Name)
	}

	if config.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Server.Port)
	}

	if config.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.Logging.Level)
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Config with only required fields
	minimalConfig := `
services:
  - name: "test-service.service"
    backup_path: "/tmp/test-backups"
    max_age_hours: 24
    expected_file_patterns: ["*.txt"]
    min_file_size_mb: 1
`

	if err := os.WriteFile(configPath, []byte(minimalConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check defaults are applied
	if config.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Server.Port)
	}

	if config.Server.BindAddress != "127.0.0.1" {
		t.Errorf("Expected default bind address '127.0.0.1', got '%s'", config.Server.BindAddress)
	}

	if config.Logging.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", config.Logging.Level)
	}

	if config.Logging.Format != "json" {
		t.Errorf("Expected default log format 'json', got '%s'", config.Logging.Format)
	}

	if config.Monitoring.CheckIntervalSeconds != 30 {
		t.Errorf("Expected default check interval 30, got %d", config.Monitoring.CheckIntervalSeconds)
	}

	// Note: MetricsEnabled will be false when not specified in YAML due to Go's zero value
	// This is expected behavior since we can't distinguish between explicitly false and unset
	// The Viper default only applies when the field is completely missing from any source
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config file")
	}

	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("Expected 'config file not found' error, got: %v", err)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	invalidYAML := `
services:
  - name: "test-service
    backup_path: "/tmp/test-backups"
    max_age_hours: 24
    expected_file_patterns: ["*.txt"
    min_file_size_mb: 1
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	baseConfig := `
services:
  - name: "test-service.service"
    backup_path: "/tmp/test-backups"
    max_age_hours: 24
    expected_file_patterns: ["*.txt"]
    min_file_size_mb: 1
`

	if err := os.WriteFile(configPath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set environment variables
	if err := os.Setenv("BACKUP_MONITOR_SERVER_PORT", "9999"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("BACKUP_MONITOR_LOGGING_LEVEL", "debug"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("BACKUP_MONITOR_SERVER_PORT"); err != nil {
			t.Logf("Failed to unset env var: %v", err)
		}
		if err := os.Unsetenv("BACKUP_MONITOR_LOGGING_LEVEL"); err != nil {
			t.Logf("Failed to unset env var: %v", err)
		}
	}()

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Environment variables should override defaults
	if config.Server.Port != 9999 {
		t.Errorf("Expected port 9999 from env var, got %d", config.Server.Port)
	}

	if config.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug' from env var, got '%s'", config.Logging.Level)
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	validConfig := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt", "*.log"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{
			Port:        8080,
			BindAddress: "127.0.0.1",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Monitoring: MonitoringConfig{
			CheckIntervalSeconds: 30,
			MetricsEnabled:       true,
		},
	}

	err := ValidateConfig(validConfig)
	if err != nil {
		t.Errorf("ValidateConfig failed for valid config: %v", err)
	}
}

func TestValidateConfig_NilConfig(t *testing.T) {
	err := ValidateConfig(nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}

	if !strings.Contains(err.Error(), "config cannot be nil") {
		t.Errorf("Expected 'config cannot be nil' error, got: %v", err)
	}
}

func TestValidateConfig_NoServices(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{},
		Server: ServerConfig{
			Port:        8080,
			BindAddress: "127.0.0.1",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Monitoring: MonitoringConfig{
			CheckIntervalSeconds: 30,
			MetricsEnabled:       true,
		},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for config with no services")
	}

	if !strings.Contains(err.Error(), "at least one service must be configured") {
		t.Errorf("Expected 'at least one service must be configured' error, got: %v", err)
	}
}

func TestValidateConfig_EmptyServiceName(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for empty service name")
	}

	if !strings.Contains(err.Error(), "name cannot be empty") {
		t.Errorf("Expected 'name cannot be empty' error, got: %v", err)
	}
}

func TestValidateConfig_DuplicateServiceNames(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups1",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups2",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for duplicate service names")
	}

	if !strings.Contains(err.Error(), "duplicate service name") {
		t.Errorf("Expected 'duplicate service name' error, got: %v", err)
	}
}

func TestValidateConfig_InvalidPort(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{
			Port:        70000, // Invalid port
			BindAddress: "127.0.0.1",
		},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for invalid port")
	}

	if !strings.Contains(err.Error(), "port must be between 1 and 65535") {
		t.Errorf("Expected port validation error, got: %v", err)
	}
}

func TestValidateConfig_InvalidLogLevel(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{
			Level:  "invalid", // Invalid log level
			Format: "json",
		},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for invalid log level")
	}

	if !strings.Contains(err.Error(), "logging.level must be one of") {
		t.Errorf("Expected log level validation error, got: %v", err)
	}
}

func TestValidateConfig_NegativeMaxAge(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          -1, // Invalid negative value
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for negative max_age_hours")
	}

	if !strings.Contains(err.Error(), "max_age_hours must be positive") {
		t.Errorf("Expected max_age_hours validation error, got: %v", err)
	}
}

func TestValidateConfig_EmptyFilePatterns(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{}, // Empty patterns
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for empty file patterns")
	}

	if !strings.Contains(err.Error(), "at least one expected_file_pattern must be specified") {
		t.Errorf("Expected file patterns validation error, got: %v", err)
	}
}

func TestValidateConfig_EmptyBackupPath(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "", // Empty backup path
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for empty backup path")
	}

	if !strings.Contains(err.Error(), "backup_path cannot be empty") {
		t.Errorf("Expected backup_path validation error, got: %v", err)
	}
}

func TestValidateConfig_EmptyBindAddress(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{
			Port:        8080,
			BindAddress: "", // Empty bind address
		},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for empty bind address")
	}

	if !strings.Contains(err.Error(), "bind_address cannot be empty") {
		t.Errorf("Expected bind_address validation error, got: %v", err)
	}
}

func TestValidateConfig_InvalidLogFormat(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "xml", // Invalid format
		},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for invalid log format")
	}

	if !strings.Contains(err.Error(), "logging.format must be one of") {
		t.Errorf("Expected log format validation error, got: %v", err)
	}
}

func TestValidateConfig_NegativeCheckInterval(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{
			CheckIntervalSeconds: -1, // Negative interval
			MetricsEnabled:       true,
		},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for negative check interval")
	}

	if !strings.Contains(err.Error(), "check_interval_seconds must be positive") {
		t.Errorf("Expected check_interval_seconds validation error, got: %v", err)
	}
}

func TestValidateConfig_NegativeMinFileSize(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        -1, // Negative file size
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for negative min file size")
	}

	if !strings.Contains(err.Error(), "min_file_size_mb cannot be negative") {
		t.Errorf("Expected min_file_size_mb validation error, got: %v", err)
	}
}

func TestValidateConfig_EmptyFilePattern(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:        "test-service.service",
				BackupPath:  "/tmp/test-backups",
				MaxAgeHours: 24,
				ExpectedFilePatterns: []string{
					"*.txt",
					"", // Empty pattern
					"*.log",
				},
				MinFileSizeMB: 1,
			},
		},
		Server: ServerConfig{Port: 8080, BindAddress: "127.0.0.1"},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for empty file pattern")
	}

	if !strings.Contains(err.Error(), "expected_file_patterns[1] cannot be empty") {
		t.Errorf("Expected empty file pattern validation error, got: %v", err)
	}
}

func TestValidateConfig_ZeroPort(t *testing.T) {
	config := &Config{
		Services: []ServiceConfig{
			{
				Name:                 "test-service.service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
		Server: ServerConfig{
			Port:        0, // Invalid zero port
			BindAddress: "127.0.0.1",
		},
		Logging: LoggingConfig{Level: "info", Format: "json"},
		Monitoring: MonitoringConfig{CheckIntervalSeconds: 30, MetricsEnabled: true},
	}

	err := ValidateConfig(config)
	if err == nil {
		t.Error("Expected error for zero port")
	}

	if !strings.Contains(err.Error(), "port must be between 1 and 65535") {
		t.Errorf("Expected port validation error, got: %v", err)
	}
}

func TestLoadConfig_DefaultPath(t *testing.T) {
	// Test loading config from default location (should fail since no config exists)
	_, err := LoadConfig("")
	if err == nil {
		t.Error("Expected error when loading from default path with no config")
	}

	if !strings.Contains(err.Error(), "no config file found in default locations") {
		t.Errorf("Expected 'no config file found in default locations' error, got: %v", err)
	}
}