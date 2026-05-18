# Feature Specification: clouds.yaml credentials for OpenstackCreds

**Feature Branch**: `1952-clouds-yaml-credentials`
**Created**: 2026-05-18
**Status**: Draft
**Input**: User description: "Introduce clouds.yaml as the credential format for OpenstackCreds CRD, with full back-compat for the existing per-field OS_* environment-variable approach. Three PRs decompose from this feature: (1) backend parser and CRD field plus microversion floor wiring from clouds.yaml's compute_api_version/volume_api_version into service clients, (2) OpenStack Application Credentials support via auth_type v3applicationcredential carried inside clouds.yaml, (3) UI input mode for clouds.yaml as the default credential entry path."

## Clarifications

### Session 2026-05-18

- Q: Can a single credential Secret containing a multi-cloud `clouds.yaml` back multiple `OpenstackCreds` resources, each selecting a different cloud entry via `cloudName`? → A: Yes for `clouds.yaml`-backed Secrets; legacy OS_*-backed Secrets remain 1:1 (back-compat constraint).
- Q: When the `OpenstackCreds` resource needs to report multiple concurrent status concerns (e.g., valid right now and expiring within 30 days), how granular should the status reporting be? → A: Kubernetes-style `conditions` slice with typed entries; multiple concurrent conditions reportable. Retire the existing flat `OpenStackValidationStatus` / `OpenStackValidationMessage` fields.
- Q: Should the operator-configurable microversion floor apply only to services where vjailbreak currently hardcodes a value (compute, volume), or to every OpenStack service vjailbreak constructs a client for? → A: Apply broadly — to every OpenStack service vjailbreak constructs a client for (compute, volume, image, network, identity).
- Q: How does vjailbreak react when the credential Secret content changes (e.g., during Application Credential rotation)? → A: Automatic via watch — the controller watches referenced Secrets and re-reconciles affected `OpenstackCreds` resources within seconds of any change.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Use clouds.yaml as the credential source (Priority: P1)

An operator preparing a vjailbreak migration already maintains a `clouds.yaml` file for their destination OpenStack cloud (used by the `openstack` CLI, OpenStack SDK, and other tooling). Today they must re-flatten that configuration into a parallel set of per-field Kubernetes Secret keys (`OS_AUTH_URL`, `OS_USERNAME`, `OS_PASSWORD`, `OS_DOMAIN_NAME`, `OS_REGION_NAME`, `OS_TENANT_NAME`, `OS_INTERFACE`, `OS_IDENTITY_API_VERSION`). With this story complete, the operator places their `clouds.yaml` content directly into the credential Secret, names which cloud entry to use, and the migration proceeds.

**Why this priority**: Foundational. Stories 2 and 3 build on this story's parser. Eliminates the most common source of credential drift for operators with multi-tool OpenStack workflows.

**Independent Test**: An operator creates a credential Secret containing a `clouds.yaml` key and a `cloudName` field on the `OpenstackCreds` resource, then triggers a migration. The migration authenticates and completes successfully against the destination cloud. A separate operator on a deployment using the existing OS_* keys upgrades vjailbreak and observes no behavior change.

**Acceptance Scenarios**:

1. **Given** a credential Secret containing a valid `clouds.yaml` with a single cloud entry, **When** an `OpenstackCreds` resource references it with `cloudName` set, **Then** vjailbreak authenticates successfully and the resource reports a healthy validation status.
2. **Given** a credential Secret containing both `clouds.yaml` and legacy OS_* keys, **When** the `OpenstackCreds` resource is reconciled, **Then** `clouds.yaml` takes precedence and the legacy keys are ignored without error.
3. **Given** a credential Secret containing only legacy OS_* keys (existing deployment pattern), **When** the `OpenstackCreds` resource is reconciled, **Then** behavior is unchanged from prior vjailbreak versions.
4. **Given** a `clouds.yaml` that specifies `compute_api_version: 2.65`, **When** vjailbreak performs a volume attach operation that would internally request microversion 2.60, **Then** the actual request uses microversion 2.65 (the operator-configured value, since it is higher).
5. **Given** a `clouds.yaml` containing multiple cloud entries and no `cloudName` set on the `OpenstackCreds` resource, **When** the resource is reconciled, **Then** the resource reports a validation error naming the available cloud entries.

---

