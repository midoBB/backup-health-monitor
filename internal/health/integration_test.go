package health

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"backup-health-monitor/internal/config"
)

// TestBackupFileCheckingIntegration tests the backup file checking against expected bash script behavior
func TestBackupFileCheckingIntegration(t *testing.T) {
	// Create temporary backup directories that mimic real backup structure
	tmpDir, err := os.MkdirTemp("", "integration_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Test scenario 1: Healthy vault backup service
	vaultBackupDir := filepath.Join(tmpDir, "vault-backups")
	err = os.MkdirAll(vaultBackupDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create vault backup dir: %v", err)
	}

	// Create recent vault backup files
	createTestFile(t, vaultBackupDir, fileInfo{
		name:     "vault-backup-2023-12-01-04-30.tar",
		ageHours: 2,
		sizeMB:   15.5,
	})
	// Create an old vault backup file (outside 36h window)
	createTestFile(t, vaultBackupDir, fileInfo{
		name:     "vault-backup-2023-11-30-04-30.tar",
		ageHours: 48, // Older than 36 hours
		sizeMB:   14.2,
	})

	vaultConfig := config.ServiceConfig{
		Name:                 "vault-backup.service",
		BackupPath:           vaultBackupDir,
		MaxAgeHours:          36,
		ExpectedFilePatterns: []string{"vault-backup-*"},
		MinFileSizeMB:        1,
	}

	vaultResult := CheckBackupFiles(vaultConfig)

	if vaultResult.Status != "healthy" {
		t.Errorf("Expected vault backup to be healthy, got %s with issues: %v", vaultResult.Status, vaultResult.Issues)
	}

	if vaultResult.RecentFiles != 1 {
		t.Errorf("Expected 1 recent vault backup file, got %d", vaultResult.RecentFiles)
	}

	// Test scenario 2: Postgres backup with file size issues
	postgresBackupDir := filepath.Join(tmpDir, "postgres-backups")
	err = os.MkdirAll(postgresBackupDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create postgres backup dir: %v", err)
	}

	// Create a recent but too small backup file
	createTestFile(t, postgresBackupDir, fileInfo{
		name:     "postgres-dump.sql.gz",
		ageHours: 12,
		sizeMB:   0.5, // Too small
	})

	postgresConfig := config.ServiceConfig{
		Name:                 "postgres-backup.service",
		BackupPath:           postgresBackupDir,
		MaxAgeHours:          36,
		ExpectedFilePatterns: []string{"*.sql.gz", "*.sql"},
		MinFileSizeMB:        5, // Minimum 5MB
	}

	postgresResult := CheckBackupFiles(postgresConfig)

	if postgresResult.Status != "warning" {
		t.Errorf("Expected postgres backup to have warning due to file size, got %s", postgresResult.Status)
	}

	if len(postgresResult.Issues) == 0 {
		t.Error("Expected postgres backup to have issues due to file size")
	}

	// Test scenario 3: No recent backups (mimics bash script behavior)
	minioBackupDir := filepath.Join(tmpDir, "minio-backups")
	err = os.MkdirAll(minioBackupDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create minio backup dir: %v", err)
	}

	// Create only old backup files
	createTestFile(t, minioBackupDir, fileInfo{
		name:     "minio-backup-old.tar.gz",
		ageHours: 48, // Older than 36 hours
		sizeMB:   20,
	})

	minioConfig := config.ServiceConfig{
		Name:                 "minio-backup.service",
		BackupPath:           minioBackupDir,
		MaxAgeHours:          36,
		ExpectedFilePatterns: []string{"*.tar.gz"},
		MinFileSizeMB:        10,
	}

	minioResult := CheckBackupFiles(minioConfig)

	if minioResult.Status != "warning" {
		t.Errorf("Expected minio backup to have warning due to no recent files, got %s", minioResult.Status)
	}

	if minioResult.RecentFiles != 0 {
		t.Errorf("Expected 0 recent minio backup files, got %d", minioResult.RecentFiles)
	}

	// Test scenario 4: Missing backup directory (graceful degradation)
	missingConfig := config.ServiceConfig{
		Name:                 "missing-backup.service",
		BackupPath:           "/path/that/does/not/exist",
		MaxAgeHours:          36,
		ExpectedFilePatterns: []string{"*.backup"},
		MinFileSizeMB:        1,
	}

	missingResult := CheckBackupFiles(missingConfig)

	if missingResult.Status != "warning" {
		t.Errorf("Expected missing backup directory to have warning, got %s", missingResult.Status)
	}

	t.Logf("Integration test results:")
	t.Logf("  Vault: %s, %d recent files, %.2f hours age, %.2f MB",
		vaultResult.Status, vaultResult.RecentFiles, vaultResult.NewestFileAge, vaultResult.NewestFileSizeMB)
	t.Logf("  Postgres: %s, %d recent files, issues: %v",
		postgresResult.Status, postgresResult.RecentFiles, postgresResult.Issues)
	t.Logf("  Minio: %s, %d recent files",
		minioResult.Status, minioResult.RecentFiles)
	t.Logf("  Missing: %s, issues: %v",
		missingResult.Status, missingResult.Issues)
}

// TestBashScriptCompatibility verifies our logic matches the bash script's find command
func TestBashScriptCompatibility(t *testing.T) {
	// Create test directory structure
	tmpDir, err := os.MkdirTemp("", "bash_compat_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	}()

	// Create files with various ages to test the bash script's "find -mtime -1.5" equivalent
	testFiles := []fileInfo{
		{name: "file_2_hours.backup", ageHours: 2, sizeMB: 1},    // Recent
		{name: "file_12_hours.backup", ageHours: 12, sizeMB: 1},  // Recent
		{name: "file_30_hours.backup", ageHours: 30, sizeMB: 1},  // Recent (within 36h)
		{name: "file_40_hours.backup", ageHours: 40, sizeMB: 1},  // Old (outside 36h)
		{name: "file_50_hours.backup", ageHours: 50, sizeMB: 1},  // Old
	}

	for _, fileInfo := range testFiles {
		createTestFile(t, tmpDir, fileInfo)
	}

	// Test with 36-hour cutoff (like our config)
	cutoff := time.Now().Add(-36 * time.Hour)
	count, _ := countRecentFiles(tmpDir, cutoff)

	// Should find files that are 2, 12, and 30 hours old (3 files)
	expectedCount := 3
	if count != expectedCount {
		t.Errorf("Expected %d recent files (within 36h), got %d", expectedCount, count)
	}

	// Test with bash script's 1.5-day cutoff (36 hours)
	bashCutoff := time.Now().Add(-time.Duration(1.5*24) * time.Hour)
	bashCount, _ := countRecentFiles(tmpDir, bashCutoff)

	if bashCount != count {
		t.Errorf("Bash compatibility test failed: 36h cutoff (%d) != bash 1.5d cutoff (%d)", count, bashCount)
	}

	t.Logf("Bash compatibility test: found %d files within 36 hours (matching bash script logic)", count)
}