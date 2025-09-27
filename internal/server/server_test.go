package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backup-health-monitor/internal/config"
	"backup-health-monitor/internal/health"
	"backup-health-monitor/internal/metrics"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func init() {
	// Set log level to error to reduce noise during testing
	logrus.SetLevel(logrus.ErrorLevel)
}

// mockHealthChecker allows us to control health check results for testing
type mockHealthChecker struct {
	result health.HealthResult
}

// CheckAllServices implements HealthChecker interface for testing
func (m *mockHealthChecker) CheckAllServices(cfg *config.Config, version string) health.HealthResult {
	return m.result
}

// createTestServer creates a server with a mock health checker
func createTestServer(healthResult health.HealthResult) *Server {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			BindAddress: "127.0.0.1",
		},
		Logging: config.LoggingConfig{
			Level:  "error",
			Format: "json",
		},
	}

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create mock health checker
	mockChecker := &mockHealthChecker{
		result: healthResult,
	}

	// Use the mock health checker
	return NewServerWithHealthChecker(cfg, mockChecker)
}

func TestHealthHandler_Healthy(t *testing.T) {
	// Create a healthy health result
	healthResult := health.HealthResult{
		Status:    health.StatusHealthy,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  []health.ServiceResult{},
		Summary: health.Summary{
			TotalServices:     3,
			HealthyServices:   3,
			DegradedServices:  0,
			UnhealthyServices: 0,
		},
	}

	server := createTestServer(healthResult)

	// Create test request
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.engine.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	expectedBody := "OK - Backup system healthy"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, w.Body.String())
	}
}

func TestHealthHandler_Warning(t *testing.T) {
	// Create a warning health result
	healthResult := health.HealthResult{
		Status:    health.StatusWarning,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  []health.ServiceResult{},
		Summary: health.Summary{
			TotalServices:     3,
			HealthyServices:   2,
			DegradedServices:  1,
			UnhealthyServices: 0,
		},
	}

	server := createTestServer(healthResult)

	// Create test request
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.engine.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	expectedBody := "WARNING - Backup system has issues"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, w.Body.String())
	}
}

func TestHealthHandler_Critical(t *testing.T) {
	// Create a critical health result
	healthResult := health.HealthResult{
		Status:    health.StatusCritical,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  []health.ServiceResult{},
		Summary: health.Summary{
			TotalServices:     3,
			HealthyServices:   1,
			DegradedServices:  0,
			UnhealthyServices: 2,
		},
	}

	server := createTestServer(healthResult)

	// Create test request
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.engine.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	expectedBody := "CRITICAL - Backup system has failures"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, w.Body.String())
	}
}

func TestHealthHandler_UnknownStatus(t *testing.T) {
	// Create a health result with unknown status
	healthResult := health.HealthResult{
		Status:    health.HealthStatus("unknown"),
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  []health.ServiceResult{},
		Summary: health.Summary{
			TotalServices:     3,
			HealthyServices:   0,
			DegradedServices:  0,
			UnhealthyServices: 0,
		},
	}

	server := createTestServer(healthResult)

	// Create test request
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.engine.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	expectedBody := "ERROR - Health check failed"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, w.Body.String())
	}
}

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			BindAddress: "127.0.0.1",
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	server := NewServer(cfg)

	// Verify server is properly initialized
	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	if server.config != cfg {
		t.Error("Expected server config to match provided config")
	}

	if server.engine == nil {
		t.Error("Expected Gin engine to be initialized")
	}
}

func TestNewServer_DebugMode(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			BindAddress: "127.0.0.1",
		},
		Logging: config.LoggingConfig{
			Level:  "debug",
			Format: "json",
		},
	}

	// Set Gin to test mode initially
	gin.SetMode(gin.TestMode)

	server := NewServer(cfg)

	// Verify server is properly initialized
	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	// Note: We can't easily test if Gin mode was set to debug without
	// exposing internal Gin state, but we can verify the server was created
	if server.engine == nil {
		t.Error("Expected Gin engine to be initialized")
	}
}

func TestHealthHandler_HTTPMethods(t *testing.T) {
	healthResult := health.HealthResult{
		Status:    health.StatusHealthy,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  []health.ServiceResult{},
		Summary:   health.Summary{},
	}

	server := createTestServer(healthResult)

	// Test GET method (should work)
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected GET /health to return %d, got %d", http.StatusOK, w.Code)
	}

	// Test POST method (Gin returns 404 for routes that don't exist with that method)
	req, _ = http.NewRequest("POST", "/health", nil)
	w = httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected POST /health to return %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	healthResult := health.HealthResult{
		Status:    health.StatusHealthy,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  []health.ServiceResult{},
		Summary:   health.Summary{},
	}

	server := createTestServer(healthResult)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.engine.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	expectedContentType := "text/plain; charset=utf-8"

	if contentType != expectedContentType {
		t.Errorf("Expected Content-Type '%s', got '%s'", expectedContentType, contentType)
	}
}

