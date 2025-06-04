#!/bin/bash 
# This script downloads the ingress-nginx controller and kube-webhook-certgen images and exports them as tar files.
# But this script has been run already in base image, so this is just a placeholder for now.
# In case in the future we need to download images, we can use this script.
set -euo pipefail

REGISTRY="quay.io"
REPO="platform9"


echo "[*] Fetching latest ingress-nginx controller tag from GitHub..."
controller_tag=$(curl -s https://api.github.com/repos/kubernetes/ingress-nginx/releases/latest | jq -r .tag_name)
echo "[+] Latest tag: $controller_tag"

# Download values.yaml from GitHub to extract image info
values_url="https://raw.githubusercontent.com/kubernetes/ingress-nginx/${controller_tag}/charts/ingress-nginx/values.yaml"
values_file=$(mktemp)
curl -sL "$values_url" -o "$values_file"

# Extract digests
controller_digest=$(awk '/controller:/{f=1} f && /digest:/{print $2; exit}' "$values_file")
certgen_digest=$(awk '/kube-webhook-certgen/{f=1} f && /digest:/{print $2; exit}' "$values_file")

# pull image references for nginx
controller_image="registry.k8s.io/ingress-nginx/controller@${controller_digest}"
certgen_image="registry.k8s.io/ingress-nginx/kube-webhook-certgen@${certgen_digest}"

# Download and export images
images=(
  "$controller_image"
  "$certgen_image"
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

# install k3s binaries and airgap images
# move files to /etc/pf9  and set permissions
sudo mkdir -p /etc/pf9
sudo mkdir -p /etc/pf9/k3s-setup
sudo mkdir -p /var/lib/rancher/k3s/agent/images
      
# install k3s binary and tar file it needs. 
echo "[*] Downloading k3s-install.sh"
sudo curl -sfL https://get.k3s.io -o /etc/pf9/k3s-setup/k3s-install.sh
sudo chmod +x /etc/pf9/k3s-setup/k3s-install.sh

echo "[*] Downloading k3s binary"
sudo curl -L https://github.com/k3s-io/k3s/releases/download/v1.33.1%2Bk3s1/k3s -o /usr/local/bin/k3s
sudo chmod +x /usr/local/bin/k3s

echo "[*] Downloading k3s-airgap-images-amd64.tar.zst"
sudo curl -LO https://github.com/k3s-io/k3s/releases/download/v1.33.1%2Bk3s1/k3s-airgap-images-amd64.tar.zst
echo "[*] Moving k3s-airgap-images-amd64.tar.zst to /var/lib/rancher/k3s/agent/images/"
sudo mv k3s-airgap-images-amd64.tar.zst /var/lib/rancher/k3s/agent/images/