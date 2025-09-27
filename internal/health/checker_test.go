package health

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"backup-health-monitor/internal/config"
)

func TestCountRecentFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "backup_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Test cases
	tests := []struct {
		name           string
		setupFiles     []fileInfo
		cutoffHours    int
		expectedCount  int
		expectNewestAge bool // whether we expect a valid newest age
	}{
		{
			name: "No files",
			setupFiles: []fileInfo{},
			cutoffHours: 24,
			expectedCount: 0,
			expectNewestAge: false,
		},
		{
			name: "Recent files only",
			setupFiles: []fileInfo{
				{name: "backup1.sql", ageHours: 2},
				{name: "backup2.sql", ageHours: 5},
				{name: "backup3.sql", ageHours: 10},
			},
			cutoffHours: 24,
			expectedCount: 3,
			expectNewestAge: true,
		},
		{
			name: "Mixed recent and old files",
			setupFiles: []fileInfo{
				{name: "recent1.sql", ageHours: 2},
				{name: "old1.sql", ageHours: 48},
				{name: "recent2.sql", ageHours: 12},
				{name: "old2.sql", ageHours: 72},
			},
			cutoffHours: 24,
			expectedCount: 2,
			expectNewestAge: true,
		},
		{
			name: "All old files",
			setupFiles: []fileInfo{
				{name: "old1.sql", ageHours: 48},
				{name: "old2.sql", ageHours: 72},
			},
			cutoffHours: 24,
			expectedCount: 0,
			expectNewestAge: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			for _, fileInfo := range tt.setupFiles {
				createTestFile(t, testDir, fileInfo)
			}

			// Calculate cutoff time
			cutoff := time.Now().Add(-time.Duration(tt.cutoffHours) * time.Hour)

			// Test the function
			count, newestAge := countRecentFiles(testDir, cutoff)

			// Verify results
			if count != tt.expectedCount {
				t.Errorf("Expected count %d, got %d", tt.expectedCount, count)
			}

			if tt.expectNewestAge && newestAge <= 0 {
				t.Errorf("Expected valid newest age, got %f", newestAge)
			}

			if !tt.expectNewestAge && newestAge != 0 {
				t.Errorf("Expected no newest age (0), got %f", newestAge)
			}

			// Clean up test directory
			if err := os.RemoveAll(testDir); err != nil {
				t.Logf("failed to remove test dir: %v", err)
			}
		})
	}
}

func TestCountRecentFilesNonExistentDirectory(t *testing.T) {
	nonExistentDir := "/path/that/does/not/exist"
	cutoff := time.Now().Add(-24 * time.Hour)

	count, age := countRecentFiles(nonExistentDir, cutoff)

	if count != 0 {
		t.Errorf("Expected count 0 for non-existent directory, got %d", count)
	}

	if age != 0 {
		t.Errorf("Expected age 0 for non-existent directory, got %f", age)
	}
}

func TestMatchesFilePatterns(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		patterns []string
		expected bool
	}{
		{
			name:     "Single pattern match",
			filename: "backup-2023-01-01.sql",
			patterns: []string{"backup-*.sql"},
			expected: true,
		},
		{
			name:     "Multiple patterns, first matches",
			filename: "vault-backup-123.tar",
			patterns: []string{"vault-backup-*", "*.sql.gz"},
			expected: true,
		},
		{
			name:     "Multiple patterns, second matches",
			filename: "database.sql.gz",
			patterns: []string{"vault-backup-*", "*.sql.gz"},
			expected: true,
		},
		{
			name:     "No pattern matches",
			filename: "random-file.txt",
			patterns: []string{"backup-*.sql", "*.gz"},
			expected: false,
		},
		{
			name:     "Empty patterns",
			filename: "any-file.txt",
			patterns: []string{},
			expected: false,
		},
		{
			name:     "Invalid pattern (should not match)",
			filename: "test.sql",
			patterns: []string{"[invalid"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesFilePatterns(tt.filename, tt.patterns)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for filename %s with patterns %v",
					tt.expected, result, tt.filename, tt.patterns)
			}
		})
	}
}

func TestCheckBackupFiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "backup_check_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	tests := []struct {
		name           string
		setupFiles     []fileInfo
		serviceConfig  config.ServiceConfig
		expectedStatus string
		expectedIssues int
		checkRecentFiles bool
	}{
		{
			name: "Healthy backup with recent files",
			setupFiles: []fileInfo{
				{name: "vault-backup-001.tar", ageHours: 2, sizeMB: 5},
				{name: "vault-backup-002.tar", ageHours: 10, sizeMB: 4},
			},
			serviceConfig: config.ServiceConfig{
				Name:                 "vault-backup.service",
				BackupPath:           "", // Will be set to test directory
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"vault-backup-*.tar"},
				MinFileSizeMB:        1,
			},
			expectedStatus: "healthy",
			expectedIssues: 0,
			checkRecentFiles: true,
		},
		{
			name: "Warning due to small file size",
			setupFiles: []fileInfo{
				{name: "backup.sql", ageHours: 2, sizeMB: 0.5},
			},
			serviceConfig: config.ServiceConfig{
				Name:                 "postgres-backup.service",
				BackupPath:           "",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.sql"},
				MinFileSizeMB:        2,
			},
			expectedStatus: "warning",
			expectedIssues: 1,
			checkRecentFiles: true,
		},
		{
			name: "Warning due to no recent files",
			setupFiles: []fileInfo{
				{name: "old-backup.sql", ageHours: 48, sizeMB: 5},
			},
			serviceConfig: config.ServiceConfig{
				Name:                 "old-backup.service",
				BackupPath:           "",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.sql"},
				MinFileSizeMB:        1,
			},
			expectedStatus: "warning",
			expectedIssues: 1,
			checkRecentFiles: false,
		},
		{
			name: "No matching patterns",
			setupFiles: []fileInfo{
				{name: "wrong-format.txt", ageHours: 2, sizeMB: 5},
			},
			serviceConfig: config.ServiceConfig{
				Name:                 "pattern-test.service",
				BackupPath:           "",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.sql", "*.tar.gz"},
				MinFileSizeMB:        1,
			},
			expectedStatus: "warning",
			expectedIssues: 1,
			checkRecentFiles: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test directory
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			// Create test files
			for _, fileInfo := range tt.setupFiles {
				createTestFile(t, testDir, fileInfo)
			}

			// Update service config with test directory
			tt.serviceConfig.BackupPath = testDir

			// Test the function
			result := CheckBackupFiles(tt.serviceConfig)

			// Verify results
			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result.Status)
			}

			if len(result.Issues) != tt.expectedIssues {
				t.Errorf("Expected %d issues, got %d: %v", tt.expectedIssues, len(result.Issues), result.Issues)
			}

			if tt.checkRecentFiles && result.RecentFiles <= 0 {
				t.Errorf("Expected recent files > 0, got %d", result.RecentFiles)
			}

			if !tt.checkRecentFiles && result.RecentFiles > 0 {
				t.Errorf("Expected no recent files, got %d", result.RecentFiles)
			}

			// Clean up test directory
			if err := os.RemoveAll(testDir); err != nil {
				t.Logf("failed to remove test dir: %v", err)
			}
		})
	}
}

func TestCheckBackupFilesNonExistentDirectory(t *testing.T) {
	serviceConfig := config.ServiceConfig{
		Name:                 "test.service",
		BackupPath:           "/path/that/does/not/exist",
		MaxAgeHours:          24,
		ExpectedFilePatterns: []string{"*.sql"},
		MinFileSizeMB:        1,
	}

	result := CheckBackupFiles(serviceConfig)

	if result.Status != "warning" {
		t.Errorf("Expected warning status for non-existent directory, got %s", result.Status)
	}

	if len(result.Issues) == 0 {
		t.Error("Expected at least one issue for non-existent directory")
	}

	if result.RecentFiles != 0 {
		t.Errorf("Expected 0 recent files for non-existent directory, got %d", result.RecentFiles)
	}
}

