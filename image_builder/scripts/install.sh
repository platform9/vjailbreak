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

# sleep for 20s for env variables to be reflected properly in the VM after startup. 
sleep 20

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

  # Install cert-manager (controller + CRDs) and apply ClusterIssuers
  if [ -x "/etc/pf9/install-cert-manager.sh" ]; then
    log "Installing cert-manager and ClusterIssuers..."
    sudo /etc/pf9/install-cert-manager.sh
    check_command "Installing cert-manager and ClusterIssuers"
  else
    log "cert-manager installer script not found at /etc/pf9/install-cert-manager.sh; skipping"
  fi

  # Derive domain hosts at first boot if not provided (use nip.io based on Ingress LB IP)
  derive_hosts_if_missing() {
    # Ensure jq available for JSON parsing
    if ! command -v jq >/dev/null 2>&1; then
      if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update -y >/dev/null 2>&1 || true
        sudo apt-get install -y jq >/dev/null 2>&1 || true
      fi
    fi

    # Default cluster issuer if not provided
    export CLUSTER_ISSUER=${CLUSTER_ISSUER:-letsencrypt-staging}

    # Try to get the ingress controller Service external address
    local svc_json
    svc_json=$(kubectl -n nginx-ingress get svc -l app.kubernetes.io/component=controller -o json 2>/dev/null || true)
    local lb_ip lb_hostname ip_for_dns
    lb_ip=$(echo "$svc_json" | jq -r '.items[0].status.loadBalancer.ingress[0].ip // empty' 2>/dev/null || true)
    lb_hostname=$(echo "$svc_json" | jq -r '.items[0].status.loadBalancer.ingress[0].hostname // empty' 2>/dev/null || true)

    if [ -n "$lb_ip" ]; then
      ip_for_dns="$lb_ip"
    elif [ -n "$lb_hostname" ]; then
      # If hostname present, attempt to resolve to an IP for sslip.io/nip.io; if not, we can use sslip.io which accepts hostnames too
      ip_for_dns=$(getent ahostsv4 "$lb_hostname" | awk '{print $1; exit}' || true)
      [ -z "$ip_for_dns" ] && ip_for_dns="$lb_hostname"
    else
      # Fallback to node IP
      ip_for_dns=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "127.0.0.1")
    fi

    # Only set hosts if they are empty; use <sub>.<ip>.nip.io pattern which auto-resolves
    if [ -z "${UI_HOST:-}" ]; then export UI_HOST="ui.${ip_for_dns}.nip.io"; fi
    if [ -z "${API_HOST:-}" ]; then export API_HOST="api.${ip_for_dns}.nip.io"; fi
    if [ -z "${GRAFANA_HOST:-}" ]; then export GRAFANA_HOST="grafana.${ip_for_dns}.nip.io"; fi
    if [ -z "${VPWNED_HOST:-}" ]; then export VPWNED_HOST="vpwned.${ip_for_dns}.nip.io"; fi

    log "Using domains - UI_HOST=${UI_HOST}, API_HOST=${API_HOST}, GRAFANA_HOST=${GRAFANA_HOST}, VPWNED_HOST=${VPWNED_HOST}, CLUSTER_ISSUER=${CLUSTER_ISSUER}"
  }

  render_ingresses() {
    # Ensure envsubst is available for templating; install if missing
    if ! command -v envsubst >/dev/null 2>&1; then
      if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update -y >/dev/null 2>&1 || true
        sudo apt-get install -y gettext-base >/dev/null 2>&1 || true
      fi
    fi

    mkdir -p /etc/pf9/yamls-rendered
    # Render known ingress files if present
    for f in \
      /etc/pf9/yamls/01ui.yaml \
      /etc/pf9/yamls/10-vpwned.yaml \
      /etc/pf9/yamls/11-vpwned-alt.yaml; do
      if [ -f "$f" ]; then
        if command -v envsubst >/dev/null 2>&1; then
          envsubst < "$f" > "/etc/pf9/yamls-rendered/$(basename "$f")"
        else
          # Fallback: naive variable replacement for common vars
          sed -e "s|\${UI_HOST}|${UI_HOST}|g" \
              -e "s|\${API_HOST}|${API_HOST}|g" \
              -e "s|\${GRAFANA_HOST}|${GRAFANA_HOST}|g" \
              -e "s|\${VPWNED_HOST}|${VPWNED_HOST}|g" \
              -e "s|\${CLUSTER_ISSUER}|${CLUSTER_ISSUER}|g" "$f" > "/etc/pf9/yamls-rendered/$(basename "$f")"
        fi
      fi
    done

    # Copy other yamls as-is
    find /etc/pf9/yamls -maxdepth 1 -type f -name '*.yaml' ! -name '01ui.yaml' ! -name '10-vpwned.yaml' ! -name '11-vpwned-alt.yaml' -exec cp {} /etc/pf9/yamls-rendered/ \;
    cp -r /etc/pf9/yamls/kube-prometheus /etc/pf9/yamls-rendered/ 2>/dev/null || true
  }

  derive_hosts_if_missing
  render_ingresses

  # Apply monitoring manifests
  log "Applying kube-prometheus manifests..."
  sudo kubectl --request-timeout=300s apply --server-side -f /etc/pf9/yamls/kube-prometheus/manifests/setup
  check_command "Applying kube-prometheus setup manifests"

  sudo kubectl wait --for condition=Established --all CustomResourceDefinition --namespace=monitoring --timeout=300s
  check_command "Waiting for CustomResourceDefinitions to be established"

  sudo kubectl --request-timeout=300s apply -f /etc/pf9/yamls/kube-prometheus/manifests/
  check_command "Applying kube-prometheus manifests"

  # Apply rendered manifests (contain substituted hosts)
  sudo kubectl --request-timeout=300s apply -f /etc/pf9/yamls-rendered/
  check_command "Applying additional manifests"

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