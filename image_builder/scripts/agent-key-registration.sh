#!/bin/bash
# File: /etc/pf9/agent-key-registration.sh

# Configuration
K3S_ENV_FILE="/etc/pf9/k3s.env"
SSH_DIR="$HOME/.ssh"
SSH_KEY_PATH="$SSH_DIR/id_rsa"
LOG_FILE="/var/log/key-registration.log"
REG_PORT=8989

# Create log file
touch "$LOG_FILE"
chown "$(whoami):$(whoami)" "$LOG_FILE"

# Log function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

# Get the MASTER_IP and K3S_TOKEN from k3s.env file
if [ -f "$K3S_ENV_FILE" ]; then
    log "Loading configuration from $K3S_ENV_FILE"
    source "$K3S_ENV_FILE"
else
    log "Error: K3s environment file not found at $K3S_ENV_FILE"
    exit 1
fi

# Verify we have the necessary variables
if [ -z "$MASTER_IP" ]; then
    log "Error: MASTER_IP not found in $K3S_ENV_FILE"
    exit 1
fi

if [ -z "$K3S_TOKEN" ]; then
    log "Error: K3S_TOKEN not found in $K3S_ENV_FILE"
    exit 1
fi

# Script should only run on agents
if [ "$IS_MASTER" == "true" ]; then
  log "ERROR: This script should only run on agents. Exiting."
  exit 1
fi

log "Using master node: $MASTER_IP"

# Create .ssh directory if it doesn't exist
mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"
chown "$(whoami):$(whoami)" "$SSH_DIR"

# Generate SSH key if it doesn't exist
if [ ! -f "$SSH_KEY_PATH" ]; then
    log "Generating new SSH key at $SSH_KEY_PATH"
    ssh-keygen -t rsa -b 2048 -f "$SSH_KEY_PATH" -N ""
    chmod 600 "$SSH_KEY_PATH" "$SSH_KEY_PATH.pub"
    chown "$(whoami):$(whoami)" "$SSH_KEY_PATH" "$SSH_KEY_PATH.pub"
fi

# Read public key
SSH_PUB_KEY=$(cat "${SSH_KEY_PATH}.pub")

# Check if master node is reachable
if ! ping -c 1 "$MASTER_IP" &> /dev/null; then
    log "Master node $MASTER_IP is not reachable. Registration aborted."
    exit 1
fi

CLIENT_IP=$(hostname -I | awk '{print $1}')

# Register key with master
log "Registering SSH key with master node"
RESPONSE=$(echo "$K3S_TOKEN $CLIENT_IP $SSH_PUB_KEY" | nc -w 5 "$MASTER_IP" "$REG_PORT")