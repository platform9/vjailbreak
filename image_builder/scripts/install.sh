#!/bin/bash

# Define the log function for easy logging
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a /var/log/pf9-install.log
}

# Function to check if the last command succeeded
check_command() {
  if [ $? -ne 0 ]; then
    log "ERROR: Command failed: $1"
    exit 1
  fi
}

# sleep for 1min for env variables to be reflected properly in the VM after startup. 
sleep 60

# Ensure the environment variables are set for cron
export PATH=/usr/sbin:/usr/bin:/sbin:/bin

# Load environment variables from k3s.env
if [ -f "/etc/pf9/k3s.env" ]; then
  source "/etc/pf9/k3s.env"
else
  log "ERROR: k3s.env file not found. Exiting."
  exit 1
fi

# Check for required environment variables
if [ -z "$IS_MASTER" ]; then
  log "ERROR: IS_MASTER is not set. Exiting."
  exit 1
fi

log "IS_MASTER: ${IS_MASTER}"
log "MASTER_IP: ${MASTER_IP}"
log "K3S_TOKEN: ${K3S_TOKEN}"

# Specify the desired K3s version here
K3S_VERSION="v1.31.5+k3s1"  # Change this to your desired version

if [ "$IS_MASTER" == "true" ]; then
  log "Setting up K3s Master..."

  # Install K3s master with the specific version
  sudo curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=$K3S_VERSION INSTALL_K3S_EXEC="--node-taint CriticalAddonsOnly=true:NoExecute" sh -
  check_command "Installing K3s master"

  # Sleep for 10 seconds after master installation
  sleep 10

  # Apply monitoring manifests
  log "Applying kube-prometheus manifests..."
  sudo kubectl --request-timeout=300s apply --server-side -f /tmp/yamls/kube-prometheus/manifests/setup
  log "Applied kube-prometheus setup manifests."

  sudo kubectl wait --for condition=Established --all CustomResourceDefinition --namespace=monitoring --timeout=300s
  log "CustomResourceDefinitions established."

  sudo kubectl --request-timeout=300s apply -f /tmp/yamls/kube-prometheus/manifests/
  log "Applied kube-prometheus manifests."

  sudo kubectl --request-timeout=300s apply -f /tmp/yamls/
  log "Applied additional manifests."

  log "K3s master setup completed."
else
  log "Setting up K3s Worker..."

  # Check required variables for worker setup
  if [ -z "$MASTER_IP" ] || [ -z "$K3S_TOKEN" ]; then
    log "ERROR: Missing MASTER_IP or K3S_TOKEN for worker. Exiting."
    exit 1
  fi

  # Echo K3S_URL and K3S_TOKEN for debugging
  export K3S_URL="https://$MASTER_IP:6443"
  export K3S_TOKEN="$K3S_TOKEN"

  log "K3S_URL: $K3S_URL"
  log "K3S_TOKEN: $K3S_TOKEN"
# Install K3s worker
  curl -sfL https://get.k3s.io | K3S_URL=$K3S_URL K3S_TOKEN=$K3S_TOKEN INSTALL_K3S_VERSION=$K3S_VERSION sh -
  check_command "Installing K3s worker"

  log "K3s worker setup completed."
fi

# Remove cron job to ensure this runs only once 
crontab -l | grep -v '@reboot /etc/pf9/install.sh' | crontab -
check_command "Removing cron job"

# End of script
exit 0