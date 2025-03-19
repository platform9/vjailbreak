#!/bin/bash
# File: /etc/pf9/vddk-sync.sh

# Configuration
K3S_ENV_FILE="/etc/pf9/k3s.env"
SOURCE_DIR="/home/ubuntu/vmware-vix-disklib-distrib/"
DEST_DIR="/home/ubuntu/vmware-vix-disklib-distrib"
SSH_USER="ubuntu"
SSH_KEY="$HOME/.ssh/id_rsa"
LOG_FILE="/var/log/vddk-sync.log"
MAX_RETRIES=5

# Log function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

# Load environment variables
if [ -f "$K3S_ENV_FILE" ]; then
    log "Loading configuration from $K3S_ENV_FILE"
    source "$K3S_ENV_FILE"
else
    log "Error: K3s environment file not found at $K3S_ENV_FILE"
    exit 1
fi

# Verify required variables
if [ -z "$MASTER_IP" ]; then
    log "Error: MASTER_IP not found in $K3S_ENV_FILE"
    exit 1
fi
log "Using master node: $MASTER_IP"

# Prevent execution on master node
if [ "$IS_MASTER" == "true" ]; then
    log "Detected master node. Disabling vddk-sync services."
    
    systemctl stop vddk-sync.timer 2>> "$LOG_FILE" || log "Failed to stop vddk-sync.timer"
    systemctl disable vddk-sync.timer 2>> "$LOG_FILE" || log "Failed to disable vddk-sync.timer"
    systemctl stop vddk-sync.service 2>> "$LOG_FILE" || log "Failed to stop vddk-sync.service"

    log "Timer and service disabled. Exiting."
    exit 0
fi

# Ensure SSH key is registered with master
if [ -x /etc/pf9/agent-key-registration.sh ]; then
    log "Ensuring SSH key is registered with master"
    /etc/pf9/agent-key-registration.sh
    if [ $? -ne 0 ]; then
        log "Failed to register SSH key. Aborting sync."
        exit 1
    fi
else
    log "Error: SSH key registration script not found or not executable."
    exit 1
fi

# Check if SSH key exists
if [ ! -f "$SSH_KEY" ]; then
    log "Error: SSH key not found at $SSH_KEY. Sync aborted."
    exit 1
fi

# Check if master node is reachable
if ! ping -c 1 "$MASTER_IP" &> /dev/null; then
    log "Master node $MASTER_IP is not reachable. Sync aborted."
    exit 1
fi

# Perform rsync with retries
log "Starting VDDK libraries sync from $SSH_USER@$MASTER_IP:$SOURCE_DIR to $DEST_DIR"
retry_count=0
sync_success=false

while [ $retry_count -lt $MAX_RETRIES ] && [ "$sync_success" != "true" ]; do
    rsync -avz --delete-after  -e "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null" \
    "$SSH_USER@$MASTER_IP:$SOURCE_DIR" "$DEST_DIR/" >> "$LOG_FILE" 2>&1
    
    if [ $? -eq 0 ]; then
        sync_success=true
        log "Sync completed successfully"
        
        # Set proper permissions
        sudo chown -R ubuntu:ubuntu "$DEST_DIR"
        
    else
        retry_count=$((retry_count+1))
        log "Sync attempt $retry_count failed. Retrying in 30 seconds..."
        sleep 30
    fi
done

if [ "$sync_success" != "true" ]; then
    log "Sync failed after $MAX_RETRIES attempts."
    exit 1
fi

exit 0