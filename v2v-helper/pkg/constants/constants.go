package constants

import "time"

const (
	RhelFirstBootScript = `#!/bin/bash

# Exit on any error
set -e

# Log to file for debugging
LOG_FILE="/var/log/network_fix.log"
echo "$(date '+%Y-%m-%d %H:%M:%S') - Starting network fix script" >> "$LOG_FILE"

# Check if NetworkManager is active
if ! systemctl is-active NetworkManager >/dev/null 2>&1; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - NetworkManager is not active, exiting" >> "$LOG_FILE"
    exit 1
fi

# Loop over non-loopback connections
nmcli -t -f NAME,TYPE connection show | grep -v ':loopback' | cut -d: -f1 | while read -r conn; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Processing connection: $conn" >> "$LOG_FILE"
    
    # Modify to DHCP, clear statics
    if ! nmcli con mod "$conn" ipv4.method auto ipv4.addresses "" ipv4.gateway ""; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Failed to modify IPv4 for $conn" >> "$LOG_FILE"
    fi
    if ! nmcli con mod "$conn" ipv6.method auto ipv6.addresses "" ipv6.gateway ""; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Failed to modify IPv6 for $conn" >> "$LOG_FILE"
    fi

    # Reload and bounce
    nmcli con reload || echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Reload failed for $conn" >> "$LOG_FILE"
    if ! nmcli con down "$conn"; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Failed to down $conn, trying to proceed" >> "$LOG_FILE"
    fi
    if ! nmcli con up "$conn"; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Error: Failed to up $conn" >> "$LOG_FILE"
    else
        echo "$(date '+%Y-%m-%d %H:%M:%S') - Successfully brought up $conn" >> "$LOG_FILE"
    fi
done

# Enable and start serial console
if ! systemctl enable --now serial-getty@ttyS0.service; then
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Warning: Failed to enable serial-getty" >> "$LOG_FILE"
else
    echo "$(date '+%Y-%m-%d %H:%M:%S') - Enabled serial-getty@ttyS0.service" >> "$LOG_FILE"
fi

echo "$(date '+%Y-%m-%d %H:%M:%S') - Network fix script completed" >> "$LOG_FILE"`

	MaxCPU = 9999999
	MaxRAM = 9999999

	// Number of intervals to wait for the volume to become available
	MaxIntervalCount = 60

	InspectOSCommand         = "inspect-os"
	LSBootCommand            = "ls /boot"
	XMLFileName              = "libxml.xml"
	MigrationSnapshotName    = "migration-snap"
	MaxHTTPRetryCount        = 5
	MaxVMActiveCheckCount    = 15
	VMActiveCheckInterval    = 20 * time.Second
	MigrationSystemNamespace = "migration-system"
	TrueString               = "true"

	LogsDir = "/var/log/pf9"

	EventMessageConvertingDisk                    = "Converting disk"
	EventMessageWaitingForCutOverStart            = "Waiting for VM Cutover start time"
	EventMessageCopyingChangedBlocksWithIteration = "Copying changed blocks"
	EventMessageWaitingForDataCopyStart           = "Waiting for data copy start time"
	EventMessageDataCopyStart                     = "Data copy start time reached"
	EventMessageWaitingForAdminCutOver            = "Waiting for Admin Cutover conditions to be met"
	EventMessageMigrationSucessful                = "VM created successfully"
	EventMessageMigrationFailed                   = "Trying to perform cleanup"
	EventMessageCopyingDisk                       = "Copying disk"
	EventMessageFailed                            = "Failed to"

	OSFamilyWindows = "windowsguest"
	OSFamilyLinux   = "linuxguest"

	PCDClusterNameNoCluster = "NO CLUSTER"
)
