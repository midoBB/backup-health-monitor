package health

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sirupsen/logrus"
)

// SystemdResult represents systemd service status information
type SystemdResult struct {
	Enabled  bool      `json:"enabled"`
	LastRun  time.Time `json:"last_run"`
	ExitCode int       `json:"exit_code"`
	Status   string    `json:"status"` // success, failed, never_run, not_enabled
	Error    string    `json:"error,omitempty"`
}

// CheckSystemdService checks the status of a systemd service
func CheckSystemdService(serviceName string) SystemdResult {
	logger := logrus.WithField("service", serviceName)
	logger.Debug("Starting systemd service check")

	result := SystemdResult{
		Status: "error",
	}

	// Input validation
	if serviceName == "" {
		logger.Error("Service name cannot be empty")
		result.Error = "Service name cannot be empty"
		return result
	}

	// Create D-Bus connection to systemd
	logger.Debug("Connecting to systemd D-Bus")
	conn, err := dbus.NewSystemdConnectionContext(context.Background())
	if err != nil {
		logger.WithError(err).Error("Failed to connect to systemd D-Bus")
		result.Error = fmt.Sprintf("Failed to connect to systemd D-Bus: %v", err)
		return result
	}
	defer conn.Close()
	logger.Debug("Connected to systemd D-Bus successfully")

	// Check if service is enabled by listing unit files
	logger.Debug("Listing unit files to check service status")
	unitFiles, err := conn.ListUnitFilesContext(context.Background())
	if err != nil {
		logger.WithError(err).Error("Failed to list unit files")
		result.Error = fmt.Sprintf("Failed to list unit files: %v", err)
		return result
	}
	logger.WithField("unit_files_count", len(unitFiles)).Debug("Retrieved unit files list")

	serviceEnabled := false
	timerEnabled := false
	timerName := strings.TrimSuffix(serviceName, ".service") + ".timer"

	for _, unit := range unitFiles {
		unitName := strings.Split(unit.Path, "/")[len(strings.Split(unit.Path, "/"))-1]
		if unitName == serviceName && unit.Type == "enabled" {
			serviceEnabled = true
		}
		if unitName == timerName && unit.Type == "enabled" {
			timerEnabled = true
		}
	}

	result.Enabled = serviceEnabled
	logger.WithFields(logrus.Fields{
		"service_enabled": serviceEnabled,
		"timer_enabled": timerEnabled,
		"timer_name": timerName,
	}).Debug("Service and timer enabled status checked")

	if !serviceEnabled {
		logger.Warn("Service is not enabled")
		result.Status = "not_enabled"
		result.Error = fmt.Sprintf("Service %s is not enabled", serviceName)
		return result
	}

	// Check if timer exists and is enabled (following bash script logic)
	if !timerEnabled {
		// Timer not found or not enabled - this is a warning condition like in bash script
		result.Status = "not_enabled"
		result.Error = fmt.Sprintf("Timer %s not found or not enabled", timerName)
		return result
	}

	// Get service properties (replicating systemctl show)
	props, err := conn.GetUnitPropertiesContext(context.Background(), serviceName)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to get properties for service %s: %v", serviceName, err)
		return result
	}

	// Parse the systemd properties
	result = parseServiceProperties(props)

	logger.WithFields(logrus.Fields{
		"enabled": result.Enabled,
		"status": result.Status,
		"exit_code": result.ExitCode,
		"last_run": result.LastRun,
	}).Debug("Systemd service check completed")

	return result
}

// parseServiceProperties extracts relevant information from systemd properties
func parseServiceProperties(props map[string]any) SystemdResult {
	result := SystemdResult{
		Enabled: true, // If we reach here, service was enabled
		Status:  "never_run",
	}

	// Extract ExecMainExitTimestamp (replicating systemctl show --property=ExecMainExitTimestamp)
	if timestampRaw, ok := props["ExecMainExitTimestamp"]; ok {
		// systemd provides timestamps as microseconds since epoch
		if timestampMicros, ok := timestampRaw.(uint64); ok && timestampMicros > 0 {
			// Convert microseconds to time.Time
			result.LastRun = time.Unix(
				int64(timestampMicros/1000000),
				int64((timestampMicros%1000000)*1000),
			)
		}
	}

	// Extract ExecMainStatus (replicating systemctl show --property=ExecMainStatus)
	if statusRaw, ok := props["ExecMainStatus"]; ok {
		if exitCode, ok := statusRaw.(int32); ok {
			result.ExitCode = int(exitCode)

			// Determine status based on exit code (following bash script logic)
			if !result.LastRun.IsZero() {
				if result.ExitCode == 0 {
					result.Status = "success"
				} else {
					result.Status = "failed"
				}
			}
		}
	}

	// Alternative property names that systemd might use
	if result.LastRun.IsZero() {
		// Try alternative timestamp property names
		if timestampStr, ok := props["ExecMainExitTimestampMonotonic"].(string); ok &&
			timestampStr != "" &&
			timestampStr != "0" {
			// This is a fallback - parse string timestamp if available
			if parsed, err := strconv.ParseInt(timestampStr, 10, 64); err == nil && parsed > 0 {
				result.LastRun = time.Unix(parsed/1000000, (parsed%1000000)*1000)
			}
		}
	}

	return result
}

