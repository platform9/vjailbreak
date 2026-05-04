# Data Model: UI Token Rotation

## TokenStore (frontend module state)

Lives in `ui/src/api/axios.ts` as module-level mutable state.

| Field | Type | Description |
|-------|------|-------------|
| `currentToken` | `string` | The active bearer token for all k8s API calls. Initialized from `import.meta.env.VITE_API_TOKEN` at module load. Updated by `setToken()` after each successful refresh. |

**Accessor**: `setToken(token: string): void`  
**Reader**: implicit — read by request interceptor on every outgoing request

---

## TokenRefreshState (frontend runtime state)

Lives in the refresh loop (App.tsx), not persisted.

| Field | Type | Description |
|-------|------|-------------|
| `expiresAt` | `number` | Unix timestamp (ms) when the current token expires. Derived from JWT `exp` on startup; from `expirationTimestamp` on subsequent refreshes. |
| `refreshTimer` | `ReturnType<setTimeout>` | Handle to the scheduled next refresh. Cleared and reset after each refresh. |

**Refresh interval**: `0.7 × (expiresAt - Date.now())`

---

## TokenRequest (k8s API contract — request)

Sent to `POST /api/v1/namespaces/migration-system/serviceaccounts/ui-manager-sa/token`.

| Field | Value |
|-------|-------|
| `apiVersion` | `"authentication.k8s.io/v1"` |
| `kind` | `"TokenRequest"` |
| `spec.expirationSeconds` | integer — matches the projected volume TTL (e.g., `86400`) |

---

## TokenResponse (k8s API contract — response)

| Field | Type | Description |
|-------|------|-------------|
| `status.token` | `string` | New JWT to replace `currentToken` |
| `status.expirationTimestamp` | `string` | ISO8601 datetime — used to schedule next refresh |

---

## ProjectedTokenVolume (k8s Deployment config)

The projected volume that replaces the automounted SA token.

| Field | Value |
|-------|-------|
| `automountServiceAccountToken` | `false` (pod spec) |
| `expirationSeconds` | `86400` (default — edit Deployment to change) |
| Mount path | `/var/run/secrets/kubernetes.io/serviceaccount/` |
| Files provided | `token`, `ca.crt`, `namespace` |

---

## State Transitions

```
[Pod starts]
     │
     ▼
currentToken = VITE_API_TOKEN (from startup.sh injection)
expiresAt    = decode JWT exp × 1000
     │
     ▼
schedule refresh at 0.7 × (expiresAt - now)
     │
     ▼ (timer fires)
POST /api/v1/.../token
     │
     ├─ success → setToken(status.token)
     │            expiresAt = parse(status.expirationTimestamp)
     │            schedule next refresh at 0.7 × (expiresAt - now)
     │
     └─ failure → keep currentToken
                  schedule retry at next interval (unchanged)
                  console.warn(error)
```
