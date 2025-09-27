package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsCollector holds all Prometheus metrics for the backup health monitor
type MetricsCollector struct {
	// Service-level metrics
	ServiceHealthy         *prometheus.GaugeVec
	ServiceLastRunTime     *prometheus.GaugeVec
	ServiceExitCode        *prometheus.GaugeVec

	// Backup-level metrics
	BackupFileCount        *prometheus.GaugeVec
	BackupFileAgeHours     *prometheus.GaugeVec
	BackupFileSizeMB       *prometheus.GaugeVec

	// Application metrics
	ChecksTotal            prometheus.Counter
	CheckDurationSeconds   prometheus.Histogram
}

// Global metrics collector instance
var Collector *MetricsCollector

// InitMetrics initializes the Prometheus metrics
func InitMetrics() {
	// Prevent double initialization
	if Collector != nil {
		return
	}
	Collector = &MetricsCollector{
		// Service-level metrics
		ServiceHealthy: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "backup_service_healthy",
				Help: "Whether a backup service is healthy (1) or not (0)",
			},
			[]string{"service"},
		),
		ServiceLastRunTime: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "backup_service_last_run_timestamp",
				Help: "Unix timestamp of the last successful run of a backup service",
			},
			[]string{"service"},
		),
		ServiceExitCode: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "backup_service_exit_code",
				Help: "Exit code of the last run of a backup service",
			},
			[]string{"service"},
		),

		// Backup-level metrics
		BackupFileCount: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "backup_file_count",
				Help: "Number of recent backup files found for a service",
			},
			[]string{"service"},
		),
		BackupFileAgeHours: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "backup_file_age_hours",
				Help: "Age of the newest backup file in hours",
			},
			[]string{"service"},
		),
		BackupFileSizeMB: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "backup_file_size_mb",
				Help: "Size of the newest backup file in megabytes",
			},
			[]string{"service"},
		),

		// Application metrics
		ChecksTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "backup_monitor_checks_total",
				Help: "Total number of backup health checks performed",
			},
		),
		CheckDurationSeconds: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name: "backup_monitor_check_duration_seconds",
				Help: "Duration of backup health checks in seconds",
				Buckets: prometheus.DefBuckets,
			},
		),
	}
}

// UpdateServiceMetrics updates all metrics for a single service
func (m *MetricsCollector) UpdateServiceMetrics(serviceName string, healthy bool, lastRun time.Time, exitCode int, fileCount int, fileAgeHours float64, fileSizeMB float64) {
	// Update service health (1 for healthy, 0 for not healthy)
	healthValue := 0.0
	if healthy {
		healthValue = 1.0
	}
	m.ServiceHealthy.WithLabelValues(serviceName).Set(healthValue)

	// Update last run timestamp (Unix timestamp)
	if !lastRun.IsZero() {
		m.ServiceLastRunTime.WithLabelValues(serviceName).Set(float64(lastRun.Unix()))
	}

	// Update exit code
	m.ServiceExitCode.WithLabelValues(serviceName).Set(float64(exitCode))

	// Update backup file metrics
	m.BackupFileCount.WithLabelValues(serviceName).Set(float64(fileCount))
	m.BackupFileAgeHours.WithLabelValues(serviceName).Set(fileAgeHours)
	m.BackupFileSizeMB.WithLabelValues(serviceName).Set(fileSizeMB)
}

// IncrementChecksTotal increments the total checks counter
func (m *MetricsCollector) IncrementChecksTotal() {
	m.ChecksTotal.Inc()
}

// ObserveCheckDuration records the duration of a health check
func (m *MetricsCollector) ObserveCheckDuration(duration time.Duration) {
	m.CheckDurationSeconds.Observe(duration.Seconds())
}

// GetRegistry returns the default Prometheus registry
func GetRegistry() *prometheus.Registry {
	return prometheus.DefaultRegisterer.(*prometheus.Registry)
}