# Data Model: UI ServiceAccount Token Security

**Date**: 2026-05-19

This feature does not introduce new Kubernetes Custom Resources. The key entities are existing system components whose configuration changes.

---

## Entities

### ServiceAccount: `ui-manager-sa`

**Namespace**: `migration-system`
**RBAC (after this feature)**:

| Resource | Group | Verbs |
|----------|-------|-------|
| configmaps | "" | create, delete, get, list, patch, update, watch |
| configmaps/status | "" | get |
| events | "" | get, list, watch |
| **pods** | "" | **get, list, watch, patch** ← restored |
| **pods/status** | "" | **get** ← restored |
| **pods/log** | "" | **get, list** ← restored |
| **secrets** | "" | **create, delete, get, list, patch, update** ← restored |
| vjailbreak CRDs | vjailbreak.k8s.pf9.io | create, delete, get, list, patch, update, watch |

Notes:
- `pods`, `pods/log`, `pods/status`, `secrets` were removed in commit `8c4c1fb4`. They must be restored because the UI Nginx (running as `ui-manager-sa`) now proxies these calls directly to the Kubernetes API, rather than delegating to vpwned.
- This is safe post-feature because the SA token never reaches the browser.

---

### Projected Volume: `sa-token`

**Mount path**: `/var/run/secrets/kubernetes.io/serviceaccount`

| Source | Key | Mount Path |
|--------|-----|------------|
| serviceAccountToken (expirationSeconds: 86400) | — | `token` |
| configMap `kube-root-ca.crt` | `ca.crt` | `ca.crt` |
| downwardAPI `metadata.namespace` | — | `namespace` |

**TTL**: Configurable via `expirationSeconds`. Default: 86400s (24h). Minimum: 600s.
**Renewal**: Kubelet auto-renews at ~80% of TTL. No pod restart required.
**Automation**: `automountServiceAccountToken: false` on the Deployment prevents the legacy long-lived token from co-existing.

---

### Nginx Proxy Configuration

The UI container's Nginx (OpenResty) gains new `location` blocks:

| Path Prefix | Upstream | Token Injection |
|-------------|----------|-----------------|
| `/api/` | `https://kubernetes.default.svc` | Yes — `Authorization: Bearer $sa_token` |
| `/apis/` | `https://kubernetes.default.svc` | Yes — `Authorization: Bearer $sa_token` |

`$sa_token` is set via `set_by_lua_block` that reads `/var/run/secrets/kubernetes.io/serviceaccount/token` on each request.

---

### Ingress Change

| Resource | Namespace | Before | After |
|----------|-----------|--------|-------|
| `vjailbreak-api-ingress` | default | Routes `/(api.*)` and `/(apis.*)` → kubernetes:443 | **Deleted** |

After deletion, these paths are caught by `vjailbreak-ui-ingress` (path `/`) → `vjailbreak-ui-service:80` → UI container Nginx → proxy to kubernetes.default.svc with SA token.

---

### Removed Token Embedding

| File | Before | After |
|------|--------|-------|
| `ui/startup.sh` | Reads SA token, substitutes into `index.html` via `envsubst` | No token reading; just starts OpenResty |
| `ui/src/api/axios.ts` | Adds `Authorization: Bearer $VITE_API_TOKEN` to all axios requests | No Authorization header added |
| `ui/src/api/kubernetes/pods.ts` | Adds `Authorization: Bearer $VITE_API_TOKEN` to `streamPodLogs` fetch | No Authorization header; endpoint changed to `/api/v1` |
| `ui/src/api/secrets/secrets.ts` | Uses `K8S_PROXY_BASE_PATH` (`/dev-api/sdk/vpw/...`) | Uses `/api/v1` directly |
| `ui/src/api/migrations/migrations.ts` | Uses `K8S_PROXY_BASE_PATH` for pod queries | Uses `/api/v1` directly |
