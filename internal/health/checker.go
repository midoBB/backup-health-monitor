package health

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"backup-health-monitor/internal/config"
	"backup-health-monitor/internal/metrics"
)

// HealthStatus represents the health status of a service
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "healthy"
	StatusWarning  HealthStatus = "warning"
	StatusCritical HealthStatus = "critical"
)

// SystemdInfo represents systemd service information in the API response
type SystemdInfo struct {
	LastRun  time.Time `json:"last_run"`
	ExitCode int       `json:"exit_code"`
	Enabled  bool      `json:"enabled"`
}

// BackupInfo represents backup file information in the API response
type BackupInfo struct {
	Path            string  `json:"path"`
	RecentFiles     int     `json:"recent_files"`
	NewestFileAge   float64 `json:"newest_file_age_hours"`
	NewestFileSizeMB float64 `json:"newest_file_size_mb"`
	Status          string  `json:"status"`
}

// ServiceResult represents the health check result for a single service
type ServiceResult struct {
	Name    string       `json:"name"`
	Status  string       `json:"status"`
	Systemd SystemdInfo  `json:"systemd"`
	Backup  BackupInfo   `json:"backup"`
	Issues  []string     `json:"issues,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// HealthResult represents the overall health check result
type HealthResult struct {
	Status    HealthStatus    `json:"status"`
	Timestamp time.Time       `json:"timestamp"`
	Version   string          `json:"version"`
	Services  []ServiceResult `json:"services"`
	Summary   Summary         `json:"summary"`
}

// Summary provides aggregate statistics
type Summary struct {
	TotalServices     int `json:"total_services"`
	HealthyServices   int `json:"healthy_services"`
	DegradedServices  int `json:"degraded_services"`
	UnhealthyServices int `json:"unhealthy_services"`
}

// CheckService performs health check for a single service
func CheckService(config config.ServiceConfig) ServiceResult {
	startTime := time.Now()
	logger := logrus.WithField("service", config.Name)
	logger.Debug("Starting service health check")

	result := ServiceResult{
		Name: config.Name,
	}

	// 1. Check systemd service status
	logger.Debug("Checking systemd service status")
	systemdResult := CheckSystemdService(config.Name)
	result.Systemd = SystemdInfo{
		LastRun:  systemdResult.LastRun,
		ExitCode: systemdResult.ExitCode,
		Enabled:  systemdResult.Enabled,
	}
	logger.WithFields(logrus.Fields{
		"enabled": systemdResult.Enabled,
		"exit_code": systemdResult.ExitCode,
		"last_run": systemdResult.LastRun,
	}).Debug("Systemd service check completed")

	// 2. Check backup files
	logger.WithField("backup_path", config.BackupPath).Debug("Checking backup files")
	backupResult := CheckBackupFiles(config)
	result.Backup = BackupInfo{
		Path:            backupResult.Path,
		RecentFiles:     backupResult.RecentFiles,
		NewestFileAge:   backupResult.NewestFileAge,
		NewestFileSizeMB: backupResult.NewestFileSizeMB,
		Status:          backupResult.Status,
	}
	logger.WithFields(logrus.Fields{
		"backup_path": backupResult.Path,
		"recent_files": backupResult.RecentFiles,
		"newest_file_age_hours": backupResult.NewestFileAge,
		"newest_file_size_mb": backupResult.NewestFileSizeMB,
		"backup_status": backupResult.Status,
	}).Debug("Backup files check completed")

	// Collect issues from both checks
	result.Issues = make([]string, 0)
	result.Issues = append(result.Issues, backupResult.Issues...)

	// 3. Determine combined health status
	result.Status = determineHealthStatus(systemdResult, backupResult)

	// Handle systemd errors
	if systemdResult.Error != "" {
		result.Error = systemdResult.Error
		// If we can't check systemd, but backup is healthy, that's a warning
		if result.Status == "healthy" && backupResult.Status == "healthy" {
			result.Status = "warning"
			result.Issues = append(result.Issues, "Could not check systemd service status")
		}
		logger.WithField("error", systemdResult.Error).Warn("Systemd check failed")
	}

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"final_status": result.Status,
		"duration_ms": float64(duration.Nanoseconds()) / 1e6,
		"issues_count": len(result.Issues),
	}).Info("Service health check completed")

	// Update Prometheus metrics if available
	if metrics.Collector != nil {
		// Determine if service is healthy for metrics
		isHealthy := result.Status == "healthy"

		// Update service metrics
		metrics.Collector.UpdateServiceMetrics(
			config.Name,
			isHealthy,
			result.Systemd.LastRun,
			result.Systemd.ExitCode,
			result.Backup.RecentFiles,
			result.Backup.NewestFileAge,
			result.Backup.NewestFileSizeMB,
		)
	}

	return result
}

// determineHealthStatus combines systemd and backup status to determine overall service health
// Health status rules:
// - CRITICAL: Systemd service not enabled, failed, or backup check failed
// - WARNING: Service succeeded but backup issues (no recent files, size issues, etc.)
// - HEALTHY: Service succeeded AND backup check is healthy
func determineHealthStatus(systemdResult SystemdResult, backupResult BackupResult) string {
	// Priority 1: Check systemd service status
	switch systemdResult.Status {
	case "not_enabled":
		return "critical"
	case "failed":
		return "critical"
	case "never_run":
		return "critical"
	case "error":
		// If systemd check failed but backup is healthy, that's still a warning
		if backupResult.Status == "healthy" {
			return "warning"
		}
		return "critical"
	case "success":
		// Service succeeded, now check backup status
		switch backupResult.Status {
		case "healthy":
			return "healthy"
		case "warning":
			return "warning"
		default:
			return "critical"
		}
	default:
		// Unknown systemd status
		return "critical"
	}
}

// CheckAllServices performs health check for all configured services
func CheckAllServices(config *config.Config, version string) HealthResult {
	startTime := time.Now()

	result := HealthResult{
		Timestamp: time.Now(),
		Version:   version,
		Services:  make([]ServiceResult, 0, len(config.Services)),
		Summary: Summary{
			TotalServices:     len(config.Services),
			HealthyServices:   0,
			DegradedServices:  0,
			UnhealthyServices: 0,
		},
	}

	// Check each service and collect results
	for _, serviceConfig := range config.Services {
		serviceResult := CheckService(serviceConfig)
		result.Services = append(result.Services, serviceResult)

		// Update summary counts based on service status
		switch serviceResult.Status {
		case "healthy":
			result.Summary.HealthyServices++
		case "warning":
			result.Summary.DegradedServices++
		case "critical":
			result.Summary.UnhealthyServices++
		default:
			// Unknown status, count as unhealthy
			result.Summary.UnhealthyServices++
		}
	}

	// Determine overall system health status
	result.Status = determineOverallStatus(result.Summary)

	// Update application-level metrics if available
	if metrics.Collector != nil {
		metrics.Collector.IncrementChecksTotal()
		metrics.Collector.ObserveCheckDuration(time.Since(startTime))
	}

	return result
}

// determineOverallStatus determines the overall system health based on service summary
func determineOverallStatus(summary Summary) HealthStatus {
	// If any services are unhealthy (critical), system is critical
	if summary.UnhealthyServices > 0 {
		return StatusCritical
	}

	// If any services are degraded (warning), system is warning
	if summary.DegradedServices > 0 {
		return StatusWarning
	}

	// If all services are healthy, system is healthy
	if summary.HealthyServices == summary.TotalServices {
		return StatusHealthy
	}

	// Fallback (shouldn't reach here with valid data)
	return StatusCritical
}

// BackupResult represents the backup file check result for a service
type BackupResult struct {
	Path             string   `json:"path"`
	RecentFiles      int      `json:"recent_files"`
	NewestFileAge    float64  `json:"newest_file_age_hours"`
	NewestFileSizeMB float64  `json:"newest_file_size_mb"`
	Status           string   `json:"status"`
	Issues           []string `json:"issues,omitempty"`
}

// countRecentFiles counts files in backup directory within the age threshold
// Returns (recent_count, newest_file_age_hours)
func countRecentFiles(backupPath string, cutoff time.Time) (int, float64) {
	// Check if directory exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return 0, 0
	}

	recentCount := 0
	newestFileAge := float64(0)

	// Walk through the directory
	err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files we can't access
			return nil
		}

		// Only check regular files
		if !info.Mode().IsRegular() {
			return nil
		}

		// Check if file is newer than cutoff
		if info.ModTime().After(cutoff) {
			recentCount++

			// Calculate age in hours for this file
			fileAge := time.Since(info.ModTime()).Hours()

			// Track the newest file (smallest age)
			if recentCount == 1 || fileAge < newestFileAge {
				newestFileAge = fileAge
			}
		}

		return nil
	})

	if err != nil {
		// If we can't walk the directory, return 0 files
		return 0, 0
	}

	return recentCount, newestFileAge
}

// matchesFilePatterns checks if a filename matches any of the expected patterns
func matchesFilePatterns(filename string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filename)
		if err != nil {
			// Invalid pattern, skip
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// findNewestMatchingFile finds the newest file that matches the patterns
func findNewestMatchingFile(backupPath string, patterns []string, cutoff time.Time) (string, float64, float64, error) {
	var newestFile string
	var newestTime time.Time
	var newestAge float64
	var newestSize float64

	err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		// Check if file matches patterns
		filename := filepath.Base(path)
		if !matchesFilePatterns(filename, patterns) {
			return nil
		}

		// Check if file is newer than cutoff
		if info.ModTime().After(cutoff) {
			// Track newest file
			if newestFile == "" || info.ModTime().After(newestTime) {
				newestFile = path
				newestTime = info.ModTime()
				newestAge = time.Since(info.ModTime()).Hours()
				newestSize = float64(info.Size()) / (1024 * 1024)
			}
		}

		return nil
	})

	if err != nil {
		return "", 0, 0, err
	}

	if newestFile == "" {
		return "", 0, 0, fmt.Errorf("no matching files found")
	}

	return newestFile, newestAge, newestSize, nil
}

// CheckBackupFiles validates backup files for a service configuration
func CheckBackupFiles(serviceConfig config.ServiceConfig) BackupResult {
	logger := logrus.WithFields(logrus.Fields{
		"service": serviceConfig.Name,
		"backup_path": serviceConfig.BackupPath,
		"max_age_hours": serviceConfig.MaxAgeHours,
		"file_patterns": serviceConfig.ExpectedFilePatterns,
	})
	logger.Debug("Starting backup files check")

	result := BackupResult{
		Path:   serviceConfig.BackupPath,
		Status: "healthy",
		Issues: make([]string, 0),
	}

	// Check if backup directory exists
	if _, err := os.Stat(serviceConfig.BackupPath); os.IsNotExist(err) {
		logger.WithError(err).Warn("Backup directory does not exist")
		result.Status = "warning"
		result.Issues = append(result.Issues, fmt.Sprintf("Backup directory does not exist: %s", serviceConfig.BackupPath))
		return result
	}
	logger.Debug("Backup directory exists")

	// Calculate cutoff time based on max age
	cutoff := time.Now().Add(-time.Duration(serviceConfig.MaxAgeHours) * time.Hour)

	// Count recent files matching patterns
	recentCount := 0
	var newestAge float64

	if len(serviceConfig.ExpectedFilePatterns) > 0 {
		// Use pattern matching
		_, newestAge, newestFileSizeMB, err := findNewestMatchingFile(serviceConfig.BackupPath, serviceConfig.ExpectedFilePatterns, cutoff)
		if err != nil {
			// No matching files found
			result.Status = "warning"
			result.Issues = append(result.Issues, "No recent backup files found matching expected patterns")
		} else {
			// We found at least one recent matching file
			result.NewestFileAge = newestAge
			result.NewestFileSizeMB = newestFileSizeMB

			// Validate file size if minimum is specified
			if serviceConfig.MinFileSizeMB > 0 && newestFileSizeMB < float64(serviceConfig.MinFileSizeMB) {
				result.Status = "warning"
				result.Issues = append(result.Issues,
					fmt.Sprintf("Newest backup file is too small: %.2f MB (minimum: %d MB)",
						newestFileSizeMB, serviceConfig.MinFileSizeMB))
			}
		}

		// Count all recent matching files
		if walkErr := filepath.Walk(serviceConfig.BackupPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			filename := filepath.Base(path)
			if matchesFilePatterns(filename, serviceConfig.ExpectedFilePatterns) && info.ModTime().After(cutoff) {
				recentCount++
			}
			return nil
		}); walkErr != nil {
			logger.WithError(walkErr).Warn("Error walking backup path to count files")
		}
	} else {
		// Fall back to checking all files (no patterns specified)
		recentCount, newestAge = countRecentFiles(serviceConfig.BackupPath, cutoff)
		result.NewestFileAge = newestAge
	}

	result.RecentFiles = recentCount

	// Final status determination
	if recentCount == 0 {
		result.Status = "warning"
		if len(result.Issues) == 0 {
			result.Issues = append(result.Issues, "No recent backup files found")
		}
	}

	logger.WithFields(logrus.Fields{
		"final_status": result.Status,
		"recent_files": result.RecentFiles,
		"newest_file_age_hours": result.NewestFileAge,
		"newest_file_size_mb": result.NewestFileSizeMB,
		"issues_count": len(result.Issues),
	}).Debug("Backup files check completed")

	return result
}
