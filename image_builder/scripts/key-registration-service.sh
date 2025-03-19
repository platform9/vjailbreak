#!/bin/bash
# File: /home/ubuntu/key-registration-service.sh
# Deploy this on the master node

# Configuration
AUTH_TOKEN=$(cat /var/lib/rancher/k3s/server/token)
SSH_DIR="/.ssh"
AUTHORIZED_KEYS="$SSH_DIR/authorized_keys"
LOG_FILE="/var/log/key-registration.log"
PORT=8989

# Log function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> $LOG_FILE
}

# Load environment variables from k3s.env
if [ -f "/etc/pf9/k3s.env" ]; then
    source "/etc/pf9/k3s.env"
else
    log "ERROR: k3s.env file not found. Exiting."
    exit 1
fi

# Check if this is the master node
if [ -z "$IS_MASTER" ]; then
    log "ERROR: IS_MASTER is not set. Exiting."
    exit 1
fi

log "IS_MASTER: ${IS_MASTER}"

if [ "$IS_MASTER" != "true" ]; then
    log "ERROR: This script should only run on the master node. Exiting."
    exit 1
fi

# Ensure SSH directory and authorized_keys file exist
mkdir -p "$SSH_DIR"
touch "$AUTHORIZED_KEYS"
chmod 700 "$SSH_DIR"
chmod 600 "$AUTHORIZED_KEYS"

log "Starting key registration service on port $PORT"

# Start netcat-based service to accept SSH key registrations
while true; do
    nc -l -k -p $PORT | while read -r token client_ip ssh_key; do
        
        log "Received request from $client_ip"

        # Validate token
        if [ "$token" = "$AUTH_TOKEN" ]; then
            # Check if key already exists
            if ! grep -q "$ssh_key" "$AUTHORIZED_KEYS"; then
                echo "$ssh_key" >> "$AUTHORIZED_KEYS"
                log "Added new SSH key for $client_ip"
                echo "SUCCESS" | nc -q 0 "$client_ip" $PORT
            else
                log "Key already exists for $client_ip"
                echo "ALREADY_EXISTS" | nc -q 0 "$client_ip" $PORT
            fi
        else
            log "Invalid token received from $client_ip"
            echo "FAILED" | nc -q 0 "$client_ip" $PORT
        fi
    done
    sleep 1
done