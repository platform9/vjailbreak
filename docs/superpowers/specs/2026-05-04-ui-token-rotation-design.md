# Design: UI Service Account Token Rotation

**Date**: 2026-05-04  
**Branch**: `ui-token-rotation`  
**Status**: Approved

## Problem

The vJailbreak UI pod authenticates to the Kubernetes API using a ServiceAccount token mounted at startup. This token never expires. If leaked (e.g., visible in browser dev tools or network captures), it grants permanent unauthorized access to the vjailbreak k8s API with full `ui-manager-role` permissions — covering all migration CRDs, secrets, configmaps, and pods.

## Goal

Replace the long-lived token with a short-lived, bounded-lifetime token that rotates automatically on a configurable interval. Active browser sessions must remain uninterrupted during rotation.

## Constraints

- Pod restart is acceptable **only** when the TTL interval changes (config change). Normal rotations must be seamless.
- No new backend services, sidecars, or Nginx endpoints.
- Changes are limited to: k3s projected token config, UI pod RBAC, and frontend code.

---

## Architecture

### 1. k3s: Projected Service Account Token

The UI pod's volume spec is updated from a standard service account token mount to a **projected service account token** with a configurable `expirationSeconds`. k3s/kubelet issues bounded-lifetime tokens and automatically refreshes the token file on disk at ~80% of the TTL.

The TTL value is sourced from a ConfigMap (e.g., `vjailbreak-settings`, key `ui-token-ttl-seconds`), injected into the pod spec as an environment variable, and used in the projected volume definition. Changing the ConfigMap value triggers a pod restart to apply the new TTL — explicitly accepted behavior.

**Minimum TTL**: 600 seconds (10 minutes) — enforced by Kubernetes API server.  
**Default TTL**: 86400 seconds (24 hours).

### TTL Change Procedure (Operators)

1. Edit `deploy/07ui-deployment.yaml` — change `expirationSeconds` under the `sa-token` projected volume.
2. Apply the updated manifest: `kubectl apply -f deploy/07ui-deployment.yaml`
3. The Deployment performs a rolling restart; the new pod starts with a token issued at the new TTL.
4. Verify: `kubectl exec -n migration-system <ui-pod> -- cat /var/run/secrets/kubernetes.io/serviceaccount/token | cut -d. -f2 | base64 -d | python3 -m json.tool | grep exp`

### 2. RBAC: TokenRequest Permission

The `ui-manager-role` ClusterRole gains one new rule:

```yaml
- apiGroups: [""]
  resources: ["serviceaccounts/token"]
  verbs: ["create"]
```

This allows the UI frontend (using the current token) to request a fresh token for itself via the Kubernetes TokenRequest API — no new endpoint, no new service, just an additional k8s API call the UI already has access to.

### 3. Frontend: Mutable Token Store + Request Interceptor

**Current behaviour**: `axiosInstance` is created once at module load with the token frozen in default headers via `import.meta.env.VITE_API_TOKEN`. The token is never updated.

**Changed behaviour**:

- A module-level mutable variable `currentToken` is initialized from `import.meta.env.VITE_API_TOKEN` (preserving today's startup behaviour).
- `axiosInstance` is created **without** a default Authorization header.
- A request interceptor reads `currentToken` on every outgoing request, so token updates are picked up immediately by all callers.
- The raw `fetch()` call in `pods.ts` (used for log streaming) is updated to read `currentToken` rather than the env var.

```typescript
// Token store
let currentToken: string = import.meta.env.VITE_API_TOKEN
export const setToken = (token: string) => { currentToken = token }

// Interceptor (replaces static header in axios.create)
axiosInstance.interceptors.request.use(config => {
  config.headers['Authorization'] = `Bearer ${currentToken}`
  return config
})
```

### 4. Frontend: Background Refresh Loop

A single refresh loop (initialized once at app startup, e.g., in `App.tsx` via `useEffect`) runs on an interval. The interval is set to **70% of the token TTL** — well before expiry — giving a comfortable safety window.

On each tick:

1. Call `POST /api/v1/namespaces/migration-system/serviceaccounts/ui-manager-sa/token` with the desired `expirationSeconds`.
2. On success: call `setToken(newToken)`. All subsequent requests use the new token.
3. On failure: log a warning, keep the existing token, and retry on the next tick. No user-visible disruption as long as the current token is still valid.

The TTL (used to compute the refresh interval) is passed to the frontend as `VITE_TOKEN_TTL_SECONDS` — injected into `index.html` by `startup.sh` alongside `VITE_API_TOKEN`, read from the same pod env var that feeds the projected volume. The refresh interval is `0.7 × TTL` seconds, keeping the cadence in sync with the actual token lifetime.

---

## Data Flow: Normal Rotation

```text
[Kubelet]                    [Frontend]                       [k8s API]
    |                            |                                |
    | refreshes token file       |                                |
    | (at ~80% of TTL)           |                                |
    |                            |                                |
    |          (at 70% of TTL)   |                                |
    |                            |-- POST .../token  ----------->|
    |                            |<-- { token: <new> } ----------|
    |                            |                                |
    |                            | setToken(newToken)             |
    |                            | (all requests now use new     |
    |                            |  token, no page reload)        |
```

---

## Data Flow: TTL Change

1. Admin updates ConfigMap `ui-token-ttl-seconds`.
2. Pod restarts (expected — acceptable per design decision).
3. New pod starts with updated `expirationSeconds` on projected volume.
4. `startup.sh` injects the new token (with new TTL) into `index.html`.
5. Frontend initializes `currentToken` from the injected value and sets refresh interval based on new TTL.

---

## Error Handling

| Scenario | Behaviour |
|----------|-----------|
| TokenRequest API call fails (transient) | Keep existing token, retry at next scheduled interval |
| TokenRequest API call fails repeatedly | Token eventually expires; user sees 401, prompted to reload page |
| TTL set below Kubernetes minimum (3600s) | Kubernetes rejects the projected volume; pod fails to start — operator must correct ConfigMap |
| Pod restarts during normal operation | Fresh token injected at startup; refresh loop reinitializes correctly |

---

## Files Changed

| File | Change |
|------|--------|
| `deploy/07ui-deployment.yaml` | Replace static SA token mount with projected token volume; add `UI_TOKEN_TTL_SECONDS` env var from ConfigMap |
| `deploy/07ui-deployment.yaml` (ClusterRole) | Add `create` on `serviceaccounts/token` to `ui-manager-role` |
| `ui/src/api/axios.ts` | Replace static header with mutable token store + request interceptor |
| `ui/src/api/kubernetes/pods.ts` | Update raw `fetch()` to use `currentToken` from token store |
| `ui/src/App.tsx` (or equivalent entry point) | Initialize background token refresh loop on mount |

---

## Assumptions

- The default TTL of 24 hours is a sufficient security baseline; operators requiring tighter security can lower it via ConfigMap.
- The `ui-manager-sa` is the correct service account to issue tokens for — no change to the SA identity or RBAC scope beyond the TokenRequest permission.
- Browser sessions last less than the token TTL in normal usage; the refresh loop handles long-lived sessions.
- The minimum 1-hour Kubernetes TTL floor is acceptable for all deployments.
