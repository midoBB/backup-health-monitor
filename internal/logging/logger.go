package logging

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"backup-health-monitor/internal/config"
)

var (
	// Logger is the global logger instance
	Logger *logrus.Logger
)

// InitializeLogger sets up the global logger based on configuration
func InitializeLogger(cfg *config.LoggingConfig) error {
	Logger = logrus.New()

	// Set log level
	level, err := parseLogLevel(cfg.Level)
	if err != nil {
		return err
	}
	Logger.SetLevel(level)

	// Set log format
	if err := setLogFormat(Logger, cfg.Format); err != nil {
		return err
	}

	// Configure output streams based on log level
	// Info and Debug go to stdout, Warn and Error go to stderr
	Logger.SetOutput(os.Stdout)

	// Add hooks for stderr output for warnings and errors
	Logger.AddHook(&StderrHook{})

	return nil
}

// parseLogLevel converts string log level to logrus level
func parseLogLevel(level string) (logrus.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn", "warning":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	default:
		return logrus.InfoLevel, nil
	}
}

// setLogFormat configures the log formatter
func setLogFormat(logger *logrus.Logger, format string) error {
	switch strings.ToLower(format) {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
			ForceColors:     false,
		})
	default:
		// Default to JSON format
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	}
	return nil
}

// StderrHook implements logrus.Hook to send warn/error logs to stderr
type StderrHook struct{}

// Levels returns the log levels that this hook should be called for
func (hook *StderrHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

// Fire is called when a log entry is fired
func (hook *StderrHook) Fire(entry *logrus.Entry) error {
	// Create a copy of the logger with stderr output
	stderrLogger := logrus.New()
	stderrLogger.SetOutput(os.Stderr)
	stderrLogger.SetFormatter(entry.Logger.Formatter)
	stderrLogger.SetLevel(entry.Logger.Level)

	// Write the entry to stderr
	switch entry.Level {
	case logrus.WarnLevel:
		stderrLogger.WithFields(entry.Data).Warn(entry.Message)
	case logrus.ErrorLevel:
		stderrLogger.WithFields(entry.Data).Error(entry.Message)
	case logrus.FatalLevel:
		stderrLogger.WithFields(entry.Data).Fatal(entry.Message)
	case logrus.PanicLevel:
		stderrLogger.WithFields(entry.Data).Panic(entry.Message)
	}

	return nil
}

// WithServiceName creates a logger with service name field
func WithServiceName(serviceName string) *logrus.Entry {
	return Logger.WithField("service", serviceName)
}

// WithOperation creates a logger with operation field
func WithOperation(operation string) *logrus.Entry {
	return Logger.WithField("operation", operation)
}

// WithDuration creates a logger with duration field
func WithDuration(operation string, duration float64) *logrus.Entry {
	return Logger.WithFields(logrus.Fields{
		"operation": operation,
		"duration_ms": duration,
	})
}

// WithPath creates a logger with path field
func WithPath(path string) *logrus.Entry {
	return Logger.WithField("path", path)
}

// WithError creates a logger with error field
func WithError(err error) *logrus.Entry {
	return Logger.WithError(err)
}

// WithFields creates a logger with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Logger.WithFields(fields)
}

// GetLogWriter returns a writer for the logger (useful for Gin middleware)
func GetLogWriter() io.Writer {
	if Logger == nil {
		return os.Stdout
	}
	return Logger.Writer()
}

// Close properly closes the logger resources
func Close() error {
	// Logger resources are automatically managed by the system
	// No explicit cleanup needed for logrus
	return nil
}