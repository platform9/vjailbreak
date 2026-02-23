#!/bin/bash
# VMware Tools Cleanup Script for Linux
# This script removes leftover VMware Tools files after VM migration to OpenStack
# Run as root during firstboot

set -e

LOG_FILE="/var/log/vmware-tools-cleanup.log"

log() {
    local level="$1"
    local message="$2"
    local timestamp
    timestamp=$(date "+%Y-%m-%d %H:%M:%S")
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

log "INFO" "=== VMware Tools Cleanup Started ==="

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    log "ERROR" "This script must be run as root"
    exit 1
fi

# Directories to remove
VMWARE_DIRS=(
    "/etc/vmware-tools"
    "/var/lib/vmware"
    "/usr/lib/vmware-tools"
    "/usr/lib/open-vm-tools"
)

# Remove VMware directories
log "INFO" "Removing VMware directories..."
for dir in "${VMWARE_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        log "INFO" "Removing directory: $dir"
        if rm -rf "$dir"; then
            log "INFO" "Successfully removed: $dir"
        else
            log "WARNING" "Failed to remove: $dir"
        fi
    else
        log "INFO" "Directory not found (skipping): $dir"
    fi
done

# Remove VMware log files from /var/log/
log "INFO" "Removing VMware log files from /var/log/..."
vmware_logs=$(find /var/log -maxdepth 1 -type f -name "vmware-*" 2>/dev/null || true)
if [ -n "$vmware_logs" ]; then
    for logfile in $vmware_logs; do
        log "INFO" "Removing log file: $logfile"
        if rm -f "$logfile"; then
            log "INFO" "Successfully removed: $logfile"
        else
            log "WARNING" "Failed to remove: $logfile"
        fi
    done
else
    log "INFO" "No VMware log files found in /var/log/"
fi

log "INFO" "=== VMware Tools Cleanup Completed ==="
log "INFO" "Log file saved to: $LOG_FILE"

exit 0
