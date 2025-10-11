#!/bin/bash

set -x

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

# Ensure required tools exist (envsubst for rendering manifests)
ensure_dependencies() {
  if ! command -v envsubst >/dev/null 2>&1; then
    if command -v apt-get >/dev/null 2>&1; then
      log "Installing dependency: gettext-base (for envsubst)"
      sudo apt-get update -y && sudo apt-get install -y gettext-base
      check_command "Installing gettext-base"
    else
      log "WARNING: envsubst not found and apt-get unavailable. Skipping install; rendering may fail."
    fi
  fi
}

# sleep for 20s for env variables to be reflected properly in the VM after startup. 
sleep 20

# Ensure the environment variables are set for cron
export PATH="/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin"
CERT_MANAGER_VERSION="v1.14.3"

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
log "INSTALL_K3S_EXEC: ${INSTALL_K3S_EXEC}"

set_default_password() {
  
  log "Setting default password for ubuntu user..."
  
  sudo usermod -p $(openssl passwd -1 "password") ubuntu
  sudo chage -d 0 ubuntu
  sudo passwd --expire ubuntu

  if grep -qE '^\s*PasswordAuthentication' /etc/ssh/sshd_config; then
    sudo sed -i 's/^\s*PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
  else
    echo 'PasswordAuthentication yes' | sudo tee -a /etc/ssh/sshd_config >/dev/null
  fi
  
  if grep -qE '^\s*ChallengeResponseAuthentication' /etc/ssh/sshd_config; then
    sudo sed -i 's/^\s*ChallengeResponseAuthentication.*/ChallengeResponseAuthentication no/' /etc/ssh/sshd_config
  else
    echo 'ChallengeResponseAuthentication no' | sudo tee -a /etc/ssh/sshd_config >/dev/null
  fi
  
  sudo systemctl restart ssh || sudo systemctl restart sshd

  log "Default password set for ubuntu user. User will need to change it on first login"
}

set_default_password
check_command "Setting default password for ubuntu user"

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

wait_for_k3s_worker() {
  local timeout=300
  local start_time=$(date +%s)

  while true; do
    if ctr version >/dev/null 2>&1; then
      log "K3s worker node is ready (containerd is responsive)."
      return 0
    fi

    local current_time
    current_time=$(date +%s)
    local elapsed_time=$((current_time - start_time))

    if [ "$elapsed_time" -ge "$timeout" ]; then
      log "ERROR: Timed out waiting for K3s worker to be ready."
      exit 1
    fi

    log "Waiting for K3s worker node (ctr)..."
    sleep 5
  done
}


