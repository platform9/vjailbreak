#!/bin/bash 
# This script downloads the ingress-nginx controller and kube-webhook-certgen images and exports them as tar files.
# But this script has been run already in base image, so this is just a placeholder for now.
# In case in the future we need to download images, we can use this script.
set -euo pipefail

REGISTRY="quay.io"
REPO="platform9"
kube_state_metrics="registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.13.0"
prometheus_adapter="registry.k8s.io/prometheus-adapter/prometheus-adapter:v0.12.0"
prometheus="quay.io/prometheus/prometheus:v2.46.0"
alertmanager="quay.io/prometheus/alertmanager:v0.27.0"
blackbox_exporter="quay.io/prometheus/blackbox-exporter:v0.25.0"
node_exporter="quay.io/prometheus/node-exporter:v1.6.1"
pushgateway="quay.io/prometheus/pushgateway:v1.5.0"
kube_rbac_proxy="quay.io/brancz/kube-rbac-proxy:v0.19.1"
prometheus_config_reloader="quay.io/prometheus-operator/prometheus-config-reloader:v0.76.0"
prometheus_operator="quay.io/prometheus-operator/prometheus-operator:v0.76.0"

# Download and export images
images=(
  "$kube_state_metrics"
  "$prometheus_adapter"
  "$prometheus"
  "$alertmanager"
  "$blackbox_exporter"
  "$node_exporter"
  "$pushgateway"
  "$kube_rbac_proxy"
  "$prometheus_config_reloader"
  "$prometheus_operator"
)

for img in "${images[@]}"; do
  echo "[*] Pulling $img"
  sudo ctr i pull "$img"

  tag=$(echo "$img" | cut -d'@' -f1)
  fname=$(echo "$tag" | tr '/:@' '_')

  echo "[*] Exporting to $fname.tar"
  sudo ctr i export "/etc/pf9/images/$fname.tar" "$img"
done

echo "[✔] All images downloaded and exported as tar files."
