#!/bin/bash
set -euo pipefail

TAG=$1

# Always use k8s.io namespace for Kubernetes images
CTR="sudo ctr -n k8s.io"

mkdir -p image_builder/images
mkdir -p image_builder/cert-manager-manifests

# -----------------------------
# Image Definitions
# -----------------------------

kube_state_metrics="registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.18.0"
prometheus_adapter="registry.k8s.io/prometheus-adapter/prometheus-adapter:v0.12.0"
prometheus="quay.io/prometheus/prometheus:v3.9.1"
alertmanager="quay.io/prometheus/alertmanager:v0.31.1"
blackbox_exporter="quay.io/prometheus/blackbox-exporter:v0.28.0"
node_exporter="quay.io/prometheus/node-exporter:v1.10.2"
pushgateway="quay.io/prometheus/pushgateway:v1.5.0"
kube_rbac_proxy="quay.io/brancz/kube-rbac-proxy:v0.20.2"
prometheus_config_reloader="quay.io/prometheus-operator/prometheus-config-reloader:v0.89.0"
prometheus_operator="quay.io/prometheus-operator/prometheus-operator:v0.89.0"
configmap_reload="ghcr.io/jimmidyson/configmap-reload:v0.15.0"
grafana="docker.io/grafana/grafana:12.3.3"

v2v_helper="quay.io/platform9/vjailbreak-v2v-helper:$TAG"
controller="quay.io/platform9/vjailbreak-controller:$TAG"
ui="quay.io/platform9/vjailbreak-ui:$TAG"
vpwned="quay.io/platform9/vjailbreak-vpwned:$TAG"
alpine="quay.io/platform9/vjailbreak:alpine"

CERT_MANAGER_VERSION="v1.16.1"
cert_manager_controller="quay.io/jetstack/cert-manager-controller:${CERT_MANAGER_VERSION}"
cert_manager_webhook="quay.io/jetstack/cert-manager-webhook:${CERT_MANAGER_VERSION}"
cert_manager_cainjector="quay.io/jetstack/cert-manager-cainjector:${CERT_MANAGER_VERSION}"

INGRESS_NGINX_VERSION="v1.14.3"
KUBE_WEBHOOK_CERTGEN_VERSION="v1.6.7"
ingress_nginx_controller="registry.k8s.io/ingress-nginx/controller:${INGRESS_NGINX_VERSION}"
kube_webhook_certgen="registry.k8s.io/ingress-nginx/kube-webhook-certgen:${KUBE_WEBHOOK_CERTGEN_VERSION}"

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
  "$v2v_helper"
  "$controller"
  "$ui"
  "$configmap_reload"
  "$grafana"
  "$alpine"
  "$vpwned"
  "$cert_manager_controller"
  "$cert_manager_webhook"
  "$cert_manager_cainjector"
  "$ingress_nginx_controller"
  "$kube_webhook_certgen"
)

# -----------------------------
# Pull + Export Loop
# -----------------------------

for img in "${images[@]}"; do
  echo "[*] Pulling $img"

  # Pull all platforms to avoid manifest export errors
  $CTR images pull --all-platforms "$img"

  tag=$(echo "$img" | cut -d'@' -f1)
  fname=$(echo "$tag" | tr '/:@' '_')

  echo "[*] Exporting to image_builder/images/$fname.tar"
  $CTR images export "image_builder/images/$fname.tar" "$img"
done

echo "[✔] All images downloaded and exported successfully."