# Feature Specification: UI ServiceAccount Token Security

**Feature Branch**: `ui-token-rotation`
**Created**: 2026-05-19
**Status**: Draft

## Context

The vJailbreak UI runs as a Kubernetes pod and must communicate with the Kubernetes API server to read and write Custom Resources (CRDs), monitor pod logs, and manage secrets. Currently, the pod's ServiceAccount token is injected into the browser at page load — it is embedded in the HTML served to the user's browser and included in every API request from that browser.

This creates a credential-exposure risk: the token is visible in browser page source, developer tools network traffic, browser caches, and browser extensions. Because the default Kubernetes automounted token has no expiration, a stolen token grants indefinite API access.

Commit `8c4c1fb4` attempted to reduce exposure by routing pod and secret operations through the vpwned server-side proxy, but the browser still holds the ServiceAccount token (vpwned validates it via Kubernetes TokenReview). The token remains browser-accessible for all API calls. This spec eliminates the token from the browser entirely: all Kubernetes API calls — CRDs, pods, secrets — are proxied through the UI container's OpenResty server, which injects the ServiceAccount token server-side. vpwned is removed from the pods/secrets path and restored to its original scope (cluster conversion operations only).

---

## User Scenarios & Testing

### User Story 1 — Secure Kubernetes API Access (Priority: P1)

A vJailbreak user opens the migration UI. The UI reads and writes Kubernetes Custom Resources (migration plans, credentials, mappings) as part of normal operation. After this change, those API calls are performed by the server on the user's behalf — the browser never receives or transmits the ServiceAccount token. The user experience is unchanged.

**Why this priority**: This is the core security gap. All other stories depend on it being solved correctly.

**Independent Test**: Deploy the UI, open page source and browser network traffic. Verify no bearer token string appears in either. Verify CRD operations (list migrations, create credential) still succeed.

**Acceptance Scenarios**:

1. **Given** the vJailbreak UI is deployed and running, **When** a user loads the page, **Then** the Kubernetes ServiceAccount token does not appear in the HTML source, JavaScript bundles, or any static asset served to the browser.

2. **Given** the UI is loaded, **When** the user performs any CRD operation (list migrations, create/update credentials, etc.), **Then** the browser's network traffic does not contain an `Authorization: Bearer` header with the ServiceAccount token value.

3. **Given** the server-side proxy is handling API calls, **When** any Kubernetes API call is made, **Then** authentication occurs at the server layer and the response is returned to the browser without exposing the credential.

---

### User Story 2 — Automatic Token Renewal (Priority: P2)

A vJailbreak operator deploys the system. Over time, the Kubernetes API credential used by the UI server automatically renews itself before it expires. The operator does not need to restart pods or manually rotate secrets to keep the system working. A pod that has been running for days still authenticates successfully.

**Why this priority**: Bounded-lifetime tokens are only useful if they actually renew without manual action. Without this, operators face periodic outages or must resort to long TTLs that defeat the purpose.

**Independent Test**: Configure a short token TTL (minimum 600 seconds). Leave the UI pod running past the initial TTL. Verify it continues to authenticate successfully.

**Acceptance Scenarios**:

1. **Given** the ServiceAccount token has a configured expiration, **When** the token approaches its expiration, **Then** the system obtains a fresh token without requiring a pod restart or operator action.

2. **Given** a fresh token has been obtained, **When** the next Kubernetes API call is made, **Then** the fresh token is used for authentication and the call succeeds.

---

### User Story 3 — Configurable Token Lifetime (Priority: P3)

A vJailbreak operator who needs a stricter security posture can reduce the ServiceAccount token TTL below the default. The operator does not need to modify source code or understand Kubernetes internals — a single documented configuration value controls it.

**Why this priority**: Teams with different compliance requirements need different TTLs. Default is reasonable but not appropriate for all environments.

**Independent Test**: Change the configured TTL value. Verify the new TTL is reflected in the token's `exp` claim after the next renewal.

**Acceptance Scenarios**:

1. **Given** an operator sets the token TTL to a non-default value, **When** the system issues a token, **Then** the token's expiration matches the configured TTL.

2. **Given** the minimum Kubernetes-enforced TTL is 600 seconds, **When** an operator configures a TTL below 600 seconds, **Then** the system enforces the 600-second minimum and documents this constraint.

---

### Edge Cases

- What happens if the token file is temporarily unavailable (e.g., volume mount failure)? The UI should return a clear error rather than silently failing or falling back to an unauthenticated state.
- What happens during a token renewal window where the old token has expired but the new one is not yet read? API calls in that window must not succeed with an expired credential.
- What happens if the SA token volume mount fails at startup? OpenResty must log the error and return a 502, not silently issue unauthenticated requests.

---

## Requirements

### Functional Requirements

- **FR-001**: The Kubernetes ServiceAccount token MUST NOT be embedded in any HTML, JavaScript, JSON, or other resource served to the browser.

- **FR-002**: The browser MUST NOT include the ServiceAccount token in any network request (headers, query parameters, or request body).

