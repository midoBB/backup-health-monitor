# Troubleshooting Guide

This guide covers common issues and their solutions when deploying and operating the Backup Health Monitor.

## Common Issues

### 1. Systemd D-Bus Permission Errors

**Symptoms**:
```
Failed to connect to systemd D-Bus: dial unix /run/systemd/private: connect: permission denied
```

**Cause**: The service user doesn't have permission to access systemd D-Bus.

**Solutions**:

#### Option A: Add User to systemd-journal Group
```bash
sudo usermod -a -G systemd-journal deploy
sudo systemctl restart backup-health-monitor.service
```

#### Option B: Use PolicyKit Rule (Recommended for Production)
Create `/etc/polkit-1/rules.d/50-backup-health-monitor.rules`:
```javascript
polkit.addRule(function(action, subject) {
    if (action.id == "org.freedesktop.systemd1.manage-units" &&
        subject.user == "deploy") {
        return polkit.Result.YES;
    }
});
```

Then restart polkit:
```bash
sudo systemctl restart polkit
```

#### Option C: Run as Root (Not Recommended)
Modify the systemd service file:
```ini
[Service]
User=root
Group=root
```

**Verification**:
```bash
# Test D-Bus access manually
sudo -u deploy systemctl status backup-health-monitor.service
```

---

### 2. Configuration File Not Found

**Symptoms**:
```
Configuration file not found: /etc/backup-health/config.yaml
Error: failed to load configuration: config file not found
```

**Cause**: Missing or incorrectly located configuration file.

**Solutions**:

#### Check File Location
```bash
ls -la /etc/backup-health/config.yaml
```

#### Create Configuration Directory
```bash
sudo mkdir -p /etc/backup-health
sudo cp examples/config-production.yaml /etc/backup-health/config.yaml
```

#### Fix File Permissions
```bash
sudo chown deploy:deploy /etc/backup-health/config.yaml
sudo chmod 644 /etc/backup-health/config.yaml
```

#### Use Alternative Config Path
```bash
# Via command line
backup-health-monitor serve --config /path/to/config.yaml

# Via environment variable
export BACKUP_HEALTH_CONFIG=/path/to/config.yaml
```

**Verification**:
```bash
# Test configuration loading
backup-health-monitor validate --config /etc/backup-health/config.yaml
```

---

### 3. Backup Directory Access Issues

**Symptoms**:
```
Backup directory does not exist: /opt/backups/service
stat /opt/backups/service: no such file or directory
```

**Cause**: Backup directories are missing or inaccessible.

**Solutions**:

#### Create Missing Directories
```bash
sudo mkdir -p /opt/backups/vault
sudo mkdir -p /opt/backups/postgres
sudo mkdir -p /opt/backups/minio
```

#### Fix Directory Permissions
```bash
# Allow read access for monitoring
sudo chmod 755 /opt/backups/*
sudo chown -R backup:backup /opt/backups/*

# Add monitoring user to backup group
sudo usermod -a -G backup deploy
```

#### Update Configuration
If backup paths have changed, update `config.yaml`:
```yaml
services:
  - name: "vault-backup.service"
    backup_path: "/actual/backup/path"  # Update this
```

**Verification**:
```bash
# Test directory access
sudo -u deploy ls -la /opt/backups/vault
```

---

### 4. HTTP Server Binding Issues

**Symptoms**:
```
listen tcp 0.0.0.0:8080: bind: address already in use
```

**Cause**: Port already in use or insufficient permissions.

**Solutions**:

#### Check What's Using the Port
```bash
sudo netstat -tlnp | grep :8080
sudo lsof -i :8080
```

#### Use Different Port
Edit `config.yaml`:
```yaml
server:
  port: 8081  # Use different port
```

Or via command line:
```bash
backup-health-monitor serve --port 8081
```

#### Stop Conflicting Service
```bash
# If another service is using the port
sudo systemctl stop other-service
```

**Verification**:
```bash
# Check if service is listening
curl http://localhost:8080/health
netstat -tln | grep :8080
```

---

### 5. Service Not Starting

**Symptoms**:
```
Job for backup-health-monitor.service failed because the control process exited with error code.
```

**Diagnosis Steps**:

#### Check Service Status
```bash
sudo systemctl status backup-health-monitor.service
```

#### View Detailed Logs
```bash
sudo journalctl -u backup-health-monitor.service -f
sudo journalctl -u backup-health-monitor.service --since "1 hour ago"
```

#### Test Manual Execution
```bash
# Run as service user to reproduce issue
sudo -u deploy /usr/local/bin/backup-health-monitor serve --config /etc/backup-health/config.yaml
```

#### Check Binary Permissions
```bash
ls -la /usr/local/bin/backup-health-monitor
# Should be executable: -rwxr-xr-x
```

**Common Fixes**:
```bash
# Fix binary permissions
sudo chmod +x /usr/local/bin/backup-health-monitor

# Fix config permissions
sudo chown deploy:deploy /etc/backup-health/config.yaml

# Restart service
sudo systemctl daemon-reload
sudo systemctl restart backup-health-monitor.service
```

---

### 6. Invalid Configuration

