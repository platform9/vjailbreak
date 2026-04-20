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

# Function to wait for network availability (default route + global IPv4 address)
wait_for_network() {
  log "Waiting for network availability..."

  while true; do
    local has_default_route=false
    local has_global_ipv4=false

    # Check for default route
    if ip route | grep -q default; then
      has_default_route=true
    fi

    # Check for global IPv4 address (non-loopback)
    if ip -4 addr show scope global | grep -q inet; then
      has_global_ipv4=true
    fi

    # Both conditions met
    if [ "$has_default_route" = true ] && [ "$has_global_ipv4" = true ]; then
      log "Network detected. Default route and global IPv4 address available."
      return 0
    fi

    # Log specific missing conditions
    if [ "$has_default_route" = false ] && [ "$has_global_ipv4" = false ]; then
      log "Waiting for network: missing default route and global IPv4 address..."
    elif [ "$has_default_route" = false ]; then
      log "Waiting for network: missing default route..."
    else
      log "Waiting for network: missing global IPv4 address..."
    fi

    sleep 60
  done
}

# Airgapped-friendly: no external package installs; we'll generate /etc/htpasswd using openssl

# sleep for 20s for env variables to be reflected properly in the VM after startup. 
sleep 20

# Ensure the environment variables are set for cron
export PATH="/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin"

# Airgapped: we'll generate /etc/htpasswd using openssl (no package installs)

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

