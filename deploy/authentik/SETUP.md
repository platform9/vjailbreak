# Authentik RBAC Setup - Automated Installation

This guide explains how to deploy and configure Authentik for vJailbreak RBAC automatically.

## Quick Start

### Automated Setup (Recommended)

Run the setup script after deploying the Authentik manifests:

```bash
# Deploy Authentik
kubectl apply -f /path/to/deploy/authentik/

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app=authentik-server -n authentik --timeout=300s

# Run automated configuration
./scripts/setup-authentik.sh
```

The script will:
1. Wait for Authentik to be fully initialized
2. Create an OIDC provider for vJailbreak
3. Configure oauth2-proxy with the correct credentials
4. Create RBAC groups (platform9-admins, platform9-users, platform9-viewers)
5. Save all credentials to `/etc/pf9/authentik-credentials.txt`

### Default Credentials

**Admin Account (auto-created on first start):**
- Email: `admin@vjailbreak.local`
- Password: `ChangeMe123!`

**Bootstrap Token:** `BOOTSTRAP_TOKEN_REPLACE_ME` (for API automation)

## Manual Configuration

If you prefer to configure manually:

### 1. Access Authentik Admin Panel

```bash
kubectl port-forward -n authentik svc/authentik-server 9000:9000
```

Open http://localhost:9000/ and log in with the admin credentials above.

### 2. Create OAuth2 Provider

1. Go to **Applications** → **Providers** → **Create**
2. Select **OAuth2/OpenID Provider**
3. Configure:
   - Name: `vjailbreak-provider`
   - Client Type: `Confidential`
   - Client ID: `vjailbreak-oauth2` (or generate one)
   - Client Secret: (generate a secure secret)
   - Redirect URIs: `http://<VM_IP>/oauth2/callback`
4. Save the provider

### 3. Create Application

1. Go to **Applications** → **Applications** → **Create**
2. Configure:
   - Name: `vjailbreak`
   - Slug: `vjailbreak`
   - Provider: Select the provider created above
   - Launch URL: `http://<VM_IP>/`
3. Save the application

### 4. Create RBAC Groups

Create three groups for role-based access:

1. **platform9-admins** - Full cluster access
2. **platform9-users** - Read-write access to migrations
3. **platform9-viewers** - Read-only access

### 5. Update oauth2-proxy Secret

```bash
kubectl patch secret oauth2-proxy-secret -n migration-system --type=json -p='[
  {"op": "replace", "path": "/data/client-id", "value": "'$(echo -n "vjailbreak-oauth2" | base64)'"},
  {"op": "replace", "path": "/data/client-secret", "value": "'$(echo -n "YOUR_CLIENT_SECRET" | base64)'"}
]'

# Restart oauth2-proxy
kubectl rollout restart deployment oauth2-proxy -n migration-system
```

## Architecture

```
User Browser
    ↓
nginx-ingress (/)
    ↓
oauth2-proxy (/oauth2)
    ↓
Authentik OIDC (:9000)
    ↓
vJailbreak UI/API (authenticated)
```

## Troubleshooting

### Authentik returns 404

The ingress path rewriting may not be configured correctly. Access Authentik directly:

```bash
kubectl port-forward -n authentik svc/authentik-server 9000:9000
```

### OAuth2-proxy can't reach Authentik

Check the oauth2-proxy configuration:

```bash
kubectl get configmap oauth2-proxy-config -n migration-system -o yaml
```

Verify `oidc_issuer_url` points to the internal service:
```
http://authentik-server.authentik.svc.cluster.local:9000/application/o/vjailbreak/
```

### Users can't log in

1. Check that users are assigned to the correct groups in Authentik
2. Verify group mappings in Kubernetes:
   ```bash
   kubectl get clusterrolebinding -l app=vjailbreak-rbac
   ```

## Security Notes

1. **Change default password** immediately after first login
2. **Bootstrap token** should be rotated or removed after setup
3. **HTTPS** should be configured for production (currently HTTP for simplicity)
4. Store credentials in `/etc/pf9/authentik-credentials.txt` with restricted permissions (600)

## Files

- `01-namespace.yaml` - Creates authentik namespace
- `02-postgres.yaml` - PostgreSQL database for Authentik
- `03-redis.yaml` - Redis cache for Authentik
- `04-authentik-server.yaml` - Authentik server and worker deployments
- `05-ingress.yaml` - Nginx ingress for Authentik (optional, for web access)
- `06-roles.yaml` - Kubernetes RBAC roles
- `07-group-bindings.yaml` - Maps Authentik groups to K8s roles
- `00-secrets.yaml` - Secret templates (auto-generated during setup)

## Next Steps

After Authentik is configured:

1. Access vJailbreak UI at `http://<VM_IP>/`
2. You'll be redirected to Authentik for login
3. Log in with a user account (create in Authentik admin panel)
4. After authentication, you'll be redirected back to vJailbreak UI

Assign users to groups in Authentik to control their permissions in vJailbreak.
