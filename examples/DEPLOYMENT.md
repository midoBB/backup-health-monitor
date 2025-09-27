# Deployment Guide

This guide covers deploying the Backup Health Monitor as a systemd service.

## Prerequisites

- Go 1.19+ (for building from source)
- systemd (for service management)
- Access to systemd D-Bus (typically requires running as `deploy` user or similar)

## Installation Steps

### 1. Build the Binary

```bash
# Clone and build
git clone <repository>
cd backup-health-monitor
make build

# Or download pre-built binary from releases
```

### 2. Install Binary

```bash
# Copy binary to system location
sudo cp build/backup-health-monitor /usr/local/bin/
sudo chmod +x /usr/local/bin/backup-health-monitor
```

### 3. Create Configuration Directory

```bash
# Create config directory
sudo mkdir -p /etc/backup-health

# Copy configuration (choose appropriate example)
sudo cp examples/config-production.yaml /etc/backup-health/config.yaml

# Create working directory
sudo mkdir -p /var/lib/backup-health-monitor
```

### 4. Create Service User

```bash
# Create dedicated user for the service
sudo useradd --system --shell /usr/sbin/nologin --home /var/lib/backup-health-monitor deploy

# Set ownership
sudo chown -R deploy:deploy /var/lib/backup-health-monitor
sudo chown deploy:deploy /etc/backup-health/config.yaml
```

### 5. Install Systemd Service

```bash
# Copy service file
sudo cp examples/backup-health-monitor.service /etc/systemd/system/

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable backup-health-monitor.service
```

### 6. Configure Access Permissions

The service needs access to systemd D-Bus to query service status:

```bash
# Add deploy user to systemd-journal group (if needed)
sudo usermod -a -G systemd-journal deploy

# Verify D-Bus access (test manually)
sudo -u deploy systemctl status backup-health-monitor.service
```

### 7. Start and Verify Service

```bash
# Start the service
sudo systemctl start backup-health-monitor.service

# Check status
sudo systemctl status backup-health-monitor.service

# View logs
sudo journalctl -u backup-health-monitor.service -f

# Test HTTP endpoint
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/status
```

## Configuration

### Environment-Specific Configs

- **Development**: Use `config-development.yaml` with debug logging
- **Minimal**: Use `config-minimal.yaml` for single-service monitoring
- **Production**: Use `config-production.yaml` with multiple services

### Service Configuration

Edit `/etc/backup-health/config.yaml` to match your environment:

```yaml
services:
  - name: "your-backup.service"
    backup_path: "/path/to/your/backups"
    max_age_hours: 36
    expected_file_patterns: ["*.backup"]
    min_file_size_mb: 1
```

## Monitoring Integration

### Prometheus

The service exposes metrics at `/metrics`:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'backup-health-monitor'
    static_configs:
      - targets: ['localhost:8080']
```

### Gatus

Configure health checking with Gatus:

```yaml
endpoints:
  - name: backup-health
    url: "http://localhost:8080/health"
    interval: 5m
    conditions:
      - "[STATUS] == 200"
```

## Security Considerations

- Run as dedicated non-root user (`deploy`)
- Enable systemd security hardening (included in service file)
- Bind to localhost only unless remote access needed
- Use firewall rules to restrict access to monitoring port
- Regularly update the binary from releases

## Troubleshooting

See [troubleshooting guide](../docs/troubleshooting.md) for common issues and solutions.