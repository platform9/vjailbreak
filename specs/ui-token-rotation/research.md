# Research: UI ServiceAccount Token Security

**Date**: 2026-05-19
**Branch**: `ui-token-rotation`

---

## Decision 1: How to intercept browser-bound k8s API calls

**Problem**: `vjailbreak-api-ingress` (namespace `default`) routes `/(api.*)` and `/(apis.*)` directly to `kubernetes:443`, bypassing the UI container entirely. Any Nginx proxy in the UI container is unreachable for these paths.

**Decision**: Delete `vjailbreak-api-ingress`. With no specific rule for `/api.*` or `/apis.*`, Nginx Ingress falls through to `vjailbreak-ui-ingress` (path `/`), which routes to `vjailbreak-ui-service:80` → UI container Nginx.

**Rationale**:
- Simplest change (deletion vs. modification)
- No cross-namespace routing issues (an Ingress in `default` cannot natively backend to a service in `migration-system`)
- After deletion, `vjailbreak-ui-ingress` with path `/` catches all unmatched paths including `/api/` and `/apis/`
- Grafana and vpwned ingresses are in separate namespaces with more-specific paths — they are unaffected

**Alternatives considered**:
- Modify `vjailbreak-api-ingress` backend to UI service → requires `ExternalName` service in `default` namespace or cross-namespace ingress extension (neither clean)
- Use `nginx.ingress.kubernetes.io/configuration-snippet` to inject token at ingress level → token must be available to ingress controller pod (different pod, no access to UI SA volume mount) → infeasible

---

## Decision 2: Fate of vpwned proxy (pods/secrets via `K8S_PROXY_BASE_PATH`)

**Problem**: Commit `8c4c1fb4` routes pods/secrets through vpwned (`/dev-api/sdk/vpw/v1/k8s/api/v1`). Vpwned validates the caller's Bearer token via Kubernetes TokenReview. If we remove the token from the browser, vpwned's validation fails → pods/secrets calls break.

**Decision**: Change `K8S_PROXY_BASE_PATH` back to `/api/v1`. Pods/secrets calls go directly to `/api/v1/...` (through UI Nginx → kubernetes.default.svc). Add back `pods` and `secrets` RBAC to `ui-manager-sa`.

**Rationale**:
- Once the SA token is server-side only, `ui-manager-sa` having `pods`/`secrets` RBAC is safe. The attacker cannot use the SA token (it's never in the browser) even if they know the routes.
- The original motivation for vpwned (reduce blast radius of browser-exposed token) no longer applies once the token is not browser-exposed.
- Avoids inventing a new browser-to-vpwned authentication mechanism.
- Restoring RBAC + removing token from browser is strictly better than the current state (limited RBAC + token still in browser via vpwned validation).

**Alternatives considered**:
- Keep vpwned, use session cookie to authenticate browser → vpwned would need to parse Nginx Basic Auth session cookies; tight coupling to UI auth mechanism
- Keep vpwned, add a shared service-to-service header → adds a new secret that must be managed; doesn't solve the fundamental browser-to-vpwned auth gap

---

## Decision 3: Nginx token injection mechanism

**Decision**: Use OpenResty Lua (`set_by_lua_block`) to read the projected SA token from `/var/run/secrets/kubernetes.io/serviceaccount/token` on each request and inject it as the `Authorization` header.

**Rationale**:
- OpenResty is already the web server in the UI container (`ui/startup.sh` starts `/usr/local/openresty/bin/openresty`)
- `ui/default.conf` already uses Lua blocks for session management
- No new dependencies
- Per-request read ensures fresh token is used after Kubernetes renews the projected token (kubelet rotates it before TTL expiry)
- Performance acceptable: this is a low-RPS internal management UI

**Alternatives considered**:
- Cache token in Nginx shared dict with TTL → adds complexity; per-request read is simpler and correct
- Use `auth_request` + a sidecar → adds a sidecar container; overkill

---

## Decision 4: Projected ServiceAccount Token

**Decision**: Add a projected volume to the UI Deployment that sources:
1. `serviceAccountToken` with `expirationSeconds: 86400` (24h default)
2. `configMap: kube-root-ca.crt` for TLS verification
3. `downwardAPI: metadata.namespace`

Set `automountServiceAccountToken: false` to prevent the default long-lived token from being mounted alongside.

**Rationale**:
- Kubernetes projected tokens are auto-renewed by kubelet (typically at 80% of TTL = ~19.2h for 24h)
- `automountServiceAccountToken: false` is required; without it the old token co-exists and undermines the TTL
- Minimum 600s enforced by Kubernetes API server regardless of configured value

**k3s flag**: `service-account-extend-token-expiration: false` must be set in k3s config. Without it, k3s silently extends projected token TTLs beyond the configured value, defeating the TTL bound.

---

## Decision 5: Token injection for existing `startup.sh`

**Decision**: Remove the `envsubst` step from `startup.sh`. The line that reads the SA token and substitutes it into `index.html` is deleted entirely.

**Rationale**: The Nginx proxy now handles authentication server-side. The browser no longer needs the token. The `VITE_API_TOKEN` environment variable is no longer relevant in production.

---

## Constraints and References

- Kubernetes projected token minimum TTL: 600s (enforced by API server; values below 600s are silently raised)
- k3s `service-account-extend-token-expiration` default: `true` (must be explicitly disabled)
- OpenResty `io.open` in Lua blocks: available, synchronous, acceptable for management UI traffic
- `deploy/installer.yaml` and `deploy/07ui-deployment.yaml`: generated by pre-commit hook (`make build-installer`) from `ui/deploy/ui.yaml`; never hand-edit these
