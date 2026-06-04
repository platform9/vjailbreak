# Feature Specification: Inventory Management / Migration Planner

**Feature Branch**: `004-migration-planner`
**Created**: 2026-06-04
**Status**: Draft
**Input**: User description: "Inventory Management page for vJailbreak — after a VMware credential is added, discover its VMs and present a Discovery summary plus persistent, named *buckets* of VMs. Each bucket carries a full migration configuration, a lifecycle status, and an optional schedule. A Default bucket is auto-created and populated with the safest VMs. A Trigger action lets the operator select multiple buckets and, before launch, see a recommended (editable) migration-agent count and a recommended bucket execution order. The existing Migration / MigrationPlan migration workflow must remain untouched."

## Overview

vJailbreak operators today create migrations one at a time through the multi-step Migration
Form, with no persistent view of what was discovered or how they intend to batch and sequence
it. This feature adds an **Inventory** section (new top-level sidebar item) that turns the flat
list of discovered VMs into an organized, schedulable, sequenced plan made of **buckets**, while
reusing — and never modifying — the existing migration execution path.

The scope of v1 is a **single VMware credential**. Buckets, the planning algorithms, and the
trigger flow all build on top of the existing CRDs by *composing* them at trigger time; the
`Migration`, `MigrationPlan`, and `RollingMigrationPlan` types and their controllers are not
changed.

---

## Clarifications

### Session 2026-06-04

- Q: What is the sidebar/page name? → A: **"Inventory."**
- Q: Where is the default bucket created — backend or frontend? → A: **Backend.** A new
  `MigrationBucket` CRD is introduced; after VM discovery completes, the controller creates the
  default bucket as a `MigrationBucket` CR.
- Q: Trigger button placement? → A: Keep **consistent with the current UI design** (match
  existing page toolbars/action bars).
- Q: How is the default bucket populated, and what if nothing matches? → A: Apply a fallback
  priority and populate with the first non-empty tier: (1) powered-off VMs with a single NIC;
  else (2) the powered-off VM(s) with the fewest NICs; else (3) the powered-on VM(s) with the
  fewest NICs; else (4) **defer** — create no bucket CR and show the discovered VMs in the
  Inventory with a message that no default bucket was created.
- Q: When duplicating a bucket, what happens to a VM already in another bucket? → A: **Block it.**
  Already-bucketed VMs appear greyed out in the selector, labelled "already in a bucket"; they
  cannot be selected.
- Q: CRD strategy — new CRD vs reuse `MigrationPlan`? → A: **New `MigrationBucket` CRD.**
- Q: Multiple VMware credentials? → A: **Single credential for v1** (data model still keys
  buckets by their source credential to keep multi-cred a clean future extension).
- Q: Must the existing migration workflow change? → A: **No.** The `Migration` / `MigrationPlan`
  object and the entire migration execution workflow must remain untouched; the planner only
  composes them at trigger time exactly as the Migration Form does today.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover inventory and get a Default bucket (Priority: P1)

After an operator adds and validates a VMware credential, they open the **Inventory** page and
see how many VMs were discovered for that credential, plus an automatically created **Default
bucket** pre-populated with the safest-to-migrate VMs. This is the foundational view; with just
this story the operator already gets organized visibility into their estate.

**Why this priority**: Nothing else in the feature is usable without discovered VMs surfaced and
a first bucket to act on. This is the MVP slice.

**Independent Test**: Add a VMware credential, wait for discovery, open Inventory, and confirm
the Discovery summary count matches discovered VMs and that a Default bucket exists containing
exactly the VMs selected by the fallback rule.

**Acceptance Scenarios**:

1. **Given** no VMware credential exists, **When** the operator opens the Inventory page,
   **Then** an empty state is shown with a call-to-action pointing to Credentials → VMware.
2. **Given** a VMware credential has just been validated and VM discovery has completed,
   **When** the operator opens Inventory, **Then** a Discovery summary shows "N VMs discovered
   from credential `<credName>`" where N equals the number of discovered VMs.
