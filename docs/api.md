# API Documentation

The Backup Health Monitor provides a REST API for programmatic access to backup system health information.

## Base URL

```
http://localhost:8080
```

## Authentication

No authentication required. The service is designed to run on localhost or internal networks.

## Content Types

- Requests: Not applicable (GET endpoints only)
- Responses: `text/plain` for simple endpoints, `application/json` for structured data

## Endpoints

### GET /health

Simple health check endpoint compatible with load balancers and monitoring tools.

**Purpose**: Quick health status for uptime monitoring

**Response Codes**:
- `200 OK`: All services healthy or in warning state
- `503 Service Unavailable`: One or more services are critical/unhealthy

**Response Body**: Plain text status message

#### Examples

**Healthy System**:
```http
GET /health HTTP/1.1
Host: localhost:8080

HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
Content-Length: 27

OK - Backup system healthy
```

**System with Warnings**:
```http
GET /health HTTP/1.1
Host: localhost:8080

HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
Content-Length: 34

WARNING - Backup system has issues
```

**Critical System**:
```http
GET /health HTTP/1.1
Host: localhost:8080

HTTP/1.1 503 Service Unavailable
Content-Type: text/plain; charset=utf-8
Content-Length: 36

CRITICAL - Backup system has failures
```

---

### GET /api/v1/status

Comprehensive system status with detailed service information.

**Purpose**: Detailed monitoring and debugging information

**Response Code**: Always `200 OK` (errors included in response body)

**Response Body**: JSON object with complete system status

#### Response Schema

```json
{
  "status": "healthy|warning|critical",
  "timestamp": "2025-09-27T14:30:00Z",
  "version": "1.0.0",
  "services": [
    {
      "name": "service-name.service",
      "status": "healthy|warning|critical",
      "systemd": {
        "last_run": "2025-09-27T04:30:00Z",
        "exit_code": 0,
        "enabled": true,
        "status": "success|failed|never_run|not_enabled|error"
      },
      "backup": {
        "path": "/path/to/backups",
        "recent_files": 3,
        "newest_file_age_hours": 2.5,
        "newest_file_size_mb": 15.2,
        "status": "healthy|warning|critical"
      },
      "issues": [
        "Human-readable issue description"
      ],
      "error": "Error message if applicable"
    }
  ],
  "summary": {
    "total_services": 3,
    "healthy_services": 2,
    "warning_services": 1,
    "critical_services": 0
  }
}
```

#### Examples

**Healthy System**:
```http
GET /api/v1/status HTTP/1.1
Host: localhost:8080

HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "status": "healthy",
  "timestamp": "2025-09-27T14:30:00Z",
  "version": "1.0.0",
  "services": [
    {
      "name": "vault-backup.service",
      "status": "healthy",
      "systemd": {
        "last_run": "2025-09-27T04:30:00Z",
        "exit_code": 0,
        "enabled": true,
        "status": "success"
      },
      "backup": {
        "path": "/opt/backups/vault",
        "recent_files": 3,
        "newest_file_age_hours": 10.2,
        "newest_file_size_mb": 25.7,
        "status": "healthy"
      },
      "issues": []
    }
  ],
  "summary": {
    "total_services": 1,
    "healthy_services": 1,
    "warning_services": 0,
    "critical_services": 0
  }
}
```

**System with Issues**:
```http
GET /api/v1/status HTTP/1.1
Host: localhost:8080

HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "status": "warning",
  "timestamp": "2025-09-27T14:30:00Z",
  "version": "1.0.0",
  "services": [
    {
      "name": "postgres-backup.service",
      "status": "warning",
      "systemd": {
        "last_run": "2025-09-27T04:30:00Z",
        "exit_code": 0,
        "enabled": true,
        "status": "success"
      },
      "backup": {
        "path": "/opt/backups/postgres",
        "recent_files": 1,
        "newest_file_age_hours": 2.5,
        "newest_file_size_mb": 2.1,
        "status": "warning"
      },
      "issues": [
        "Newest backup file is too small: 2.1 MB (minimum: 5 MB)"
      ]
    }
  ],
  "summary": {
    "total_services": 1,
    "healthy_services": 0,
    "warning_services": 1,
    "critical_services": 0
  }
}
```

