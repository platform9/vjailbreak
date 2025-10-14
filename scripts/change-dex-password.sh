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

# Check arguments
USERNAME=${1:-}
NEW_PASSWORD=${2:-}

if [ -z "$USERNAME" ] || [ -z "$NEW_PASSWORD" ]; then
    error "Usage: $0 <username> <new_password>
    
Example: $0 admin 'MyNewSecurePassword123!'

This script will:
1. Generate a bcrypt hash of the new password
2. Update the Dex ConfigMap with the new password
3. Restart the Dex deployment to apply changes"
fi

if [ ${#NEW_PASSWORD} -lt 12 ]; then
    warn "Password is less than 12 characters. Consider using a stronger password."
fi

log "Changing password for user: $USERNAME"

# Verify kubectl is available
if ! kubectl cluster-info &> /dev/null; then
    error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
fi

# Generate password hash using Dex
log "Generating bcrypt hash for new password..."
PASSWORD_HASH=$(kubectl exec -n dex deploy/dex -- /usr/local/bin/dex hash-password "$NEW_PASSWORD" 2>/dev/null | grep -v "Defaulted" | tail -n 1)

if [ -z "$PASSWORD_HASH" ]; then
    error "Failed to generate password hash"
fi

log "Password hash generated successfully"

# Get current ConfigMap
log "Retrieving current Dex configuration..."
kubectl get configmap dex-config -n dex -o yaml > /tmp/dex-config-backup.yaml

# Update the ConfigMap
log "Updating Dex ConfigMap with new password..."

# Create a temporary file with updated config
cat > /tmp/update-password.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: dex-config
  namespace: dex
data:
  config.yaml: |
    issuer: http://\${HOST_IP}:5556/dex
    
    storage:
      type: kubernetes
      config:
        inCluster: true
    
    web:
      http: 0.0.0.0:5556
      allowedOrigins: ['*']
    
    telemetry:
      http: 0.0.0.0:5558
    
    oauth2:
      skipApprovalScreen: true
      responseTypes: ["code", "token", "id_token"]
      
    enablePasswordDB: true
    
    staticPasswords:
    - email: "${USERNAME}@vjailbreak.local"
      hash: "$PASSWORD_HASH"
      username: "$USERNAME"
      userID: "08a8684b-db88-4b73-90a9-3cd1661f5466"
      
    connectors:
    - type: local
      id: local
      name: Local
    
    staticClients:
    - id: vjailbreak-ui
      redirectURIs:
        - 'http://\${HOST_IP}/oauth2/callback'
      name: 'vJailbreak UI'
      secret: vjailbreak-ui-secret-change-me
      
    - id: vjailbreak-cli
      redirectURIs:
        - 'urn:ietf:wg:oauth:2.0:oob'
        - 'http://localhost:8000'
        - 'http://localhost:5555/callback'
      name: 'vJailbreak CLI'
      public: true
EOF

# Get HOST_IP from current config or environment
HOST_IP=$(kubectl get configmap dex-config -n dex -o jsonpath='{.data.config\.yaml}' | grep -oP 'issuer: http://\K[^:]+' || echo "HOST_IP")

# Replace HOST_IP placeholder
sed -i "s/\${HOST_IP}/$HOST_IP/g" /tmp/update-password.yaml

# Apply the updated ConfigMap
kubectl apply -f /tmp/update-password.yaml

log "ConfigMap updated successfully"

# Restart Dex deployment
log "Restarting Dex deployment..."
kubectl rollout restart deployment/dex -n dex

log "Waiting for Dex to be ready..."
kubectl rollout status deployment/dex -n dex

log "Password changed successfully!"
log "Backup of old configuration saved to: /tmp/dex-config-backup.yaml"
log ""
log "New credentials:"
echo "  - Username: ${USERNAME}@vjailbreak.local"
echo "  - Password: [hidden]"
echo ""
log "You can now login with the new password"

# Clean up temporary files
rm -f /tmp/update-password.yaml
