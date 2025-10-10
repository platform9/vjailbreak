#!/bin/bash

set -e

# Simple Authentik configuration using kubectl exec
AUTHENTIK_NAMESPACE="authentik"
ADMIN_EMAIL="admin@vjailbreak.local"
ADMIN_PASSWORD="vjb!@#"
APP_NAME="vjailbreak"
VM_IP=$(hostname -I | awk '{print $1}')
AUTHENTIK_PORT="30900"  # NodePort for Authentik
CLIENT_ID="vjailbreak-oauth2"
CLIENT_SECRET=$(openssl rand -base64 32)
COOKIE_SECRET=$(openssl rand -hex 16)  # Exactly 32 bytes for AES

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

log "=========================================="
log "Authentik Setup for vJailbreak"
log "=========================================="

# Wait for Authentik
log "Waiting for Authentik to be ready..."
kubectl wait --for=condition=ready pod -l app=authentik-server -n $AUTHENTIK_NAMESPACE --timeout=300s
sleep 10

POD=$(kubectl get pod -n $AUTHENTIK_NAMESPACE -l app=authentik-server -o jsonpath='{.items[0].metadata.name}')
log "Using pod: $POD"

# Create a token for API access using the bootstrap admin
log "Creating API token..."
TOKEN=$(kubectl exec -n $AUTHENTIK_NAMESPACE $POD -- ak create_token -e 3600 akadmin 2>/dev/null || echo "")

if [ -z "$TOKEN" ]; then
  log "Failed to create token, trying alternative method..."
  # Try using the bootstrap password to login and get a token
  log "Manual configuration required in Authentik UI"
  log ""
  log "Next steps:"
  log ""
  log "1. Access Authentik at: http://$VM_IP:$AUTHENTIK_PORT/"
  log "   Email: $ADMIN_EMAIL"
  log "   Password: $ADMIN_PASSWORD"
  log ""
  log "2. Create an OAuth2/OpenID Provider:"
  log "   - Name: vjailbreak-provider"
  log "   - Client ID: $CLIENT_ID"
  log "   - Client Secret: $CLIENT_SECRET"
  log "   - Redirect URI: http://$VM_IP/oauth2/callback"
  log ""
  log "3. Create an Application:"
  log "   - Name: vjailbreak"
  log "   - Slug: vjailbreak"
  log "   - Provider: vjailbreak-provider"
  log ""
  log "4. Create groups: platform9-admins, platform9-users, platform9-viewers"
  log ""
  
  # Update the oauth2-proxy secret with the credentials
  log "Configuring oauth2-proxy with generated credentials..."
  kubectl patch secret oauth2-proxy-secret -n migration-system --type=json -p="[
    {\"op\": \"replace\", \"path\": \"/data/client-id\", \"value\": \"$(echo -n $CLIENT_ID | base64 | tr -d '\n')\"},
    {\"op\": \"replace\", \"path\": \"/data/client-secret\", \"value\": \"$(echo -n $CLIENT_SECRET | base64 | tr -d '\n')\"},
    {\"op\": \"replace\", \"path\": \"/data/cookie-secret\", \"value\": \"$(echo -n $COOKIE_SECRET | base64 | tr -d '\n')\"}
  ]" 2>/dev/null || log "WARNING: Failed to patch oauth2-proxy secret. May need to create it first."
  
  # Save credentials
  mkdir -p /etc/pf9
  cat > /etc/pf9/authentik-credentials.txt <<EOF
===========================================
Authentik Configuration for vJailbreak
===========================================

Authentik Web UI:
  URL: http://$VM_IP:$AUTHENTIK_PORT/
  
Admin Credentials:
  Email: $ADMIN_EMAIL
  Password: $ADMIN_PASSWORD
  
OAuth2 Provider Configuration (use these in Authentik UI):
  Provider Name: vjailbreak-provider
  Client ID: $CLIENT_ID
  Client Secret: $CLIENT_SECRET
  Redirect URI: http://$VM_IP/oauth2/callback
  Authorization Flow: default-provider-authorization-implicit-consent
  
Application Configuration:
  Name: vjailbreak
  Slug: vjailbreak
  Provider: vjailbreak-provider

RBAC Groups to Create:
  - platform9-admins (full access)
  - platform9-users (read-write)
  - platform9-viewers (read-only)

After configuring in Authentik UI, restart oauth2-proxy:
  kubectl rollout restart deployment oauth2-proxy -n migration-system

EOF
  
  chmod 600 /etc/pf9/authentik-credentials.txt
  log ""
  log "Credentials saved to /etc/pf9/authentik-credentials.txt"
  log ""
  log "OAuth2-proxy has been configured automatically."
  log "Complete the manual steps in Authentik UI, then restart oauth2-proxy."
  log "=========================================="
  exit 0
fi

log "Token created successfully, proceeding with automated setup..."
log "This feature is not yet implemented - please follow manual steps above"

exit 0
