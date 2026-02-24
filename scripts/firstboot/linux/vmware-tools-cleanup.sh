#!/bin/bash
# VMware Tools Cleanup Script for Linux
# This script removes leftover VMware Tools files after VM migration to OpenStack
# Run as root during firstboot

set -e

# Check if running as root first
if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: This script must be run as root"
    exit 1
fi

LOG_FILE="/var/log/vmware-tools-cleanup.log"

# Ensure log directory exists and is writable
if [ ! -d "$(dirname "$LOG_FILE")" ]; then
    mkdir -p "$(dirname "$LOG_FILE")" || {
        echo "ERROR: Cannot create log directory $(dirname "$LOG_FILE")"
        exit 1
    }
fi

if [ ! -w "$(dirname "$LOG_FILE")" ]; then
    echo "ERROR: No write permission for log directory $(dirname "$LOG_FILE")"
    exit 1
fi

log() {
    local level="$1"
    local message="$2"
    local timestamp
    timestamp=$(date "+%Y-%m-%d %H:%M:%S")
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

log "INFO" "=== VMware Tools Cleanup Started ==="

# Stop and disable VMware services if running
log "INFO" "Stopping VMware services..."
VMWARE_SERVICES=("vmware" "vmware-tools" "vmtoolsd" "open-vm-tools")
for service in "${VMWARE_SERVICES[@]}"; do
    if systemctl is-active --quiet "$service" 2>> "$LOG_FILE"; then
        log "INFO" "Stopping service: $service"
        if systemctl stop "$service" 2>> "$LOG_FILE"; then
            log "INFO" "Successfully stopped: $service"
            # Disable service from starting on boot
            if systemctl disable "$service" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully disabled: $service"
            else
                log "WARNING" "Failed to disable: $service"
            fi
        else
            log "WARNING" "Failed to stop: $service"
        fi
    else
        log "INFO" "Service not running (skipping): $service"
        # Still try to disable it if it exists
        if systemctl list-unit-files 2>> "$LOG_FILE" | grep -q "$service"; then
            if systemctl disable "$service" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully disabled: $service"
            else
                log "WARNING" "Failed to disable: $service"
            fi
        fi
    fi
done

# Remove VMware packages if installed
log "INFO" "Removing VMware packages..."
VMWARE_PACKAGES=("open-vm-tools" "vmware-tools-core" "vmware-tools")

# For apt-based systems (Debian/Ubuntu)
if command -v apt-get &>/dev/null && apt-get --version &>/dev/null; then
    for pkg in "${VMWARE_PACKAGES[@]}"; do
        if dpkg -l "$pkg" 2>> "$LOG_FILE" | grep -q "^ii"; then
            log "INFO" "Purging package: $pkg"
            if apt-get purge -y "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully purged package: $pkg"
            else
                log "WARNING" "Failed to purge package: $pkg"
            fi
        fi
    done
# For yum/dnf-based systems (RHEL/CentOS/Fedora)
elif command -v dnf &>/dev/null && dnf --version &>/dev/null; then
    for pkg in "${VMWARE_PACKAGES[@]}"; do
        if rpm -q "$pkg" 2>> "$LOG_FILE" >/dev/null; then
            log "INFO" "Removing package: $pkg"
            if dnf remove -y "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully removed package: $pkg"
            else
                log "WARNING" "Failed to remove package: $pkg"
            fi
        fi
    done
elif command -v yum &>/dev/null && yum --version &>/dev/null; then
    for pkg in "${VMWARE_PACKAGES[@]}"; do
        if rpm -q "$pkg" 2>> "$LOG_FILE" >/dev/null; then
            log "INFO" "Removing package: $pkg"
            if yum remove -y "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully removed package: $pkg"
            else
                log "WARNING" "Failed to remove package: $pkg"
            fi
        fi
    done
else
    log "INFO" "No supported package manager found, skipping package removal"
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
mapfile -t vmware_logs < <(find /var/log -maxdepth 1 -type f \( -name "vmware-*" -o -name "*vmtools*" -o -name "*vm-tools*" \) 2>> "$LOG_FILE")
if [ ${#vmware_logs[@]} -gt 0 ]; then
    for logfile in "${vmware_logs[@]}"; do
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
