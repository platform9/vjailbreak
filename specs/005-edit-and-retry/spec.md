# Feature Specification: Edit and Retry Failed Migrations

**Feature Branch**: `private/main/sarika/feat/edit-and-retry`
**Created**: 2026-06-11
**Status**: Draft
**Input**: User description: "Edit and Retry for failed migrations (GitHub issues #1750, #929). When a migration fails, the Retry action should open the migration form pre-populated with the failed migration's full existing configuration so the user can correct mistakes (flavor, mappings, advanced options) and retry, instead of blindly re-running the same failing configuration or deleting everything and refilling the form from scratch."

## Problem Statement

Today, retrying a failed migration simply removes the failed migration record so the system re-runs it with the exact same configuration. If the failure was caused by user configuration (wrong target flavor, incorrect network/storage mapping, bad advanced options, wrong cutover window), the retry fails again for the same reason. The only way to change configuration is to delete the migration entirely and refill the whole migration form by hand — losing all previously entered settings and any data-copy progress (GitHub issues #1750, #929).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Edit configuration and retry a failed migration (Priority: P1)

A migration administrator sees a migration in the "Failed" state in the migrations list. They click Retry. Instead of the migration silently re-running, the migration form opens pre-populated with everything they originally configured for that VM: the failed VM pre-selected (and locked), source and destination environments shown read-only, and all editable settings — network mapping, storage mapping, target flavor, security groups, server group, migration strategy (type, data-copy and cutover windows), post-migration actions, and advanced options (granular networks/volumes/ports, periodic sync, image profiles, per-NIC IP/MAC overrides) — filled in with the current values. The administrator corrects the setting that caused the failure (e.g., picks a different flavor) and chooses **"Edit & Retry"**. The system saves the changed configuration and re-runs the migration with the corrected settings.

**Why this priority**: This is the core value of the feature — without it, configuration-caused failures are a dead end that forces full re-entry. It directly addresses both GitHub issues.

**Independent Test**: Fail a migration by giving it an invalid target flavor. Click Retry, confirm the form opens with all original values pre-filled, change only the flavor, choose "Edit & Retry", and verify the migration re-runs and uses the new flavor.

**Acceptance Scenarios**:

1. **Given** a failed migration, **When** the user clicks Retry, **Then** the migration form opens with the failed VM pre-selected and locked, and every editable setting pre-populated with the values used by the failed migration.
2. **Given** the retry form is open, **When** the user changes one or more editable settings and chooses "Edit & Retry", **Then** the changed configuration is persisted and the migration re-runs using the new configuration.
3. **Given** the retry form is open, **When** the user changes nothing and chooses "Edit & Retry", **Then** the migration re-runs with identical configuration (equivalent to a plain retry).
4. **Given** a migration re-run after an edit, **When** the migration progresses, **Then** previously completed data-copy progress is preserved wherever the changed settings do not invalidate it (no forced full re-copy for changes unrelated to disk copy).

---

### User Story 2 - Retry without editing (Priority: P2)

The administrator clicks Retry on a failed migration, the pre-populated form opens, and they realize the failure was transient (e.g., a network blip) — the configuration is fine. They choose **"Retry without editing"** and the migration re-runs exactly as before, identical to today's retry behavior.

**Why this priority**: Preserves the existing one-click retry path so the new form never makes transient-failure retries slower or riskier.

**Independent Test**: Fail a migration, click Retry, choose "Retry without editing" without touching any field, and verify the migration re-runs with unchanged configuration and no edits are written to any stored configuration.

**Acceptance Scenarios**:

1. **Given** the retry form is open, **When** the user chooses "Retry without editing", **Then** no stored configuration is modified and the migration re-runs with its original settings.
2. **Given** the retry form is open and the user has made edits, **When** they choose "Retry without editing", **Then** their on-screen edits are discarded (with a confirmation prompt) and the migration re-runs unchanged.

---

### User Story 3 - Guard rails for shared and missing configuration (Priority: P3)

The administrator retries a failed migration that belongs to a migration plan containing several VMs. When they edit a setting that is shared across the whole plan (e.g., migration strategy or target flavor source), the form clearly warns them which other VMs will be affected. Separately, if the credentials originally used for the migration have since been deleted, the form tells the administrator up front that the migration cannot be retried until valid credentials exist, rather than failing later.

**Why this priority**: Prevents the new editing power from silently breaking sibling migrations or producing confusing late failures; valuable but the feature is usable without it.

**Independent Test**: Create a plan with two VMs, fail one, retry it, edit a plan-wide setting, and verify a warning lists the sibling VM. Delete the source credentials and verify the retry form surfaces a blocking message.

**Acceptance Scenarios**:

1. **Given** a failed migration in a multi-VM plan, **When** the user edits a plan-wide setting in the retry form, **Then** the form warns that the change also applies to the named sibling VMs before allowing "Edit & Retry".
2. **Given** a failed migration in a multi-VM plan, **When** the user edits a per-VM setting (e.g., per-NIC IP/MAC overrides for the failed VM), **Then** only the failed VM's configuration changes and sibling VMs are untouched.
3. **Given** the credentials used by the failed migration no longer exist, **When** the user clicks Retry, **Then** the form opens with a clear blocking message identifying the missing credentials, and both retry actions are disabled until credentials are restored.
4. **Given** a failed migration marked not retryable (e.g., the VM uses raw device mapping disks), **When** the user views the migrations list, **Then** no retry/edit-and-retry action is offered for it (unchanged from today).

---

### Edge Cases

- Failed migration whose parent plan or template was deleted out-of-band: the retry form must detect the missing configuration and show a blocking error instead of opening a half-empty form.
- Network/storage mappings referenced by the original migration were deleted or renamed: form must surface this and require the user to re-select valid mappings before "Edit & Retry" is enabled.
- The destination environment no longer has the originally selected flavor, network, or volume type: pre-population must show the stale value as invalid and require correction.
- Two administrators retry the same failed migration concurrently: the second submission must not corrupt configuration; last-write-wins with a clear refresh/conflict message is acceptable.
- The user edits a plan-wide setting while a sibling VM in the same plan is actively migrating: the system must either block the edit or clearly warn that in-flight migrations are unaffected and only future runs pick up the change.
- The failed VM was deleted or is no longer visible in the source inventory: retry form must show a blocking message.
- Migration fails again after an edited retry: the user can repeat edit-and-retry any number of times.
- User closes/cancels the retry form: nothing is modified and the failed migration remains in its failed state.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Retry action on a failed migration MUST open the migration form pre-populated with the complete configuration used by that migration, instead of immediately re-running it.
- **FR-002**: The retry form MUST pre-select the failed VM and prevent changing the VM selection.
- **FR-003**: The retry form MUST display source and destination environment/credential selections as read-only; changing credentials or source/destination environments is out of scope.
- **FR-004**: The retry form MUST pre-populate all editable settings with their current values: network mapping, storage mapping, target flavor, security groups, server group, migration strategy (migration type, data-copy start, cutover windows, admin-initiated cutover, health checks, source-network disconnect), post-migration actions (rename, folder move), and advanced options (granular volume types, granular networks, granular ports, periodic sync, image profiles, per-NIC IP/MAC overrides).
- **FR-005**: The retry form MUST verify on open that the credentials, plan, template, and mappings referenced by the failed migration still exist; if any are missing, it MUST show a blocking message identifying what is missing and disable retry actions that cannot succeed.
- **FR-006**: The retry form MUST offer exactly two completion actions: "Retry without editing" and "Edit & Retry".
- **FR-007**: "Retry without editing" MUST re-run the failed migration with unchanged configuration, behaviorally identical to the pre-existing retry, and MUST NOT write any configuration changes.
- **FR-008**: "Edit & Retry" MUST persist the user's edits to the migration's stored configuration and then re-run the migration so that the re-run uses the edited configuration end-to-end (including any derived per-migration runtime configuration).
- **FR-009**: "Edit & Retry" MUST guarantee that no configuration change affects sibling VMs in the same plan. For multi-VM plans, the system achieves this by cloning the plan for the retried VM and removing that VM from the original plan — the user does not choose between per-VM and plan-wide scope.
- **FR-010**: For multi-VM plans, the retry form MUST display a visible informational banner naming the other VMs in the plan, so the user knows their edits apply only to the retried VM and the clone is self-contained.
- **FR-011**: The retry action (both variants) MUST NOT be offered for migrations marked not retryable (e.g., VMs with raw device mapping disks), preserving current behavior.
- **FR-012**: Pre-populated values that are no longer valid in the destination environment (deleted flavor, network, volume type, mapping) MUST be flagged in the form, and "Edit & Retry" MUST be blocked until the user corrects them; "Retry without editing" remains available with a warning that it will likely fail again.
- **FR-013**: A re-run after "Edit & Retry" MUST preserve previously completed incremental data-copy progress whenever the edited settings do not invalidate the copied data; configuration changes unrelated to disk content MUST NOT force a full re-copy.
- **FR-014**: Closing or cancelling the retry form MUST leave the failed migration and all stored configuration unmodified.
- **FR-015**: The user MUST be able to repeat edit-and-retry on the same migration after subsequent failures, with the form always reflecting the latest stored configuration.
- **FR-016**: All retry-form actions MUST provide clear progress and error feedback (e.g., persisting edits failed, re-run could not be started), leaving the system in a consistent state on partial failure — either the edits are fully applied and the retry started, or neither.

### Key Entities

- **Migration**: A single VM's migration run, with a lifecycle phase (e.g., Pending, Copying, Failed, Succeeded) and a retryable indicator. Retrying re-creates this run from stored configuration.
- **Migration Plan**: The grouping that owns one or more VM migrations and stores shared run settings (strategy, windows, advanced options, post-migration actions, per-VM network overrides). The parent from which a retried migration is re-created.
- **Migration Template**: Reusable source/destination configuration referenced by a plan: credentials references, network mapping, storage mapping, target cluster, flavor-selection behavior.
- **Network Mapping / Storage Mapping**: Named mappings from source networks/datastores to destination networks/volume types, referenced by the template and editable from the retry form.
- **Per-migration runtime configuration**: Derived per-VM settings (resolved flavor, networks, ports, strategy, advanced options) regenerated by the system when a migration is re-created; must reflect post-edit values.
- **Credentials (source/destination)**: Referenced by the template; shown read-only in the retry form and validated for existence.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can correct a configuration-caused failure and start the retry in under 2 minutes, without re-entering any setting they are not changing (0 fields require re-entry beyond the corrected ones).
- **SC-002**: 100% of settings shown in the original migration creation form are pre-populated with their stored values when the retry form opens for a failed migration.
- **SC-003**: Retrying without edits remains a flow of 3 clicks or fewer and produces a re-run configured identically to the original.
- **SC-004**: Editing a per-VM setting and retrying changes the configuration of 0 sibling VMs in the same plan.
- **SC-005**: For configuration changes unrelated to disk content, retried migrations resume from previously copied data in 100% of cases (no full re-copy).
- **SC-006**: Support/issue reports requesting "modify and retry" capability (the subject of issues #1750 and #929) can be closed; no workaround of delete-and-recreate is required for configuration fixes.

## Assumptions

- The existing migration creation form is reused for retry (same layout and steps), opened in a distinct retry mode, rather than building a separate editing screen.
- Changing source/destination credentials, clusters, or the selected VM during retry is out of scope for this feature; these are displayed read-only.
- When "Edit & Retry" fires on a multi-VM plan, the system automatically creates a clone plan containing only the retried VM, with all edits scoped to that clone. The original plan is patched to remove the retried VM. This ensures no other VM in the plan is affected by the retried VM's configuration changes. 1-VM plans are patched in place (no clone).
- Per-VM scope is available only for settings that are already stored per VM (e.g., per-NIC IP/MAC overrides); all other settings (strategy, securityGroups, advancedOptions) are cloned with the retried VM to preserve isolation.
- The system's existing behavior of re-creating a deleted migration run from its plan is the retry mechanism being built upon; edits are applied to stored configuration before the re-run is triggered.
- Migrations marked not retryable today (e.g., raw device mapping disks) stay not retryable; this feature does not expand retryability.
- Concurrent edits to the same configuration by multiple users are resolved last-write-wins; pessimistic locking is out of scope.
- Edit-and-retry applies to migrations in the Failed state only; editing in-flight or succeeded migrations is out of scope.
