# Feature Specification: UI Service Account Token Security

**Feature Branch**: `ui-token-rotation`  
**Created**: 2026-05-04  
**Updated**: 2026-05-04 (revised after design review — Nginx proxy approach adopted)  
**Status**: Draft  
**Input**: User description: "Currently UI pod uses a ServiceAccount token associated with a ui SA to make k8s api calls. But that token never expires, so if someone gets the hold of that token, the person can make API calls to vjailbreak k8s API. We want UI token to change on configurable intervals"

## Problem Statement

The current architecture has two compounding security issues:

1. **Token exposed in browser**: The ServiceAccount token is injected into `index.html` at pod startup and sent by the browser in every request's `Authorization` header. It is visible in browser dev tools, network captures, and page source. Anyone with browser access can extract it.

2. **Token never expires**: The token injected at startup is a traditional long-lived ServiceAccount token. Once leaked, it provides indefinite unauthorized access to the vJailbreak Kubernetes API with full `ui-manager-role` permissions (all migration CRDs, secrets, configmaps, pods).

Rotation alone (shortening the token's lifetime) reduces the damage window but does not eliminate the exposure. The root fix is to prevent the token from ever reaching the browser.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Token Never Reaches the Browser (Priority: P1)

As a vJailbreak operator, I want the ServiceAccount token to remain on the server and never be transmitted to or stored in the browser, so that browser-side attacks (dev tools, XSS, network capture) cannot extract it.

**Why this priority**: This is the root fix. Rotation without elimination still exposes the token in every browser session. Eliminating browser exposure removes the entire attack surface for credential theft via the UI.

**Independent Test**: Can be fully tested by opening the UI, inspecting page source, network requests, and browser memory — no token string should be findable in any of these.

**Acceptance Scenarios**:

1. **Given** the UI is loaded in a browser, **When** a user inspects the page source or JavaScript bundle, **Then** no Kubernetes ServiceAccount token is present in any form.
2. **Given** an attacker captures all HTTP traffic between the browser and the UI server, **When** they search for a bearer token, **Then** no ServiceAccount token is found in any request or response.
3. **Given** the UI makes a Kubernetes API call (e.g., listing migrations), **When** the request is proxied through the server, **Then** the token is added server-side and the browser receives only the API response.

---

### User Story 2 - Token Has a Bounded Lifetime (Priority: P2)

As a vJailbreak operator, I want the server-side token to expire and be automatically replaced on a configurable interval, so that a token leaked from the server (e.g., via log exposure or file system access) has a limited window of usefulness.

**Why this priority**: Eliminates the "never expires" problem for the server-side credential. Even if the server is compromised, the attacker's window is bounded.

**Independent Test**: Can be fully tested by configuring a short TTL, waiting for expiry, and verifying the token file on disk has been updated by the system automatically.

**Acceptance Scenarios**:

1. **Given** the system is running with a configured token TTL, **When** the TTL elapses, **Then** the token on disk is automatically replaced with a new one by the cluster, without any human intervention.
2. **Given** the token has just been rotated, **When** the UI makes a Kubernetes API call, **Then** the new token is used and the call succeeds.
3. **Given** no TTL has been explicitly configured, **When** the system starts, **Then** a safe default TTL is applied automatically.

---

### User Story 3 - Configurable Token TTL (Priority: P3)

As a vJailbreak administrator, I want to control how long the server-side token remains valid before it is rotated, so that I can align with my organization's security policy.

**Why this priority**: Different deployments have different risk profiles. Providing a configurable TTL lets operators balance security stringency against operational simplicity.

**Independent Test**: Can be fully tested by changing the TTL configuration, redeploying, and observing that the token on disk has the new expiry.

**Acceptance Scenarios**:

1. **Given** I am an administrator, **When** I update the token TTL in the deployment configuration and redeploy, **Then** the new token is issued with the updated lifetime.
2. **Given** no TTL is configured, **When** the system starts, **Then** a default TTL of 24 hours is applied.
3. **Given** a TTL below the system minimum (600 seconds), **When** I attempt to deploy, **Then** the system rejects the configuration with a clear error.

---

### Edge Cases

- What happens if the proxy cannot read the token file (permission error, file missing)?
- What happens if the token expires before kubelet has rotated it (e.g., clock skew, kubelet delay)?
- If the pod restarts while a browser session is open, does the session recover gracefully?
- What is the behavior under very short TTLs (near the 600-second minimum)?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST route all Kubernetes API calls from the browser through a server-side proxy that injects the ServiceAccount token — the token MUST NOT be transmitted to or stored in the browser.
- **FR-002**: The ServiceAccount token used by the proxy MUST have a bounded lifetime configured via `expirationSeconds` on the projected volume.
- **FR-003**: The proxy MUST read the token from disk on each request so that kubelet-rotated tokens are picked up automatically with no restart or code change.
- **FR-004**: The cluster MUST be configured to honor token expiration without silently extending lifetimes.
- **FR-005**: The token TTL MUST be configurable by an administrator by updating the deployment configuration. A pod restart is acceptable when changing the TTL.
- **FR-006**: The token TTL MUST support values from a minimum of 600 seconds to a maximum of 30 days.
- **FR-007**: The system MUST apply a default TTL of 86400 seconds (24 hours) when no explicit value is configured.
- **FR-008**: All Kubernetes API paths used by the UI (`/api/`, `/apis/`) MUST be proxied. WebSocket connections (used for log streaming) MUST also be supported.
- **FR-009**: The proxy MUST verify the Kubernetes API server's TLS certificate using the cluster CA.

### Key Entities

- **Server-Side Proxy**: The Nginx component that intercepts all browser-to-k8s API calls, injects the SA token, and forwards requests. Token is never sent to or stored in the browser.
- **Projected ServiceAccount Token**: The short-lived token file on disk, managed by kubelet. Has a creation timestamp, configurable TTL (`expirationSeconds`), and is automatically refreshed before expiry.
- **k3s Token Expiration Config**: The cluster-level flag (`service-account-extend-token-expiration=false`) that prevents silent extension of token lifetimes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero browser sessions contain a Kubernetes ServiceAccount token in page source, network traffic, or JavaScript memory — verified by inspection.
- **SC-002**: A token leaked from the server becomes unusable within the configured TTL (default: 24 hours), limiting unauthorized access to a bounded window.
- **SC-003**: Token rotation is completely transparent to end users — no errors, no page reloads, no re-authentication required during normal rotation.
- **SC-004**: All Kubernetes API calls (REST and streaming/WebSocket) succeed through the proxy without regression.
- **SC-005**: Changing the TTL and redeploying takes effect within one pod restart cycle.

## Assumptions

- The UI pod runs OpenResty (Nginx with Lua support), which is already the case in the current deployment.
- The projected token is mounted at the standard SA path; the proxy reads it from that location.
- Pod restart on TTL change is acceptable — documented and expected behavior.
- The browser UI does not need to know about the token at any point; all authentication is handled opaquely by the proxy.
- End users of the UI do not observe any difference in behavior — the change is entirely server-side.
- The default TTL of 24 hours is a sufficient security baseline; operators requiring tighter security can lower it via deployment configuration.
- k3s is the cluster runtime; `service-account-extend-token-expiration=false` is the relevant flag to prevent silent TTL extension.
