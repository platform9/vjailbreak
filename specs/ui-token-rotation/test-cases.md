# Test Cases: UI ServiceAccount Token Security

**Date**: 2026-05-19
**Branch**: `ui-token-rotation`

---

## Unit Tests (Automated — Vitest)

### TC-U01: `axios.ts` — No Authorization header in requests

**File**: `ui/src/api/__tests__/axios.test.ts`
**Type**: Unit

**Normal flow**:

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | `getHeaders()` returns headers without Authorization | Call `getHeaders()` | Returned object has `Content-Type` but no `Authorization` key |
| 2 | Axios GET does not add auth header | Mock axios; call `axios.get({ endpoint: '/api/v1/test' })` | Outgoing request headers contain no `Authorization: Bearer` |
| 3 | Axios POST does not add auth header | Mock axios; call `axios.post({ endpoint: '/apis/test', data: {} })` | Outgoing request headers contain no `Authorization: Bearer` |

**Corner cases**:

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 4 | `VITE_API_TOKEN` env var is set (dev mode) | Set `import.meta.env.VITE_API_TOKEN = 'test-token'` | `getHeaders()` still does NOT include `Authorization` — the token is no longer read in production code path |
| 5 | Empty headers object | Call `getHeaders()` with no env vars set | Returns `{ common: { 'Content-Type': 'application/json;charset=UTF-8' } }` |

---

### TC-U02: `pods.ts` — Correct endpoint and no auth header

**File**: `ui/src/api/kubernetes/__tests__/pods.test.ts`
**Type**: Unit

**Normal flow**:

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | `fetchPods` uses `/api/v1` path | Call `fetchPods('migration-system')` | Endpoint is `/api/v1/namespaces/migration-system/pods` |
| 2 | `fetchPods` with label selector | Call `fetchPods('migration-system', 'app=vjailbreak')` | Endpoint correct; `labelSelector` param present |
| 3 | `streamPodLogs` uses `/api/v1` path | Call `streamPodLogs('migration-system', 'my-pod')` | Endpoint is `/api/v1/namespaces/migration-system/pods/my-pod/log` |
| 4 | `streamPodLogs` has no Authorization header | Mock `fetch`; call `streamPodLogs(...)` | `fetch` called with headers that do NOT contain `Authorization` |
| 5 | `streamPodLogs` default params | Call `streamPodLogs('migration-system', 'my-pod')` | URL contains `follow=true&tailLines=100&limitBytes=500000` |

**Corner cases**:

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 6 | `streamPodLogs` with custom options | `{ follow: false, tailLines: '50', limitBytes: 100000 }` | URL reflects custom params |
| 7 | `streamPodLogs` with AbortSignal | Pass `signal: controller.signal` | `fetch` called with the signal |
| 8 | `streamPodLogs` server returns non-OK | Mock `fetch` to return status 403 | Throws `Error` containing pod name and status code |
| 9 | Dev mode URL prefix | Set `MODE = 'development'` | URL starts with `/dev-api/api/v1/...` |
| 10 | Production mode URL prefix | Set `MODE = 'production'` | URL starts with `/api/v1/...` (no `/dev-api` prefix) |

---

### TC-U03: `secrets.ts` — Correct endpoint (no vpwned path)

**File**: `ui/src/api/secrets/__tests__/secrets.test.ts`
**Type**: Unit

| # | Scenario | Input | Expected |
|---|----------|-------|----------|
| 1 | `createSecret` endpoint | Call `createSecret('my-secret', {}, 'migration-system')` | Endpoint is `/api/v1/namespaces/migration-system/secrets` |
| 2 | `getSecret` endpoint | Call `getSecret('my-secret')` | Endpoint is `/api/v1/namespaces/migration-system/secrets/my-secret` |
| 3 | `deleteSecret` endpoint | Call `deleteSecret('my-secret')` | Endpoint is `/api/v1/namespaces/migration-system/secrets/my-secret` |
| 4 | `listSecrets` endpoint | Call `listSecrets()` | Endpoint is `/api/v1/namespaces/migration-system/secrets` |
| 5 | Endpoint does NOT contain `sdk` or `vpw` | Any secret call | Endpoint string does not contain `/sdk/` or `/vpw/` |

---

## Integration / Smoke Tests (Manual — Requires Live Cluster)

These tests require a deployed vJailbreak instance with this branch applied.

### TC-I01: Token not in browser

**Prerequisite**: Deploy UI pod with changes applied.