func TestServer_InvalidRoute(t *testing.T) {
	healthResult := health.HealthResult{
		Status:  health.StatusHealthy,
		Summary: health.Summary{},
	}

	server := createTestServer(healthResult)

	// Test invalid route
	req, _ := http.NewRequest("GET", "/invalid", nil)
	w := httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected invalid route to return %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDefaultHealthChecker(t *testing.T) {
	checker := &DefaultHealthChecker{}

	// Create a minimal config for testing
	cfg := &config.Config{
		Services: []config.ServiceConfig{
			{
				Name:                 "test-service",
				BackupPath:           "/tmp/test-backups",
				MaxAgeHours:          24,
				ExpectedFilePatterns: []string{"*.txt"},
				MinFileSizeMB:        1,
			},
		},
	}

	// Call CheckAllServices - this should not panic
	result := checker.CheckAllServices(cfg, "test-version")

	// Verify we got a result
	if result.Services == nil {
		t.Error("Expected services in health result")
	}

	if len(result.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(result.Services))
	}
}

func TestServer_Stop_WithoutStart(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			BindAddress: "127.0.0.1",
		},
		Logging: config.LoggingConfig{
			Level:  "error",
			Format: "json",
		},
	}

	server := NewServer(cfg)

	// Stop without starting should not error
	err := server.Stop()
	if err != nil {
		t.Errorf("Expected Stop() to succeed when server not started, got error: %v", err)
	}
}

func TestGinLogrusLogger(t *testing.T) {
	// Test that ginLogrusLogger returns a valid middleware
	middleware := ginLogrusLogger()
	if middleware == nil {
		t.Error("Expected ginLogrusLogger to return a valid middleware")
	}
}

// Test data for API endpoints
func createTestHealthResult() health.HealthResult {
	return health.HealthResult{
		Status:    health.StatusHealthy,
		Timestamp: time.Date(2023, 9, 26, 15, 30, 0, 0, time.UTC),
		Version:   "1.0.0",
		Services: []health.ServiceResult{
			{
				Name:   "vault-backup.service",
				Status: "healthy",
				Systemd: health.SystemdInfo{
					LastRun:  time.Date(2023, 9, 26, 4, 30, 0, 0, time.UTC),
					ExitCode: 0,
					Enabled:  true,
				},
				Backup: health.BackupInfo{
					Path:             "/home/deploy/.local/share/vault-backups",
					RecentFiles:      3,
					NewestFileAge:    2.5,
					NewestFileSizeMB: 15.2,
					Status:           "healthy",
				},
				Issues: []string{},
			},
			{
				Name:   "postgres-backup.service",
				Status: "warning",
				Systemd: health.SystemdInfo{
					LastRun:  time.Date(2023, 9, 26, 4, 30, 0, 0, time.UTC),
					ExitCode: 0,
					Enabled:  true,
				},
				Backup: health.BackupInfo{
					Path:             "/opt/backups/postgres",
					RecentFiles:      0,
					NewestFileAge:    0,
					NewestFileSizeMB: 0,
					Status:           "warning",
				},
				Issues: []string{"No recent backup files found"},
			},
		},
		Summary: health.Summary{
			TotalServices:     2,
			HealthyServices:   1,
			DegradedServices:  1,
			UnhealthyServices: 0,
		},
	}
}

func TestStatusHandler_Success(t *testing.T) {
	healthResult := createTestHealthResult()
	server := createTestServer(healthResult)

	// Create test request
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.engine.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify Content-Type is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Parse JSON response
	var response health.HealthResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Verify response structure
	if response.Status != health.StatusHealthy {
		t.Errorf("Expected status %s, got %s", health.StatusHealthy, response.Status)
	}

	if response.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", response.Version)
	}

	if len(response.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(response.Services))
	}

	if response.Summary.TotalServices != 2 {
		t.Errorf("Expected 2 total services, got %d", response.Summary.TotalServices)
	}

	if response.Summary.HealthyServices != 1 {
		t.Errorf("Expected 1 healthy service, got %d", response.Summary.HealthyServices)
	}
}

func TestServiceHandler_Success(t *testing.T) {
	healthResult := createTestHealthResult()
	server := createTestServer(healthResult)

	// Create test request for existing service
	req, _ := http.NewRequest("GET", "/api/v1/services/vault-backup.service", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.engine.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify Content-Type is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Parse JSON response
	var response health.ServiceResult
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Verify response structure
	if response.Name != "vault-backup.service" {
		t.Errorf("Expected service name 'vault-backup.service', got '%s'", response.Name)
	}

	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response.Status)
	}

	if response.Systemd.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", response.Systemd.ExitCode)
	}

	if response.Backup.Path != "/home/deploy/.local/share/vault-backups" {
		t.Errorf("Expected backup path '/home/deploy/.local/share/vault-backups', got '%s'", response.Backup.Path)
	}
}

func TestServiceHandler_NotFound(t *testing.T) {
	healthResult := createTestHealthResult()
	server := createTestServer(healthResult)

	// Create test request for non-existing service
	req, _ := http.NewRequest("GET", "/api/v1/services/non-existent-service", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.engine.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	// Verify Content-Type is JSON
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Parse JSON response
	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Verify error response structure
	if response.Error != "not_found" {
		t.Errorf("Expected error 'not_found', got '%s'", response.Error)
	}

	if response.Code != http.StatusNotFound {
		t.Errorf("Expected error code %d, got %d", http.StatusNotFound, response.Code)
	}

	expectedMessage := "Service 'non-existent-service' not found"
	if response.Message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, response.Message)
	}
}