### User Story 2 — Authenticate via OpenStack Application Credentials (Priority: P2)

An operator running production migrations wants to avoid embedding a long-lived user password in a Kubernetes Secret. They create an OpenStack Application Credential scoped to only the roles vjailbreak requires (`member` plus a custom migration role), set an expiration date covering the migration window, and place the credential ID and secret in `clouds.yaml` with `auth_type: v3applicationcredential`. After the migration, they revoke the credential without affecting the underlying user account.

**Why this priority**: Security-meaningful improvement. Builds on Story 1's parser. Optional for less regulated environments but required for production-grade or air-gapped/regulated deployments.

**Independent Test**: An operator creates an Application Credential on the destination cloud, populates a `clouds.yaml` with `auth_type: v3applicationcredential`, triggers a migration, and confirms it completes successfully. A second test revokes the credential mid-validation; vjailbreak reports a clear, actionable error.

**Acceptance Scenarios**:

1. **Given** a `clouds.yaml` with `auth_type: v3applicationcredential` and a valid Application Credential ID and secret, **When** vjailbreak validates the credentials, **Then** authentication succeeds without requiring a username or user password.
2. **Given** an Application Credential whose `expires_at` is within 30 days of the validation time, **When** vjailbreak validates the credentials, **Then** validation succeeds and an expiration-warning condition is surfaced on the `OpenstackCreds` resource.
3. **Given** an Application Credential that has been revoked, **When** vjailbreak validates the credentials, **Then** validation fails with an error message stating that the Application Credential is revoked or its secret is invalid.
4. **Given** an Application Credential that does not carry the role required to call a Cinder scheduler-stats endpoint, **When** vjailbreak attempts the destination storage discovery step, **Then** the resulting error message identifies missing role permissions and references the Application Credential rather than a user account.

---

### User Story 3 — Enter clouds.yaml through the web UI (Priority: P3)

An operator who prefers the web UI over direct Kubernetes Secret editing wants to enter their `clouds.yaml` in the credential form rather than typing each OS_* field. The form accepts pasted YAML or an uploaded file, parses it client-side, lets them select which cloud entry to use when more than one is defined, and submits the credential without needing CLI access to the cluster.

**Why this priority**: UX improvement that broadens access to the new credential format. CLI-driven workflows are already complete after Story 1. Story 3 is independent of Story 2 — UI users gain access to Application Credentials automatically because the new auth method flows through `clouds.yaml`.

**Independent Test**: An operator opens the credential creation form in the web UI, pastes a valid `clouds.yaml`, selects a cloud entry from the dropdown that populates after parsing, submits, and observes the resulting `OpenstackCreds` resource and credential Secret in the cluster contain the expected values.

**Acceptance Scenarios**:

1. **Given** the operator opens the credential creation form, **When** the form loads, **Then** the `clouds.yaml` input tab is shown by default and a legacy individual-fields tab is available as a secondary option.
2. **Given** the operator pastes a valid `clouds.yaml` with multiple cloud entries, **When** the parse completes, **Then** a cloud-name dropdown is populated with the entry keys and the operator can pick one.
3. **Given** the operator pastes invalid YAML, **When** the form attempts to parse it, **Then** the parse error is shown inline next to the input with enough detail (e.g., line/column) to locate and correct the problem.
4. **Given** the operator submits a `clouds.yaml` containing `auth_type: v3applicationcredential`, **When** the form renders the post-parse summary, **Then** an "Application Credential" indicator is shown so the operator can confirm the auth method before submission.

---

### Edge Cases

