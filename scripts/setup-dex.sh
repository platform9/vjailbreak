#!/bin/bash

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

warn() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if running as root or with sudo
if [ "$EUID" -eq 0 ]; then
    warn "Running as root. Some kubectl commands may need adjustment."
fi

# Parse arguments
HOST_IP=${1:-}

if [ -z "$HOST_IP" ]; then
    error "Usage: $0 <HOST_IP>
    
Example: $0 10.9.2.145

This script will:
1. Install cert-manager for certificate management
2. Deploy Dex IdP with local authentication
3. Deploy OAuth2 Proxy for authentication
4. Setup RBAC roles and bindings
5. Update UI deployment with authentication"
fi

log "Starting Dex IdP setup for vJailbreak on host IP: $HOST_IP"

# Verify kubectl is available
if ! command -v kubectl &> /dev/null; then
    error "kubectl is not installed or not in PATH"
fi

# Verify cluster connectivity
if ! kubectl cluster-info &> /dev/null; then
    error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
fi

log "Kubernetes cluster is accessible"

# Function to replace HOST_IP in files
replace_host_ip() {
    local file=$1
    if [ -f "$file" ]; then
        sed -i.bak "s/HOST_IP/$HOST_IP/g" "$file"
        log "Updated $file with HOST_IP: $HOST_IP"
    else
        warn "File $file not found, skipping..."
    fi
}

# Create temporary directory for modified manifests
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

log "Created temporary directory: $TEMP_DIR"

# Base directory for manifests
BASE_DIR="/etc/pf9/yamls/k8s"
if [ ! -d "$BASE_DIR" ]; then
    BASE_DIR="$(pwd)/k8s"
    log "Using local k8s directory: $BASE_DIR"
fi

# Step 1: Install cert-manager if not already installed
log "Checking if cert-manager is installed..."
if ! kubectl get namespace cert-manager &> /dev/null; then
    log "Installing cert-manager..."
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.2/cert-manager.yaml
    
    log "Waiting for cert-manager to be ready..."
    kubectl wait --for=condition=Available --timeout=300s \
        deployment/cert-manager -n cert-manager
    kubectl wait --for=condition=Available --timeout=300s \
        deployment/cert-manager-webhook -n cert-manager
    kubectl wait --for=condition=Available --timeout=300s \
        deployment/cert-manager-cainjector -n cert-manager
    
    log "cert-manager installed successfully"
else
    log "cert-manager is already installed"
fi

# Step 2: Apply cert-manager issuers
log "Applying cert-manager issuers..."
if [ -f "$BASE_DIR/cert-manager/00-selfsigned-issuer.yaml" ]; then
    kubectl apply -f "$BASE_DIR/cert-manager/00-selfsigned-issuer.yaml"
    sleep 5
    log "Cert-manager issuers applied"
else
    warn "Cert-manager issuer file not found"
fi

# Step 3: Copy Dex manifests and replace HOST_IP
log "Preparing Dex manifests..."
cp -r "$BASE_DIR/dex" "$TEMP_DIR/"
for file in "$TEMP_DIR/dex"/*.yaml; do
    replace_host_ip "$file"
done

# Apply Dex manifests
log "Deploying Dex IdP..."
kubectl apply -f "$TEMP_DIR/dex/"

log "Waiting for Dex to be ready..."
kubectl wait --for=condition=Available --timeout=300s \
    deployment/dex -n dex || warn "Dex deployment timeout - check manually with: kubectl get pods -n dex"

# Step 4: Copy OAuth2 Proxy manifests and replace HOST_IP
log "Preparing OAuth2 Proxy manifests..."
cp -r "$BASE_DIR/oauth2-proxy" "$TEMP_DIR/"
for file in "$TEMP_DIR/oauth2-proxy"/*.yaml; do
    replace_host_ip "$file"
done

# Apply OAuth2 Proxy manifests
log "Deploying OAuth2 Proxy..."
kubectl apply -f "$TEMP_DIR/oauth2-proxy/"

log "Waiting for OAuth2 Proxy to be ready..."
kubectl wait --for=condition=Available --timeout=300s \
    deployment/oauth2-proxy -n oauth2-proxy || warn "OAuth2 Proxy deployment timeout - check manually"

# Step 5: Apply RBAC roles
log "Applying RBAC roles and bindings..."
if [ -d "$BASE_DIR/rbac" ]; then
    kubectl apply -f "$BASE_DIR/rbac/"
    log "RBAC roles applied successfully"
else
    warn "RBAC directory not found at $BASE_DIR/rbac"
fi

# Step 6: Update UI deployment
log "Updating UI deployment with authentication..."
UI_MANIFEST="/etc/pf9/yamls/deploy/04ui-with-dex.yaml"
if [ ! -f "$UI_MANIFEST" ]; then
    UI_MANIFEST="$BASE_DIR/../deploy/04ui-with-dex.yaml"
fi
if [ -f "$UI_MANIFEST" ]; then
    cp "$UI_MANIFEST" "$TEMP_DIR/04ui-with-dex.yaml"
    replace_host_ip "$TEMP_DIR/04ui-with-dex.yaml"
    kubectl apply -f "$TEMP_DIR/04ui-with-dex.yaml"
    
    log "Restarting UI deployment..."
    kubectl rollout restart deployment/vjailbreak-ui -n migration-system
    kubectl rollout status deployment/vjailbreak-ui -n migration-system
else
    warn "UI manifest not found at $UI_MANIFEST"
fi

# Generate password change instructions
log "=========================================="
log "Dex IdP Setup Complete!"
log "=========================================="
echo ""
log "Access URLs:"
echo "  - Main UI: http://$HOST_IP"
echo "  - Dex UI: http://$HOST_IP/dex"
echo "  - OAuth2 Callback: http://$HOST_IP/oauth2/callback"
echo ""
log "Default Credentials:"
echo "  - Username: admin@vjailbreak.local"
echo "  - Password: admin"
echo ""
warn "IMPORTANT: You MUST change the default password immediately!"
echo ""
log "To change the default password:"
echo "  1. Access http://$HOST_IP/dex/.well-known/openid-configuration"
echo "  2. Use the password change API or update the static password in Dex ConfigMap"
echo ""
log "To update admin password (recommended method):"
echo "  Run: kubectl exec -n dex deploy/dex -- /usr/local/bin/dex hash-password admin <new-password>"
echo "  Then update the ConfigMap with the new hash"
echo ""
log "RBAC Roles Created:"
echo "  - vjailbreak-admin: Full access to all resources"
echo "  - vjailbreak-operator: Can create migrations, read-only credentials"
echo "  - vjailbreak-viewer: Read-only access to all resources"
echo "  - vjailbreak-credential-manager: Can manage credentials only"
echo ""
log "Check deployment status:"
echo "  kubectl get pods -n dex"
echo "  kubectl get pods -n oauth2-proxy"
echo "  kubectl get pods -n migration-system"
echo ""
log "View logs:"
echo "  kubectl logs -n dex -l app=dex"
echo "  kubectl logs -n oauth2-proxy -l app=oauth2-proxy"
echo ""

# Test connectivity
log "Testing Dex connectivity..."
if curl -s -o /dev/null -w "%{http_code}" "http://$HOST_IP/dex/healthz" | grep -q "200"; then
    log "Dex is accessible at http://$HOST_IP/dex"
else
    warn "Cannot reach Dex at http://$HOST_IP/dex - please check ingress and service configuration"
fi

log "Setup completed successfully!"
