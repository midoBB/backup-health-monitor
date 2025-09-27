package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func TestInitMetrics(t *testing.T) {
	// Test that InitMetrics creates all expected metrics
	InitMetrics()

	if Collector == nil {
		t.Fatal("Collector should not be nil after InitMetrics")
	}

	// Verify all metrics are properly initialized
	if Collector.ServiceHealthy == nil {
		t.Error("ServiceHealthy metric not initialized")
	}
	if Collector.ServiceLastRunTime == nil {
		t.Error("ServiceLastRunTime metric not initialized")
	}
	if Collector.ServiceExitCode == nil {
		t.Error("ServiceExitCode metric not initialized")
	}
	if Collector.BackupFileCount == nil {
		t.Error("BackupFileCount metric not initialized")
	}
	if Collector.BackupFileAgeHours == nil {
		t.Error("BackupFileAgeHours metric not initialized")
	}
	if Collector.BackupFileSizeMB == nil {
		t.Error("BackupFileSizeMB metric not initialized")
	}
	if Collector.ChecksTotal == nil {
		t.Error("ChecksTotal metric not initialized")
	}
	if Collector.CheckDurationSeconds == nil {
		t.Error("CheckDurationSeconds metric not initialized")
	}
}

func TestUpdateServiceMetrics(t *testing.T) {
	// Initialize metrics
	InitMetrics()

	// Test data
	serviceName := "test-service"
	lastRun := time.Now()

	// Update metrics
	Collector.UpdateServiceMetrics(
		serviceName,
		true,     // healthy
		lastRun,  // last run
		0,        // exit code
		3,        // file count
		2.5,      // file age hours
		15.2,     // file size MB
	)

	// Verify service healthy metric
	healthyValue := testutil.ToFloat64(Collector.ServiceHealthy.WithLabelValues(serviceName))
	if healthyValue != 1.0 {
		t.Errorf("Expected healthy value 1.0, got %f", healthyValue)
	}

	// Verify last run timestamp
	timestampValue := testutil.ToFloat64(Collector.ServiceLastRunTime.WithLabelValues(serviceName))
	expectedTimestamp := float64(lastRun.Unix())
	if timestampValue != expectedTimestamp {
		t.Errorf("Expected timestamp %f, got %f", expectedTimestamp, timestampValue)
	}

	// Verify exit code
	exitCodeValue := testutil.ToFloat64(Collector.ServiceExitCode.WithLabelValues(serviceName))
	if exitCodeValue != 0.0 {
		t.Errorf("Expected exit code 0.0, got %f", exitCodeValue)
	}

	// Verify backup file count
	fileCountValue := testutil.ToFloat64(Collector.BackupFileCount.WithLabelValues(serviceName))
	if fileCountValue != 3.0 {
		t.Errorf("Expected file count 3.0, got %f", fileCountValue)
	}

	// Verify backup file age
	fileAgeValue := testutil.ToFloat64(Collector.BackupFileAgeHours.WithLabelValues(serviceName))
	if fileAgeValue != 2.5 {
		t.Errorf("Expected file age 2.5, got %f", fileAgeValue)
	}

	// Verify backup file size
	fileSizeValue := testutil.ToFloat64(Collector.BackupFileSizeMB.WithLabelValues(serviceName))
	if fileSizeValue != 15.2 {
		t.Errorf("Expected file size 15.2, got %f", fileSizeValue)
	}
}

func TestUpdateServiceMetricsUnhealthy(t *testing.T) {
	// Initialize metrics
	InitMetrics()

	serviceName := "unhealthy-service"

	// Update metrics for unhealthy service
	Collector.UpdateServiceMetrics(
		serviceName,
		false,    // not healthy
		time.Time{}, // zero time (never run)
		1,        // exit code 1 (failure)
		0,        // no files
		0,        // no age
		0,        // no size
	)

	// Verify service healthy metric is 0
	healthyValue := testutil.ToFloat64(Collector.ServiceHealthy.WithLabelValues(serviceName))
	if healthyValue != 0.0 {
		t.Errorf("Expected healthy value 0.0, got %f", healthyValue)
	}

	// Verify exit code
	exitCodeValue := testutil.ToFloat64(Collector.ServiceExitCode.WithLabelValues(serviceName))
	if exitCodeValue != 1.0 {
		t.Errorf("Expected exit code 1.0, got %f", exitCodeValue)
	}
}

func TestIncrementChecksTotal(t *testing.T) {
	// Initialize metrics
	InitMetrics()

	// Get initial value
	initialValue := testutil.ToFloat64(Collector.ChecksTotal)

	// Increment counter
	Collector.IncrementChecksTotal()
	Collector.IncrementChecksTotal()

	// Verify counter increased by 2
	finalValue := testutil.ToFloat64(Collector.ChecksTotal)
	if finalValue != initialValue+2 {
		t.Errorf("Expected checks total to increase by 2, got %f", finalValue-initialValue)
	}
}

func TestObserveCheckDuration(t *testing.T) {
	// Initialize metrics
	InitMetrics()

	// Record durations
	duration1 := 100 * time.Millisecond
	duration2 := 200 * time.Millisecond

	Collector.ObserveCheckDuration(duration1)
	Collector.ObserveCheckDuration(duration2)

	// Verify histogram has recorded observations
	// We can't easily test exact values, but we can verify the count increased
	metric := &dto.Metric{}
	if err := Collector.CheckDurationSeconds.(prometheus.Metric).Write(metric); err != nil {
		t.Fatalf("failed to write metric: %v", err)
	}

	// The histogram should have recorded 2 observations
	if metric.GetHistogram().GetSampleCount() != 2 {
		t.Errorf("Expected 2 histogram observations, got %d", metric.GetHistogram().GetSampleCount())
	}
}

func TestMetricsPrometheusFormat(t *testing.T) {
	// Initialize metrics
	InitMetrics()

	// Update some metrics
	Collector.UpdateServiceMetrics("test-service", true, time.Now(), 0, 3, 2.5, 15.2)
	Collector.IncrementChecksTotal()

	// Get the registry and gather metrics
	registry := prometheus.DefaultGatherer
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// Check that we have the expected metrics
	expectedMetrics := []string{
		"backup_service_healthy",
		"backup_service_last_run_timestamp",
		"backup_service_exit_code",
		"backup_file_count",
		"backup_file_age_hours",
		"backup_file_size_mb",
		"backup_monitor_checks_total",
		"backup_monitor_check_duration_seconds",
	}

	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamilies {
		for _, expected := range expectedMetrics {
			if strings.Contains(mf.GetName(), expected) {
				foundMetrics[expected] = true
			}
		}
	}

	// Verify all expected metrics are present
	for _, expected := range expectedMetrics {
		if !foundMetrics[expected] {
			t.Errorf("Expected metric %s not found in gathered metrics", expected)
		}
	}
}

func TestGetRegistry(t *testing.T) {
	registry := GetRegistry()
	if registry == nil {
		t.Error("GetRegistry should not return nil")
	}

	// Verify it's actually the default registry
	if registry != prometheus.DefaultRegisterer.(*prometheus.Registry) {
		t.Error("GetRegistry should return the default Prometheus registry")
	}
}