**Critical System**:
```http
GET /api/v1/status HTTP/1.1
Host: localhost:8080

HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "status": "critical",
  "timestamp": "2025-09-27T14:30:00Z",
  "version": "1.0.0",
  "services": [
    {
      "name": "minio-backup.service",
      "status": "critical",
      "systemd": {
        "last_run": "2025-09-27T04:30:00Z",
        "exit_code": 1,
        "enabled": true,
        "status": "failed"
      },
      "backup": {
        "path": "/opt/backups/minio",
        "recent_files": 0,
        "newest_file_age_hours": 72.5,
        "newest_file_size_mb": 0,
        "status": "critical"
      },
      "issues": [
        "Systemd service failed with exit code 1",
        "No recent backup files found (max age: 48 hours)"
      ]
    }
  ],
  "summary": {
    "total_services": 1,
    "healthy_services": 0,
    "warning_services": 0,
    "critical_services": 1
  }
}
```

---

### GET /api/v1/services/{service_name}

Individual service status for detailed troubleshooting.

**Purpose**: Focus on specific service issues

**Parameters**:
- `service_name` (path): Name of the systemd service (e.g., "vault-backup.service")

**Response Codes**:
- `200 OK`: Service found
- `404 Not Found`: Service not configured

**Response Body**: JSON object with single service details

#### Examples

**Valid Service**:
```http
GET /api/v1/services/vault-backup.service HTTP/1.1
Host: localhost:8080

HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8

{
  "name": "vault-backup.service",
  "status": "healthy",
  "systemd": {
    "last_run": "2025-09-27T04:30:00Z",
    "exit_code": 0,
    "enabled": true,
    "status": "success"
  },
  "backup": {
    "path": "/opt/backups/vault",
    "recent_files": 3,
    "newest_file_age_hours": 10.2,
    "newest_file_size_mb": 25.7,
    "status": "healthy"
  },
  "issues": []
}
```

**Service Not Found**:
```http
GET /api/v1/services/nonexistent.service HTTP/1.1
Host: localhost:8080

HTTP/1.1 404 Not Found
Content-Type: application/json; charset=utf-8

{
  "error": "Service not found",
  "service": "nonexistent.service"
}
```

**URL Encoding Example**:
```http
GET /api/v1/services/my%2Dbackup%40service.service HTTP/1.1
Host: localhost:8080
```

---

### GET /metrics

Prometheus-compatible metrics endpoint.

**Purpose**: Integration with Prometheus monitoring

**Response Code**: Always `200 OK`

**Response Body**: Prometheus text format metrics

#### Metrics Exposed

| Metric | Type | Description | Labels |
|--------|------|-------------|---------|
| `backup_service_healthy` | Gauge | Service health status (1=healthy, 0=unhealthy) | `service` |
| `backup_service_last_run_timestamp` | Gauge | Unix timestamp of last service run | `service` |
| `backup_service_exit_code` | Gauge | Last exit code of service | `service` |
| `backup_file_count` | Gauge | Number of recent backup files | `service` |
| `backup_file_age_hours` | Gauge | Age of newest backup file in hours | `service` |
| `backup_file_size_mb` | Gauge | Size of newest backup file in MB | `service` |
| `backup_monitor_checks_total` | Counter | Total number of health checks performed | |
| `backup_monitor_check_duration_seconds` | Histogram | Duration of health check operations | |

#### Example

