#!/bin/bash

set -euo pipefail

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a /var/log/pf9-install.log
}

wait_for_ns_ready() {
  local ns=$1
  local timeout=${2:-300}
  local start=$(date +%s)
  while true; do
    # ready when all deployments available
    if kubectl -n "$ns" get deploy >/dev/null 2>&1; then
      local unavail
      unavail=$(kubectl -n "$ns" get deploy -o jsonpath='{range .items[*]}{.status.unavailableReplicas}{"\n"}{end}' | grep -v '^$' || true)
      if [ -z "$unavail" ]; then
        log "Namespace '$ns' deployments are available."
        return 0
      fi
    fi
    local now=$(date +%s)
    if [ $((now-start)) -gt $timeout ]; then
      log "Timeout waiting for namespace '$ns' to be ready."
      kubectl -n "$ns" get all || true
      return 1
    fi
    sleep 5
  done
}

main() {
  log "Installing cert-manager..."
  # Prefer vendored manifest if present inside the image
  if [ -f "/etc/pf9/yamls/cert-manager/cert-manager.yaml" ]; then
    log "Applying vendored cert-manager manifest"
    kubectl apply -f /etc/pf9/yamls/cert-manager/cert-manager.yaml
  else
    # Fallback to online install (requires internet)
    log "Vendored cert-manager manifest not found, installing from upstream"
    CM_VER="v1.15.3"
    kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CM_VER}/cert-manager.yaml"
  fi

  # Wait for cert-manager to come up
  wait_for_ns_ready cert-manager 600

  # Apply ClusterIssuers if present
  if [ -f "/etc/pf9/yamls/cert-manager/cluster-issuers.yaml" ]; then
    log "Applying ClusterIssuers"
    kubectl apply -f /etc/pf9/yamls/cert-manager/cluster-issuers.yaml
  else
    log "ClusterIssuers file not found at /etc/pf9/yamls/cert-manager/cluster-issuers.yaml (skipping)"
  fi

  log "cert-manager installation step completed."
}

main "$@"
