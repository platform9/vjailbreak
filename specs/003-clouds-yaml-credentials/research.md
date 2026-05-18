# Phase 0 Research — clouds.yaml credentials

## R-1: clouds.yaml parsing library

**Decision**: Use `github.com/gophercloud/utils/openstack/clientconfig` exclusively.

**Rationale**:
- Already part of the gophercloud ecosystem vendored by vjailbreak.
- Handles all standard `auth_type` values including `v3password`, `v3applicationcredential`, `v3oidcpassword`.
- Auto-discovers `auth_url`, region, interface, and identity API version from clouds.yaml.
- Builds `*gophercloud.AuthOptions` ready for `openstack.AuthenticatedClient(...)`.
- Honors `cacert`, `verify`, and `auth_type`-specific fields without custom code.

**Alternatives considered**:
- Roll a custom YAML parser keyed on the `clouds:` top-level — rejected: duplicates `clientconfig` for no gain, risks divergence from OpenStack ecosystem semantics.
- Use `os-client-config` Python via subprocess — rejected: introduces a Python runtime dependency to a Go binary, harms testability and packaging.

**Implementation note**: `clientconfig.AuthOptions(opts)` expects either env vars or a file on disk via `OS_CLIENT_CONFIG_FILE`. Since vjailbreak reads `clouds.yaml` from a Kubernetes Secret, write it to a tmpfile inside the controller pod (`/tmp/...`, mode `0600`, cleaned up after parse). Alternative: `clientconfig.GetCloudFromYAML(yamlBytes, cloudName)` if exposed — verify API surface during implementation.

## R-2: Kubernetes Conditions API for OpenstackCreds

**Decision**: Use a `[]metav1.Condition` slice with `meta.SetStatusCondition` from `k8s.io/apimachinery/pkg/api/meta`.

**Rationale**:
- Standard Kubernetes convention for nuanced status reporting (Deployment, Pod, Ingress all use Conditions).
- controller-runtime provides built-in helpers for set/get/remove.
- `kubectl describe` and `kubectl get -o jsonpath` render Conditions natively.
- Supports `LastTransitionTime` and `ObservedGeneration` for drift detection.

**Alternatives considered**:
- Extending the flat `OpenStackValidationStatus` field with new enumerated values — rejected: cannot represent multiple concurrent concerns (e.g., valid AND expiring).
- Keeping both flat fields and Conditions slice — rejected: doubles the surface area without identified back-compat consumers.

**Migration**: Retire the existing flat fields. Existing `OpenstackCreds` resources upgraded from prior versions will have their flat status fields cleared on first reconcile after upgrade and the new Conditions populated. See `data-model.md` for the full Condition Type catalog and Reasons.

## R-3: Controller-runtime Secret watch

**Decision**: Use `controller-runtime` `Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(...))` in the `OpenstackCreds` controller setup, mapping Secret events to all `OpenstackCreds` resources whose `SecretRef` points to that Secret.

**Rationale**:
- The same controller can watch multiple types; no separate goroutine required.
- Informer cache keeps overhead minimal — Secret list/watch happens once per controller, not per resource.
- The map function naturally supports the many-to-one relationship (multiple `OpenstackCreds` referencing one Secret per FR-016).
- Re-reconciliation latency in the sub-second range under typical load.

**Alternatives considered**:
- Periodic resync only (no Secret watch) — rejected per spec clarification Q4: rotation observation latency too high.
- Field-indexer (use a `SecretRef.Name` index) for efficient lookups — accepted as a secondary optimization within the controller implementation; not a separate alternative.

## R-4: Application Credentials auth flow

**Decision**: Rely on `clientconfig` to construct `gophercloud.AuthOptions` from clouds.yaml's `auth_type: v3applicationcredential`. The resulting `AuthOptions` has `ApplicationCredentialID` and `ApplicationCredentialSecret` set; gophercloud's `tokens.Create` issues the correct token request automatically.

**Rationale**:
- Application Credential auth is a standard Keystone v3 flow; no project scope in the token request (scope is bound at credential creation time).
- gophercloud handles the auth shape transparently when `AuthOptions` is constructed correctly.
- No new auth code needed in vjailbreak.

**Validation surfacing**:
- 401 from token endpoint → `Condition CredentialsValidated=False, Reason=CredentialInvalidOrRevoked`.
- 403 from a downstream service → `Condition RolesSufficient=False, Reason=InsufficientRoles` (extract role names from the error body if Keystone provides them).
- Application Credential `expires_at` field — read via `applicationcredentials.Get(...)` after successful auth to drive the `Expiring` / `Expired` conditions.

**Minimum Keystone version**: Queens (2018). Document in the operator credentials guide as a prerequisite.

## R-5: Microversion floor semantics

**Decision**: Implement a `MicroversionFloor(configValue, hardcodedValue string) string` helper that returns the higher of the two values via semantic comparison of the `major.minor` form. The function returns `hardcodedValue` when `configValue` is empty.