func TestCheckBackupFilesEmptyDirectory(t *testing.T) {
	// Create empty temporary directory
	tmpDir, err := os.MkdirTemp("", "empty_backup_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	serviceConfig := config.ServiceConfig{
		Name:                 "empty-test.service",
		BackupPath:           tmpDir,
		MaxAgeHours:          24,
		ExpectedFilePatterns: []string{"*.sql"},
		MinFileSizeMB:        1,
	}

	result := CheckBackupFiles(serviceConfig)

	if result.Status != "warning" {
		t.Errorf("Expected warning status for empty directory, got %s", result.Status)
	}

	if result.RecentFiles != 0 {
		t.Errorf("Expected 0 recent files for empty directory, got %d", result.RecentFiles)
	}
}

// Helper struct and function for creating test files
type fileInfo struct {
	name     string
	ageHours int
	sizeMB   float64
}

func createTestFile(t *testing.T, dir string, info fileInfo) {
	filePath := filepath.Join(dir, info.name)

	// Create file with specified size
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create test file %s: %v", filePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Logf("failed to close file: %v", err)
		}
	}()

	// Write data to achieve desired size
	if info.sizeMB > 0 {
		dataSize := int64(info.sizeMB * 1024 * 1024)
		err = file.Truncate(dataSize)
		if err != nil {
			t.Fatalf("Failed to set file size for %s: %v", filePath, err)
		}
	}

	// Set modification time
	modTime := time.Now().Add(-time.Duration(info.ageHours) * time.Hour)
	err = os.Chtimes(filePath, modTime, modTime)
	if err != nil {
		t.Fatalf("Failed to set modification time for %s: %v", filePath, err)
	}
}

func TestFindNewestMatchingFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "newest_file_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create test files
	testFiles := []fileInfo{
		{name: "backup-old.sql", ageHours: 48, sizeMB: 2},
		{name: "backup-newer.sql", ageHours: 12, sizeMB: 3},
		{name: "backup-newest.sql", ageHours: 2, sizeMB: 5},
		{name: "other-file.txt", ageHours: 1, sizeMB: 1}, // Doesn't match pattern
	}

	for _, info := range testFiles {
		createTestFile(t, tmpDir, info)
	}

	patterns := []string{"backup-*.sql"}
	cutoff := time.Now().Add(-24 * time.Hour)

	newestFile, newestAge, newestSize, err := findNewestMatchingFile(tmpDir, patterns, cutoff)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "backup-newest.sql")
	if newestFile != expectedFile {
		t.Errorf("Expected newest file %s, got %s", expectedFile, newestFile)
	}

	// Age should be approximately 2 hours (with some tolerance for test execution time)
	if newestAge < 1.9 || newestAge > 2.1 {
		t.Errorf("Expected age around 2 hours, got %f", newestAge)
	}

	// Size should be 5 MB
	if newestSize < 4.9 || newestSize > 5.1 {
		t.Errorf("Expected size around 5 MB, got %f", newestSize)
	}
}

func TestFindNewestMatchingFileNoMatches(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "no_matches_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create test files that don't match pattern
	testFiles := []fileInfo{
		{name: "other-file.txt", ageHours: 1, sizeMB: 1},
		{name: "another-file.log", ageHours: 2, sizeMB: 2},
	}

	for _, info := range testFiles {
		createTestFile(t, tmpDir, info)
	}

	patterns := []string{"backup-*.sql"}
	cutoff := time.Now().Add(-24 * time.Hour)

	_, _, _, err = findNewestMatchingFile(tmpDir, patterns, cutoff)

	if err == nil {
		t.Error("Expected error when no matching files found")
	}
}

