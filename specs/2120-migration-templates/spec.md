# Feature Specification: Migration Templates and Saved Configurations

**Feature Branch**: `2120-migration-templates`
**Created**: 2026-07-15
**Last updated**: 2026-07-22 (post-implementation, second pass — Edit/Create Template entry points and Templates-tab type filter added; see "Implementation Reality" below)
**Status**: Implemented (US1–US7 shipped; FR-009's inline stale-reference warning and FR-011's usage tracking were NOT implemented — see notes)
**Input**: GitHub issue [platform9/vjailbreak#2120](https://github.com/platform9/vjailbreak/issues/2120) — "Add Support for Migration Templates and Saved Configurations"

## Implementation Reality (read this first)

The backend shipped in PR [platform9/vjailbreak#2158](https://github.com/platform9/vjailbreak/pull/2158) took a **different technical direction** than this spec originally assumed (see Clarifications below) — it added a brand-new CRD, `MigrationBlueprint` (`k8s/migration/api/v1alpha1/migrationblueprint_types.go`, plural `migrationblueprints`), instead of extending the existing `MigrationTemplate` CRD. The frontend was integrated against the real PR, not the mock. Concretely:

- **No status subresource exists on `MigrationBlueprint`** — `timesUsed`/`lastUsedAt` usage tracking (FR-011, part of FR-005/FR-006, US2 Scenario 1, US4 Scenario 1) is **not possible server-side** and was dropped from the UI entirely (not deferred, not faked client-side). Templates tab does not show "last used" or "times used" anywhere, and sort has no "last used" option (only "Newest" and "Name").
- **Owner/shared/private visibility** was already correctly resolved as out-of-scope in the original Clarifications below — that part of the spec was accurate and remains so.
- **`vmwareClusterName` was added to the CRD after this doc's last check (2026-07-20 → now)** — the blueprint spec now has a source cluster field alongside `vmwareRef`. Save (`MigrationForm.tsx`'s `buildSaveTemplateInput`) and Apply (`useApplyTemplatePrefill.ts`, name→id resolution) both round-trip it into the New Migration form's `vmwareCluster` dropdown. It is **not displayed** anywhere in the Templates tab (card, table, or detail drawer) — captured and restored, but invisible. A second new field, `noVMwareClusterFilter` (bool), also landed on the CRD/model but is wired nowhere in the UI — dead field for now.
- **`targetPCDClusterName` is a cluster name, not the PCD cluster id** the New Migration form's `FormValues.pcdCluster` expects — prefill resolves name→id via the same `pcdData.find(...)` lookup pattern `useRetryPrefill.ts` already uses (see `useApplyTemplatePrefill.ts`).
- **FR-009's "clear inline warning" for stale/missing references was not implemented.** `useApplyTemplatePrefill.ts` silently leaves a field unresolved (e.g. an unmatched cluster name falls through to a raw string the form's dropdowns won't match) with no `staleReferenceWarnings` array and no `Alert` rendered. This is a real gap against the original spec, not a scope decision — flag it if picking this back up.
- **Tenant/project display** (shown as a "tenant · cluster" second line on cards/table and a "Tenant / project" row in the detail drawer) is derived at **display time only**, by looking up the blueprint's `pcdRef` against the live OpenStack creds list's `spec.projectName` (`useTemplateTenantLookup.ts`) — it is not stored on the blueprint itself, and was not part of the original FR-005/FR-006 wording (added because the real design mockups reviewed mid-implementation showed it).
- **Migration Options section** in the detail drawer (copy mode, cutover, guest OS, advanced-options summary) is a real addition beyond original FR-006, added because it's fully derivable from existing blueprint spec fields (`migrationStrategy`, `osFamily`, `advancedOptions`, `postMigrationAction`, `firstBootScript`) — see data-model.md.
- **List/grid view (FR-004)**: both are fully real now — grid is `TemplateCard.tsx` (3-per-row), list is a real dense `TemplatesTable.tsx` (not a re-flowed card column as first built).
- Ground truth for the actual data model now lives in `data-model.md` and `contracts/crds.md` (both rewritten to match `MigrationBlueprint`) — `plan.md`/`research.md`/`tasks.md` describe the **original, superseded** technical plan and are kept for historical record only, each with a pointer back here.

### 2026-07-22 addendum — Edit/Create Template entry points, type filter, drawer fixes (new ground not in original scope)

A second implementation pass added capability beyond the original five user stories, all now covered
by new User Stories 6–7 and FR-014–FR-016 below:

- **Edit Template** (US6): templates were previously read-only after creation (only Use/Clone/Delete existed). An Edit action (pencil icon on the card, table row, and detail-drawer footer) now opens the **same New Migration form**, prefilled via the existing `useApplyTemplatePrefill` path (same as "Use"), with `templateMode='edit'`. The footer swaps "Start Migration"/"Save as template" for a single "Save Changes" button, which `PUT`s the **same** `MigrationBlueprint` object (not a new one) via `useUpdateTemplate`, sending `metadata.resourceVersion` for standard Kubernetes optimistic concurrency. `SavedTemplate` gained a `resourceVersion: string` field to carry this through. See `data-model.md` and `contracts/crds.md` for the wire-level detail.
- **Create Template** (US7): a second, standalone way to create a template that does **not** require first configuring an in-progress migration (US1 remains — "Save as template" from inside a live New Migration session is unchanged). The Templates tab's primary button ("Create New Template") opens the same form with `templateMode='create'` and no prefill; footer shows only "Create Template". Internally this still goes through the same `POST` as any other create — no new backend behavior, just a second UI entry point into the existing save flow.
- **Filter by copy method** (extends FR-004): the Templates tab gained a FilterList icon + menu (Hot copy / Cold copy / Mock copy / All types), alongside the existing search and sort controls — `filterTemplates()` in `templateFilters.ts` took a third `copyMethod` parameter.
- **Detail drawer fixes**: (a) the "Advanced" row was a real bug — it rendered only the *names* of set advanced options (e.g. "Post-migration script · Rename VM"), never their actual values. Replaced with a proper "Advanced options" section listing each set option as its own labeled row with its real value (script content, rename suffix, target folder, security groups, image profiles, periodic-sync interval, etc.) via a new `buildAdvancedOptionRows()` helper in `templateLabels.ts`. (b) Created-date moved out of its own full-width info-block row into a small pill next to the title, freeing vertical space. (c) Drawer width increased 460px → 560px.
- **Card polish** (`TemplateCard.tsx`): title is single-line ellipsis (was unbounded, could overflow the card on a long name); description no longer reserves fixed dead space when absent or short (was over-engineered through several iterations before landing on: render conditionally, clamp to 2 lines, no forced min-height — see file history for the false starts, including one where a `min-height` on a `-webkit-line-clamp` element caused text to visibly bleed past the card into the row below); hover now shows a subtle `primary.main` border (matching the same affordance already used on the Migrations-tab stat-style cards elsewhere in the app); clone/delete/edit icons are always visible, not hover-only; the copy-method avatar uses a softer `alpha()`-tinted background instead of a solid saturated block.
- **Search bar**: iterated through several visual variants (outlined-with-hidden-border, then MUI `standard` variant to match the Migrations tab's own search field) before ultimately being **relocated** — see the Migrations-tab note directly below, since the two tabs' toolbars were unified into one shared row.
- **Migrations tab** — same session, same shared files (`MigrationsTable.tsx`, `MigrationsPage.tsx`), but genuinely **out of this feature's scope** (Migration Templates); noted here only because it touched files this spec also touches, expanding on the existing brief mention below:
  - The "In Progress / Awaiting Action / Pending / Succeeded / Failed" stat cards were removed entirely (they duplicated the table's own status filter and ate a lot of vertical space at small viewport widths).
  - Search, the date-range filter, the status filter, and Refresh were relocated off their own row (and off the page header, for Refresh) into a single row shared with the Migrations/Templates tabs themselves — `CustomSearchToolbar` (`src/components/grid/`) gained a "standalone controlled search" mode (`searchValue`/`onSearchChange` props, rendering a plain `TextField` instead of `GridToolbarQuickFilter`) specifically so it could be reused outside a DataGrid context for this. `MigrationsTable` is now a controlled component for search/status/date-filter (props, with defaults so the one other embedded consumer — `RollingMigrationsTable.tsx`'s drawer — keeps working unfiltered).
  - The "Showing X of Y" counter next to the search box was removed.
  - The tab labels ("Migrations 3", "Templates 3") moved their count from a `Typography` next to the page `<h1>` into a small circular pill badge on the tab itself (filled when active, muted when not).
  - The Source → Destination column is now hidden by default (`columnVisibilityModel`), still togglable via the grid's own column menu.
  - The Progress column: a from-scratch redesign (structured `{primaryText, secondaryText, barColor}` derived per-phase, no fabricated ETA/throughput/error-codes) was built, then **explicitly reverted** back to the original icon+tooltip+`LinearProgress` rendering on request — net change here is zero; mentioned only so a future session doesn't rediscover and re-attempt the same redesign without knowing it was already tried and backed out.

---

## Overview

Operators migrating many VMs with similar settings (same source vCenter, destination cluster, network/storage mappings, copy method, cutover policy) currently re-enter that configuration from scratch on every migration. This feature lets an operator save the configuration of an in-progress migration as a named, reusable **Migration Template**, browse existing templates from a new "Templates" tab next to "Migrations", and apply a template to a new migration to pre-populate the form. Templates can also be cloned and deleted.

This is a UI- and CRD-level feature; it does not change the underlying migration execution engine (v2v-helper, disk copy/conversion) at all — it only changes how a `MigrationPlan`'s inputs get authored.

---

## Clarifications

### Session 2026-07-15

- Q: The CRD design direction — reuse the existing `MigrationTemplate` CRD (today an ephemeral, uuid-named, auto-created/auto-deleted per-migration-session config blob) vs. introduce a new CRD? → A: Extend the existing `MigrationTemplate` CRD rather than add a new CRD. This is the user's stated leaning, **not yet fully confirmed** — flagged as a risk in Assumptions and Edge Cases below, since it requires carefully gating the existing auto-patch (`useCredentialFetching.ts`) and auto-delete-on-cancel (`useMigrationFormSubmit.ts`) behavior so saved templates are never silently mutated or deleted by an unrelated New Migration session.
  - **RESOLVED DIFFERENTLY (2026-07-20)**: Backend PR #2158 shipped a brand-new CRD, `MigrationBlueprint`, instead. See "Implementation Reality" above. The ephemeral per-session `MigrationTemplate` object and its auto-patch/auto-delete lifecycle are entirely untouched by this feature — saved templates live in a completely separate CRD, so no `Spec.Saved` guard was ever needed.

- Q: vJailbreak today has no per-user login/identity system (single shared admin session per appliance, API-token auth). The mockup showed "Owner" avatars and "N shared / M private" counts. Is a real multi-user identity/ownership model in scope? → A: No — and since there is no real identity to attach, owner and shared/private visibility are dropped entirely from v1. Every template is visible to every operator of this appliance; there is no per-template ownership label or visibility filter. Real multi-tenant RBAC (and any ownership/visibility concept built on top of it) is out of scope. **(Confirmed as implemented — still correct.)**
- Q: What counts as a template "use" for the times-used/last-used counters? → A: Clicking "Use template" only prefills the New Migration form — it is NOT itself a countable use event. The counters increment only when a migration created from that prefilled form is actually submitted successfully.
  - **RESOLVED DIFFERENTLY (2026-07-20)**: Moot — `MigrationBlueprint` has no status subresource, so no counter of any kind could be implemented server-side. Usage tracking (times-used, last-used) was dropped from the UI entirely, not just the increment-trigger semantics.

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

1. **Given** one or more templates exist, **When** the operator opens the "Templates" tab, **Then** they see a card per template showing: name, description, source→destination summary (with tenant/project and target cluster), copy-method/cutover/mapping-count tags, and a "Use" action. ~~last-used recency, usage count~~ — **dropped; see Implementation Reality (no backend status subresource exists to track this).**
2. **Given** the operator types into the search box, **When** the query matches a template's name or description, **Then** only matching templates remain visible; the visible count updates.
3. **Given** the operator changes the sort control, **When** "Newest" (default) or "Name" is selected, **Then** the card order updates accordingly. ~~"Last used"~~ **dropped — no data to sort by.**
4. **Given** no templates exist yet, **When** the operator opens the Templates tab, **Then** an empty state is shown with guidance on how to create the first template (pointing back to the New Migration flow's "Save as template" action).

---

### User Story 3 - Apply a Template to a New Migration (Priority: P1)

An operator starts a new migration and chooses to base it on an existing template, so the form is pre-populated with that template's saved configuration instead of being filled out manually.

**Why this priority**: This is the core value delivery of the feature — the whole point of saving templates is to reuse them quickly on future migrations.

**Independent Test**: With a saved template, click "Use" (from the Templates list or its detail drawer), and confirm the New Migration drawer opens with the template's source/destination, mappings, and options pre-filled, then the operator can still edit any field before submitting.

**Acceptance Scenarios**:

1. **Given** a template with a saved source vCenter, destination cluster, network/storage mappings, and migration options, **When** the operator clicks "Use" on that template, **Then** the New Migration drawer opens with all of those fields pre-populated, and the VM selection step is left for the operator to choose VMs available under the pre-filled source/destination (VM selection itself is not templated, since VM inventory changes over time).
2. **Given** a template is applied and the drawer is pre-filled, **When** the operator changes one or more fields before submitting, **Then** the migration proceeds using the edited values — applying a template never locks fields as read-only.
3. **Given** a template references a source vCenter, destination cluster, network mapping, storage mapping, or credential that no longer exists (e.g. deleted since the template was saved), **When** the operator applies that template, **Then** the affected fields are left blank/unset with a clear inline warning identifying which referenced resource is missing, rather than silently failing or crashing the form. **NOT IMPLEMENTED — see Implementation Reality.** `useApplyTemplatePrefill.ts` leaves the field unresolved with no warning shown; this scenario still describes the intended behavior, not what ships today.
4. ~~Given a template is successfully applied and the resulting migration is submitted, When the submission succeeds, Then the template's "times used" count increments and "last used" timestamp updates~~ — **dropped, no backend field to write to (see Implementation Reality).**

---

### User Story 4 - View Template Details (Priority: P2)

An operator clicks a template card to see its full configuration in a detail panel before deciding whether to use, clone, or delete it.

**Why this priority**: Useful for confidence and auditability before reuse, but the feature is still usable without it if List (US2) already surfaces enough summary data to act on.

**Independent Test**: Click a template card and confirm a detail drawer opens showing full usage/created metadata, source & destination fields, and the full list of network/storage mappings, independent of taking any action from the drawer.

**Acceptance Scenarios**:

1. **Given** a template card, **When** the operator clicks it, **Then** a right-side detail drawer opens (list dims behind it) showing: name, description; an info block with ~~Times Used, Last Used,~~ Created (usage stats dropped — see Implementation Reality); a "Source & Destination" section (source vCenter, destination, tenant/project, target cluster); a "Network & Storage Mappings" section listing every mapping pair plus copy method; and a **Migration Options** section (copy mode, cutover, guest OS, advanced-options summary — not in the original spec, added because it's fully derivable from real blueprint fields).
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

### User Story 6 - Edit an Existing Template (Priority: P2) — *added 2026-07-22*

An operator realizes a saved template's configuration is out of date (e.g. the target cluster changed) and wants to correct it in place, rather than delete-and-recreate or leave it stale.

**Why this priority**: Templates were originally create-once/read-only after saving; without Edit, any drift between a template and current infrastructure required deleting and re-saving from scratch, losing the template's identity (name, description) in the process.

**Independent Test**: Open a template's Edit action, change a field (e.g. the target cluster or a mapping), click "Save Changes", and confirm the same template (same name, same row in the list) now reflects the new value — no duplicate template was created.

**Acceptance Scenarios**:

1. **Given** a template exists, **When** the operator clicks its Edit action (available on the card, the list-view row, and the detail drawer footer), **Then** the New Migration form opens pre-filled with that template's saved configuration (the same prefill path US3 uses for "Use"), and the footer shows a single "Save Changes" action in place of "Start Migration"/"Save as template".
2. **Given** the Edit form is open, **When** the operator changes one or more fields and clicks "Save Changes", **Then** the same underlying template object is updated in place (same name, same identity in the Templates list) — this MUST NOT create a second, duplicate template.
3. **Given** the Edit form is open, **When** the operator clicks "Save Changes" but the template was concurrently modified elsewhere since the form opened (stale `resourceVersion`), **Then** the save fails with an error surfaced to the operator rather than silently overwriting the concurrent change.

---

### User Story 7 - Create a Template Directly (Priority: P2) — *added 2026-07-22*

An operator wants to create a new template from scratch — without first configuring and abandoning an in-progress migration just to reach the "Save as template" action.

**Why this priority**: US1's "Save as template" only exists inside an active New Migration session, which is an awkward path when the operator's actual goal from the start is "make a template," not "start (and then not finish) a migration."

**Independent Test**: From the Templates tab, click "Create New Template", fill out source/destination/mappings/options in the resulting form, click "Create Template", and confirm the new template appears in the Templates list — without any `Migration`/`MigrationPlan` having been created along the way.

**Acceptance Scenarios**:

1. **Given** the operator is on the Templates tab, **When** they click "Create New Template", **Then** the same New Migration form opens with no prefilled values and a footer showing only a "Create Template" action (no "Start Migration", no separate "Save as template" secondary button).
2. **Given** the Create Template form is open and the operator fills in source/destination/mappings/options, **When** they click "Create Template", **Then** a new template is saved (same underlying create path as US1) and appears in the Templates list; no migration is started or implied by this flow.

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
- **FR-004**: The Templates view MUST support: free-text search (by name and description), sorting (at minimum by "last used"), switching between grid and list layouts, and filtering by copy method (hot/cold/mock — see FR-016, added 2026-07-22).
- **FR-005**: Each template card MUST display: name, description, source→destination summary (source vCenter, destination, tenant/project, target cluster), relevant configuration tags (copy method — hot/cold/mock; cutover policy; mapping count), and a primary "Use" action. ~~last-used recency, usage count~~ **dropped — no backend field exists (see Implementation Reality).** Tenant/project is resolved at display time from the live OpenStack creds list, not stored on the template.
- **FR-006**: Clicking a template MUST open a detail drawer (reusing the existing `DrawerShell` slide-in panel pattern) showing metadata (created date; ~~times used, last used~~ dropped), full source & destination fields, the complete list of network/storage mappings plus copy method, and a Migration Options summary (copy mode, cutover, guest OS, advanced options — real addition beyond original scope), with Delete, Clone, and "Use template" actions.
- **FR-007**: Selecting "Use" (from the list card or the detail drawer) MUST open the New Migration drawer with the template's saved source/destination, network/storage mappings, and migration options (copy method, cutover policy, GPU/flavor options, etc.) pre-populated; VM selection MUST remain the operator's own choice from the live inventory of the pre-filled source/destination. Source cluster (`vmwareClusterName`) IS captured and restored into the `vmwareCluster` dropdown (`useApplyTemplatePrefill.ts`) — narrower gap than previously noted, just source datacenter is still not stored.
- **FR-008**: All fields pre-populated from a template MUST remain fully editable before submission — applying a template MUST NOT lock or disable any form field.
- **FR-009**: If a template references a credential, network mapping, storage mapping, or cluster that no longer exists at apply time, the system MUST leave the corresponding field(s) unset and display an inline warning identifying the missing reference, rather than failing to open the form or silently guessing a substitute value. **NOT IMPLEMENTED** — `useApplyTemplatePrefill.ts` leaves the field unresolved with no warning surfaced. Real gap, not a scope decision.
- **FR-010**: The system MUST support deleting a template (with confirmation) and cloning a template (producing an independent copy with a distinguishing default name); deleting a template MUST NOT alter any Migration/MigrationPlan previously created from it. **Implemented** — plus hover-revealed clone/delete icons on the grid card and always-visible icons in the list/table view (UI polish beyond original wording).
- **FR-011**: ~~The system MUST track and display each template's usage count and last-used timestamp...~~ **DROPPED ENTIRELY** — `MigrationBlueprint` has no status subresource; no counter of any kind is possible server-side (see Implementation Reality).
- **FR-012**: The existing disposable, per-session `MigrationTemplate` object created and auto-managed by the New Migration drawer (`useCredentialFetching.ts` auto-patch, `useMigrationFormSubmit.ts` auto-delete-on-cancel) MUST continue to function exactly as today for migrations not created from a saved template, and MUST NOT be able to overwrite or delete a saved template. **Moot but trivially satisfied** — saved templates are `MigrationBlueprint` objects, a completely separate CRD from the ephemeral `MigrationTemplate`, so there is no shared-object risk at all; no guard code was needed.
- **FR-013**: Saving, applying, deleting, and cloning templates MUST NOT alter the existing `MigrationPlan`/`MigrationPlanReconciler` consumption path that reads `MigrationTemplate` by name to drive an actual migration (network/storage mapping resolution, virtio driver selection, GPU flavor, HotAdd proxy VM reference, etc.). **Satisfied** — `MigrationPlanReconciler` never reads `MigrationBlueprint`; a blueprint is purely a UI-side prefill source, resolved into a fresh `MigrationTemplate`/`MigrationPlan` at submit time exactly as a manually-filled form would be.
- **FR-014** *(added 2026-07-22)*: The system MUST allow an operator to edit an existing template's configuration in place (US6), reusing the New Migration form's prefill and field UI, and MUST update the same underlying template object rather than creating a duplicate. **Implemented** — `templateMode='edit'`, footer's "Save Changes" → `useUpdateTemplate` → `PUT .../migrationblueprints/{name}` with `metadata.resourceVersion` for optimistic concurrency (stale-version conflicts surface as an error, not a silent overwrite).
- **FR-015** *(added 2026-07-22)*: The system MUST provide a way to create a new template directly from the Templates tab (US7), without requiring the operator to first configure and abandon an in-progress migration. **Implemented** — "Create New Template" button opens the New Migration form with `templateMode='create'`, no prefill, footer showing only "Create Template"; saves through the same create path as FR-001.
- **FR-016** *(added 2026-07-22)*: The Templates view MUST support filtering by copy method (hot/cold/mock), in addition to the search and sort already required by FR-004. **Implemented** — FilterList icon + menu on the Templates tab toolbar; `filterTemplates()` takes a `copyMethod` parameter.

### Key Entities

- **Migration Template** (real name: `MigrationBlueprint`, a brand-new CRD — see Implementation Reality): A saved, named, reusable migration configuration. Attributes: name (unique, user-provided `displayName`, sanitized to the k8s object name), `resourceVersion` (added 2026-07-22, used only for Edit Template's optimistic-concurrency `PUT` — see FR-014), optional description, created timestamp (`metadata.creationTimestamp` — no times-used/last-used, no status subresource at all), source VMware credential reference (`vmwareRef`) plus source cluster name (`vmwareClusterName` — round-trips through save/apply but not shown in the Templates tab UI; no source datacenter field), destination OpenStack/PCD credential reference (`pcdRef`) and target cluster name (`targetPCDClusterName` — a name, not an id; tenant/project is resolved live from OpenStack creds at display time, not stored), network mappings, storage mappings, storage copy method, migration strategy (copy method hot/cold/mock, cutover policy, health-check/array-offload/network-disconnect flags), advanced options, post-migration action, first-boot script, GPU flavor flag, OS family. Fully independent of the pre-existing ephemeral `MigrationTemplate` CRD — no "saved" marker needed since they're different CRDs entirely. See `data-model.md` for the authoritative field list.
- **Migration Configuration**: The full set of form values (source/destination, mappings, options) captured at "save as template" time or applied at "use template" time — corresponds to the existing `FormValues` shape already used by the New Migration drawer.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can save a fully-configured migration as a named template in under 15 seconds (name entry + confirm), without leaving the New Migration drawer.
- **SC-002**: An operator can locate a previously saved template (via search, filter, or sort) and open its detail view in under 10 seconds from landing on the Templates tab.
- **SC-003**: Applying a template to a new migration reduces the number of fields the operator must manually fill in by at least 80% compared to configuring the same migration from scratch (excluding VM selection, which remains manual by design).
- **SC-004**: 100% of migrations created from a template where all referenced resources (credentials, mappings, clusters) still exist require zero corrective edits to source/destination/mapping fields before submission.
- **SC-005**: 100% of templates referencing a since-deleted resource clearly surface a warning identifying the missing reference at apply time, with zero form crashes or silent misconfiguration. **NOT MET** — no warning is surfaced today (FR-009 not implemented); the form does not crash, but silently leaves the field unresolved, which is itself a form of silent misconfiguration this criterion was meant to rule out.
- **SC-006**: Deleting or cloning a template never alters the behavior or configuration of any previously submitted migration — verified by zero regressions in existing Migration/MigrationPlan submission and retry flows.

---

## Assumptions

- ~~The existing `MigrationTemplate` CRD is extended...~~ **Superseded** — backend PR #2158 added a new CRD, `MigrationBlueprint`, instead. See Implementation Reality.
- No genuine multi-user identity/authorization system exists in vJailbreak today (single shared appliance session, API-token auth). Per Clarifications, v1 drops owner and shared/private visibility entirely rather than faking them with unverified UI labels — every template is visible to every operator of this appliance, with no ownership or visibility concept. Real multi-tenant RBAC (and any ownership/visibility feature built on it) is out of scope. **(Confirmed as implemented.)**
- VM selection is never templated — only source/destination, mappings, and options are saved/applied, since the specific VMs to migrate are expected to differ per migration even when the surrounding configuration is identical. **(Confirmed as implemented.)** Note the source *cluster* is templated (`vmwareClusterName`, round-trips through save/apply, added after initial ship) but not surfaced in the Templates tab UI; source *datacenter* still has no field at all.
- ~~Template "times used" / "last used" tracking...~~ **Moot** — dropped entirely, no backend field exists.
- The Templates tab is additive UI (new tab alongside "Migrations"); it does not change the existing Migrations list, its route, or its table behavior. **(Confirmed — though the Migrations tab itself was independently redesigned across two passes in later work: status chips, a Source→Destination column, stat cards added then removed, a Progress-column redesign added then reverted, search/filter/refresh relocated inline with the tabs, etc. That redesign is a separate scope from this feature — see the "2026-07-22 addendum" in Implementation Reality above for the specific list, kept there only because it touched the same files.)**
- This feature does not introduce any new backend REST endpoints in `pkg/vpwned` — all template CRUD continues to go through the existing generic Kubernetes custom-resource API path, now against `.../migrationblueprints` instead of `.../migrationtemplates`. **(Confirmed as implemented.)**
- The two existing near-duplicate frontend API modules for migration templates (`ui/src/api/migration-templates/` and `ui/src/features/migration/api/migration-templates/`) are a pre-existing consolidation opportunity for the **ephemeral** `MigrationTemplate` type; unaffected by this feature, since saved templates live entirely in a new, separate `ui/src/features/migration/api/migration-blueprints/` module.
