#!/bin/bash
# This script downloads static container images that don't change between releases.
# These images are baked into the base image to speed up release builds.
# Run this script ONCE before building the base image.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_BUILDER_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${IMAGE_BUILDER_DIR}/base-images"

mkdir -p "$OUTPUT_DIR"

# Static images that don't change per release (monitoring stack, cert-manager, ingress-nginx, etc.)
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

CERT_MANAGER_VERSION="v1.16.1"
cert_manager_controller="quay.io/jetstack/cert-manager-controller:${CERT_MANAGER_VERSION}"
cert_manager_webhook="quay.io/jetstack/cert-manager-webhook:${CERT_MANAGER_VERSION}"
cert_manager_cainjector="quay.io/jetstack/cert-manager-cainjector:${CERT_MANAGER_VERSION}"

ingress_nginx_controller="registry.k8s.io/ingress-nginx/controller@sha256:4eea9a4cc2cb6ddcb7da14d377aaf452e68bd3dbe87fe280755d225c4d5e7e4e"
kube_webhook_certgen="registry.k8s.io/ingress-nginx/kube-webhook-certgen@sha256:d7e8257f8d8bce64b6df55f81fba92011a6a77269b3350f8b997b152af348dba"

# TODO(suhas): Create a separate repository for alpine image in quay
alpine="quay.io/platform9/vjailbreak:alpine"

virtiowin="https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso"
virtiowinserver12="https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.185-1/virtio-win-0.1.185.iso"

# Download cert-manager manifests
CERT_MANAGER_URL="https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
MANIFEST_PATH="${IMAGE_BUILDER_DIR}/cert-manager-manifests/cert-manager.yaml"
mkdir -p "${IMAGE_BUILDER_DIR}/cert-manager-manifests"
echo "[*] Downloading cert-manager manifests..."
curl -L "${CERT_MANAGER_URL}" -o "${MANIFEST_PATH}"
sed -i 's/imagePullPolicy: Always/imagePullPolicy: IfNotPresent/g' "${MANIFEST_PATH}"

# Static images array
images=(
  "$kube_state_metrics"
  "$prometheus_adapter"
  "$prometheus"
  "$alertmanager"
  "$blackbox_exporter"
  "$node_exporter"
  "$pushgateway"
  "$prometheus_config_reloader"
  "$prometheus_operator"
  "$configmap_reload"
  "$grafana"
  "$alpine"
  "$cert_manager_controller"
  "$cert_manager_webhook"
  "$cert_manager_cainjector"
  "$ingress_nginx_controller"
  "$kube_webhook_certgen"
)

for img in "${images[@]}"; do
  echo "[*] Pulling $img"
  sudo ctr i pull --platform linux/amd64 "$img"

  tag=$(echo "$img" | cut -d'@' -f1)
  fname=$(echo "$tag" | tr '/:@' '_')

  echo "[*] Exporting to $fname.tar"
  sudo ctr i export "$OUTPUT_DIR/$fname.tar" "$img"
done

# kube-rbac-proxy needs all platforms
echo "[*] Pulling kube-rbac-proxy (all platforms)"
ctr images pull --all-platforms quay.io/brancz/kube-rbac-proxy:v0.20.2
sleep 10
ctr images export "$OUTPUT_DIR/kube-rbac-proxy.tar" quay.io/brancz/kube-rbac-proxy:v0.20.2

echo "[✔] All static images downloaded and exported as tar files."

# Download virtio-win ISOs
echo "[*] Downloading virtio-win.iso"
wget -O "$OUTPUT_DIR/virtio-win.iso" "$virtiowin"

echo "[*] Downloading virtio-win-server12.iso"
wget -O "$OUTPUT_DIR/virtio-win-server12.iso" "$virtiowinserver12"

echo "[✔] Base images download complete. Output directory: $OUTPUT_DIR"
echo ""
echo "Next steps:"
echo "  1. Run: packer build vjailbreak-base-image.pkr.hcl"
echo "  2. Upload the base image to your image repository"
