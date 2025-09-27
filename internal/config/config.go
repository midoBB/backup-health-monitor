package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config represents the main configuration structure
type Config struct {
	Services   []ServiceConfig  `yaml:"services" mapstructure:"services"`
	Server     ServerConfig     `yaml:"server" mapstructure:"server"`
	Logging    LoggingConfig    `yaml:"logging" mapstructure:"logging"`
	Monitoring MonitoringConfig `yaml:"monitoring" mapstructure:"monitoring"`
}

// ServiceConfig represents configuration for a single backup service
type ServiceConfig struct {
	Name                 string   `yaml:"name" mapstructure:"name"`
	BackupPath           string   `yaml:"backup_path" mapstructure:"backup_path"`
	MaxAgeHours          int      `yaml:"max_age_hours" mapstructure:"max_age_hours"`
	ExpectedFilePatterns []string `yaml:"expected_file_patterns" mapstructure:"expected_file_patterns"`
	MinFileSizeMB        int      `yaml:"min_file_size_mb" mapstructure:"min_file_size_mb"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Port        int    `yaml:"port" mapstructure:"port"`
	BindAddress string `yaml:"bind_address" mapstructure:"bind_address"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" mapstructure:"level"`   // debug, info, warn, error
	Format string `yaml:"format" mapstructure:"format"` // json, text
}