**Rationale**:
- Keeps the floor logic in one tested helper.
- Comparator must handle the `latest` sentinel (treat as greater than any specific version).
- Service-specific hardcoded floors apply only at the call site that hardcodes a microversion; everywhere else the configured value applies directly.

**Alternatives considered**:
- Apply configured value unconditionally (let it override hardcoded down too) — rejected: would re-break the multi-attach attach call if a misconfigured `clouds.yaml` declared `compute_api_version: 2.1`.
- Apply only at construction time, ignore per-call hardcoded values — rejected: per-call values exist because some operations require a higher microversion than the default service client; ignoring them re-introduces the v0.4.5 multi-attach bug.

**Application points**:
- Constructor (e.g., `NewComputeClient`): set `Microversion = MicroversionFloor(clouds["compute_api_version"], "")` — typically empty hardcoded value at construction.
- Per-call hot paths (e.g., `AttachVolumeToVM`): set `Microversion = MicroversionFloor(clouds["compute_api_version"], "2.60")` — hardcoded operation-specific floor.

## R-6: Client-side YAML parsing in the UI

**Decision**: Add `js-yaml` (or already-bundled equivalent — verify) for client-side `clouds.yaml` parsing in the credential form.

**Rationale**:
- `js-yaml` is the canonical Node/browser YAML library; small bundle size (~30-50 KB minified), schema-safe by default (no arbitrary code execution).
- Client-side parsing allows inline error reporting before submission and immediate `cloudName` dropdown population (FR-013).
- The parsed object can be inspected for `auth_type` to drive the "Application Credential" badge in the form (FR-014).

**Alternatives considered**:
- Server-side parse (round-trip to API server) — rejected: extra latency for trivial validation, worse UX on parse errors.
- No client-side parse, accept any string — rejected: violates FR-013 (cloud-name dropdown) and FR-014 (auth-method badge).

**Bundle impact**: Acceptable per spec Assumption. Confirm UI build pipeline (Vite) handles tree-shaking so only `js-yaml.load` is shipped, not the full library surface.

## R-7: Upgrade and back-compat behavior

**Decision**:
- Existing `OpenstackCreds` resources using OS_* keys: behavior unchanged. The controller's parser branches on the presence of `clouds.yaml` in the referenced Secret.
- Existing resources' flat status fields: on first reconcile after upgrade, the controller writes the equivalent Conditions and clears the flat fields.
- Existing resources continue to validate exactly as before; new Conditions surface the same information operators previously read from the flat fields, plus the new Expiring / Expired / RolesSufficient states once Application Credentials are introduced (PR #2).

**Rationale**:
- Constitution V (Module Independence) and spec FR-002 require no operator action on upgrade.
- Clearing the flat fields prevents stale status from confusing operators after the Conditions API takes over.

**One-shot migration tracking** (optional, plan-level decision): a transient `MigratedFromFlatStatus=True` Condition (Reason: `Upgrade`, set once) lets operators identify resources where the controller performed the flat-to-Conditions transition. This Condition can be cleared after a configurable number of reconciles or version bumps. Defer the final decision on whether to include this tracking to PR #1 implementation review.

## R-8: Validation cadence

**Decision**: Re-validate on each Secret-watch trigger (event-driven, near-real-time) and on a periodic interval (every 1 hour) as a backstop for time-sensitive Conditions (`Expiring`, `Expired`).

**Rationale**:
- Spec FR-009 requires a 30-day expiration warning; this is meaningful only when the controller periodically re-evaluates the App Cred state even if the Secret is not changing.
- 1-hour cadence is sufficient for day-level granularity warnings; lower cadence introduces unnecessary Keystone load (single-digit auth attempts per hour per `OpenstackCreds` resource).
- Combined event + periodic gives both prompt rotation observation and freshness of time-sensitive Conditions.

**Alternatives considered**:
- Event-driven only — rejected: misses idle expiration cases (App Cred sitting unused expires; status never updates).
- Periodic only (no Secret watch) — rejected: rotation observation latency too high (covered by Q4 clarification).

**Configurability**: The 1-hour interval should be a controller flag or environment variable for environments with tighter or looser SLA needs. Default: 1 hour.

## R-9: Log redaction and observability

**Decision**: Never log Application Credential `application_credential_secret`, user `password`, or any other Secret-sourced credential values in controller logs, Kubernetes Events, or `OpenstackCreds.status.conditions` messages.

**Rationale**:
- Security baseline for any credential-handling controller.
- Air-gapped and regulated environments (typical vjailbreak deployment posture) require credentials to remain in the Secret store, not visible in log aggregators.

**Implementation**:
- All log statements touching credential structures must reference identifiers (cloud name, auth URL, project) and never the secret material.
- Add unit tests asserting that no secret value appears in any log line emitted during reconcile and validation paths.
- Condition messages must use generic phrasings ("authentication failed", "credential invalid or revoked") rather than echoing OpenStack error responses that may carry token fragments.

**Alternatives considered**:
- Implement opt-in verbose logging behind a flag — rejected: makes accidental leak too easy.
