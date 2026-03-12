#!/bin/bash
# This script downloads only the version-specific vjailbreak container images.
# Static images (monitoring stack, cert-manager, ingress-nginx, virtio-win, etc.)
# are already baked into the base image via download_base_images.sh.
#
# Usage: ./download_images.sh <TAG>
# Example: ./download_images.sh v1.0.0
set -euo pipefail

if [ $# -lt 1 ]; then
  echo "Usage: $0 <TAG>"
  echo "Example: $0 v1.0.0"
  exit 1
fi

TAG=$1

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_BUILDER_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${IMAGE_BUILDER_DIR}/images"

mkdir -p "$OUTPUT_DIR"

# Version-specific vjailbreak images
v2v_helper="quay.io/platform9/vjailbreak-v2v-helper:$TAG"
controller="quay.io/platform9/vjailbreak-controller:$TAG"
ui="quay.io/platform9/vjailbreak-ui:$TAG"
vpwned="quay.io/platform9/vjailbreak-vpwned:$TAG"

# Download and export version-specific images
images=(
  "$v2v_helper"
  "$controller"
  "$ui"
  "$vpwned"
)

for img in "${images[@]}"; do
  echo "[*] Pulling $img"
  sudo ctr i pull --platform linux/amd64 "$img"

  tag=$(echo "$img" | cut -d'@' -f1)
  fname=$(echo "$tag" | tr '/:@' '_')

  echo "[*] Exporting to $fname.tar"
  sudo ctr i export "$OUTPUT_DIR/$fname.tar" "$img"
done

echo "[✔] Version-specific images downloaded and exported as tar files."
echo ""
echo "Next steps:"
echo "  1. Run: packer build vjailbreak-image.pkr.hcl"