- **Conflicting credential representations**: A Secret contains both `clouds.yaml` and OS_* keys. `clouds.yaml` takes precedence; legacy keys are silently ignored on this reconciliation pass.
- **Ambiguous cloud selection**: A `clouds.yaml` defines multiple clouds and `cloudName` is not set. The `OpenstackCreds` validation status reports an error naming the available entries instead of guessing.
- **Microversion floor below hardcoded need**: A `clouds.yaml` specifies a compute microversion lower than what an internal operation requires. The internal hardcoded value applies for that operation; the operator's configured floor applies only when higher.
- **Application Credential expired before migration**: Validation fails immediately with an expiration date in the error message; the migration does not start.
- **Application Credential expires during migration**: vjailbreak surfaces the OpenStack-provided auth error from the failing API call; the migration aborts with a clear cause attributed to credential expiration.
- **YAML upload too large**: UI rejects files above a reasonable size threshold (e.g., 1 MB) with a clear message; clouds.yaml files in practice are well under this size.
- **clouds.yaml references an external cacert file path**: The credential Secret must include the certificate inline (e.g., via `cacert: |` block); on-disk paths in the operator's environment are not resolvable inside the vjailbreak controller pod. This limitation is documented and surfaced as a validation hint if a path-style cacert is detected.
- **Same Secret referenced by multiple OpenstackCreds resources**: When two or more `OpenstackCreds` resources reference the same `clouds.yaml`-backed Secret with different `cloudName` values, each resource validates and operates against its own selected cloud entry independently. A Secret update is observed by all referencing resources via the controller's Secret watch, triggering re-reconciliation for each.
- **Credential Secret changes during an active migration**: A new credential Secret content (e.g., rotated Application Credential ID and secret) is detected by the controller's Secret watch. Re-validation runs against the new content. Any migration already in flight continues using the auth token it acquired at start (no mid-flight token invalidation); subsequent migrations and the next validation pass use the rotated credential.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST accept credentials in standard OpenStack `clouds.yaml` format placed in the credential Secret under a `clouds.yaml` key.
- **FR-002**: The system MUST continue to accept credentials supplied via the existing OS_* keys for back-compatibility, with no required action by operators on existing deployments.
- **FR-003**: When `clouds.yaml` is present in the credential Secret, the system MUST use it as the credential source and ignore any OS_* keys in the same Secret.
- **FR-004**: The `OpenstackCreds` resource MUST expose a `cloudName` field selecting which cloud entry to use from a `clouds.yaml` containing multiple entries.
- **FR-005**: When `clouds.yaml` contains a single cloud entry and `cloudName` is unset, the system MUST use that entry without requiring an explicit `cloudName` value.
- **FR-006**: When `clouds.yaml` contains multiple cloud entries and `cloudName` is unset, the system MUST report a validation error listing the available cloud entries.
- **FR-007**: The system MUST support authenticating to the destination cloud via OpenStack Application Credentials, declared in `clouds.yaml` using `auth_type: v3applicationcredential` with `application_credential_id` and `application_credential_secret`.
- **FR-008**: The system MUST surface Application-Credential-specific error conditions during validation, distinguishing between an invalid or revoked credential, an expired credential, and a credential missing required roles.
- **FR-009**: The system MUST emit a warning condition on the `OpenstackCreds` resource when an Application Credential's `expires_at` is within 30 days of the validation time, without failing validation.
- **FR-010**: The system MUST honor per-service API version values from `clouds.yaml` (including `compute_api_version`, `volume_api_version`, `image_api_version`, `network_api_version`, and `identity_api_version`) as an operator-configured floor on every OpenStack service client the system constructs. When the configured version is higher than the internal hardcoded value for a given operation, the configured value is used; when the configured value is lower or absent, the internal hardcoded value (or the service-default value where no hardcoded value exists) is used.
- **FR-011**: The web UI MUST present a credential input mode that accepts `clouds.yaml` content via paste or file upload, parses it client-side, and reports parse errors inline.
- **FR-012**: The web UI MUST default to the `clouds.yaml` credential input mode and offer the legacy per-field input mode as a secondary, non-default option.
- **FR-013**: After parsing `clouds.yaml`, the web UI MUST populate a cloud-name selector from the entries defined in the YAML.
- **FR-014**: After parsing `clouds.yaml`, the web UI MUST display which auth method is configured (password versus Application Credential) so the operator can confirm before submission.
- **FR-015**: The system MUST mask Application Credential secrets and user passwords from any UI display after parse, in the same way existing password fields are handled today.
- **FR-016**: Multiple `OpenstackCreds` resources MAY reference the same credential Secret when that Secret contains `clouds.yaml`; each `OpenstackCreds` resource independently selects its cloud entry via `cloudName`. Credential Secrets containing only legacy OS_* keys MUST continue to be treated as a 1:1 binding with their `OpenstackCreds` resource (preserving existing back-compat semantics).
- **FR-017**: The `OpenstackCreds` resource MUST report its validation state via a `status.conditions` slice using the standard Kubernetes Condition shape (`Type`, `Status`, `Reason`, `Message`, `LastTransitionTime`). Multiple concurrent conditions MAY be present on a single resource. The previously flat `OpenStackValidationStatus` and `OpenStackValidationMessage` status fields MUST be retired in favor of this Conditions API.
- **FR-018**: The controller MUST watch every credential Secret referenced by an `OpenstackCreds` resource and re-reconcile the affected `OpenstackCreds` resources when their referenced Secret content changes. The watch MUST trigger re-validation against the new content without requiring operator action (no annotation bump, no controller restart). Already-running migrations MUST NOT be aborted by a credential Secret change observed during their execution; the change applies to subsequent validation passes and new migrations.

