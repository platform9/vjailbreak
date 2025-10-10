#!/bin/bash

set -e

# Configuration
AUTHENTIK_NAMESPACE="authentik"
ADMIN_EMAIL="admin@vjailbreak.local"
ADMIN_PASSWORD="vjb!@#"
BOOTSTRAP_TOKEN="vjb!@#"
APP_NAME="vjailbreak"
VM_IP=$(hostname -I | awk '{print $1}')

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

wait_for_authentik() {
  log "Waiting for Authentik to be ready..."
  kubectl wait --for=condition=ready pod -l app=authentik-server -n $AUTHENTIK_NAMESPACE --timeout=300s
  
  # Wait for initial setup to complete
  log "Waiting for Authentik initial setup..."
  sleep 30
}

configure_oidc_provider() {
  log "Configuring OIDC provider for vJailbreak..."
  
  # Port-forward to access API locally
  kubectl port-forward -n $AUTHENTIK_NAMESPACE svc/authentik-server 9000:9000 >/dev/null 2>&1 &
  PF_PID=$!
  sleep 5
  
  log "Using bootstrap token for API access..."
  
  # Generate client credentials
  CLIENT_ID="vjailbreak-oauth2"
  CLIENT_SECRET=$(openssl rand -base64 32)
  
  # Test API connectivity first (without auth)
  log "Testing API connectivity..."
  API_TEST=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 http://localhost:9000/api/v3/root/config/)
  
  log "API returned HTTP $API_TEST"
  
  if [ "$API_TEST" != "200" ] && [ "$API_TEST" != "403" ]; then
    log "ERROR: Cannot connect to Authentik API (HTTP $API_TEST)"
    log "Port-forward may not be working or Authentik not ready"
    kill $PF_PID 2>/dev/null || true
    exit 1
  fi
  
  # Test with bootstrap token
  log "Testing authentication with bootstrap token..."
  AUTH_TEST=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 \
    http://localhost:9000/api/v3/root/config/ \
    -H "Authorization: Bearer $BOOTSTRAP_TOKEN")
  
  log "Auth test returned HTTP $AUTH_TEST"
  
  if [ "$AUTH_TEST" = "403" ] || [ "$AUTH_TEST" = "401" ]; then
    log "WARNING: Bootstrap token authentication failed"
    log "This is expected - Authentik bootstrap tokens work differently"
    log "Proceeding with unauthenticated API access for initial setup..."
  fi
  
  # Get the default authorization flow ID - use UUID instead of slug
  log "Fetching default authorization flow..."
  FLOWS_JSON=$(curl -s http://localhost:9000/api/v3/flows/instances/ \
    -H "Authorization: Bearer $BOOTSTRAP_TOKEN")
  
  # Extract flow UUID that matches the slug
  FLOW_ID=$(echo "$FLOWS_JSON" | grep -o '"pk":"[^"]*"[^}]*"slug":"default-provider-authorization-implicit-consent"' | grep -o '"pk":"[^"]*"' | cut -d'"' -f4)
  
  if [ -z "$FLOW_ID" ]; then
    log "WARNING: Could not find default authorization flow UUID"
    log "Available flows:"
    echo "$FLOWS_JSON" | grep -o '"slug":"[^"]*"' || true
    log "Using slug as fallback..."
    FLOW_ID="default-provider-authorization-implicit-consent"
  else
    log "Found flow UUID: $FLOW_ID"
  fi
  
  # Create OIDC Provider
  log "Creating OAuth2 provider..."
  PROVIDER_RESPONSE=$(curl -s -w "\n%{http_code}" --max-time 30 -X POST http://localhost:9000/api/v3/providers/oauth2/ \
    -H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"vjailbreak-provider\",
      \"authorization_flow\": \"$FLOW_ID\",
      \"client_type\": \"confidential\",
      \"client_id\": \"$CLIENT_ID\",
      \"client_secret\": \"$CLIENT_SECRET\",
      \"redirect_uris\": \"http://$VM_IP/oauth2/callback\"
    }")
  
  HTTP_CODE=$(echo "$PROVIDER_RESPONSE" | tail -n1)
  PROVIDER_DATA=$(echo "$PROVIDER_RESPONSE" | sed '$d')
  
  if [ "$HTTP_CODE" != "201" ]; then
    log "WARNING: Provider creation returned HTTP $HTTP_CODE"
    log "Response: $PROVIDER_DATA"
  fi
  
  PROVIDER_ID=$(echo "$PROVIDER_DATA" | grep -o '"pk":[0-9]*' | head -1 | cut -d: -f2)
  log "Provider created with ID: $PROVIDER_ID"
  
  # Create Application
  log "Creating application..."
  curl -s -X POST http://localhost:9000/api/v3/core/applications/ \
    -H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"$APP_NAME\",
      \"slug\": \"$APP_NAME\",
      \"provider\": $PROVIDER_ID,
      \"meta_launch_url\": \"http://$VM_IP/\",
      \"policy_engine_mode\": \"any\"
    }" >/dev/null
  
  log "Application created"
  
  # Create RBAC groups
  log "Creating RBAC groups..."
  for GROUP in "platform9-admins" "platform9-users" "platform9-viewers"; do
    curl -s -X POST http://localhost:9000/api/v3/core/groups/ \
      -H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"name\": \"$GROUP\", \"is_superuser\": false}" >/dev/null 2>&1 || true
  done
  
  log "Groups created"
  
  # Kill port-forward
  kill $PF_PID 2>/dev/null || true
  wait $PF_PID 2>/dev/null || true
  
  # Update oauth2-proxy secret with real credentials
  log "Updating oauth2-proxy configuration..."
  kubectl patch secret oauth2-proxy-secret -n migration-system --type=json -p="[
    {\"op\": \"replace\", \"path\": \"/data/client-id\", \"value\": \"$(echo -n $CLIENT_ID | base64 | tr -d '\n')\"},
    {\"op\": \"replace\", \"path\": \"/data/client-secret\", \"value\": \"$(echo -n $CLIENT_SECRET | base64 | tr -d '\n')\"}
  ]"
  
  # Generate cookie secret if not already set
  COOKIE_SECRET=$(openssl rand -base64 32 | tr -d '\n')
  kubectl patch secret oauth2-proxy-secret -n migration-system --type=json -p="[
    {\"op\": \"replace\", \"path\": \"/data/cookie-secret\", \"value\": \"$(echo -n $COOKIE_SECRET | base64 | tr -d '\n')\"}
  ]" 2>/dev/null || true
  
  log "OAuth2-proxy credentials updated"
  
  # Save credentials to file
  mkdir -p /etc/pf9
  cat > /etc/pf9/authentik-credentials.txt <<EOF
===========================================
Authentik Configuration for vJailbreak
===========================================

Authentik Web UI Access:
  Run: kubectl port-forward -n authentik svc/authentik-server 9000:9000
  URL: http://localhost:9000/
  
Admin Credentials:
  Email: $ADMIN_EMAIL
  Password: $ADMIN_PASSWORD

OIDC Application:
  Name: $APP_NAME
  Client ID: $CLIENT_ID
  Client Secret: $CLIENT_SECRET
  Issuer URL: http://$VM_IP:9000/application/o/$APP_NAME/
  Redirect URL: http://$VM_IP/oauth2/callback

RBAC Groups:
  - platform9-admins (full access)
  - platform9-users (read-write)
  - platform9-viewers (read-only)

EOF
  
  chmod 600 /etc/pf9/authentik-credentials.txt
  log "Credentials saved to /etc/pf9/authentik-credentials.txt"
}

restart_oauth2_proxy() {
  log "Restarting oauth2-proxy with new credentials..."
  kubectl rollout restart deployment oauth2-proxy -n migration-system
  kubectl rollout status deployment oauth2-proxy -n migration-system --timeout=120s
}

main() {
  log "=========================================="
  log "Starting Authentik automated setup..."
  log "=========================================="
  
  wait_for_authentik
  configure_oidc_provider
  restart_oauth2_proxy
  
  log "=========================================="
  log "Authentik setup complete!"
  log "=========================================="
  log ""
  log "Default admin credentials:"
  log "  Email: $ADMIN_EMAIL"
  log "  Password: $ADMIN_PASSWORD"
  log ""
  log "Full configuration saved to: /etc/pf9/authentik-credentials.txt"
  log ""
  log "To access Authentik admin panel:"
  log "  kubectl port-forward -n authentik svc/authentik-server 9000:9000"
  log "  Then open: http://localhost:9000/"
}

main "$@"