// MonitoringConfig represents monitoring configuration
type MonitoringConfig struct {
	CheckIntervalSeconds int  `yaml:"check_interval_seconds" mapstructure:"check_interval_seconds"`
	MetricsEnabled       bool `yaml:"metrics_enabled" mapstructure:"metrics_enabled"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Configure environment variable support
	v.SetEnvPrefix("BACKUP_MONITOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Determine config file path
	var actualConfigPath string
	if configPath != "" {
		// Use provided config path
		v.SetConfigFile(configPath)
		actualConfigPath = configPath
	} else {
		// Look for config in default locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/backup-health/")
		v.AddConfigPath("./configs/")
		v.AddConfigPath(".")
	}

	// Log configuration file loading attempt
	if actualConfigPath != "" {
		logrus.WithField("config_path", actualConfigPath).Debug("Attempting to load configuration file")
	} else {
		logrus.Debug("Searching for configuration file in default locations")
	}

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if configPath != "" {
				logrus.WithField("config_path", configPath).Error("Configuration file not found")
				return nil, fmt.Errorf("config file not found: %s", configPath)
			}
			logrus.Error("No configuration file found in default locations")
			return nil, fmt.Errorf("no config file found in default locations")
		}
		// Check if it's a file not found error (when specific path is provided)
		if configPath != "" && strings.Contains(err.Error(), "no such file or directory") {
			logrus.WithFields(logrus.Fields{
				"config_path": configPath,
				"error": err.Error(),
			}).Error("Configuration file not found")
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
		logrus.WithFields(logrus.Fields{
			"config_path": v.ConfigFileUsed(),
			"error": err.Error(),
		}).Error("Error reading configuration file")
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	actualConfigPath = v.ConfigFileUsed()
	logrus.WithField("config_path", actualConfigPath).Debug("Configuration file loaded successfully")

	// Unmarshal into Config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		logrus.WithFields(logrus.Fields{
			"config_path": actualConfigPath,
			"error": err.Error(),
		}).Error("Error unmarshaling configuration")
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Apply defaults for zero values
	applyDefaults(&config)

	logrus.WithFields(logrus.Fields{
		"config_path": actualConfigPath,
		"services_count": len(config.Services),
		"log_level": config.Logging.Level,
		"log_format": config.Logging.Format,
	}).Debug("Configuration processing completed")

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.bind_address", "127.0.0.1")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	// Monitoring defaults
	v.SetDefault("monitoring.check_interval_seconds", 30)
	v.SetDefault("monitoring.metrics_enabled", true)
}

// applyDefaults applies default values for zero values after unmarshaling
func applyDefaults(config *Config) {
	// Server defaults
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.BindAddress == "" {
		config.Server.BindAddress = "127.0.0.1"
	}

	// Logging defaults
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}

	// Monitoring defaults
	if config.Monitoring.CheckIntervalSeconds == 0 {
		config.Monitoring.CheckIntervalSeconds = 30
	}
	// Note: MetricsEnabled defaults to false in Go's zero value,
	// but we want it to be true by default. Since we can't distinguish
	// between explicitly set false and zero value, we'll use Viper's
	// default handling for this field.
}

// ValidateConfig validates the loaded configuration
func ValidateConfig(config *Config) error {
	logrus.Debug("Starting configuration validation")

	if config == nil {
		logrus.Error("Configuration validation failed: config is nil")
		return fmt.Errorf("config cannot be nil")
	}

	// Validate services
	if len(config.Services) == 0 {
		logrus.Error("Configuration validation failed: no services configured")
		return fmt.Errorf("at least one service must be configured")
	}

	serviceNames := make(map[string]bool)
	for i, service := range config.Services {
		// Check for required fields
		if strings.TrimSpace(service.Name) == "" {
			return fmt.Errorf("service[%d]: name cannot be empty", i)
		}

		// Check for duplicate service names
		if serviceNames[service.Name] {
			return fmt.Errorf("service[%d]: duplicate service name '%s'", i, service.Name)
		}
		serviceNames[service.Name] = true

		// Validate backup path
		if strings.TrimSpace(service.BackupPath) == "" {
			return fmt.Errorf("service[%d] (%s): backup_path cannot be empty", i, service.Name)
		}

		// Validate max age hours
		if service.MaxAgeHours <= 0 {
			return fmt.Errorf("service[%d] (%s): max_age_hours must be positive, got %d", i, service.Name, service.MaxAgeHours)
		}

		// Validate file patterns
		if len(service.ExpectedFilePatterns) == 0 {
			return fmt.Errorf("service[%d] (%s): at least one expected_file_pattern must be specified", i, service.Name)
		}

		for j, pattern := range service.ExpectedFilePatterns {
			if strings.TrimSpace(pattern) == "" {
				return fmt.Errorf("service[%d] (%s): expected_file_patterns[%d] cannot be empty", i, service.Name, j)
			}
		}

		// Validate minimum file size
		if service.MinFileSizeMB < 0 {
			return fmt.Errorf("service[%d] (%s): min_file_size_mb cannot be negative, got %d", i, service.Name, service.MinFileSizeMB)
		}
	}

	// Validate server configuration
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", config.Server.Port)
	}

	if strings.TrimSpace(config.Server.BindAddress) == "" {
		return fmt.Errorf("server.bind_address cannot be empty")
	}

	// Validate logging configuration
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLogLevels[config.Logging.Level] {
		return fmt.Errorf("logging.level must be one of [debug, info, warn, error], got '%s'", config.Logging.Level)
	}

	validLogFormats := map[string]bool{
		"json": true,
		"text": true,
	}

	if !validLogFormats[config.Logging.Format] {
		return fmt.Errorf("logging.format must be one of [json, text], got '%s'", config.Logging.Format)
	}

	// Validate monitoring configuration
	if config.Monitoring.CheckIntervalSeconds <= 0 {
		logrus.WithField("check_interval", config.Monitoring.CheckIntervalSeconds).Error("Configuration validation failed: invalid monitoring check interval")
		return fmt.Errorf("monitoring.check_interval_seconds must be positive, got %d", config.Monitoring.CheckIntervalSeconds)
	}

	logrus.WithFields(logrus.Fields{
		"services_count": len(config.Services),
		"server_port": config.Server.Port,
		"log_level": config.Logging.Level,
		"log_format": config.Logging.Format,
	}).Debug("Configuration validation completed successfully")

	return nil
}