- **FR-003**: All Kubernetes API calls that currently originate from the browser (CRD reads/writes, pod log streaming, pod listing, secret CRUD) MUST instead be proxied server-side, with the server injecting credentials. This includes calls previously routed through vpwned (`K8S_PROXY_BASE_PATH`).

- **FR-004**: The ServiceAccount token MUST have a bounded lifetime. The default lifetime MUST NOT exceed 24 hours.

- **FR-005**: The token MUST be automatically renewed before expiration. Renewal MUST NOT require a pod restart.

- **FR-006**: The token lifetime MUST be operator-configurable via a single documented value. The minimum configurable lifetime is 600 seconds (Kubernetes-enforced floor).

- **FR-007**: All existing functionality — CRD management, pod log streaming, secret operations, Grafana proxy — MUST continue to work correctly after the change.

- **FR-008**: Pod and secret operations MUST be routed directly through the UI container's server-side proxy (same OpenResty proxy as CRD operations), NOT through vpwned. `K8S_PROXY_BASE_PATH` is removed; `ui-manager-sa` RBAC must be restored for `pods`, `pods/log`, `pods/status`, and `secrets` resources. vpwned continues to exist for cluster conversion operations but is no longer in the pods/secrets path from the UI.

- **FR-009**: `vjailbreak-api-ingress` (currently routing `/api.*` and `/apis.*` directly to `kubernetes:443`, bypassing the UI container) MUST be deleted. No modification of this Ingress is acceptable; deletion is the required action. Deleting it causes all unmatched paths to fall through to `vjailbreak-ui-ingress` → `vjailbreak-ui-service:80` → UI container OpenResty proxy.

- **FR-010**: k3s MUST be configured with `service-account-extend-token-expiration: false` to ensure projected token TTLs are not silently extended beyond the configured `expirationSeconds`. Without this flag, k3s may indefinitely extend TTLs, defeating FR-004.

### Key Entities

- **ServiceAccount Token**: A Kubernetes credential scoped to `ui-manager-sa` in `migration-system`. Controls what k8s API operations the UI server can perform.
- **Token Source**: The mounted file at `/var/run/secrets/kubernetes.io/serviceaccount/token`. Currently an automounted default (no TTL). Must become a projected token with a bounded TTL.
- **Server-Side Proxy**: The component (Nginx, a sidecar, or the ingress) that reads the token from disk and injects it into outbound k8s API requests. Never exposes the token to the browser.
- **Ingress Routing**: The set of `Ingress` resources that determine where requests go. Currently `/api.*` and `/apis.*` bypass the UI container entirely — this is a critical routing constraint for any server-side proxy approach.

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: A security scan of the vJailbreak UI's page source, JavaScript bundles, and network traffic finds zero occurrences of the Kubernetes ServiceAccount token string.

- **SC-002**: An attacker who captures the browser's full network traffic cannot extract a credential that provides Kubernetes API access.

- **SC-003**: The ServiceAccount token used for Kubernetes API calls expires within 24 hours by default. The effective window of a stolen token is bounded by this TTL.

- **SC-004**: After initial deployment, the system continues to authenticate to the Kubernetes API indefinitely without operator intervention (no manual token rotation required).

- **SC-005**: All UI functional tests pass after the change — zero regressions in CRD operations, pod log streaming, secret management, or Grafana access.

---

## Assumptions

- The vpwned proxy (`pkg/vpwned/server/k8s_proxy_handler.go`) remains deployed but is removed from the browser UI's pods/secrets path. It continues to serve cluster conversion operations. No changes to vpwned code are required.
- The existing `vjailbreak-api-ingress` Ingress resource currently routes `/api.*` and `/apis.*` directly to the Kubernetes API server (`kubernetes:443`), bypassing the UI container's Nginx. Any implementation that wants to proxy these paths through the UI container Nginx must also modify this Ingress.
- Kubernetes projected ServiceAccount tokens have a minimum TTL of 600 seconds (enforced by the API server). Shorter values will be silently raised to 600s.
- The default automounted SA token (`automountServiceAccountToken: true`) must be disabled alongside any projected token mount — otherwise both tokens co-exist and the long-lived automounted token is still accessible.
- k3s may silently extend projected token TTLs via the `service-account-extend-token-expiration` flag. If this is enabled, the actual token lifetime will be longer than configured. Disabling this flag is a prerequisite for the configured TTL to be authoritative.
- Dev-mode operation (`VITE_API_TOKEN` + Vite dev-server proxy) is out of scope. This spec targets production deployment only.

---

## Clarifications

### Session 2026-05-19

- Q: How should pod and secret API calls be routed after the browser token is removed — through OpenResty (same proxy as CRDs), or keep vpwned as the intermediary and change its auth? → A: Route through OpenResty (Option A). Restore `pods`/`secrets` RBAC on `ui-manager-sa`. Remove `K8S_PROXY_BASE_PATH`. vpwned stays for cluster conversion only, no vpwned code changes needed.