func TestCheckBackupFilesWithSubdirectories(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "subdirs_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create subdirectories with files
	subDir1 := filepath.Join(tmpDir, "subdir1")
	subDir2 := filepath.Join(tmpDir, "subdir2")

	err = os.MkdirAll(subDir1, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir1: %v", err)
	}

	err = os.MkdirAll(subDir2, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir2: %v", err)
	}

	// Create files in root and subdirectories
	createTestFile(t, tmpDir, fileInfo{name: "root-backup.sql", ageHours: 2, sizeMB: 5})
	createTestFile(t, subDir1, fileInfo{name: "sub1-backup.sql", ageHours: 3, sizeMB: 3})
	createTestFile(t, subDir2, fileInfo{name: "sub2-backup.sql", ageHours: 4, sizeMB: 4})

	serviceConfig := config.ServiceConfig{
		Name:                 "subdirs-test.service",
		BackupPath:           tmpDir,
		MaxAgeHours:          24,
		ExpectedFilePatterns: []string{"*backup.sql"},
		MinFileSizeMB:        1,
	}

	result := CheckBackupFiles(serviceConfig)

	if result.Status != "healthy" {
		t.Errorf("Expected healthy status with subdirectories, got %s", result.Status)
	}

	// Should find files in subdirectories too
	if result.RecentFiles < 3 {
		t.Errorf("Expected at least 3 recent files (including subdirs), got %d", result.RecentFiles)
	}
}

func TestCountRecentFilesPerformanceWithManyFiles(t *testing.T) {
	// Skip this test in short mode to avoid long test runs
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "perf_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create many test files (100 instead of 1000 to keep tests reasonable)
	numFiles := 100
	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("backup-%03d.sql", i)
		createTestFile(t, tmpDir, fileInfo{
			name:     filename,
			ageHours: i % 48, // Mix of recent and old files
			sizeMB:   1,
		})
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	start := time.Now()

	count, _ := countRecentFiles(tmpDir, cutoff)

	duration := time.Since(start)

	// Should complete reasonably quickly (under 1 second for 100 files)
	if duration > time.Second {
		t.Errorf("Performance test took too long: %v", duration)
	}

	// Should find roughly half the files (those with age < 24 hours)
	expectedCount := numFiles / 2
	if count < expectedCount-10 || count > expectedCount+10 {
		t.Errorf("Expected approximately %d recent files, got %d", expectedCount, count)
	}

	t.Logf("Performance test: %d files processed in %v, found %d recent files", numFiles, duration, count)
}

func TestCheckBackupFilesWithMixedPermissions(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "permissions_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create accessible files
	createTestFile(t, tmpDir, fileInfo{name: "good-backup.sql", ageHours: 2, sizeMB: 5})
	createTestFile(t, tmpDir, fileInfo{name: "another-backup.sql", ageHours: 4, sizeMB: 3})

	// Create a subdirectory with restricted permissions (if we're not root)
	if os.Getuid() != 0 {
		restrictedDir := filepath.Join(tmpDir, "restricted")
		err = os.MkdirAll(restrictedDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create restricted dir: %v", err)
		}

		// Create a file in the restricted directory
		createTestFile(t, restrictedDir, fileInfo{name: "restricted-backup.sql", ageHours: 1, sizeMB: 2})

		// Remove read permissions from the directory
		err = os.Chmod(restrictedDir, 0000)
		if err != nil {
			t.Fatalf("Failed to restrict directory permissions: %v", err)
		}

		// Restore permissions for cleanup
		defer func() {
			if err := os.Chmod(restrictedDir, 0755); err != nil {
				t.Logf("failed to restore permissions: %v", err)
			}
		}()
	}

	serviceConfig := config.ServiceConfig{
		Name:                 "permissions-test.service",
		BackupPath:           tmpDir,
		MaxAgeHours:          24,
		ExpectedFilePatterns: []string{"*backup.sql"},
		MinFileSizeMB:        1,
	}

	result := CheckBackupFiles(serviceConfig)

	// Should still work with accessible files, gracefully handle permission errors
	if result.Status != "healthy" {
		t.Errorf("Expected healthy status despite permission issues, got %s", result.Status)
	}

	// Should find at least the accessible files
	if result.RecentFiles < 2 {
		t.Errorf("Expected at least 2 recent files (accessible ones), got %d", result.RecentFiles)
	}
}

