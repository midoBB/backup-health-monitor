package server

import (
	"backup-health-monitor/internal/config"
	"backup-health-monitor/internal/health"
	"backup-health-monitor/internal/metrics"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// HealthChecker interface for dependency injection in tests
type HealthChecker interface {
	CheckAllServices(cfg *config.Config, version string) health.HealthResult
}

// ErrorResponse represents a standard JSON error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// DefaultHealthChecker implements HealthChecker using the health package
type DefaultHealthChecker struct{}

// CheckAllServices implements HealthChecker interface
func (d *DefaultHealthChecker) CheckAllServices(
	cfg *config.Config,
	version string,
) health.HealthResult {
	return health.CheckAllServices(cfg, version)
}

// Server represents the HTTP server
type Server struct {
	config        *config.Config
	engine        *gin.Engine
	httpServer    *http.Server
	healthChecker HealthChecker
	version       string
}

// NewServer creates a new HTTP server instance
func NewServer(cfg *config.Config) *Server {
	return NewServerWithHealthChecker(cfg, &DefaultHealthChecker{})
}

// NewServerWithHealthChecker creates a new HTTP server instance with custom health checker
func NewServerWithHealthChecker(cfg *config.Config, healthChecker HealthChecker) *Server {
	// Initialize Prometheus metrics
	metrics.InitMetrics()

	// Set Gin mode based on log level
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	s := &Server{
		config:        cfg,
		healthChecker: healthChecker,
	}

	// Setup Gin engine with middleware
	s.engine = gin.New()
	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	logrus.Info("Shutting down HTTP server...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := s.httpServer.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Error during server shutdown")
		return err
	}

	logrus.Info("HTTP server stopped gracefully")
	return nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.BindAddress, s.config.Server.Port)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.engine,
		// Set reasonable timeouts
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logrus.WithFields(logrus.Fields{
		"address":      addr,
		"port":         s.config.Server.Port,
		"bind_address": s.config.Server.BindAddress,
	}).Info("Starting HTTP server")

	// Start server in a goroutine since ListenAndServe blocks
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.WithError(err).Error("HTTP server failed to start")
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// setupMiddleware configures Gin middleware
func (s *Server) setupMiddleware() {
	// Use logrus for request logging
	s.engine.Use(ginLogrusLogger())
	// Add recovery middleware
	s.engine.Use(gin.Recovery())
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Main health endpoint - exact bash script behavior
	s.engine.GET("/health", s.healthHandler)

	// Metrics endpoint
	s.engine.GET("/metrics", s.metricsHandler)

	// API v1 routes
	v1 := s.engine.Group("/api/v1")
	{
		v1.GET("/status", s.statusHandler)
		v1.GET("/services/:service_name", s.serviceHandler)
	}
}

// healthHandler handles GET /health requests
// Replicates exact bash script behavior for Gatus compatibility
func (s *Server) healthHandler(c *gin.Context) {
	logrus.Debug("Processing health check request")

	// Perform health check using existing health logic
	result := s.performHealthCheck()

	// Map health status to HTTP response exactly like bash script
	switch result.Status {
	case health.StatusHealthy:
		c.String(http.StatusOK, "OK - Backup system healthy")
	case health.StatusWarning:
		c.String(http.StatusOK, "WARNING - Backup system has issues")
	case health.StatusCritical:
		c.String(http.StatusServiceUnavailable, "CRITICAL - Backup system has failures")
	default:
		logrus.WithField("status", result.Status).Error("Unknown health status")
		c.String(http.StatusInternalServerError, "ERROR - Health check failed")
	}

	logrus.WithFields(logrus.Fields{
		"status":             result.Status,
		"services_total":     result.Summary.TotalServices,
		"services_healthy":   result.Summary.HealthyServices,
		"services_degraded":  result.Summary.DegradedServices,
		"services_unhealthy": result.Summary.UnhealthyServices,
	}).Info("Health check completed")
}

// ginLogrusLogger creates a Gin middleware that logs requests using logrus
func ginLogrusLogger() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			// Log using structured logrus instead of Gin's default formatter
			logrus.WithFields(logrus.Fields{
				"method":     param.Method,
				"path":       param.Path,
				"status":     param.StatusCode,
				"latency":    param.Latency,
				"client_ip":  param.ClientIP,
				"user_agent": param.Request.UserAgent(),
				"error":      param.ErrorMessage,
			}).Info("HTTP request")
			return ""
		},
		Output: logrus.StandardLogger().Writer(),
	})
}

// statusHandler handles GET /api/v1/status requests
// Returns comprehensive status information in JSON format
func (s *Server) statusHandler(c *gin.Context) {
	logrus.Debug("Processing status API request")

	// Perform health check using existing health logic
	result := s.performHealthCheck()

	// Return the complete health result as JSON
	c.JSON(http.StatusOK, result)

	logrus.WithFields(logrus.Fields{
		"status":             result.Status,
		"services_total":     result.Summary.TotalServices,
		"services_healthy":   result.Summary.HealthyServices,
		"services_degraded":  result.Summary.DegradedServices,
		"services_unhealthy": result.Summary.UnhealthyServices,
	}).Info("Status API request completed")
}

// serviceHandler handles GET /api/v1/services/{service_name} requests
// Returns individual service details for troubleshooting
func (s *Server) serviceHandler(c *gin.Context) {
	serviceName := c.Param("service_name")

	logrus.WithField("service_name", serviceName).Debug("Processing individual service API request")

	// Perform health check to get all services
	result := s.performHealthCheck()

	// Find the requested service
	var foundService *health.ServiceResult
	for i := range result.Services {
		if result.Services[i].Name == serviceName {
			foundService = &result.Services[i]
			break
		}
	}

	// Return 404 if service not found
	if foundService == nil {
		errorResponse := ErrorResponse{
			Error:   "not_found",
			Message: fmt.Sprintf("Service '%s' not found", serviceName),
			Code:    http.StatusNotFound,
		}
		c.JSON(http.StatusNotFound, errorResponse)

		logrus.WithField("service_name", serviceName).Warn("Service not found")
		return
	}

	// Return the individual service details
	c.JSON(http.StatusOK, foundService)

	logrus.WithFields(logrus.Fields{
		"service_name":   serviceName,
		"service_status": foundService.Status,
	}).Info("Individual service API request completed")
}

// metricsHandler handles GET /metrics requests
// Serves Prometheus metrics in the standard format
func (s *Server) metricsHandler(c *gin.Context) {
	logrus.Debug("Processing metrics request")

	// Use the Prometheus HTTP handler to serve metrics
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)

	logrus.Debug("Metrics request completed")
}

// performHealthCheck executes health checks and returns results
func (s *Server) performHealthCheck() health.HealthResult {
	return s.healthChecker.CheckAllServices(s.config, s.version)
}
