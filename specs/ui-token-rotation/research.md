# Research: UI Token Rotation

**Phase**: 0 — Research  
**Date**: 2026-05-04  
**Updated**: 2026-05-04 — all decisions implemented  
**Feature**: UI Service Account Token Security

**Implementation status**: ✅ All decisions implemented on branch `ui-token-rotation`.

---

## Decision 1: Projected Token Replaces Automount

**Decision**: Set `automountServiceAccountToken: false` on the Deployment pod spec AND mount a projected `serviceAccountToken` volume at `/var/run/secrets/kubernetes.io/serviceaccount/`.

**Rationale**: Kubernetes projected volumes _supplement_ (not replace) the automounted token by default. If `automountServiceAccountToken` is not disabled, the original never-expiring token continues to exist at `/var/run/secrets/kubernetes.io/serviceaccount/token` in parallel. The security fix is negated unless automounting is explicitly disabled.

**Alternatives considered**:
- Mount projected token at a different path and update `startup.sh` — rejected for unnecessary disturbance to startup.sh.
- Patch the ServiceAccount's `automountServiceAccountToken: false` — would affect all pods using this SA; safer to scope to the Deployment pod spec.

---

## Decision 2: Full Projected Volume (token + ca.crt + namespace)

**Decision**: The projected volume includes all three files that the automount provides: `token` (serviceAccountToken source), `ca.crt` (configMap `kube-root-ca.crt`), `namespace` (downwardAPI metadata.namespace).

**Rationale**: The automounted SA volume always provides three files. Providing only `token` would break any process that reads `ca.crt` or `namespace` from the standard path (e.g., in-cluster client libraries, shell scripts). Replacing it fully is safe and prevents subtle regressions.

**Alternatives considered**:
- Token-only projected volume — rejected because ca.crt and namespace files would disappear from the standard path.

```yaml
# Full projected volume replacing automount
volumes:
- name: sa-token
  projected:
    sources:
    - serviceAccountToken:
        expirationSeconds: 86400   # configurable — admin edits Deployment to change
        path: token
    - configMap:
        name: kube-root-ca.crt
        items:
        - key: ca.crt
          path: ca.crt
    - downwardAPI:
        items:
        - path: namespace
          fieldRef:
            apiVersion: v1
            fieldPath: metadata.namespace
volumeMounts:
- name: sa-token
  mountPath: /var/run/secrets/kubernetes.io/serviceaccount
  readOnly: true
```

---

## Decision 3: TTL Lives in Deployment Spec (No ConfigMap)

**Decision**: `expirationSeconds` is hardcoded in the Deployment YAML (default: `86400`). To change the TTL, an admin updates the Deployment → triggers a rolling restart (explicitly accepted in the design).

**Rationale**: Kubernetes projected volume specs do not support env var interpolation — `expirationSeconds` is a static integer field. A separate ConfigMap + controller watching it to patch the Deployment would add significant complexity for a rare admin operation. The native Kubernetes way is to treat the Deployment spec itself as the configuration.

**Alternatives considered**:
- ConfigMap + operator watching it to patch Deployment — rejected (too complex for rare change).
- Hard-coded 24h forever — rejected (admin needs the ability to tighten/loosen per security policy).

---

## Decision 4: Frontend Derives Refresh Schedule from JWT `exp` Claim

**Decision**: The frontend decodes the JWT `exp` claim from the current token (no signature verification needed) to compute when the token expires. It schedules the next refresh at 70% of the remaining lifetime.

**Rationale**: Avoids needing a separate `VITE_TOKEN_TTL_SECONDS` env var. The token itself carries its own expiry. After each TokenRequest refresh, the response provides `expirationTimestamp` directly — no JWT decoding needed for subsequent refreshes.

**JWT decode**: `JSON.parse(atob(token.split('.')[1]))` — browser-native, no library needed.

**Alternatives considered**:
- Pass TTL via `VITE_TOKEN_TTL_SECONDS` env var — rejected: adds build-time coupling; JWT `exp` is more accurate and self-contained.
- Fixed 60-minute interval — rejected: would be incorrect if admin sets TTL < 60 min.

---

## Decision 5: TokenRequest API Payload

**Endpoint**: `POST /api/v1/namespaces/migration-system/serviceaccounts/ui-manager-sa/token`

**Request body**:
```json
{
  "apiVersion": "authentication.k8s.io/v1",
  "kind": "TokenRequest",
  "spec": {
    "expirationSeconds": 86400
  }
}
```

**Response**: `status.token` (new JWT), `status.expirationTimestamp` (ISO8601 string).

**Note**: Server may return a different `expirationSeconds` than requested. Always derive next refresh schedule from `status.expirationTimestamp`, not from the requested value.

**RBAC**: Add `create` on `serviceaccounts/token` to `ui-manager-role`. No other RBAC change needed.

---

## Decision 6: Minimum TTL is 600 Seconds

**Finding**: Kubernetes enforces a minimum `expirationSeconds` of **600 seconds** (10 minutes), not 1 hour as originally stated in the spec. Values below 600 are rejected by the API server.

**Implication**: Update FR-006 in spec to reflect minimum of 600 seconds. The spec's stated minimum of "1 hour" is a conservative recommendation; the absolute floor is 600s.

---

## k3s Compatibility

**Finding**: k3s implements projected service account tokens and the TokenRequest API identically to upstream Kubernetes. No k3s-specific flags or configuration needed. Token projection and refresh work out of the box.
