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
export PATH="/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin"

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
K3S_VERSION="v1.31.5+k3s1" 

# Function to wait for K3s to be ready
wait_for_k3s() {
  local timeout=300
  local start_time=$(date +%s)

  while true; do
    if kubectl get nodes > /dev/null 2>&1; then
      log "K3s is ready."
      return 0
    fi

    local current_time=$(date +%s)
    local elapsed_time=$((current_time - start_time))

    if [ $elapsed_time -ge $timeout ]; then
      log "ERROR: Timed out waiting for K3s to be ready."
      exit 1
    fi

    log "Waiting for K3s to be ready..."
    sleep 10
  done
}

if [ "$IS_MASTER" == "true" ]; then
  log "Setting up K3s Master..."

  # Install K3s master with the specific version
  sudo curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=$K3S_VERSION sh -s - --disable traefik
  check_command "Installing K3s master"

  # Wait for K3s to be ready
  wait_for_k3s

  # Move kubeconfig to ~/.kube/config so that helm can pick it up 
  mkdir -p ~/.kube
  sudo kubectl config view --raw > ~/.kube/config
  check_command "Moving kubeconfig"

  # Create a configuration YAML file with the master IP populated
  cat <<EOF > values.yaml
controller:
  service:
    loadBalancerIP: "$MASTER_IP"
    ingressClass: nginx
EOF

  log "YAML file 'values.yaml' has been created with the master IP."

  # Using helm to install nginx-ingress-controller.
  helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
  helm repo update
  helm install nginx-ingress ingress-nginx/ingress-nginx --namespace nginx-ingress --create-namespace --values=values.yaml
  check_command "Installing NGINX Ingress Controller"

  # Wait for NGINX Ingress Controller to be ready
  kubectl wait --namespace nginx-ingress --for=condition=ready pod --selector=app.kubernetes.io/name=ingress-nginx --timeout=300s
  check_command "Waiting for NGINX Ingress Controller to be ready"

  # Apply monitoring manifests
  log "Applying kube-prometheus manifests..."
  sudo kubectl --request-timeout=300s apply --server-side -f /etc/pf9/yamls/kube-prometheus/manifests/setup
  check_command "Applying kube-prometheus setup manifests"

  sudo kubectl wait --for condition=Established --all CustomResourceDefinition --namespace=monitoring --timeout=300s
  check_command "Waiting for CustomResourceDefinitions to be established"

  sudo kubectl --request-timeout=300s apply -f /etc/pf9/yamls/kube-prometheus/manifests/
  check_command "Applying kube-prometheus manifests"

  sudo kubectl --request-timeout=300s apply -f /etc/pf9/yamls/
  check_command "Applying additional manifests"

  log "K3s master setup completed"
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