3. **Given** discovery found one or more powered-off VMs with a single NIC, **When** the Default
   bucket is created, **Then** it contains exactly those VMs.
4. **Given** discovery found no powered-off single-NIC VMs but did find other powered-off VMs,
   **When** the Default bucket is created, **Then** it contains the powered-off VM(s) with the
   fewest NICs.
5. **Given** discovery found no powered-off VMs at all, **When** the Default bucket is created,
   **Then** it contains the powered-on VM(s) with the fewest NICs.
6. **Given** discovery found no VMs that satisfy any tier (e.g., empty inventory), **When** the
   page loads, **Then** no Default bucket is created and the Inventory shows the discovered VMs
   with a message that no default bucket was created.

---

### User Story 2 - Organize VMs into buckets (Priority: P1)

The operator refines the plan by creating buckets: duplicating an existing bucket while choosing
which VMs to keep, editing a bucket's VM set and migration configuration, and deleting buckets
they no longer need. Throughout, the system enforces that every bucket has at least one VM and
that no VM belongs to two buckets.

**Why this priority**: Buckets are the core unit of the planner; the operator must be able to
shape them before any trigger or scheduling is meaningful.

**Independent Test**: From a populated Inventory, duplicate the Default bucket selecting a subset
of VMs, confirm a new bucket is created, confirm already-bucketed VMs are non-selectable, edit a
bucket and save changes, delete a non-default bucket, and confirm the invariants hold.

**Acceptance Scenarios**:

1. **Given** the Default bucket exists, **When** the operator opens its actions, **Then** only
   **Edit** and **Duplicate** are offered (no Delete).
2. **Given** any non-default bucket, **When** the operator opens its actions, **Then** **Edit**,
   **Duplicate**, and **Delete** are offered; Delete asks for confirmation.
3. **Given** the operator clicks Duplicate, **When** the VM-selection dialog opens, **Then** it
   lists all inventory VMs, and VMs already assigned to a bucket are shown greyed out with an
   "already in a bucket" label and cannot be selected.
4. **Given** the operator selects one or more available VMs and confirms Duplicate, **When** the
   new bucket is created, **Then** it contains exactly the selected VMs and inherits the source
   bucket's migration configuration as a starting point.
5. **Given** the operator clicks Edit on a bucket, **When** the editor opens, **Then** it shows a
   form equivalent to the Migration Form with the bucket's VMs pre-selected and its saved
   configuration pre-filled, and the primary action is **Save** (it does not launch a migration).
6. **Given** an edit/duplicate that would leave a bucket with zero VMs, **When** the operator
   tries to save, **Then** the action is blocked with a clear message.
7. **Given** a VM already in bucket X, **When** any operation attempts to also place it in bucket
   Y, **Then** the operation is rejected by both the UI and the backend.

---

### User Story 3 - Configure bucket defaults and schedule (Priority: P2)

When a bucket is created or edited, its migration configuration is pre-filled with sensible
auto-defaults (source cluster from the VMs, first destination cluster, first network/storage
mappings, no security/server groups), and the operator may set an optional future schedule time
for the bucket.

**Why this priority**: Auto-defaults are what make the planner fast to use; scheduling adds
control but is not required for a first migration.

**Independent Test**: Create a bucket and verify each field is pre-filled per the default rules;
set a schedule time and confirm past times are not selectable.

**Acceptance Scenarios**:

1. **Given** a bucket's VMs, **When** the configuration is pre-filled, **Then** the VMware source
   cluster is derived from the selected VMs, the destination cluster defaults to the first
   destination cluster from the OpenStack credential, and the first source→first destination
   network and storage mappings are pre-selected.
2. **Given** a new bucket, **When** the configuration is pre-filled, **Then** security groups and
   server group are empty and all other advanced options are at their default (unselected) state.
