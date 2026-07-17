# Feature Specification: Migration Templates and Saved Configurations

**Feature Branch**: `2120-migration-templates`
**Created**: 2026-07-15
**Status**: Draft
**Input**: GitHub issue [platform9/vjailbreak#2120](https://github.com/platform9/vjailbreak/issues/2120) — "Add Support for Migration Templates and Saved Configurations"

## Overview

Operators migrating many VMs with similar settings (same source vCenter, destination cluster, network/storage mappings, copy method, cutover policy) currently re-enter that configuration from scratch on every migration. This feature lets an operator save the configuration of an in-progress migration as a named, reusable **Migration Template**, browse existing templates from a new "Templates" tab next to "Migrations", and apply a template to a new migration to pre-populate the form. Templates can also be cloned and deleted.

This is a UI- and CRD-level feature; it does not change the underlying migration execution engine (v2v-helper, disk copy/conversion) at all — it only changes how a `MigrationPlan`'s inputs get authored.

---

## Clarifications

### Session 2026-07-15

- Q: The CRD design direction — reuse the existing `MigrationTemplate` CRD (today an ephemeral, uuid-named, auto-created/auto-deleted per-migration-session config blob) vs. introduce a new CRD? → A: Extend the existing `MigrationTemplate` CRD rather than add a new CRD. This is the user's stated leaning, **not yet fully confirmed** — flagged as a risk in Assumptions and Edge Cases below, since it requires carefully gating the existing auto-patch (`useCredentialFetching.ts`) and auto-delete-on-cancel (`useMigrationFormSubmit.ts`) behavior so saved templates are never silently mutated or deleted by an unrelated New Migration session.

- Q: vJailbreak today has no per-user login/identity system (single shared admin session per appliance, API-token auth). The mockup showed "Owner" avatars and "N shared / M private" counts. Is a real multi-user identity/ownership model in scope? → A: No — and since there is no real identity to attach, owner and shared/private visibility are dropped entirely from v1. Every template is visible to every operator of this appliance; there is no per-template ownership label or visibility filter. Real multi-tenant RBAC (and any ownership/visibility concept built on top of it) is out of scope.
- Q: What counts as a template "use" for the times-used/last-used counters? → A: Clicking "Use template" only prefills the New Migration form — it is NOT itself a countable use event. The counters increment only when a migration created from that prefilled form is actually submitted successfully.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Save a Migration Configuration as a Template (Priority: P1)

An operator is filling out a migration in the New Migration drawer. Instead of losing that configuration once the drawer closes, they save it as a named template with an optional description so a future migration can reuse it.

**Why this priority**: Without the ability to create a template, there is nothing to browse or apply — this is the foundational capability.

**Independent Test**: Fill out a migration form (source/destination, VMs, network/storage mappings, options), click "Save as template", provide a name, and confirm the template appears in the Templates list with the correct summary data — independent of ever submitting an actual migration.

**Acceptance Scenarios**:

1. **Given** an operator has configured source/destination, network/storage mappings, and migration options in the New Migration drawer, **When** they choose "Save as template" and enter a name (description optional), **Then** a new template is created and appears in the Templates list with that name, description, source→destination summary, mapping count, and copy-method/cutover tags matching the form's current values.
2. **Given** an operator provides a template name that duplicates an existing template's name, **When** they attempt to save, **Then** the system rejects the save with a clear "name already in use" message (template names are unique per FR-002).
3. **Given** an operator has not yet selected a source and destination, **When** they attempt "Save as template", **Then** the action is disabled or blocked with a message indicating which required fields (at minimum source vCenter, destination cluster) are missing.
4. **Given** a template has been saved from an in-progress form, **When** the operator continues and submits the migration (or cancels it), **Then** the saved template is unaffected — it is not modified or deleted by that migration session's own auto-managed working config.

---

### User Story 2 - Browse Available Templates (Priority: P1)

An operator navigates to the "Templates" tab (sibling to "Migrations") to see all templates available to them, so they can find the right one to reuse.

**Why this priority**: Discovery is required before templates provide any value; without a list, saved templates are invisible.

**Independent Test**: With at least one saved template, navigate to Templates and confirm it is listed with accurate summary information, independent of applying or deleting anything.

**Acceptance Scenarios**:

1. **Given** one or more templates exist, **When** the operator opens the "Templates" tab, **Then** they see a card per template showing: name, description, source→destination summary (with tenant/project and target cluster), copy-method/cutover/mapping-count tags, last-used recency, usage count, and a "Use" action.
2. **Given** the operator types into the search box, **When** the query matches a template's name or description, **Then** only matching templates remain visible; the visible count updates.
3. **Given** the operator changes the sort control, **When** "Last used" (default) or another supported sort is selected, **Then** the card order updates accordingly.
4. **Given** no templates exist yet, **When** the operator opens the Templates tab, **Then** an empty state is shown with guidance on how to create the first template (pointing back to the New Migration flow's "Save as template" action).

---

### User Story 3 - Apply a Template to a New Migration (Priority: P1)

An operator starts a new migration and chooses to base it on an existing template, so the form is pre-populated with that template's saved configuration instead of being filled out manually.

**Why this priority**: This is the core value delivery of the feature — the whole point of saving templates is to reuse them quickly on future migrations.

**Independent Test**: With a saved template, click "Use" (from the Templates list or its detail drawer), and confirm the New Migration drawer opens with the template's source/destination, mappings, and options pre-filled, then the operator can still edit any field before submitting.

**Acceptance Scenarios**:

1. **Given** a template with a saved source vCenter, destination cluster, network/storage mappings, and migration options, **When** the operator clicks "Use" on that template, **Then** the New Migration drawer opens with all of those fields pre-populated, and the VM selection step is left for the operator to choose VMs available under the pre-filled source/destination (VM selection itself is not templated, since VM inventory changes over time).
2. **Given** a template is applied and the drawer is pre-filled, **When** the operator changes one or more fields before submitting, **Then** the migration proceeds using the edited values — applying a template never locks fields as read-only.
3. **Given** a template references a source vCenter, destination cluster, network mapping, storage mapping, or credential that no longer exists (e.g. deleted since the template was saved), **When** the operator applies that template, **Then** the affected fields are left blank/unset with a clear inline warning identifying which referenced resource is missing, rather than silently failing or crashing the form.
4. **Given** a template is successfully applied and the resulting migration is submitted, **When** the submission succeeds, **Then** the template's "times used" count increments and "last used" timestamp updates (see Clarifications — usage-tracking definition).

---

### User Story 4 - View Template Details (Priority: P2)

An operator clicks a template card to see its full configuration in a detail panel before deciding whether to use, clone, or delete it.

**Why this priority**: Useful for confidence and auditability before reuse, but the feature is still usable without it if List (US2) already surfaces enough summary data to act on.

**Independent Test**: Click a template card and confirm a detail drawer opens showing full usage/created metadata, source & destination fields, and the full list of network/storage mappings, independent of taking any action from the drawer.

**Acceptance Scenarios**:

1. **Given** a template card, **When** the operator clicks it, **Then** a right-side detail drawer opens (list dims behind it) showing: name, description; an info block with Times Used, Last Used, Created; a "Source & Destination" section (source vCenter, destination, tenant/project, target cluster); and a "Network & Storage Mappings" section listing every mapping pair.
2. **Given** the detail drawer is open, **When** the operator clicks the close (X) control or clicks outside the drawer, **Then** the drawer closes and the underlying Templates list is restored to its prior (undimmed) state.

---

### User Story 5 - Delete and Clone Templates (Priority: P2)

An operator removes a template that is no longer relevant, or clones an existing template as the starting point for a new, slightly different one.

**Why this priority**: Lifecycle management keeps the template list useful over time and avoids duplicate manual re-creation of similar templates, but is not required for the first save/browse/apply loop to deliver value.

**Independent Test**: From a template's detail drawer, delete it and confirm it disappears from the list; separately, clone a template and confirm a new, independent template is created with the same configuration and a distinguishing name.

**Acceptance Scenarios**:

1. **Given** a template's detail drawer is open, **When** the operator clicks "Delete" and confirms, **Then** the template is permanently removed and no longer appears in the Templates list. Deleting a template MUST NOT affect any `MigrationPlan` or `Migration` previously created from it (those retain their own resolved configuration and continue to run/display normally).
2. **Given** a template's detail drawer is open, **When** the operator clicks "Clone", **Then** a new template is created with the same configuration and a default name indicating it is a copy (e.g. "Production RHEL · East (copy)"); the original template is unmodified.

---

### Edge Cases

- **Deleting a resource referenced by a template**: If a VMware/OpenStack credential, network mapping, or storage mapping referenced by a saved template is deleted, the template itself is NOT deleted — it becomes "stale" and surfaces the missing-reference warning described in US3 Scenario 3 when applied or viewed.
- **Concurrent template name collision**: Two operators attempt to save a template with the same name at nearly the same time — the second save MUST fail with the duplicate-name error (FR-002), not silently overwrite the first.
- **Existing per-session `MigrationTemplate` objects**: The disposable, uuid-named `MigrationTemplate` objects that today's New Migration drawer auto-creates/patches/deletes per session MUST remain functionally unchanged — i.e., MigrationPlan/retry-prefill's existing consumption of these objects must not regress. Saved templates must be distinguishable from these (e.g. a "saved" marker) so the existing auto-delete-on-cancel logic skips them.
- **Cloning a stale template**: Cloning a template that has a missing/stale reference (see above) MUST carry over the same stale reference as-is (with the same warning on the clone), rather than failing the clone operation.
- **Empty search/filter results**: Searching or filtering to zero matching templates shows an explicit "no templates match" empty state, distinct from the true zero-templates empty state in US2 Scenario 5.
- **Applying a template mid-edit**: If the operator has already made manual edits in the New Migration drawer and then applies a template, the template's values MUST overwrite the current form state (with a confirmation step if unsaved manual edits would be lost), to avoid ambiguous partial-merge behavior.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow an operator to save the current New Migration form configuration as a named template, with an optional free-text description, from within the New Migration drawer.
- **FR-002**: Template names MUST be unique; the system MUST reject a save/clone/rename that would produce a duplicate name with a clear error message.
- **FR-003**: The system MUST provide a "Templates" tab, presented as a sibling to the existing "Migrations" tab, showing a count badge of available templates.
- **FR-004**: The Templates view MUST support: free-text search (by name and description), sorting (at minimum by "last used"), and switching between grid and list layouts.
- **FR-005**: Each template card MUST display: name, description, source→destination summary (source vCenter, destination, tenant/project, target cluster), relevant configuration tags (copy method — hot/cold/mock; cutover policy; mapping count), last-used recency, usage count, and a primary "Use" action.
- **FR-006**: Clicking a template MUST open a detail drawer (reusing the existing `DrawerShell` slide-in panel pattern) showing full metadata (times used, last used, created date), full source & destination fields, and the complete list of network/storage mappings, with Delete, Clone, and "Use template" actions.
- **FR-007**: Selecting "Use" (from the list card or the detail drawer) MUST open the New Migration drawer with the template's saved source/destination, network/storage mappings, and migration options (copy method, cutover policy, GPU/flavor options, etc.) pre-populated; VM selection MUST remain the operator's own choice from the live inventory of the pre-filled source/destination.
- **FR-008**: All fields pre-populated from a template MUST remain fully editable before submission — applying a template MUST NOT lock or disable any form field.
- **FR-009**: If a template references a credential, network mapping, storage mapping, or cluster that no longer exists at apply time, the system MUST leave the corresponding field(s) unset and display an inline warning identifying the missing reference, rather than failing to open the form or silently guessing a substitute value.
- **FR-010**: The system MUST support deleting a template (with confirmation) and cloning a template (producing an independent copy with a distinguishing default name); deleting a template MUST NOT alter any Migration/MigrationPlan previously created from it.
- **FR-011**: The system MUST track and display each template's usage count and last-used timestamp. Clicking "Use template" (prefilling the form) MUST NOT itself increment these counters; they MUST increment only when a migration created from that prefilled form is submitted successfully.
- **FR-012**: The existing disposable, per-session `MigrationTemplate` object created and auto-managed by the New Migration drawer (`useCredentialFetching.ts` auto-patch, `useMigrationFormSubmit.ts` auto-delete-on-cancel) MUST continue to function exactly as today for migrations not created from a saved template, and MUST NOT be able to overwrite or delete a saved template.
- **FR-013**: Saving, applying, deleting, and cloning templates MUST NOT alter the existing `MigrationPlan`/`MigrationPlanReconciler` consumption path that reads `MigrationTemplate` by name to drive an actual migration (network/storage mapping resolution, virtio driver selection, GPU flavor, HotAdd proxy VM reference, etc.).

### Key Entities

- **Migration Template**: A saved, named, reusable migration configuration. Attributes: name (unique, user-provided), optional description, created timestamp, times-used count, last-used timestamp, source vCenter/VMware reference, destination OpenStack/PCD reference (tenant/project, target cluster), network mappings, storage mappings, migration options (copy method, cutover policy, GPU/flavor settings, post-migration actions). Extends the existing ephemeral `MigrationTemplate` CRD rather than introducing a new CRD (per Clarifications), distinguished from disposable per-session config objects by a "saved" marker.
- **Migration Configuration**: The full set of form values (source/destination, mappings, options) captured at "save as template" time or applied at "use template" time — corresponds to the existing `FormValues` shape already used by the New Migration drawer.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can save a fully-configured migration as a named template in under 15 seconds (name entry + confirm), without leaving the New Migration drawer.
- **SC-002**: An operator can locate a previously saved template (via search, filter, or sort) and open its detail view in under 10 seconds from landing on the Templates tab.
- **SC-003**: Applying a template to a new migration reduces the number of fields the operator must manually fill in by at least 80% compared to configuring the same migration from scratch (excluding VM selection, which remains manual by design).
- **SC-004**: 100% of migrations created from a template where all referenced resources (credentials, mappings, clusters) still exist require zero corrective edits to source/destination/mapping fields before submission.
- **SC-005**: 100% of templates referencing a since-deleted resource clearly surface a warning identifying the missing reference at apply time, with zero form crashes or silent misconfiguration.
- **SC-006**: Deleting or cloning a template never alters the behavior or configuration of any previously submitted migration — verified by zero regressions in existing Migration/MigrationPlan submission and retry flows.

---

## Assumptions

- The existing `MigrationTemplate` CRD is extended (not replaced by a new CRD) to represent saved templates, per the user's stated direction — flagged as **not yet fully confirmed** in Clarifications; this spec proceeds on that basis but the implementation plan should re-confirm before any breaking schema change ships.
- No genuine multi-user identity/authorization system exists in vJailbreak today (single shared appliance session, API-token auth). Per Clarifications, v1 drops owner and shared/private visibility entirely rather than faking them with unverified UI labels — every template is visible to every operator of this appliance, with no ownership or visibility concept. Real multi-tenant RBAC (and any ownership/visibility feature built on it) is out of scope.
- VM selection is never templated — only source/destination, mappings, and options are saved/applied, since the specific VMs to migrate are expected to differ per migration even when the surrounding configuration is identical.
- Template "times used" / "last used" tracking increments only on successful migration submission from an applied template, not on the "Use template" click that merely prefills the form.
- The Templates tab is additive UI (new tab alongside "Migrations"); it does not change the existing Migrations list, its route, or its table behavior.
- This feature does not introduce any new backend REST endpoints in `pkg/vpwned` — all template CRUD continues to go through the existing generic Kubernetes custom-resource API path already used by the current `MigrationTemplate` frontend modules.
- The two existing near-duplicate frontend API modules for migration templates (`ui/src/api/migration-templates/` and `ui/src/features/migration/api/migration-templates/`) are a pre-existing consolidation opportunity; this feature does not require fixing that duplication, but implementers should avoid deepening it by adding saved-template logic to only one of the two without checking both call sites.