| # | Scenario | Steps | Expected |
|---|----------|-------|----------|
| 1 | Token absent from page source | Open UI URL in browser → View Page Source (Ctrl+U) | No string matching `eyJ` (JWT prefix) appears in HTML source |
| 2 | Token absent from network traffic | Open DevTools → Network tab → filter by XHR → perform a migration list action | No request header contains `Authorization: Bearer eyJ...` |
| 3 | Token absent from JS bundles | Open DevTools → Sources → search all JS files for `eyJ` | No matches |

---

### TC-I02: Kubernetes API operations work through Nginx proxy

| # | Scenario | Steps | Expected |
|---|----------|-------|----------|
| 1 | CRD list (GET /apis/) | Open Credentials page | VMware/OpenStack credentials load without error |
| 2 | CRD create (POST /apis/) | Create a new VMware credential | Credential created successfully; appears in list |
| 3 | CRD update (PATCH /apis/) | Edit an existing credential | Update saved; reflected in UI |
| 4 | CRD delete (DELETE /apis/) | Delete a credential | Credential removed |
| 5 | Pod list (GET /api/v1/pods) | Open Migrations page (shows agent pods) | Pod list loads correctly |
| 6 | Pod log streaming | Click "View Logs" on a migration | Logs stream in real time |
| 7 | Secrets CRUD | Create OpenStack credential (uses k8s Secrets) | Secret created/read without error |

---

### TC-I03: Token expiry and auto-renewal

| # | Scenario | Steps | Expected |
|---|----------|-------|----------|
| 1 | Token file exists at startup | `kubectl exec <ui-pod> -- cat /var/run/secrets/kubernetes.io/serviceaccount/token` | File is readable and contains a JWT |
| 2 | Token has bounded TTL | Decode the token JWT: `kubectl exec <ui-pod> -- cat .../token \| cut -d'.' -f2 \| base64 -d \| python3 -m json.tool` | `exp` claim is set; TTL ≤ 86400 seconds from `iat` |
| 3 | Token auto-renews (short TTL test) | Set `expirationSeconds: 600` in ui.yaml, deploy, wait 8+ minutes | Token file content changes (kubelet renewed it); API calls continue to succeed |
| 4 | No long-lived token co-exists | `kubectl exec <ui-pod> -- ls /var/run/secrets/kubernetes.io/serviceaccount/` | Only `token`, `ca.crt`, `namespace` (no additional token files from automount) |

---

### TC-I04: Ingress routing

| # | Scenario | Steps | Expected |
|---|----------|-------|----------|
| 1 | `vjailbreak-api-ingress` is gone | `kubectl get ingress -n default` | No ingress named `vjailbreak-api-ingress` |
| 2 | `/api/` requests reach UI Nginx | `curl -v https://<vjailbreak-host>/api/v1/namespaces` (without auth header) | Returns 401 from Kubernetes (proxied through UI Nginx with SA token) — NOT a direct k8s SSL error |
| 3 | `/apis/` requests reach UI Nginx | `curl -v https://<vjailbreak-host>/apis` | Returns valid API discovery response (proxied through UI Nginx) |

---

### TC-I05: Grafana and vpwned paths unaffected

| # | Scenario | Steps | Expected |
|---|----------|-------|----------|
| 1 | Grafana still accessible | Navigate to `https://<host>/grafana` | Grafana UI loads |
| 2 | vpwned ingress still present | `kubectl get ingress -n migration-system migration-vpwned-ingress` | Ingress exists (vpwned still deployed, though UI no longer routes to it) |

---

## Corner Cases — Security Properties

| # | Scenario | Expected |
|---|----------|----------|
| 1 | Token file temporarily unavailable (e.g., volume detach) | Nginx returns 502 Bad Gateway; no fallback to unauthenticated request |
| 2 | Token file contains trailing newline | Lua `gsub("%s+$", "")` strips it; Authorization header is valid |
| 3 | Token file is empty | Nginx sets `$sa_token = ""`; k8s returns 401; Nginx returns 401 to browser |
| 4 | Token file contains an expired token (between kubelet renewal cycles) | k8s returns 401; Nginx proxies 401 to browser; no crash |
| 5 | `automountServiceAccountToken: false` effect | No `/var/run/secrets/kubernetes.io/serviceaccount` directory from automount; only projected volume mount present |
| 6 | k3s `service-account-extend-token-expiration` set to true (misconfigured) | Token TTL silently extended; `exp` claim in token reflects extended value; doc warning applies |