// TestDetermineHealthStatus tests the core health status determination logic
func TestDetermineHealthStatus(t *testing.T) {
	tests := []struct {
		name           string
		systemdResult  SystemdResult
		backupResult   BackupResult
		expectedStatus string
	}{
		{
			name: "Healthy: Service success + backup healthy",
			systemdResult: SystemdResult{
				Status:   "success",
				Enabled:  true,
				ExitCode: 0,
			},
			backupResult: BackupResult{
				Status: "healthy",
			},
			expectedStatus: "healthy",
		},
		{
			name: "Warning: Service success + backup warning",
			systemdResult: SystemdResult{
				Status:   "success",
				Enabled:  true,
				ExitCode: 0,
			},
			backupResult: BackupResult{
				Status: "warning",
			},
			expectedStatus: "warning",
		},
		{
			name: "Critical: Service not enabled",
			systemdResult: SystemdResult{
				Status:  "not_enabled",
				Enabled: false,
			},
			backupResult: BackupResult{
				Status: "healthy",
			},
			expectedStatus: "critical",
		},
		{
			name: "Critical: Service failed",
			systemdResult: SystemdResult{
				Status:   "failed",
				Enabled:  true,
				ExitCode: 1,
			},
			backupResult: BackupResult{
				Status: "healthy",
			},
			expectedStatus: "critical",
		},
		{
			name: "Critical: Service never run",
			systemdResult: SystemdResult{
				Status:  "never_run",
				Enabled: true,
			},
			backupResult: BackupResult{
				Status: "healthy",
			},
			expectedStatus: "critical",
		},
		{
			name: "Warning: Systemd error but backup healthy",
			systemdResult: SystemdResult{
				Status: "error",
				Error:  "D-Bus connection failed",
			},
			backupResult: BackupResult{
				Status: "healthy",
			},
			expectedStatus: "warning",
		},
		{
			name: "Critical: Systemd error and backup unhealthy",
			systemdResult: SystemdResult{
				Status: "error",
				Error:  "D-Bus connection failed",
			},
			backupResult: BackupResult{
				Status: "critical",
			},
			expectedStatus: "critical",
		},
		{
			name: "Critical: Service success + backup critical",
			systemdResult: SystemdResult{
				Status:   "success",
				Enabled:  true,
				ExitCode: 0,
			},
			backupResult: BackupResult{
				Status: "critical",
			},
			expectedStatus: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineHealthStatus(tt.systemdResult, tt.backupResult)
			if result != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result)
			}
		})
	}
}

// TestDetermineOverallStatus tests the overall system health determination
func TestDetermineOverallStatus(t *testing.T) {
	tests := []struct {
		name           string
		summary        Summary
		expectedStatus HealthStatus
	}{
		{
			name: "All healthy",
			summary: Summary{
				TotalServices:     3,
				HealthyServices:   3,
				DegradedServices:  0,
				UnhealthyServices: 0,
			},
			expectedStatus: StatusHealthy,
		},
		{
			name: "Some degraded",
			summary: Summary{
				TotalServices:     3,
				HealthyServices:   2,
				DegradedServices:  1,
				UnhealthyServices: 0,
			},
			expectedStatus: StatusWarning,
		},
		{
			name: "Some unhealthy",
			summary: Summary{
				TotalServices:     3,
				HealthyServices:   1,
				DegradedServices:  1,
				UnhealthyServices: 1,
			},
			expectedStatus: StatusCritical,
		},
		{
			name: "All unhealthy",
			summary: Summary{
				TotalServices:     2,
				HealthyServices:   0,
				DegradedServices:  0,
				UnhealthyServices: 2,
			},
			expectedStatus: StatusCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineOverallStatus(tt.summary)
			if result != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result)
			}
		})
	}
}