3. **Given** the operator sets a bucket schedule, **When** the date-time picker is shown, **Then**
   only future times can be selected (past times are disabled).

---

### User Story 4 - Trigger multiple buckets with an agent-count suggestion (Priority: P2)

The operator selects several buckets and triggers them. Before launch, the system recommends how
many migration agents to scale up (editable with +/-) and shows the reasoning, so the operator
can right-size capacity rather than guess.

**Why this priority**: Multi-bucket launch with capacity guidance is the headline value over the
one-at-a-time Migration Form, but it depends on buckets (US1/US2) existing first.

**Independent Test**: With multiple buckets present, open the trigger drawer, select a subset,
proceed, and verify a non-negative agent-count suggestion with a visible derivation that updates
when the selection changes; verify +/- override works.

**Acceptance Scenarios**:

1. **Given** two or more buckets exist, **When** the operator clicks Trigger, **Then** a drawer
   lists all buckets with checkboxes and the operator can select any subset.
2. **Given** a non-empty selection, **When** the operator proceeds, **Then** a recommended agent
   count is shown alongside a plain-language derivation of how it was computed.
3. **Given** a recommended agent count, **When** the operator clicks +/-, **Then** the value
   changes within allowed bounds (never below 0; capped at the configured maximum), and a note is
   shown if the workload exceeds one-shot capacity.

---

### User Story 5 - Recommended order and trigger-now vs schedule (Priority: P3)

When triggering, the system also recommends the order in which the selected buckets migrate —
prioritizing buckets most likely to succeed (powered-off-dominant, fewer NICs) so easy wins come
first. The operator chooses to trigger now (overriding per-bucket schedules) or to honor each
bucket's schedule.

**Why this priority**: Ordering and trigger-now/schedule precedence refine the launch but are not
required for a basic multi-bucket migration.

**Independent Test**: Select multiple buckets of differing power-state/NIC profiles, open the
trigger confirmation, and verify the recommended order places higher-success buckets first;
verify that choosing "Trigger now" ignores schedules for the included buckets.

**Acceptance Scenarios**:

1. **Given** buckets with differing powered-off ratios and NIC profiles, **When** the trigger
   confirmation is shown, **Then** the recommended order places higher-success buckets earlier
   (default bucket first when present).
2. **Given** buckets that have per-bucket schedule times, **When** the operator chooses **Trigger
   now**, **Then** those schedules are ignored for this run and the buckets start as soon as
   capacity allows.
3. **Given** the operator confirms the trigger, **When** execution begins, **Then** the selected
   buckets are launched via the existing migration path and each affected bucket's status updates
   to Scheduled or In progress.

---

### Edge Cases

- **Empty inventory / no eligible VMs**: no Default bucket CR is created; the page shows VMs (if
  any) with a "no default bucket created" message (US1 scenario 6).
- **Bucket would become empty**: any edit/duplicate that results in zero VMs is blocked.
- **VM already bucketed**: blocked in both UI (greyed, labelled) and backend (validation).
- **VM removed from inventory** (no longer discovered, e.g., deleted in vCenter) while it is a
  bucket member: bucket membership must reconcile gracefully — [NEEDS CLARIFICATION: drop the VM
  silently, or flag the bucket as needing attention?].
- **Bucket spans multiple source clusters**: [NEEDS CLARIFICATION (Q5): constrain a bucket to a
  single source cluster, or allow multi-cluster and expand per cluster at trigger time?].
- **Agent suggestion exceeds environment capacity** (`A_raw > A_max`): show a note that the run
  will proceed in waves rather than all at once.
- **Trigger with a bucket already In progress/Migrated**: such buckets should be non-selectable or
  clearly flagged in the trigger drawer.
- **Credential re-validation / re-discovery**: new VMs appearing later must not silently alter
  existing buckets; only the Default-bucket auto-population runs once at first discovery.

---

## Requirements *(mandatory)*

### Functional Requirements

**Navigation & discovery**

