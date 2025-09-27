package health

import (
	"testing"
	"time"
)

func TestParseServiceProperties(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]any
		expected SystemdResult
	}{
		{
			name: "successful service execution",
			props: map[string]any{
				"ExecMainExitTimestamp": uint64(1727332200000000), // Example timestamp in microseconds
				"ExecMainStatus":        int32(0),
			},
			expected: SystemdResult{
				Enabled:  true,
				LastRun:  time.Unix(1727332200, 0),
				ExitCode: 0,
				Status:   "success",
			},
		},
		{
			name: "failed service execution",
			props: map[string]any{
				"ExecMainExitTimestamp": uint64(1727332200000000),
				"ExecMainStatus":        int32(1),
			},
			expected: SystemdResult{
				Enabled:  true,
				LastRun:  time.Unix(1727332200, 0),
				ExitCode: 1,
				Status:   "failed",
			},
		},
		{
			name: "service never run",
			props: map[string]any{
				"ExecMainExitTimestamp": uint64(0),
				"ExecMainStatus":        int32(0),
			},
			expected: SystemdResult{
				Enabled:  true,
				LastRun:  time.Time{}, // Zero time
				ExitCode: 0,
				Status:   "never_run",
			},
		},
		{
			name: "service never run - no timestamp property",
			props: map[string]any{
				"ExecMainStatus": int32(0),
			},
			expected: SystemdResult{
				Enabled:  true,
				LastRun:  time.Time{}, // Zero time
				ExitCode: 0,
				Status:   "never_run",
			},
		},
		{
			name: "service with exit code but no timestamp",
			props: map[string]any{
				"ExecMainStatus": int32(2),
			},
			expected: SystemdResult{
				Enabled:  true,
				LastRun:  time.Time{}, // Zero time
				ExitCode: 2,
				Status:   "never_run", // No timestamp means never run
			},
		},
		{
			name:  "empty properties",
			props: map[string]any{},
			expected: SystemdResult{
				Enabled:  true,
				LastRun:  time.Time{},
				ExitCode: 0,
				Status:   "never_run",
			},
		},
		{
			name: "timestamp with microseconds precision",
			props: map[string]any{
				"ExecMainExitTimestamp": uint64(1727332200123456), // With microseconds
				"ExecMainStatus":        int32(0),
			},
			expected: SystemdResult{
				Enabled:  true,
				LastRun:  time.Unix(1727332200, 123456000), // Correctly converted
				ExitCode: 0,
				Status:   "success",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseServiceProperties(tt.props)

			// Compare all fields
			if result.Enabled != tt.expected.Enabled {
				t.Errorf("Enabled = %v, want %v", result.Enabled, tt.expected.Enabled)
			}
			if !result.LastRun.Equal(tt.expected.LastRun) {
				t.Errorf("LastRun = %v, want %v", result.LastRun, tt.expected.LastRun)
			}
			if result.ExitCode != tt.expected.ExitCode {
				t.Errorf("ExitCode = %v, want %v", result.ExitCode, tt.expected.ExitCode)
			}
			if result.Status != tt.expected.Status {
				t.Errorf("Status = %v, want %v", result.Status, tt.expected.Status)
			}
		})
	}
}

func TestParseServicePropertiesWithWrongTypes(t *testing.T) {
	// Test with wrong property types to ensure graceful handling
	props := map[string]any{
		"ExecMainExitTimestamp": "not a number",
		"ExecMainStatus":        "not a number",
	}

	result := parseServiceProperties(props)

	expected := SystemdResult{
		Enabled:  true,
		LastRun:  time.Time{},
		ExitCode: 0,
		Status:   "never_run",
	}

	if result.Enabled != expected.Enabled {
		t.Errorf("Enabled = %v, want %v", result.Enabled, expected.Enabled)
	}
	if !result.LastRun.Equal(expected.LastRun) {
		t.Errorf("LastRun = %v, want %v", result.LastRun, expected.LastRun)
	}
	if result.ExitCode != expected.ExitCode {
		t.Errorf("ExitCode = %v, want %v", result.ExitCode, expected.ExitCode)
	}
	if result.Status != expected.Status {
		t.Errorf("Status = %v, want %v", result.Status, expected.Status)
	}
}

func TestCheckSystemdServiceInputValidation(t *testing.T) {
	// Test with empty service name
	result := CheckSystemdService("")
	if result.Status != "error" {
		t.Errorf("Expected error status for empty service name, got %s", result.Status)
	}
	if result.Error == "" {
		t.Error("Expected error message for empty service name")
	}

	// Test with invalid service name (this will likely fail to connect to D-Bus in test environment)
	result = CheckSystemdService("nonexistent-service.service")
	// In test environment, this will likely fail with D-Bus connection error
	// which is expected behavior
	if result.Status == "" {
		t.Error("Expected some status to be set")
	}
}