// TestCheckAllServices tests the complete service aggregation logic
func TestCheckAllServices(t *testing.T) {
	// Create temporary directories for test services
	tmpDir1, err := os.MkdirTemp("", "service1_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir1); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	tmpDir2, err := os.MkdirTemp("", "service2_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir2); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create backup files for service 1 (healthy)
	createTestFile(t, tmpDir1, fileInfo{name: "service1-backup.sql", ageHours: 2, sizeMB: 5})

	// Don't create backup files for service 2 (will be warning)

	// Create test configuration
	testConfig := &config.Config{
		Services: []config.ServiceConfig{
			{
				Name:                 "test-service1.service",
				BackupPath:           tmpDir1,
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"service1-*.sql"},
				MinFileSizeMB:        1,
			},
			{
				Name:                 "test-service2.service",
				BackupPath:           tmpDir2,
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"service2-*.sql"},
				MinFileSizeMB:        1,
			},
		},
	}

	result := CheckAllServices(testConfig, "test-version")

	// Verify basic structure
	if len(result.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(result.Services))
	}

	if result.Summary.TotalServices != 2 {
		t.Errorf("Expected total services 2, got %d", result.Summary.TotalServices)
	}

	// Verify service results structure
	for _, service := range result.Services {
		if service.Name == "" {
			t.Error("Service name should not be empty")
		}

		// Check that systemd and backup info are populated
		if service.Backup.Path == "" {
			t.Error("Backup path should not be empty")
		}

		// Status should be one of the valid values
		if service.Status != "healthy" && service.Status != "warning" && service.Status != "critical" {
			t.Errorf("Invalid service status: %s", service.Status)
		}
	}

	// Verify timestamp and version are set
	if result.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}

	if result.Version == "" {
		t.Error("Version should be set")
	}

	// Verify summary counts add up
	totalCounted := result.Summary.HealthyServices + result.Summary.DegradedServices + result.Summary.UnhealthyServices
	if totalCounted != result.Summary.TotalServices {
		t.Errorf("Summary counts don't add up: %d + %d + %d != %d",
			result.Summary.HealthyServices, result.Summary.DegradedServices,
			result.Summary.UnhealthyServices, result.Summary.TotalServices)
	}
}

// TestCheckServiceIntegration tests the complete CheckService function integration
func TestCheckServiceIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "integration_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create test backup file
	createTestFile(t, tmpDir, fileInfo{name: "test-backup.sql", ageHours: 2, sizeMB: 5})

	serviceConfig := config.ServiceConfig{
		Name:                 "test-service.service",
		BackupPath:           tmpDir,
		MaxAgeHours:          24,
		ExpectedFilePatterns: []string{"test-*.sql"},
		MinFileSizeMB:        1,
	}

	result := CheckService(serviceConfig)

	// Verify structure matches API spec
	if result.Name != "test-service.service" {
		t.Errorf("Expected service name 'test-service.service', got %s", result.Name)
	}

	// Verify systemd info is populated (even if service doesn't exist)
	// The Enabled field should be populated based on systemd check
	_ = result.Systemd.Enabled // Just verify the field exists

	// Verify backup info is populated
	if result.Backup.Path != tmpDir {
		t.Errorf("Expected backup path %s, got %s", tmpDir, result.Backup.Path)
	}

	if result.Backup.RecentFiles == 0 {
		t.Error("Expected to find recent backup files")
	}

	if result.Backup.Status == "" {
		t.Error("Backup status should not be empty")
	}

	// Issues should be initialized (even if empty)
	if result.Issues == nil {
		t.Error("Issues slice should be initialized")
	}
}