install_time_settings_apply_script() {
  log "Installing vJailbreak time settings apply script (NTP/timezone)..."

  sudo mkdir -p /etc/pf9

  sudo tee /etc/pf9/apply-time-settings.sh > /dev/null <<'EOF'
#!/bin/bash
set -euo pipefail

LOG_DIR="/var/log/pf9"
STATE_DIR="/var/lib/pf9"
LOG_FILE="${LOG_DIR}/time-settings.log"
STATE_FILE="${STATE_DIR}/time-settings.state"
TIMESYNCD_CONF_DIR="/etc/systemd/timesyncd.conf.d"
TIMESYNCD_CONF_FILE="${TIMESYNCD_CONF_DIR}/99-vjailbreak.conf"

mkdir -p "$LOG_DIR" "$STATE_DIR"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

normalize_servers() {
  printf '%s' "${1:-}" | tr ',\n' '  ' | xargs || true
}

is_valid_ntp_server() {
  local server="$1"

  [ -n "$server" ] || return 1

  if [[ "$server" == *"://"* ]] || [[ "$server" == */* ]]; then
    return 1
  fi

  if [[ "$server" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    local o1 o2 o3 o4
    IFS='.' read -r o1 o2 o3 o4 <<< "$server"
    for octet in "$o1" "$o2" "$o3" "$o4"; do
      if [ "$octet" -lt 0 ] || [ "$octet" -gt 255 ]; then
        return 1
      fi
    done
    return 0
  fi

  if [[ "$server" =~ ^[a-zA-Z0-9.-]+$ ]] && [[ "$server" != .* ]] && [[ "$server" != *..* ]]; then
    IFS='.' read -r -a labels <<< "$server"
    for label in "${labels[@]}"; do
      if [ -z "$label" ] || [ "${#label}" -gt 63 ] || [[ ! "$label" =~ ^[a-zA-Z0-9-]+$ ]] || [[ "$label" == -* ]] || [[ "$label" == *- ]]; then
        return 1
      fi
    done
    return 0
  fi

  return 1
}

filter_valid_ntp_servers() {
  local raw="$1"
  local valid=""
  local invalid=""
  local server

  for server in $raw; do
    if is_valid_ntp_server "$server"; then
      valid+=" $server"
    else
      invalid+=" $server"
    fi
  done

  valid="$(echo "$valid" | xargs || true)"
  invalid="$(echo "$invalid" | xargs || true)"

  if [ -n "$invalid" ]; then
    log "Ignoring invalid NTP server entries: $invalid"
  fi

  printf '%s' "$valid"
}

write_timesyncd_conf() {
  local servers="$1"
  mkdir -p "$TIMESYNCD_CONF_DIR"
  cat <<CONF | tee "$TIMESYNCD_CONF_FILE" >/dev/null
[Time]
NTP=${servers}
CONF
}

clear_timesyncd_conf() {
  rm -f "$TIMESYNCD_CONF_FILE"
}

update_pf9_env_timezone() {
  local tz="$1"
  if [ -z "$tz" ]; then
    return 0
  fi

  if [ -f /etc/pf9/env ]; then
    if grep -q '^TZ=' /etc/pf9/env; then
      sudo sed -i "s#^TZ=.*#TZ=${tz}#" /etc/pf9/env || true
    else
      printf '\nTZ=%s\n' "$tz" | sudo tee -a /etc/pf9/env >/dev/null
    fi
  fi

  if kubectl -n migration-system get configmap pf9-env >/dev/null 2>&1; then
    kubectl -n migration-system patch configmap pf9-env --type merge -p "{\"data\":{\"TZ\":\"${tz}\"}}" >/dev/null 2>&1 || true
    for deployment in migration-controller-manager migration-vpwned-sdk vjailbreak-ui; do
      kubectl -n migration-system rollout restart deployment "$deployment" >/dev/null 2>&1 || true
    done
  fi
}

if [ -f "/etc/pf9/k3s.env" ]; then
  source "/etc/pf9/k3s.env" || true
fi

if [ "${IS_MASTER:-}" != "true" ]; then
  exit 0
fi

if ! command -v kubectl >/dev/null 2>&1; then
  log "kubectl not found yet; time settings will be applied by watcher when ready"
  exit 0
fi

if ! kubectl -n migration-system get configmap vjailbreak-settings >/dev/null 2>&1; then
  log "vjailbreak-settings ConfigMap not available yet; watcher will handle it"
  exit 0
fi

get_cm_val() {
  local key="$1"
  kubectl -n migration-system get configmap vjailbreak-settings -o jsonpath="{.data.${key}}" 2>/dev/null || true
}

timezone="$(get_cm_val TIMEZONE)"
ntp_servers_raw="$(get_cm_val NTP_SERVERS)"

timezone="$(echo "${timezone:-}" | xargs || true)"
ntp_servers="$(filter_valid_ntp_servers "$(normalize_servers "${ntp_servers_raw:-}")")"

desired_fingerprint="$(printf '%s\n%s\n' "${timezone}" "${ntp_servers}" | sha256sum | awk '{print $1}')"
current_fingerprint=""
if [ -f "$STATE_FILE" ]; then
  current_fingerprint="$(cat "$STATE_FILE" 2>/dev/null || true)"
fi

if [ "$desired_fingerprint" = "$current_fingerprint" ]; then
  exit 0
fi

sync_enabled="false"
target_timezone=""

if [ -n "$ntp_servers" ]; then
  sync_enabled="true"
  write_timesyncd_conf "$ntp_servers"
  if [ -n "$timezone" ] && [ -f "/usr/share/zoneinfo/${timezone}" ]; then
    target_timezone="$timezone"
  else
    target_timezone="UTC"
    log "No timezone configured with custom NTP servers; defaulting timezone to UTC"
  fi
elif [ -n "$timezone" ] && [ -f "/usr/share/zoneinfo/${timezone}" ]; then
  sync_enabled="true"
  target_timezone="$timezone"
  clear_timesyncd_conf
else
  clear_timesyncd_conf
  target_timezone="UTC"
fi

if [ -n "$ntp_servers" ]; then
  log "Applying time settings: TIMEZONE=${target_timezone} NTP_SERVERS=${ntp_servers}"
elif [ -n "$timezone" ]; then
  log "Applying time settings: TIMEZONE=${target_timezone} NTP_SERVERS=<default pools>"
else
  log "Applying time settings: no timezone or NTP configured; disabling NTP sync, resetting to UTC"
fi

if [ -n "$target_timezone" ]; then
  current_tz="$(timedatectl show -p Timezone --value 2>/dev/null || true)"
  if [ "$current_tz" != "$target_timezone" ]; then
    if timedatectl set-timezone "$target_timezone"; then
      log "Timezone updated to ${target_timezone}"
    else
      log "Failed to set timezone to ${target_timezone}"
    fi
  fi
fi

update_pf9_env_timezone "$target_timezone"

if [ "$sync_enabled" = "true" ]; then
  timedatectl set-ntp true >/dev/null 2>&1 || true
  systemctl enable --now systemd-timesyncd >/dev/null 2>&1 || true
  systemctl restart systemd-timesyncd >/dev/null 2>&1 || true
else
  timedatectl set-ntp false >/dev/null 2>&1 || true
  systemctl disable --now systemd-timesyncd >/dev/null 2>&1 || true
fi

echo "$desired_fingerprint" > "$STATE_FILE"
log "Time settings applied"
EOF

  sudo chmod +x /etc/pf9/apply-time-settings.sh

  sudo rm -f /etc/pf9/watch-time-settings.sh
  sudo rm -f /etc/logrotate.d/pf9-time-settings
  sudo rm -f /etc/systemd/system/vjailbreak-time-settings-watcher.service
  sudo rm -f /etc/systemd/system/vjailbreak-time-settings.timer
  sudo rm -f /etc/systemd/system/vjailbreak-time-settings.service
  sudo systemctl daemon-reload
  sudo systemctl disable --now vjailbreak-time-settings-watcher.service >/dev/null 2>&1 || true
  sudo systemctl disable --now vjailbreak-time-settings.timer >/dev/null 2>&1 || true
  sudo systemctl disable --now vjailbreak-time-settings.service >/dev/null 2>&1 || true
  log "Time settings apply script installed. Watcher service removed."
}

install_time_settings_apply_script

# Create /etc/htpasswd with ubuntu user using openssl apr1 hash (airgapped-safe)
sudo sh -c 'umask 0177; mkdir -p /etc; echo "admin:$(openssl passwd -apr1 password)" > /etc/htpasswd'
sudo chmod 644 /etc/htpasswd
sudo chown root:root /etc/htpasswd

# Install vjbctl as a system-wide command in /usr/local/bin so it's available to all users (including root)
sudo tee /usr/local/bin/vjbctl > /dev/null << 'EOF'
#!/bin/bash
# Source the main script to load all functions (user management, support-bundle, etc.)
source /etc/pf9/pf9-htpasswd.sh
# Call the main entry point function, passing all command-line arguments
# "$@" expands to all arguments passed to this script (e.g., "user create admin" becomes three separate args)
_pf9_ht_main "$@"
EOF
sudo chmod +x /usr/local/bin/vjbctl

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

  # Wait for network availability before installing K3s
  wait_for_network

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

  # Start the rsync daemon
  kubectl apply -f /etc/pf9/yamls/daemonset.yaml
  check_command "Installing rsync daemon"

  log "Rsync daemon started successfully."

  # Create a config map from env file. 
  kubectl create configmap pf9-env -n migration-system --from-env-file=/etc/pf9/env
  check_command "Creating config map from env file"
  log "Config map created successfully."

  log "Installing cert-manager"
  if [ -d "/etc/pf9/yamls/cert-manager" ]; then
      sudo kubectl apply -f /etc/pf9/yamls/cert-manager/cert-manager.yaml
      check_command "Applying cert-manager manifests"
      log "Waiting for cert-manager deployments to become available"
      sudo kubectl -n cert-manager wait --for=condition=Available deployment --all --timeout=300s
      check_command "Waiting for cert-manager deployments"
      if [ -f "/etc/pf9/yamls/cert-manager/00-selfsigned-issuer.yaml" ]; then
          sudo kubectl apply -f /etc/pf9/yamls/cert-manager/00-selfsigned-issuer.yaml
          check_command "Applying private CA setup"
      fi
    else
      log "WARNING: /etc/pf9/yamls/cert-manager not found. Skipping cert-manager installation."
  fi

  install_time_settings_apply_script

else
  log "Setting up K3s Worker..."

  # Check required variables for worker setup
  if [ -z "$MASTER_IP" ] || [ -z "$K3S_TOKEN" ]; then
    log "ERROR: Missing MASTER_IP or K3S_TOKEN for worker. Exiting."
    exit 1
  fi

  # Wait for network availability before installing K3s
  wait_for_network
  
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
log "removing the cron job"
# Remove cron job to ensure this runs only once 
sed -i 's;^@reboot root /etc/pf9/install.sh;;' /etc/crontab
check_command "Removing cron job"
# End of script
exit 0
