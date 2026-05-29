#!/bin/bash
# VMware Tools Cleanup Script for Linux
# This script removes leftover VMware Tools files after VM migration to OpenStack
# Run as root during firstboot
#
# Compatibility: Bash 3.1+, systemd and SysV init, apt/dnf/yum/zypper

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

# Detect init system once
HAS_SYSTEMD=false
if command -v systemctl > /dev/null 2>&1 && systemctl --version > /dev/null 2>&1; then
    HAS_SYSTEMD=true
fi

stop_and_disable_service() {
    local service="$1"
    if $HAS_SYSTEMD; then
        if systemctl is-active --quiet "$service" 2>> "$LOG_FILE"; then
            log "INFO" "Stopping service: $service"
            if systemctl stop "$service" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully stopped: $service"
            else
                log "WARNING" "Failed to stop: $service"
            fi
        else
            log "INFO" "Service not running (skipping): $service"
        fi
        if systemctl list-unit-files 2>> "$LOG_FILE" | grep -q "$service"; then
            if systemctl disable "$service" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully disabled: $service"
            else
                log "WARNING" "Failed to disable: $service"
            fi
        fi
    elif [ -f "/etc/init.d/$service" ]; then
        # SysV init fallback (pre-systemd: SUSE 11, RHEL 5/6, Debian 6, etc.)
        if "/etc/init.d/$service" status > /dev/null 2>&1; then
            log "INFO" "Stopping service (SysV): $service"
            if "/etc/init.d/$service" stop 2>> "$LOG_FILE"; then
                log "INFO" "Successfully stopped: $service"
            else
                log "WARNING" "Failed to stop: $service"
            fi
        else
            log "INFO" "Service not running (skipping): $service"
        fi
        if command -v chkconfig > /dev/null 2>&1; then
            chkconfig "$service" off 2>> "$LOG_FILE" && log "INFO" "Disabled (chkconfig): $service" || log "WARNING" "Failed to disable: $service"
        elif command -v update-rc.d > /dev/null 2>&1; then
            update-rc.d "$service" disable 2>> "$LOG_FILE" && log "INFO" "Disabled (update-rc.d): $service" || log "WARNING" "Failed to disable: $service"
        fi
    else
        log "INFO" "Service not found (skipping): $service"
    fi
}

log "INFO" "=== VMware Tools Cleanup Started ==="

# Stop and disable VMware services
log "INFO" "Stopping VMware services..."
for service in vmware vmware-tools vmtoolsd open-vm-tools; do
    stop_and_disable_service "$service"
done

# Remove VMware packages
log "INFO" "Removing VMware packages..."
VMWARE_PACKAGES="open-vm-tools vmware-tools-core vmware-tools"

# For apt-based systems (Debian/Ubuntu)
if command -v apt-get > /dev/null 2>&1 && apt-get --version > /dev/null 2>&1; then
    for pkg in $VMWARE_PACKAGES; do
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
elif command -v zypper > /dev/null 2>&1 && zypper --version > /dev/null 2>&1; then
    # SUSE / openSUSE
    for pkg in $VMWARE_PACKAGES; do
        if rpm -q "$pkg" > /dev/null 2>&1; then
            log "INFO" "Removing package (zypper): $pkg"
            if zypper --non-interactive remove "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully removed package: $pkg"
            else
                log "WARNING" "Failed to remove package: $pkg"
            fi
        fi
    done
elif command -v dnf > /dev/null 2>&1 && dnf --version > /dev/null 2>&1; then
    for pkg in $VMWARE_PACKAGES; do
        if rpm -q "$pkg" > /dev/null 2>&1; then
            log "INFO" "Removing package (dnf): $pkg"
            if dnf remove -y "$pkg" 2>> "$LOG_FILE"; then
                log "INFO" "Successfully removed package: $pkg"
            else
                log "WARNING" "Failed to remove package: $pkg"
            fi
        fi
    done
elif command -v yum > /dev/null 2>&1 && yum --version > /dev/null 2>&1; then
    for pkg in $VMWARE_PACKAGES; do
        if rpm -q "$pkg" > /dev/null 2>&1; then
            log "INFO" "Removing package (yum): $pkg"
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

# Remove VMware directories
log "INFO" "Removing VMware directories..."
for dir in /etc/vmware-tools /var/lib/vmware /usr/lib/vmware-tools /usr/lib/open-vm-tools; do
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
# Uses temp file instead of mapfile/process substitution for Bash 3.x compatibility
log "INFO" "Removing VMware log files from /var/log/..."
_tmpfile=$(mktemp /tmp/vmware-cleanup-XXXXXX)
find /var/log -maxdepth 1 -type f \( -name "vmware-*" -o -name "*vmtools*" -o -name "*vm-tools*" \) > "$_tmpfile" 2>> "$LOG_FILE" || true
if [ -s "$_tmpfile" ]; then
    while IFS= read -r logfile; do
        log "INFO" "Removing log file: $logfile"
        if rm -f "$logfile"; then
            log "INFO" "Successfully removed: $logfile"
        else
            log "WARNING" "Failed to remove: $logfile"
        fi
    done < "$_tmpfile"
else
    log "INFO" "No VMware log files found in /var/log/"
fi
rm -f "$_tmpfile"

log "INFO" "=== VMware Tools Cleanup Completed ==="
log "INFO" "Log file saved to: $LOG_FILE"

exit 0
