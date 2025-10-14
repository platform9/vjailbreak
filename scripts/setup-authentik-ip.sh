#!/bin/bash
# Script to configure Authentik RBAC for IP-based deployment
# Usage: ./setup-authentik-ip.sh <IP_ADDRESS>

set -e

if [ -z "$1" ]; then
  echo "Usage: $0 <IP_ADDRESS>"
  echo "Example: $0 10.9.3.50"
  exit 1
fi

IP_ADDRESS=$1
echo "Configuring Authentik for IP: $IP_ADDRESS"

# Generate random secrets
POSTGRES_PASSWORD=$(openssl rand -base64 32)
AUTHENTIK_SECRET_KEY=$(openssl rand -base64 32)
COOKIE_SECRET=$(python3 -c 'import os,base64; print(base64.urlsafe_b64encode(os.urandom(32)).decode())')

echo "Generated secrets. Updating configuration files..."

# Create secrets directory if it doesn't exist
DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../deploy" && pwd)"
AUTHENTIK_DIR="$DEPLOY_DIR/authentik"
OAUTH2_DIR="$DEPLOY_DIR/oauth2-proxy"

# Update postgres secret
cat > "$AUTHENTIK_DIR/00-secrets.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: authentik-postgres-secret
  namespace: authentik
type: Opaque
stringData:
  username: "authentik"
  password: "$POSTGRES_PASSWORD"
EOF

# Update Authentik secret key
sed -i.bak "s|CHANGE_ME_GENERATE_RANDOM_KEY|$AUTHENTIK_SECRET_KEY|g" "$AUTHENTIK_DIR/04-authentik-server.yaml"

# Update OAuth2 proxy config with IP
sed -i.bak "s|http://10.9.3.50|http://$IP_ADDRESS|g" "$OAUTH2_DIR/01-deployment.yaml"

# Update OAuth2 proxy cookie secret
sed -i.bak "s|GENERATE_WITH_python_-c_'import os,base64; print(base64.urlsafe_b64encode(os.urandom(32)).decode())'|$COOKIE_SECRET|g" "$OAUTH2_DIR/01-deployment.yaml"

echo ""
echo "✓ Configuration updated for IP: $IP_ADDRESS"
echo ""
echo "IMPORTANT: You must manually update the following in OAuth2 proxy secret:"
echo "  - client-id: Get from Authentik after OIDC provider setup"
echo "  - client-secret: Get from Authentik after OIDC provider setup"
echo ""
echo "File: $OAUTH2_DIR/01-deployment.yaml"
echo ""
echo "Next steps:"
echo "1. Deploy Authentik: kubectl apply -f $AUTHENTIK_DIR/"
echo "2. Access Authentik: http://$IP_ADDRESS/authentik"
echo "3. Create OIDC provider with redirect: http://$IP_ADDRESS/oauth2/callback"
echo "4. Update OAuth2 proxy secret with client-id and client-secret"
echo "5. Deploy OAuth2 proxy: kubectl apply -f $OAUTH2_DIR/"
echo ""
