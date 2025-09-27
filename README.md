# Backup Health Monitor

A Go-based service that monitors backup system health by checking systemd services and backup file timestamps. Provides HTTP endpoints for integration with monitoring tools like Gatus.

## Quick Start

```bash
# Run one-time health check
./backup-health-monitor check

# Start HTTP server
./backup-health-monitor serve --port 8080

# Validate configuration
./backup-health-monitor validate
```

## Configuration

Create `config.yaml`:

```yaml
services:
  - name: "vault-backup.service"
    backup_path: "/home/deploy/.local/share/vault-backups"
    max_age_hours: 36
    min_file_size_mb: 1

  - name: "postgres-backup.service"
    backup_path: "/opt/backups/postgres"
    max_age_hours: 36
    min_file_size_mb: 5

server:
  port: 8080
  bind_address: "127.0.0.1"
```

## API Endpoints

- `GET /health` - Simple health check (200/503 status)
- `GET /api/v1/status` - Detailed JSON status
- `GET /metrics` - Prometheus metrics

## Installation

```bash
# Build from source
go build -o backup-health-monitor cmd/backup-health-monitor/main.go

# Install
sudo cp backup-health-monitor /usr/local/bin/
sudo cp config.yaml /etc/backup-health/
```

## Health Status

- **Healthy**: Service succeeded + recent backup files present
- **Warning**: Service succeeded but backup issues detected
- **Critical**: Service failed or major backup problems
