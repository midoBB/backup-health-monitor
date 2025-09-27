// This file contains manual test functions for debugging systemd integration
// These are not automated tests but can be run manually for debugging

package health

import (
	"fmt"
	"log"
)

// ManualTestSystemdService allows manual testing of systemd service checking
// This can be used for debugging real systemd services when D-Bus is available
func ManualTestSystemdService(serviceName string) {
	fmt.Printf("=== Manual Test for Service: %s ===\n", serviceName)

	result := CheckSystemdService(serviceName)

	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Enabled: %v\n", result.Enabled)
	fmt.Printf("Last Run: %v\n", result.LastRun)
	fmt.Printf("Exit Code: %d\n", result.ExitCode)
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}
	fmt.Println()
}

// ManualTestCommonServices tests several common system services
func ManualTestCommonServices() {
	services := []string{
		"systemd-logind.service",
		"systemd-journald.service",
		"systemd-networkd.service",
		"ssh.service",
		"sshd.service",
	}

	fmt.Println("=== Testing Common System Services ===")
	for _, service := range services {
		ManualTestSystemdService(service)
	}
}

// LogSystemdConnection logs whether we can connect to systemd
func LogSystemdConnection() {
	result := CheckSystemdService("test.service")
	if result.Status == "error" {
		log.Printf("Systemd D-Bus connection: %s", result.Error)
	} else {
		log.Println("Systemd D-Bus connection: OK")
	}
}

// DemoBackupServiceCheck shows how to use the systemd checking in practice
func DemoBackupServiceCheck() {
	// Example of checking the three services from our config
	services := []string{
		"vault-backup.service",
		"postgres-backup.service",
		"minio-backup.service",
	}

	fmt.Println("=== Checking Backup Services ===")
	for _, service := range services {
		result := CheckSystemdService(service)

		fmt.Printf("%s: %s", service, result.Status)
		if result.Error != "" {
			fmt.Printf(" (%s)", result.Error)
		}
		fmt.Println()
	}
}