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
EMAIL=${2:-}
PASSWORD=${3:-}
GROUPS=${4:-"vjailbreak-viewers"}

if [ -z "$USERNAME" ] || [ -z "$EMAIL" ] || [ -z "$PASSWORD" ]; then
    error "Usage: $0 <username> <email> <password> [groups]
    
Example: $0 operator1 operator1@vjailbreak.local 'SecurePass123!' 'vjailbreak-operators'

Available groups:
  - vjailbreak-admins (full access)
  - vjailbreak-operators (create migrations, read credentials)
  - vjailbreak-viewers (read-only access)
  - vjailbreak-credential-managers (manage credentials only)

Multiple groups can be specified separated by commas"
fi

log "Adding new user: $USERNAME ($EMAIL)"
log "Groups: $GROUPS"

# Verify kubectl is available
if ! kubectl cluster-info &> /dev/null; then
    error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
fi

# Generate password hash
log "Generating password hash..."
PASSWORD_HASH=$(kubectl exec -n dex deploy/dex -- /usr/local/bin/dex hash-password "$PASSWORD" 2>/dev/null | grep -v "Defaulted" | tail -n 1)

if [ -z "$PASSWORD_HASH" ]; then
    error "Failed to generate password hash"
fi

# Generate UUID for user
USER_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')

log "Generated user ID: $USER_ID"

# Get current ConfigMap
kubectl get configmap dex-config -n dex -o yaml > /tmp/dex-config-current.yaml

# Extract current config
CURRENT_CONFIG=$(kubectl get configmap dex-config -n dex -o jsonpath='{.data.config\.yaml}')

# Check if user already exists
if echo "$CURRENT_CONFIG" | grep -q "username: \"$USERNAME\""; then
    error "User $USERNAME already exists. Please use change-dex-password.sh to update password."
fi

# Create new user entry
USER_ENTRY="    - email: \"$EMAIL\"
      hash: \"$PASSWORD_HASH\"
      username: \"$USERNAME\"
      userID: \"$USER_ID\""

# Add user to staticPasswords section
# This is a simplified version - for production, use a proper YAML parser
log "Updating Dex configuration..."

# Get the existing config and append new user
kubectl get configmap dex-config -n dex -o jsonpath='{.data.config\.yaml}' | \
  awk -v user="$USER_ENTRY" '/staticPasswords:/{print; print user; next}1' > /tmp/new-config.yaml

# Create updated ConfigMap
kubectl create configmap dex-config --from-file=config.yaml=/tmp/new-config.yaml \
  -n dex --dry-run=client -o yaml | kubectl apply -f -

log "ConfigMap updated successfully"

# Create RBAC bindings for the user
log "Creating RBAC bindings..."

IFS=',' read -ra GROUP_ARRAY <<< "$GROUPS"
for group in "${GROUP_ARRAY[@]}"; do
    group=$(echo "$group" | xargs) # trim whitespace
    
    case $group in
        vjailbreak-admins)
            ROLE="vjailbreak-admin"
            ;;
        vjailbreak-operators)
            ROLE="vjailbreak-operator"
            ;;
        vjailbreak-viewers)
            ROLE="vjailbreak-viewer"
            ;;
        vjailbreak-credential-managers)
            ROLE="vjailbreak-credential-manager"
            ;;
        *)
            warn "Unknown group: $group, skipping..."
            continue
            ;;
    esac
    
    # Create ClusterRoleBinding for the user
    cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: vjailbreak-${USERNAME}-${group}-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: $ROLE
subjects:
- kind: User
  name: $EMAIL
  apiGroup: rbac.authorization.k8s.io
EOF
    
    log "Created binding for $EMAIL to role $ROLE"
done

# Restart Dex deployment
log "Restarting Dex deployment..."
kubectl rollout restart deployment/dex -n dex
kubectl rollout status deployment/dex -n dex

log "User added successfully!"
log ""
log "User credentials:"
echo "  - Username: $EMAIL"
echo "  - Password: [hidden]"
echo "  - Groups: $GROUPS"
echo ""
log "The user can now login to vJailbreak"

# Clean up
rm -f /tmp/new-config.yaml /tmp/dex-config-current.yaml
