package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"backup-health-monitor/internal/config"
)

func TestInitializeLogger(t *testing.T) {
	tests := []struct {
		name          string
		config        config.LoggingConfig
		expectError   bool
		expectedLevel logrus.Level
	}{
		{
			name: "JSON format with info level",
			config: config.LoggingConfig{
				Level:  "info",
				Format: "json",
			},
			expectError:   false,
			expectedLevel: logrus.InfoLevel,
		},
		{
			name: "Text format with debug level",
			config: config.LoggingConfig{
				Level:  "debug",
				Format: "text",
			},
			expectError:   false,
			expectedLevel: logrus.DebugLevel,
		},
		{
			name: "Error level",
			config: config.LoggingConfig{
				Level:  "error",
				Format: "json",
			},
			expectError:   false,
			expectedLevel: logrus.ErrorLevel,
		},
		{
			name: "Warning level",
			config: config.LoggingConfig{
				Level:  "warn",
				Format: "text",
			},
			expectError:   false,
			expectedLevel: logrus.WarnLevel,
		},
		{
			name: "Invalid level defaults to info",
			config: config.LoggingConfig{
				Level:  "invalid",
				Format: "json",
			},
			expectError:   false,
			expectedLevel: logrus.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InitializeLogger(&tt.config)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if Logger == nil {
				t.Fatal("Logger should not be nil after initialization")
			}

			if Logger.Level != tt.expectedLevel {
				t.Errorf("Expected log level %v, got %v", tt.expectedLevel, Logger.Level)
			}

			// Test formatter type
			switch tt.config.Format {
			case "json":
				if _, ok := Logger.Formatter.(*logrus.JSONFormatter); !ok {
					t.Errorf("Expected JSONFormatter, got %T", Logger.Formatter)
				}
			case "text":
				if _, ok := Logger.Formatter.(*logrus.TextFormatter); !ok {
					t.Errorf("Expected TextFormatter, got %T", Logger.Formatter)
				}
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected logrus.Level
	}{
		{"debug", logrus.DebugLevel},
		{"info", logrus.InfoLevel},
		{"warn", logrus.WarnLevel},
		{"warning", logrus.WarnLevel},
		{"error", logrus.ErrorLevel},
		{"invalid", logrus.InfoLevel}, // defaults to info
		{"", logrus.InfoLevel},        // defaults to info
		{"DEBUG", logrus.DebugLevel},  // case insensitive
		{"INFO", logrus.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseLogLevel(tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLogOutput(t *testing.T) {
	// Capture stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	// Create buffers to capture output (not used directly in this test)

	// Create pipes
	rStdout, wStdout, _ := os.Pipe()
	rStderr, wStderr, _ := os.Pipe()
	os.Stdout = wStdout
	os.Stderr = wStderr

	// Initialize logger with JSON format
	config := &config.LoggingConfig{
		Level:  "debug",
		Format: "json",
	}
	err := InitializeLogger(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Test info log (should go to stdout)
	Logger.Info("test info message")

	// Test error log (should go to stderr via hook)
	Logger.Error("test error message")

	// Close writers and read output
	if err := wStdout.Close(); err != nil {
		t.Logf("failed to close wStdout: %v", err)
	}
	if err := wStderr.Close(); err != nil {
		t.Logf("failed to close wStderr: %v", err)
	}

	// Read stdout
	stdoutBytes := make([]byte, 1024)
	n, _ := rStdout.Read(stdoutBytes)
	stdoutOutput := string(stdoutBytes[:n])

	// Read stderr
	stderrBytes := make([]byte, 1024)
	n, _ = rStderr.Read(stderrBytes)
	stderrOutput := string(stderrBytes[:n])

	// Verify info message went to stdout
	if !strings.Contains(stdoutOutput, "test info message") {
		t.Errorf("Info message not found in stdout: %s", stdoutOutput)
	}

	// Verify error message went to stderr
	if !strings.Contains(stderrOutput, "test error message") {
		t.Errorf("Error message not found in stderr: %s", stderrOutput)
	}

	if err := rStdout.Close(); err != nil {
		t.Logf("failed to close rStdout: %v", err)
	}
	if err := rStderr.Close(); err != nil {
		t.Logf("failed to close rStderr: %v", err)
	}
}

func TestJSONLogging(t *testing.T) {
	// Capture output
	var buf bytes.Buffer

	// Initialize logger with JSON format
	config := &config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}
	err := InitializeLogger(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Temporarily redirect output to buffer
	Logger.SetOutput(&buf)

	// Log a message with fields
	Logger.WithFields(logrus.Fields{
		"service":   "test-service",
		"operation": "test-operation",
		"duration":  123.45,
	}).Info("test message")

	// Parse JSON output
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	if err != nil {
		t.Fatalf("Failed to parse JSON log output: %v", err)
	}

	// Verify JSON structure
	if logEntry["message"] != "test message" {
		t.Errorf("Expected message 'test message', got %v", logEntry["message"])
	}
	if logEntry["level"] != "info" {
		t.Errorf("Expected level 'info', got %v", logEntry["level"])
	}
	if logEntry["service"] != "test-service" {
		t.Errorf("Expected service 'test-service', got %v", logEntry["service"])
	}
	if logEntry["operation"] != "test-operation" {
		t.Errorf("Expected operation 'test-operation', got %v", logEntry["operation"])
	}
	if logEntry["duration"] != 123.45 {
		t.Errorf("Expected duration 123.45, got %v", logEntry["duration"])
	}

	// Verify timestamp field exists
	if _, exists := logEntry["timestamp"]; !exists {
		t.Error("Timestamp field missing from JSON output")
	}
}

func TestTextLogging(t *testing.T) {
	// Capture output
	var buf bytes.Buffer

	// Initialize logger with text format
	config := &config.LoggingConfig{
		Level:  "info",
		Format: "text",
	}
	err := InitializeLogger(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Temporarily redirect output to buffer
	Logger.SetOutput(&buf)

	// Log a message with fields
	Logger.WithFields(logrus.Fields{
		"service":   "test-service",
		"operation": "test-operation",
	}).Info("test message")

	output := buf.String()

	// Verify text format contains expected elements
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message 'test message' in output: %s", output)
	}
	if !strings.Contains(output, "level=info") {
		t.Errorf("Expected 'level=info' in output: %s", output)
	}
	if !strings.Contains(output, "service=test-service") {
		t.Errorf("Expected 'service=test-service' in output: %s", output)
	}
	if !strings.Contains(output, "operation=test-operation") {
		t.Errorf("Expected 'operation=test-operation' in output: %s", output)
	}
}

func TestHelperFunctions(t *testing.T) {
	// Initialize logger
	config := &config.LoggingConfig{
		Level:  "debug",
		Format: "json",
	}
	err := InitializeLogger(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Test WithServiceName
	entry := WithServiceName("test-service")
	if entry.Data["service"] != "test-service" {
		t.Errorf("Expected service field 'test-service', got %v", entry.Data["service"])
	}

	// Test WithOperation
	entry = WithOperation("test-operation")
	if entry.Data["operation"] != "test-operation" {
		t.Errorf("Expected operation field 'test-operation', got %v", entry.Data["operation"])
	}

	// Test WithDuration
	entry = WithDuration("check-service", 123.45)
	if entry.Data["operation"] != "check-service" {
		t.Errorf("Expected operation field 'check-service', got %v", entry.Data["operation"])
	}
	if entry.Data["duration_ms"] != 123.45 {
		t.Errorf("Expected duration_ms field 123.45, got %v", entry.Data["duration_ms"])
	}

	// Test WithPath
	entry = WithPath("/test/path")
	if entry.Data["path"] != "/test/path" {
		t.Errorf("Expected path field '/test/path', got %v", entry.Data["path"])
	}

	// Test WithFields
	fields := logrus.Fields{
		"key1": "value1",
		"key2": "value2",
	}
	entry = WithFields(fields)
	if entry.Data["key1"] != "value1" {
		t.Errorf("Expected key1 field 'value1', got %v", entry.Data["key1"])
	}
	if entry.Data["key2"] != "value2" {
		t.Errorf("Expected key2 field 'value2', got %v", entry.Data["key2"])
	}
}

func TestLogLevelFiltering(t *testing.T) {
	// Capture output
	var buf bytes.Buffer

	// Initialize logger with warn level (should filter out info and debug)
	config := &config.LoggingConfig{
		Level:  "warn",
		Format: "json",
	}
	err := InitializeLogger(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Temporarily redirect output to buffer
	Logger.SetOutput(&buf)

	// Log messages at different levels
	Logger.Debug("debug message")
	Logger.Info("info message")
	Logger.Warn("warn message")
	Logger.Error("error message")

	output := buf.String()

	// Debug and info should be filtered out
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should be filtered out")
	}
	if strings.Contains(output, "info message") {
		t.Error("Info message should be filtered out")
	}

	// Warn and error should be present
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message should be present")
	}
}