### Key Entities

- **OpenstackCreds resource**: The Kubernetes Custom Resource representing destination cloud credentials, their reference to a Kubernetes Secret, the selected cloud entry, and the current validation state. Validation state is reported as a `status.conditions` slice; multiple concurrent conditions may be present (for example, a credential reported as valid and simultaneously approaching expiration).
- **Credential Secret**: A Kubernetes Secret containing either a single `clouds.yaml` key (new path) or a set of OS_* keys (legacy path). The two are mutually preferred: `clouds.yaml` takes precedence when both are present. A `clouds.yaml`-backed Secret may be referenced by multiple `OpenstackCreds` resources concurrently (each selecting its own cloud entry); a legacy OS_*-backed Secret retains its existing 1:1 binding with a single `OpenstackCreds` resource.
- **Cloud entry**: A top-level key in `clouds.yaml` identifying a named cloud configuration. The `cloudName` field on `OpenstackCreds` selects which entry to consume.
- **Application Credential**: An OpenStack Keystone object consisting of an ID, a secret, an optional expiration, and a subset of the issuing user's roles. Used as an alternative to user password authentication.
- **Microversion configuration**: Per-service API version values (`compute_api_version`, `volume_api_version`, etc.) carried inside `clouds.yaml`, interpreted by the system as operator-configurable floors over internal hardcoded values.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator with an existing `clouds.yaml` can configure vjailbreak credentials without re-flattening any field, eliminating one full configuration step from the operator onboarding sequence.
- **SC-002**: 100% of existing deployments using OS_* credential keys continue to function without changes following the upgrade to a vjailbreak release containing this feature.
- **SC-003**: An operator can change the destination compute or volume API microversion floor used by vjailbreak by editing a single value in `clouds.yaml` and applying the Secret, with no code changes and no controller restart required.
- **SC-004**: An operator can perform a migration using an Application Credential that grants only the roles necessary for the migration, and revoke that credential after migration completes without modifying any user account.
- **SC-005**: An operator can configure vjailbreak credentials via the web UI by pasting a `clouds.yaml` and selecting a cloud entry, without opening a terminal or editing a Kubernetes Secret directly.
- **SC-006**: An operator pasting an invalid `clouds.yaml` into the UI can identify the offending line within a single read of the inline error message and correct it without external tooling.
- **SC-007**: Credential validation reports an Application Credential expiration warning at least 30 days before expiration, giving operators time to issue a replacement without disruption to scheduled migrations.

## Assumptions

- Destination OpenStack clouds support Application Credentials. This feature was introduced in Keystone Queens (2018); deployments older than Queens are not in scope for this feature.
- The `clouds.yaml` format used is the same one understood by `python-openstackclient`, `openstacksdk`, and other standard OpenStack tooling. No vjailbreak-specific extensions to the format are introduced.
- Operators with existing OS_* credential Secrets retain those Secrets unchanged through any release containing this feature; migration to `clouds.yaml` is opt-in, not required.
- Application Credentials are created by operators using existing OpenStack tooling (CLI or API) outside of vjailbreak. The system consumes them; it does not create them.
- `clouds.yaml` values containing `cacert: |` inline certificate content are usable inside the vjailbreak controller pod. References to on-disk certificate paths in the operator's local environment are not resolvable inside the controller and are out of scope.
- The web UI's client-side YAML parsing introduces a small additional JavaScript bundle dependency. The trade-off in bundle size is acceptable in exchange for the user-experience improvement and is consistent with the project's existing approach to UI dependencies.
- Existing role-scoping requirements for destination OpenStack operations (e.g., admin-equivalent permissions for Cinder scheduler-stats and Nova hypervisor discovery during the mapping phase) are unchanged by this feature. Application Credentials must be issued with sufficient roles to cover those operations.