if [ "$IS_MASTER" == "true" ]; then
  log "Setting up K3s Master..."

  # Install K3s master with the specific version
  INSTALL_K3S_SKIP_DOWNLOAD=true /etc/pf9/k3s-setup/k3s-install.sh --disable traefik
  check_command "Installing K3s master"

  # Wait for K3s to be ready
  wait_for_k3s

  # Ensure dependencies are present
  ensure_dependencies

  # Move kubeconfig to ~/.kube/config so that helm can pick it up 
  mkdir -p ~/.kube
  sudo kubectl config view --raw > ~/.kube/config
  check_command "Moving kubeconfig"

  # Load images
  log "Loading all the images in /etc/pf9/images..."
  for img in /etc/pf9/images/*.tar; do
    sudo ctr --address /run/k3s/containerd/containerd.sock -n k8s.io images import "$img"
    check_command "Loading image: $img"
  done

  # Using helm to install nginx-ingress-controller.
  helm install nginx-ingress /etc/pf9/ingress-nginx --namespace nginx-ingress --create-namespace 
  check_command "Installing NGINX Ingress Controller"

  # Wait for NGINX Ingress Controller to be ready
  kubectl wait --namespace nginx-ingress --for=condition=ready pod --selector=app.kubernetes.io/name=ingress-nginx --timeout=300s
  check_command "Waiting for NGINX Ingress Controller to be ready"

  # Detect the primary IP address to be used as the hostname
  detect_public_ip() {
    # Try to detect the primary routable IPv4 address
    local ip
    ip=$(ip -4 -o addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | head -n1)
    if [ -z "$ip" ]; then
      ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    fi
    echo "$ip"
  }

  PUBLIC_IP=$(detect_public_ip)
  if [ -z "$PUBLIC_IP" ]; then
    log "ERROR: Could not detect PUBLIC_IP"
    exit 1
  fi
  export PUBLIC_IP
  log "Detected PUBLIC_IP: ${PUBLIC_IP}"

  # Apply monitoring manifests
  log "Applying kube-prometheus manifests..."
  sudo kubectl --request-timeout=300s apply --server-side -f /etc/pf9/yamls/kube-prometheus/manifests/setup
  check_command "Applying kube-prometheus setup manifests"

  sudo kubectl wait --for condition=Established --all CustomResourceDefinition --namespace=monitoring --timeout=300s
  check_command "Waiting for CustomResourceDefinitions to be established"

  sudo kubectl --request-timeout=300s apply -f /etc/pf9/yamls/kube-prometheus/manifests/
  check_command "Applying kube-prometheus manifests"

  # Apply all additional manifests with environment substitution so ${PUBLIC_IP} is resolved
  TMP_YAMLS_DIR=$(mktemp -d)
  log "Rendering manifests from /etc/pf9/yamls with PUBLIC_IP=${PUBLIC_IP} into ${TMP_YAMLS_DIR}"
  while IFS= read -r -d '' file; do
    rel_path=${file#/etc/pf9/yamls/}
    mkdir -p "${TMP_YAMLS_DIR}/$(dirname "$rel_path")"
    envsubst < "$file" > "${TMP_YAMLS_DIR}/$rel_path"
  done < <(find /etc/pf9/yamls -type f \( -name '*.yaml' -o -name '*.yml' \) -print0)

  sudo kubectl --request-timeout=300s apply -R -f "${TMP_YAMLS_DIR}"
  check_command "Applying additional manifests (envsubst-rendered)"

  # Optionally open firewall for HTTP/HTTPS if ufw is present
  if command -v ufw >/dev/null 2>&1; then
    log "Configuring UFW to allow ports 80 and 443"
    sudo ufw allow 80/tcp || true
    sudo ufw allow 443/tcp || true
  fi

  # Install cert-manager (self-signed issuer will be created below)
  log "Installing cert-manager (${CERT_MANAGER_VERSION})..."
  kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
  check_command "Applying cert-manager core manifests"

  # Wait for cert-manager to be ready
  kubectl -n cert-manager wait --for=condition=Available deployment --all --timeout=300s
  check_command "Waiting for cert-manager to be available"

  # Create a self-signed ClusterIssuer
  log "Creating self-signed ClusterIssuer..."
  kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-cluster-issuer
spec:
  selfSigned: {}
EOF
  check_command "Creating ClusterIssuer"

  # Create Certificates in required namespaces using PUBLIC_IP and ${PUBLIC_IP}.nip.io
  log "Creating Certificates for namespaces (migration-system, default, monitoring, vpwned)..."
  kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: vjailbreak-ui-cert
  namespace: migration-system
spec:
  secretName: vjailbreak-ui-tls
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned-cluster-issuer
  commonName: "${PUBLIC_IP}"
  dnsNames:
  - "${PUBLIC_IP}.nip.io"
  ipAddresses:
  - "${PUBLIC_IP}"
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: grafana-cert
  namespace: monitoring
spec:
  secretName: grafana-tls
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned-cluster-issuer
  commonName: "${PUBLIC_IP}"
  dnsNames:
  - "${PUBLIC_IP}.nip.io"
  ipAddresses:
  - "${PUBLIC_IP}"
EOF
  check_command "Creating Certificates"

  # Wait for certificates to be Ready before proceeding
  log "Waiting for certificates to become Ready..."
  kubectl -n migration-system wait certificate vjailbreak-ui-cert --for=condition=Ready --timeout=180s
  check_command "Waiting for vjailbreak-ui-cert to be Ready"
  kubectl -n monitoring wait certificate grafana-cert --for=condition=Ready --timeout=180s
  check_command "Waiting for grafana-cert to be Ready"

  # Output final access endpoints
  log "Ingress is configured for host: https://${PUBLIC_IP}.nip.io"
  log "- UI:      https://${PUBLIC_IP}.nip.io/"
  log "- Grafana: https://${PUBLIC_IP}.nip.io/grafana"

  log "K3s master setup completed"

  # Start the rsync daemon
  kubectl apply -f /etc/pf9/yamls/daemonset.yaml
  check_command "Installing rsync daemon"

  log "Rsync daemon started successfully."

  # Create a config map from env file. 
  kubectl create configmap pf9-env -n migration-system --from-file=/etc/pf9/env
  check_command "Creating config map from env file"
  log "Config map created successfully."

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
  export INSTALL_K3S_EXEC="$INSTALL_K3S_EXEC"

  log "K3S_URL: $K3S_URL"
  log "K3S_TOKEN: $K3S_TOKEN"
  log "INSTALL_K3S_EXEC: $INSTALL_K3S_EXEC"

  # Install K3s worker
  K3S_URL=$K3S_URL K3S_TOKEN=$K3S_TOKEN INSTALL_K3S_EXEC=$INSTALL_K3S_EXEC INSTALL_K3S_SKIP_DOWNLOAD=true /etc/pf9/k3s-setup/k3s-install.sh
  check_command "Installing K3s worker"

  # wait until ctr becomes responsive
  wait_for_k3s_worker
  check_command "Waiting for k3s worker to come up"
  
  # Load images
  log "Loading all the images in /etc/pf9/images..."
  for img in /etc/pf9/images/*.tar; do
    sudo ctr --address /run/k3s/containerd/containerd.sock -n k8s.io images import "$img"
    check_command "Loading image: $img"
  done

  log "K3s worker setup completed."
  sleep 20 

fi

# Remove cron job to ensure this runs only once 
crontab -l | grep -v '@reboot /etc/pf9/install.sh' | crontab -
check_command "Removing cron job"

# End of script
exit 0