```http
GET /metrics HTTP/1.1
Host: localhost:8080

HTTP/1.1 200 OK
Content-Type: text/plain; version=0.0.4; charset=utf-8

# HELP backup_service_healthy Service health status (1=healthy, 0=unhealthy)
# TYPE backup_service_healthy gauge
backup_service_healthy{service="vault-backup.service"} 1
backup_service_healthy{service="postgres-backup.service"} 0

# HELP backup_service_last_run_timestamp Unix timestamp of last service run
# TYPE backup_service_last_run_timestamp gauge
backup_service_last_run_timestamp{service="vault-backup.service"} 1727332200
backup_service_last_run_timestamp{service="postgres-backup.service"} 1727328600

# HELP backup_service_exit_code Last exit code of service
# TYPE backup_service_exit_code gauge
backup_service_exit_code{service="vault-backup.service"} 0
backup_service_exit_code{service="postgres-backup.service"} 1

# HELP backup_file_count Number of recent backup files
# TYPE backup_file_count gauge
backup_file_count{service="vault-backup.service"} 3
backup_file_count{service="postgres-backup.service"} 0

# HELP backup_file_age_hours Age of newest backup file in hours
# TYPE backup_file_age_hours gauge
backup_file_age_hours{service="vault-backup.service"} 2.5
backup_file_age_hours{service="postgres-backup.service"} 72.1

# HELP backup_file_size_mb Size of newest backup file in MB
# TYPE backup_file_size_mb gauge
backup_file_size_mb{service="vault-backup.service"} 15.2
backup_file_size_mb{service="postgres-backup.service"} 0

# HELP backup_monitor_checks_total Total number of health checks performed
# TYPE backup_monitor_checks_total counter
backup_monitor_checks_total 1234

# HELP backup_monitor_check_duration_seconds Duration of health check operations
# TYPE backup_monitor_check_duration_seconds histogram
backup_monitor_check_duration_seconds_bucket{le="0.01"} 45
backup_monitor_check_duration_seconds_bucket{le="0.05"} 1150
backup_monitor_check_duration_seconds_bucket{le="0.1"} 1200
backup_monitor_check_duration_seconds_bucket{le="0.5"} 1234
backup_monitor_check_duration_seconds_bucket{le="1"} 1234
backup_monitor_check_duration_seconds_bucket{le="+Inf"} 1234
backup_monitor_check_duration_seconds_sum 52.3
backup_monitor_check_duration_seconds_count 1234
```

## Status Codes Summary

| Endpoint | Success | Error Conditions |
|----------|---------|------------------|
| `/health` | 200 (healthy/warning), 503 (critical) | N/A |
| `/api/v1/status` | 200 | N/A |
| `/api/v1/services/{name}` | 200 | 404 (service not found) |
| `/metrics` | 200 | N/A |

## Client Examples

### curl

```bash
# Simple health check
curl -s http://localhost:8080/health

# Detailed status
curl -s http://localhost:8080/api/v1/status | jq .

# Specific service
curl -s http://localhost:8080/api/v1/services/vault-backup.service | jq .

# Metrics
curl -s http://localhost:8080/metrics
```

### Python

```python
import requests

# Health check
response = requests.get('http://localhost:8080/health')
print(f"Status: {response.status_code}, Body: {response.text}")

# Detailed status
response = requests.get('http://localhost:8080/api/v1/status')
data = response.json()
print(f"Overall status: {data['status']}")
for service in data['services']:
    print(f"Service {service['name']}: {service['status']}")
```

### Monitoring Integration

#### Gatus Configuration

```yaml
endpoints:
  - name: backup-health
    url: "http://localhost:8080/health"
    interval: 5m
    conditions:
      - "[STATUS] == 200"

  - name: backup-api
    url: "http://localhost:8080/api/v1/status"
    interval: 5m
    conditions:
      - "[STATUS] == 200"
      - "[BODY].status != 'critical'"
```

#### Prometheus Alerting

```yaml
groups:
  - name: backup-health
    rules:
      - alert: BackupServiceUnhealthy
        expr: backup_service_healthy == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Backup service {{ $labels.service }} is unhealthy"

      - alert: BackupFileTooOld
        expr: backup_file_age_hours > 48
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Backup files for {{ $labels.service }} are older than 48 hours"
```