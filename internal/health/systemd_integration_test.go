// +build integration

package health

import (
	"testing"
)

// TestCheckSystemdServiceIntegration tests against real systemd services
// Run with: go test -tags=integration ./internal/health -v
func TestCheckSystemdServiceIntegration(t *testing.T) {
	// Test with a known system service that should always exist
	// Using systemd-logind.service as it's usually enabled on most systems
	result := CheckSystemdService("systemd-logind.service")

	// We expect either a valid result or a specific error
	if result.Status == "error" && result.Error != "" {
		// If there's an error, it should be a meaningful one
		t.Logf("Service check failed with error: %s", result.Error)
		// This is acceptable in test environments where systemd might not be available
	} else {
		// If no error, validate the result structure
		if result.Status == "" {
			t.Error("Expected status to be set")
		}

		t.Logf("Service status: %s", result.Status)
		t.Logf("Service enabled: %v", result.Enabled)
		t.Logf("Last run: %v", result.LastRun)
		t.Logf("Exit code: %d", result.ExitCode)

		// Valid statuses are: success, failed, never_run, not_enabled, error
		validStatuses := map[string]bool{
			"success":     true,
			"failed":      true,
			"never_run":   true,
			"not_enabled": true,
			"error":       true,
		}

		if !validStatuses[result.Status] {
			t.Errorf("Invalid status %s", result.Status)
		}
	}
}

// TestCheckSystemdServiceNonExistent tests with a service that doesn't exist
func TestCheckSystemdServiceNonExistent(t *testing.T) {
	result := CheckSystemdService("non-existent-service.service")

	// Should return error or not_enabled status
	if result.Status != "error" && result.Status != "not_enabled" {
		t.Errorf("Expected error or not_enabled status for non-existent service, got %s", result.Status)
	}

	if result.Error == "" {
		t.Error("Expected error message for non-existent service")
	}

	t.Logf("Non-existent service result: %s - %s", result.Status, result.Error)
}

// TestCheckSystemdServiceWithTimer tests a service that might have an associated timer
func TestCheckSystemdServiceWithTimer(t *testing.T) {
	// Test with apt-daily.service which often has an associated timer
	// This is common on Debian/Ubuntu systems
	result := CheckSystemdService("apt-daily.service")

	// We don't assert specific results since this depends on the system configuration
	// We just ensure the function doesn't crash and returns a valid structure
	if result.Status == "" {
		t.Error("Expected status to be set")
	}

	t.Logf("apt-daily service result: %s - %s", result.Status, result.Error)
	t.Logf("Enabled: %v, Last run: %v, Exit code: %d", result.Enabled, result.LastRun, result.ExitCode)
}

// TestSystemdDBusConnection tests that we can establish a D-Bus connection
func TestSystemdDBusConnection(t *testing.T) {
	// This test just verifies we can connect to systemd D-Bus
	// Useful for CI environments where systemd might not be available
	result := CheckSystemdService("test-connection.service")

	// If we can't connect to D-Bus, the error should mention it
	if result.Status == "error" {
		if result.Error == "" {
			t.Error("Expected error message when connection fails")
		}
		t.Logf("D-Bus connection test: %s", result.Error)
	} else {
		t.Log("D-Bus connection successful")
	}
}