**Symptoms**:
```
Configuration validation failed: no services configured
yaml: line 5: did not find expected key
```

**Cause**: YAML syntax errors or missing required fields.

**Solutions**:

#### Validate YAML Syntax
```bash
# Use yamllint if available
yamllint /etc/backup-health/config.yaml

# Or use built-in validation
backup-health-monitor validate --config /etc/backup-health/config.yaml
```

#### Check Required Fields
Ensure all required fields are present:
```yaml
services:
  - name: "service-name.service"        # Required
    backup_path: "/path/to/backups"     # Required
    max_age_hours: 36                   # Required
    expected_file_patterns: ["*.backup"] # Required
    min_file_size_mb: 1                 # Required

server:
  port: 8080                            # Required
  bind_address: "127.0.0.1"            # Required
```

#### Fix Common YAML Issues
- Use spaces, not tabs for indentation
- Quote strings with special characters
- Ensure proper array syntax for file patterns

**Verification**:
```bash
backup-health-monitor validate --config /etc/backup-health/config.yaml
```

---

### 7. No Recent Backup Files Found

**Symptoms**:
```
Service status: warning
Issues: ["No recent backup files found (max age: 36 hours)"]
```

**Cause**: Backup files are older than configured threshold or don't match patterns.

**Diagnosis**:

#### Check Backup Directory Contents
```bash
ls -la /opt/backups/service/
find /opt/backups/service/ -type f -mtime -2  # Files newer than 2 days
```

#### Verify File Patterns
Check if files match configured patterns:
```bash
# If pattern is "*.sql.gz"
ls -la /opt/backups/postgres/*.sql.gz

# Check timestamps
stat /opt/backups/postgres/latest.sql.gz
```

#### Review Configuration
```yaml
services:
  - name: "postgres-backup.service"
    expected_file_patterns: ["*.sql.gz", "backup-*.sql"]  # Add more patterns
    max_age_hours: 48  # Increase threshold if needed
```

**Solutions**:
- Update file patterns to match actual backup file names
- Adjust `max_age_hours` if backup schedule is different
- Verify backup jobs are running correctly
- Check backup job logs for failures

---

### 8. Memory or Performance Issues

**Symptoms**:
- High memory usage
- Slow response times
- Timeouts

**Diagnosis**:

#### Monitor Resource Usage
```bash
# Check memory usage
sudo systemctl status backup-health-monitor.service
ps aux | grep backup-health-monitor

# Check response times
time curl http://localhost:8080/health
```

#### Check for Large Backup Directories
```bash
# Count files in backup directories
find /opt/backups -type f | wc -l
du -sh /opt/backups/*
```

**Solutions**:

#### Optimize Configuration
```yaml
monitoring:
  check_interval_seconds: 60  # Reduce check frequency
```

#### Limit Systemd Service Resources
Add to systemd service file:
```ini
[Service]
MemoryLimit=128M
CPUQuota=50%
```

#### Clean Up Old Backup Files
```bash
# Remove files older than 30 days
find /opt/backups -type f -mtime +30 -delete
```

---

## Diagnostic Commands

### Service Health Check
```bash
# Quick health status
curl -s http://localhost:8080/health

# Detailed status
curl -s http://localhost:8080/api/v1/status | jq .

# Check specific service
curl -s http://localhost:8080/api/v1/services/vault-backup.service | jq .
```

### System Status
```bash
# Service status
sudo systemctl status backup-health-monitor.service

# Recent logs
sudo journalctl -u backup-health-monitor.service -n 50

# Configuration validation
backup-health-monitor validate --config /etc/backup-health/config.yaml

# Manual execution for debugging
sudo -u deploy backup-health-monitor check --config /etc/backup-health/config.yaml --verbose
```

### Network and Port Testing
```bash
# Check if port is listening
sudo netstat -tlnp | grep :8080

# Test from remote host
curl -v http://server-ip:8080/health

# Check firewall rules
sudo iptables -L | grep 8080
```

### File System Checks
```bash
# Check backup directory permissions
ls -la /opt/backups/
sudo -u deploy ls -la /opt/backups/vault/

# Check recent files
find /opt/backups -type f -mtime -2 -ls

# Check disk space
df -h /opt/backups
```

---

## Getting Help

### Log Analysis
When reporting issues, include:

1. **Service logs**:
   ```bash
   sudo journalctl -u backup-health-monitor.service --since "1 hour ago" > logs.txt
   ```

2. **Configuration**:
   ```bash
   backup-health-monitor validate --config /etc/backup-health/config.yaml
   ```

3. **System information**:
   ```bash
   systemctl --version
   uname -a
   /usr/local/bin/backup-health-monitor --version
   ```

4. **Manual test results**:
   ```bash
   sudo -u deploy backup-health-monitor check --config /etc/backup-health/config.yaml --verbose --json
   ```

### Debug Mode
Enable debug logging temporarily:
```yaml
logging:
  level: "debug"
  format: "text"
```

Then restart the service and check logs for detailed information.

### Community Support
- GitHub Issues: Report bugs and feature requests
- Documentation: Check README and API documentation
- Examples: Review configuration examples for different scenarios