- **FR-001**: The system MUST add a new top-level **Inventory** item to the existing sidebar,
  consistent with the current navigation structure.
- **FR-002**: The Inventory page MUST show a Discovery summary stating the number of VMs
  discovered for the (single) VMware credential and the credential's name.
- **FR-003**: When no VMware credential exists, the Inventory page MUST show an empty state
  directing the operator to add a VMware credential.

**Buckets — model & default**

- **FR-004**: The system MUST represent each bucket as a persistent backend resource (a new
  `MigrationBucket` custom resource) keyed to its source VMware credential.
- **FR-005**: After VM discovery for the first credential completes, the backend MUST
  automatically create exactly one **Default bucket** unless no VM is eligible.
- **FR-006**: The Default bucket MUST be populated using the first non-empty tier of: (1)
  powered-off VMs with a single NIC; (2) powered-off VM(s) with the fewest NICs; (3) powered-on
  VM(s) with the fewest NICs. If no tier yields a VM, the system MUST NOT create a Default bucket
  and MUST surface a message in the Inventory.
- **FR-007**: Each bucket MUST carry a name, its set of VMs, a migration configuration, an
  optional schedule time, and a lifecycle status.
- **FR-008**: The Default bucket MUST be marked as such and MUST NOT be deletable; all other
  buckets MUST be deletable.

**Bucket actions & invariants**

- **FR-009**: Every bucket MUST offer **Edit** and **Duplicate**; non-default buckets MUST also
  offer **Delete** (with confirmation).
- **FR-010**: Edit MUST present a form equivalent to the existing Migration Form with the bucket's
  VMs pre-selected and configuration pre-filled, with a **Save** action that updates the bucket
  and does not launch a migration.
- **FR-011**: Duplicate MUST let the operator choose which inventory VMs to keep in the new
  bucket; VMs already assigned to another bucket MUST be shown greyed with an "already in a bucket"
  label and MUST NOT be selectable.
- **FR-012**: The system MUST enforce, in both UI and backend, that a bucket has at least one VM
  (no empty buckets).
- **FR-013**: The system MUST enforce, in both UI and backend, that any VM belongs to at most one
  bucket at a time (VM uniqueness across buckets).

**Bucket configuration defaults**

- **FR-014**: A bucket's configuration MUST default the VMware source cluster from its VMs'
  cluster, the destination cluster to the first destination cluster of the OpenStack credential,
  and network/storage mappings to the first source→first destination entries.
- **FR-015**: A bucket's configuration MUST default security groups and server group to empty and
  all other advanced options to their existing Migration Form defaults.
- **FR-016**: Bucket schedule selection MUST allow only future times (past times disabled).

**Bucket status**

- **FR-017**: Each bucket MUST display a status of Not migrated, Scheduled, In progress, or
  Migrated, derived from the execution state of the migration objects it produced.

**Trigger & planning**

- **FR-018**: The Inventory page MUST provide a **Trigger** action (placed consistently with
  existing UI) that opens a drawer listing buckets with checkboxes for multi-select.
- **FR-019**: On triggering a selection, the system MUST present a recommended migration-agent
  count that is editable via +/- controls, bounded at a minimum of 0 and a configured maximum.
- **FR-020**: The agent-count recommendation MUST be explainable — the UI MUST show the figures it
  was derived from.
- **FR-021**: On triggering a selection, the system MUST present a recommended execution order for
  the selected buckets that prioritizes buckets most likely to succeed (greater powered-off
  proportion, then fewer NICs per VM), with the Default bucket leading when present.
- **FR-022**: The trigger flow MUST offer **Trigger now** and **Schedule**; choosing Trigger now
  MUST ignore the per-bucket schedules of the buckets included in that trigger.
- **FR-023**: On confirmation, the system MUST scale migration agents toward the chosen count and
  launch the selected buckets in the chosen order.

**Reuse / non-regression (hard constraint)**