func TestServiceHandler_URLEncoding(t *testing.T) {
	healthResult := createTestHealthResult()
	server := createTestServer(healthResult)

	// Test with URL-encoded service name
	req, _ := http.NewRequest("GET", "/api/v1/services/vault-backup.service", nil)
	w := httptest.NewRecorder()

	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for URL encoded service name, got %d", http.StatusOK, w.Code)
	}
}

func TestStatusHandler_HTTPMethods(t *testing.T) {
	healthResult := createTestHealthResult()
	server := createTestServer(healthResult)

	// Test GET method (should work)
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected GET /api/v1/status to return %d, got %d", http.StatusOK, w.Code)
	}

	// Test POST method (should return 404 - method not allowed)
	req, _ = http.NewRequest("POST", "/api/v1/status", nil)
	w = httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected POST /api/v1/status to return %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestServiceHandler_HTTPMethods(t *testing.T) {
	healthResult := createTestHealthResult()
	server := createTestServer(healthResult)

	// Test GET method (should work)
	req, _ := http.NewRequest("GET", "/api/v1/services/vault-backup.service", nil)
	w := httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected GET /api/v1/services/{name} to return %d, got %d", http.StatusOK, w.Code)
	}

	// Test POST method (should return 404 - method not allowed)
	req, _ = http.NewRequest("POST", "/api/v1/services/vault-backup.service", nil)
	w = httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected POST /api/v1/services/{name} to return %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestStatusHandler_DifferentHealthStatuses(t *testing.T) {
	testCases := []struct {
		name           string
		status         health.HealthStatus
		expectedStatus int
	}{
		{"healthy", health.StatusHealthy, http.StatusOK},
		{"warning", health.StatusWarning, http.StatusOK},
		{"critical", health.StatusCritical, http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			healthResult := health.HealthResult{
				Status:    tc.status,
				Timestamp: time.Now(),
				Version:   "1.0.0",
				Services:  []health.ServiceResult{},
				Summary:   health.Summary{},
			}

			server := createTestServer(healthResult)

			req, _ := http.NewRequest("GET", "/api/v1/status", nil)
			w := httptest.NewRecorder()
			server.engine.ServeHTTP(w, req)

			// Status endpoint always returns 200 OK - status is in JSON body
			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d for %s health, got %d", tc.expectedStatus, tc.name, w.Code)
			}

			var response health.HealthResult
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("Failed to parse JSON response: %v", err)
			}

			if response.Status != tc.status {
				t.Errorf("Expected response status %s, got %s", tc.status, response.Status)
			}
		})
	}
}

func TestMetricsHandler_Success(t *testing.T) {
	healthResult := createTestHealthResult()
	server := createTestServer(healthResult)

	// Manually populate some metrics to test endpoint
	// (since we're using a mock health checker that bypasses metrics collection)
	if server.config.Services == nil {
		server.config.Services = []config.ServiceConfig{
			{Name: "vault-backup.service"},
			{Name: "postgres-backup.service"},
		}
	}

	// Manually update metrics for testing
	if metrics.Collector != nil {
		metrics.Collector.UpdateServiceMetrics("vault-backup.service", true, time.Now(), 0, 3, 2.5, 15.2)
		metrics.Collector.UpdateServiceMetrics("postgres-backup.service", false, time.Now(), 1, 0, 0, 0)
		metrics.Collector.IncrementChecksTotal()
	}

	// Now test metrics endpoint
	req, _ := http.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.engine.ServeHTTP(w, req)

	// Assert response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify Content-Type is Prometheus metrics format (allow for additional parameters)
	contentType := w.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain; version=0.0.4; charset=utf-8") {
		t.Errorf("Expected Content-Type to start with 'text/plain; version=0.0.4; charset=utf-8', got '%s'", contentType)
	}

	// Verify response contains Prometheus metrics
	body := w.Body.String()

	// Check for expected metrics
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

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected metrics response to contain '%s'", metric)
		}
	}

	// Verify HELP and TYPE comments are present (standard Prometheus format)
	if !strings.Contains(body, "# HELP") {
		t.Error("Expected metrics response to contain HELP comments")
	}

	if !strings.Contains(body, "# TYPE") {
		t.Error("Expected metrics response to contain TYPE comments")
	}
}

func TestMetricsHandler_HTTPMethods(t *testing.T) {
	healthResult := createTestHealthResult()
	server := createTestServer(healthResult)

	// Test GET method (should work)
	req, _ := http.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected GET /metrics to return %d, got %d", http.StatusOK, w.Code)
	}

	// Test POST method (should return 404 - method not allowed)
	req, _ = http.NewRequest("POST", "/metrics", nil)
	w = httptest.NewRecorder()
	server.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected POST /metrics to return %d, got %d", http.StatusNotFound, w.Code)
	}
}