- **FR-024**: The system MUST NOT modify the existing `Migration` / `MigrationPlan` object schemas
  or the existing migration execution workflow. The planner MUST launch migrations by composing
  the existing CRDs at trigger time, the same way the current Migration Form does.
- **FR-025**: The feature MUST reuse the existing migration configuration UI (the Migration Form
  step components) rather than introducing a parallel configuration model.

### Key Entities

- **MigrationBucket** (new): a persistent, named group of VMs for one source credential. Key
  attributes: name, source VMware credential reference, list of member VMs, `isDefault` flag,
  embedded migration configuration (mirroring the Migration Form output: source/destination
  cluster, network/storage mappings, security/server groups, advanced options), optional schedule
  time, and a derived lifecycle status (Not migrated / Scheduled / In progress / Migrated).
  Relationships: references one VMware credential; references inventory VMs; at trigger time it is
  compiled into existing migration objects.
- **Discovered VM (inventory item)**: an already-existing discovered-VM record (one per VM) used
  read-only here. Relevant attributes consumed by the planner: power state, NIC count, source
  cluster, and disk size. Relationship: a VM is a member of at most one MigrationBucket.
- **Migration agent / worker**: an existing migration worker node. The planner reads existing
  agents' availability to recommend a scale-up count and triggers scaling; it does not change how
  agents execute migrations.
- **Execution objects (existing, unchanged)**: the existing migration-plan and rolling-migration
  objects that the trigger flow creates to actually run migrations. The planner produces these;
  it does not alter their behavior.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: From a validated VMware credential, an operator can reach an organized Inventory
  with a populated Default bucket without any manual VM selection.
- **SC-002**: An operator can go from "credential added" to "multi-bucket migration triggered" in
  fewer steps than creating the equivalent migrations one-by-one in the Migration Form (target:
  at least 50% fewer operator actions for 3+ buckets).
- **SC-003**: 100% of buckets at all times satisfy the invariants (≥1 VM; no VM in two buckets),
  verified by backend validation — violations are impossible via UI or API.
- **SC-004**: The recommended agent count is never negative and never exceeds the configured
  maximum, and its derivation is visible to the operator in 100% of trigger flows.
- **SC-005**: For a mixed set of buckets, the recommended order places the highest-success bucket
  (most powered-off, fewest NICs) first in 100% of cases.
- **SC-006**: Existing migration regression tests pass unchanged — there is zero modification to
  the `Migration` / `MigrationPlan` schemas and existing migration workflow behavior.
- **SC-007**: A migration launched via the planner produces the same migration outcome as the same
  configuration created through the Migration Form.

---

## Assumptions

- **Single VMware credential** for v1; the data model keys buckets by source credential so
  multiple credentials are a clean future extension.
- VM discovery already produces a per-VM record exposing power state, NIC count, source cluster,
  and disk size; the planner consumes these read-only.
- The OpenStack/PCD credential exposes an ordered list of destination clusters, networks, and
  volume types; "first entry" is well-defined for defaulting.
- Migration agents can be scaled up programmatically before a run; a configured maximum agent
  count (`A_max`) exists or will be defined.
- The existing Migration Form step components can be reused to render the bucket configuration
  editor.
- The planner launches migrations exclusively through the existing migration objects/workflow;
  no change to migration execution is in scope.

### Open clarifications carried into planning

- **Q5 — Multi-cluster buckets**: whether a bucket is constrained to a single source cluster or
  may span clusters and be expanded per cluster at trigger time.
- **Q8 — Agent algorithm constants**: concrete sources/values for per-migration CPU request,
  fresh-agent capacity, and the maximum agent ceiling `A_max`.
- **Q9 — Order editability**: whether operator drag-reordering of the recommended order ships in
  v1 or is read-only initially.
- **VM-removed-from-inventory** handling for an existing bucket member (drop vs flag).

> Algorithm details (agent-count and bucket-ordering) are specified in `DESIGN.md` §9 and will be
> captured in `research.md` during `/plan`.
