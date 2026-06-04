---
description: "Task list for Inventory Management / Migration Planner"
---

# Tasks: Inventory Management / Migration Planner

**Input**: `specs/004-migration-planner/spec.md` and `DESIGN.md`
**Prerequisites**: spec.md ✅, DESIGN.md ✅ (plan.md / data-model.md to follow)

**Build order (per request)**: **UI first**, then **backend**, then **integration** (swap UI
data layer from mock → real API). The existing `Migration` / `MigrationPlan` /
`RollingMigrationPlan` types and the migration **execution workflow are NOT modified** (FR-024).

**Component reuse rule (per request)**: every UI task MUST reuse existing common /
design-system / Storybook components. Import from the barrel `components/design-system/ui`,
the grid in `components/grid`, dialogs in `components/dialogs`, and RHF form controls in
`shared/components/forms/rhf`. Do NOT hand-roll a component that already exists. Add a
`*.stories.tsx` for every new composite component (Storybook is the project norm).

## Format: `[ID] [P?] [Story] Description`
- **[P]** = can run in parallel (different files, no dependency).
- **[Story]** = US1…US5 from spec.md; `INFRA` = shared.

### Reusable components catalogue (use these — do not recreate)
- Layout/box: `SurfaceCard`, `Section`, `SectionHeader`, `Row`, `KeyValueGrid`
- Drawer: `DrawerShell` + `DrawerHeader` / `DrawerBody` / `DrawerFooter`
- Status: `StatusChip` (`tone`: success | warning | info | error | default)
- Buttons: `ActionButton` (`tone`, `loading`)
- Banner/help: `Banner`, `InlineHelp`
- Tabs/nav: `NavTabs`, `SectionNav`
- Grid: `CommonDataGrid` + `CustomSearchToolbar` + `ListingToolbar`
- Dialog: `ConfirmationDialog`
- Forms: `DesignSystemForm`, `RHFSelect`, `RHFAutocomplete`, `RHFTextField`,
  `RHFCheckbox`, `RHFRadioGroup`, `RHFDateField` / `RHFDateTimeField`, `RHFToggleField`,
  `FormGrid`, `FieldLabel`
- Migration config (reuse READ-ONLY, do not edit): `features/migration/steps/*` —
  `SourceDestinationClusterSelection`, `VmsSelectionStep`, `NetworkAndStorageMappingStep`,
  `SecurityGroupAndServerGroup`, `MigrationOptionsAlt`
- Data hooks (reuse): `useVMwareMachinesQuery`, `useVmwareCredentialsQuery`,
  `useOpenstackCredentialsQuery`, `useMigrationsQuery`, `useRollingMigrationPlansQuery`

---

## Phase 1 — UI Setup & scaffolding (INFRA)

**Purpose**: create the Inventory feature shell and make it reachable, with no business logic.

- [x] T001 [INFRA] Create feature folder skeleton `ui/src/features/inventory/` with subdirs
  `pages/ components/ hooks/ api/migration-buckets/ utils/` and empty `types.ts`, `constants.ts`.
- [x] T002 [INFRA] Add the **Inventory** top-level nav item in `ui/src/config/navigation.tsx`
  (`Inventory2` icon, path `/dashboard/inventory`, placed between Migrations and Credentials).
- [x] T003 [INFRA] Register the route in `ui/src/App.tsx` under the `/dashboard`
  `DashboardLayout` block → `inventory` → `InventoryPage`.
- [x] T004 [P] [INFRA] Create `pages/InventoryPage.tsx` as a stub page using `Section` /
  `SectionHeader` (title "Inventory") from `src/components`; lint-clean.

---

## Phase 2 — UI Foundational: types + mockable data layer (INFRA)

**Purpose**: define the `MigrationBucket` client contract and a swappable data layer so the
entire UI can be built and demoed before the backend CRD exists. Later (Phase 8) the client
base flips from mock → real API with no component changes.

- [x] T005 [INFRA] TS types: `InventoryVm` / `VmPowerState` / `InventoryData` in
  `features/inventory/types.ts`; `MigrationBucket` + `BucketStatus` (NotMigrated | Scheduled |
  InProgress | Migrated) live in `api/migration-buckets/model.ts` and are re-exported.
  (Note: UI VM model has no per-disk capacity, so `diskCount` is the FE size proxy; precise
  size-based ordering is backend-only.)
- [x] T006 [INFRA] `api/migration-buckets/model.ts` (CRD shape) + `migrationBuckets.ts` with
  CRUD behind `BUCKETS_DATA_SOURCE` switch (mock ↔ k8s API), seeded mock fixtures.
- [x] T007 [P] [INFRA] `hooks/useMigrationBucketsQuery.ts` (react-query) + `useCreateBucket` /
  `useUpdateBucket` / `useDeleteBucket` mutations with list invalidation.
- [x] T008 [P] [INFRA] `hooks/useInventoryVms.ts` wrapping `useVMwareMachinesQuery` (+ creds
  hooks), mapping each VM to `InventoryVm` (powerState normalized, `nicCount`, clusterName via
  label, `diskCount`) and exposing a `bucketIdByVm` index.
- [x] T009 [P] [INFRA] `utils/bucketStatus.ts` → `getBucketStatus` + `bucketStatusLabel` +
  `bucketStatusTone` (reuses the design-system `StatusChip` palette).

---

## Phase 3 — US1: Discovery + Default bucket (Priority: P1) 🎯 MVP (UI)

**Goal**: operator sees the discovery summary and the auto-created Default bucket.
**Independent Test**: with mock data, Inventory shows "N VMs discovered…" and a Default bucket
card containing the fallback-rule VMs.

- [x] T010 [P] [US1] `components/DiscoveryCard.tsx` using `SurfaceCard` to show
  "N VMs discovered from credential `<credName>`" (presentational; data via `useInventoryVms`).
- [x] T011 [P] [US1] `utils/defaultBucketSelection.ts` implementing the fallback tiers
  (powered-off single-NIC → powered-off fewest-NIC → powered-on fewest-NIC → none) over
  `InventoryVm[]`; pure function. (Mirrors backend FR-006 for preview.)
- [x] T012 [US1] `components/BucketCard.tsx` using `SurfaceCard` (title + VM count,
  `StatusChip` for status, MUI Menu for actions). Default bucket shows Edit + Duplicate only.
- [x] T013 [US1] `components/BucketList.tsx` rendering `DiscoveryCard` + a vertical stack of
  `BucketCard`s; no-buckets state uses `Banner` (US1 scenario 6).
- [x] T014 [US1] Wire `BucketList` into `InventoryPage`; no-credential state uses `Banner`
  with a CTA to Credentials → VMware (FR-003).
- [x] T015 [P] [US1] `BucketCard.stories.tsx` + `DiscoveryCard.stories.tsx` (Storybook)
  covering default/non-default and each `BucketStatus`.

**Checkpoint**: Inventory + Default bucket viewable end-to-end on mock data.

---

## Phase 4 — US2: Organize VMs into buckets (Priority: P1) (UI)

**Goal**: duplicate / edit / delete buckets with invariants enforced client-side.
**Independent Test**: duplicate selecting a subset, confirm already-bucketed VMs are blocked,
edit + save, delete a non-default bucket; empty/duplicate-VM attempts are blocked.

- [x] T016 [P] [US2] `components/DuplicateBucketDrawer.tsx` using `DrawerShell` +
  `RHFAutocomplete multiple` (extended with an additive `getOptionDisabled` prop); VMs already
  in a bucket render **greyed + disabled** with an "already in a bucket" label (block, FR-011)
  via `bucketIdByVm`. Presentational; container creates the bucket via `useCreateBucket`.
- [x] T017 [US2] "No empty bucket" (FR-012) enforced in the Duplicate drawer: Save disabled
  until name set + ≥1 VM; `InlineHelp` validation via `validateBucketVms`.
- [x] T018 [P] [US2] `components/EditBucketDrawer.tsx` — **now mirrors the Migration Form exactly**:
  the form body + wiring were extracted into a shared `features/migration/components/MigrationConfigForm.tsx`
  (render-prop + lifted state). `MigrationForm` is a thin wrapper (submit = create migration);
  `EditBucketDrawer` reuses the SAME component (submit = **Save** → writes the chosen config into
  `MigrationBucket.spec.config`, round-tripped via `config.formValues`). No migration execution code
  changed. (Supersedes the earlier membership-only scope.) ⚠️ Needs runtime testing of both flows.
- [x] T019 [US2] **Delete** on non-default `BucketCard` via `ConfirmationDialog` (FR-008/FR-009);
  wired to `useDeleteBucket` in the `InventoryPage` container.
- [x] T020 [US2] Uniqueness guard (FR-013): `utils/bucketMembership.ts`
  (`isVmBlocked`/`assertVmsUnique`/`validateBucketVms`) used by edit + duplicate; selecting an
  already-owned VM is blocked (greyed).
- [x] T021 [P] [US2] `DuplicateBucketDrawer.stories.tsx` + `EditBucketDrawer.stories.tsx`
  (Storybook) incl. the greyed "already in a bucket" state.

**Checkpoint**: full bucket CRUD on mock data with invariants visible.

---

## Phase 5 — US3: Bucket config defaults + schedule (Priority: P2) (UI)

**Goal**: auto-defaults pre-fill + future-only schedule.
**Independent Test**: new bucket pre-fills source cluster, first dest cluster, first
network/storage mappings, empty SG/server group; schedule picker disables past times.

- [x] T022 [P] [US3] `utils/bucketDefaults.ts`: source cluster from `InventoryVm.clusterName`,
  dest cluster = `pcdHostConfig[0].clusterName`, every source network→first dest network and
  source datastore→first dest volume type, empty SG/server group (FR-014/FR-015). Pure.
  (`InventoryVm` extended with `networks[]`/`datastores[]`.)
- [x] T023 [US3] Defaults applied at duplicate-create (compute when source has no mappings) and
  surfaced read-only as a config summary (`KeyValueGrid`) in `EditBucketDrawer`; SG/server group
  + advanced default unselected. (Inline editing of these controls is a follow-up.)
- [x] T024 [P] [US3] `components/BucketScheduleField.tsx` using `RHFDateTimeField` with
  `disablePast` (future-only, FR-016); surfaced in `EditBucketDrawer`, saved to `spec.schedule`.
- [x] T025 [P] [US3] `BucketScheduleField.stories.tsx`.

**Checkpoint**: buckets carry sensible defaults + optional future schedule.

---

## Phase 6 — US4: Trigger drawer + agent-count suggestion (Priority: P2) (UI)

**Goal**: multi-select buckets → explainable, editable agent count.
**Independent Test**: open trigger, select subset, see a non-negative agent count with visible
derivation; +/- works within [0, A_max].

- [x] T026 [US4] `components/TriggerDrawer.tsx` using `DrawerShell` — bucket list with MUI
  checkboxes + `StatusChip`; buckets already In progress/Migrated are disabled (FR-018).
  Surfaced via a "Trigger migrations" `ActionButton` in the page `SectionHeader`.
- [x] T027 [P] [US4] `utils/agentRecommendation.ts` implementing DESIGN §9.1:
  `A = clamp(ceil(max(0, CR-(m+ΣΔ))/F), 0, A_max)`; returns `{ value, rawValue, exceedsCapacity,
  derivation }`. Pure. Capacity inputs from `DEFAULT_AGENT_PARAMS` placeholders (real values wired
  in T043/T047; Q8).
- [x] T028 [US4] `components/AgentCountStepper.tsx` +/- bounded [0, A_max] (FR-019); derivation
  shown by the host dialog via `InlineHelp` (FR-020).
- [x] T029 [US4] `components/TriggerPlanDialog.tsx` (MUI Dialog) hosting `AgentCountStepper`;
  `Banner` "runs in waves" note when `rawValue > A_max`.
- [x] T030 [P] [US4] Stories for `TriggerDrawer` + `AgentCountStepper` + `TriggerPlanDialog`.

**Checkpoint**: trigger flow shows capacity guidance (no real launch yet — stubbed).

---

## Phase 7 — US5: Recommended order + trigger-now/schedule (Priority: P3) (UI)

**Goal**: success-first ordering + trigger-now precedence.
**Independent Test**: mixed buckets → order shows highest-success first; "Trigger now" path
clearly overrides per-bucket schedule.

- [x] T031 [P] [US5] `utils/bucketOrdering.ts` implementing DESIGN §9.2: per-bucket
  `f=(PO+1)/(PON+1)`, `modeNic` via counting tally, `size` (diskCount proxy); stable multi-key
  sort (f desc, modeNic asc, size asc, name asc). Pure (`scoreBucket` + `orderBucketsBySuccess`).
- [x] T032 [US5] Ordered (read-only) bucket list in `TriggerPlanDialog` from
  `orderBucketsBySuccess` (drag-reorder deferred per Q9).
- [x] T033 [US5] **Trigger now** vs **Use each bucket's schedule** controls (MUI `RadioGroup`) in
  `TriggerPlanDialog`; Trigger-now shows an `InlineHelp` that it ignores per-bucket schedules
  (FR-022); precedence carried into `handlePlanConfirm`.
- [x] T034 [P] [US5] `TriggerPlanDialog.stories.tsx` updated to cover the ordered list +
  trigger-now/schedule toggle.

**Checkpoint**: complete UI on mock data; ready to back with a real API.

---

## Phase 8 — Backend: `MigrationBucket` CRD + reconciler

**Purpose**: durable buckets, default-bucket automation, invariants — WITHOUT touching existing
migration types/workflow (FR-024). Follow `003-hot-add-proxy` as the CRD/reconciler reference.

- [x] T035 [INFRA] `k8s/migration/api/v1alpha1/migrationbucket_types.go`: `MigrationBucketSpec`
  (`vmwareCredsRef`, `vms []string`, `isDefault`, `schedule`, embedded `config` with structured
  fields + `formValues`/`selectedOptions` as `RawExtension` PreserveUnknownFields),
  `MigrationBucketStatus` (`phase`, `message`). Markers + printcolumns.
- [ ] T036 [INFRA] ⚠️ **RUN LOCALLY** `cd k8s/migration && make generate` (deepcopy + CRD YAML).
  Required before build — my sandbox has no Go toolchain. Do NOT hand-edit generated files.
- [x] T037 [P] [INFRA] RBAC markers on the reconciler (migrationbuckets CRUD/status + read
  vmwaremachines/openstackcreds). Regenerate role with `make manifests`.
- [~] T038 [US1] Default-bucket creation: **currently created by the UI via the API** on first
  visit (works, persists). Moving it into a reconciler/watch (backend-owned per Q2) is a follow-up.
- [~] T039 [P] [US1] `selectDefaultBucketVms` exists on the FE (`utils/defaultBucketSelection.ts`);
  the Go `pkg/utils/bucketutils.go` port is deferred until backend creation (T038) lands.
- [~] T040 [US2] Invariants: enforced in the UI (block/greyed + validation); the reconciler
  **surfaces** the no-empty-bucket violation in `status.message`. A hard validating webhook +
  cross-bucket uniqueness check is a follow-up.
- [x] T041 [P] [US2] `migrationbucket_controller_test.go`: reconciler unit tests (phase default,
  empty-bucket message, not-found no-error) using the fake client. ⚠️ Run via `make test`.

> **Phase 8 build note:** Go isn't available in the authoring environment, so the source above
> was written but not compiled here. Run `cd k8s/migration && make generate && make manifests &&
> make test`, then rebuild/deploy the controller and apply the CRD. See `contracts/crds.md`.

---

## Phase 9 — Backend: trigger compile + agent scaling (no workflow change)

**Purpose**: turn selected buckets into existing execution objects + scale workers. The
planner only *creates* `MigrationPlan` / `RollingMigrationPlan` exactly as the Migration Form
does (FR-024) — their controllers are untouched.

- [ ] T042 [US4] `pkg/utils/bucketcompile.go`: compile selected buckets → `MigrationPlan`(s) +
  one `RollingMigrationPlan` (encode chosen order into `Spec.VMMigrationPlans` /
  `Spec.ClusterSequence`); reuse existing creation paths. Unit-tested.
- [ ] T043 [P] [US4] `pkg/utils/agentrecommendation.go`: Go port of DESIGN §9.1 (single source of
  truth shared with the UI util's logic) reading node allocatable + settings + VjailbreakNodes.
  Table-driven unit tests. Expose via the API the UI calls (or compute server-side at trigger).
- [ ] T044 [US4] On trigger confirm: scale `VjailbreakNode` worker count toward the chosen
  number, then run the compile (T042). Update bucket `Status.Phase` → Scheduled/InProgress.
- [ ] T045 [P] [US5] Go port of DESIGN §9.2 bucket ordering (or accept the UI-provided order);
  unit-tested. Honor Trigger-now > schedule precedence when setting `DataCopyStart`.
- [ ] T046 [US3] Map bucket `schedule` → `MigrationPlanStrategy.DataCopyStart` at compile time
  (future-only validated); rely on existing `AwaitingDataCopyStart` phase. Unit-tested.

---

## Phase 10 — Integration: swap UI data layer mock → real API

- [x] T047 [INFRA] `BUCKETS_DATA_SOURCE = 'api'` — bucket CRUD now targets the real
  MigrationBucket k8s API; names sanitized to DNS-1123. (Requires the CRD deployed; errors until
  then.) Mock path retained for Storybook.
- [ ] T048 [US4] Wire `TriggerPlanDialog` confirm → backend trigger/compile + scale (T044).
  **Deferred** — creates real migrations + provisions nodes (needs interactive image/flavor and a
  live cluster). The confirm currently records intent; see "remaining" note below.
- [x] T049 [P] [INFRA] `deriveBucketStatus(bucket, phaseByVmName)` (in `utils/bucketStatus.ts`)
  derives live `BucketStatus` from real `Migration.status.phase` (matched by `spec.vmName`),
  wired through `BucketList`/`BucketCard` via `useMigrationsQuery` (FR-017). Best-effort name match.

---

## Phase 11 — Verification & polish

- [ ] T050 [INFRA] Playwright/Cypress e2e for the happy path (discovery → default bucket →
  duplicate → trigger) using the existing mock-data harness.
- [ ] T051 [P] [INFRA] **Non-regression check (FR-024/SC-006)**: confirm zero changes to
  `Migration`/`MigrationPlan` schemas + existing migration controllers; run
  `cd k8s/migration && make test` and the existing migration e2e — all pass unchanged.
- [ ] T052 [US4] Verify agent-count UI util and Go util produce identical numbers on shared
  fixtures (parity test).
- [ ] T053 [P] [INFRA] `cd ui && yarn lint && yarn test`; build Storybook to confirm all new
  stories render.
- [ ] T054 [INFRA] Resolve the carried clarifications before merge: Q5 (multi-cluster bucket),
  Q8 (`C`/`F`/`A_max` sources), Q9 (order edit in v1), VM-removed-from-inventory handling;
  record decisions in `research.md`.

---

## Dependencies & execution order

- **Phase 1 → 2** before any UI story.
- **UI stories**: US1 (P3 phases) is the MVP; US2 depends on US1's `BucketCard`/`BucketList`;
  US3 depends on `EditBucketDrawer` (US2/T018); US4 depends on a bucket list existing; US5
  depends on US4's `TriggerPlanDialog`.
- **Backend (Phase 8–9)** can start in parallel with UI once types (T005) are agreed, but
  Phase 10 integration requires Phase 8–9 complete.
- **Phase 11** last.

### Parallelizable now
- T004, T007, T008, T009 (different files) after T001/T005/T006.
- All `*.stories.tsx` tasks ([P]) alongside their component.
- Pure utils (T011, T022, T027, T031) are independent and unit-testable in isolation.

## Notes
- Reuse-first: if a UI need maps to a catalogue component, use it; only create a new component
  by composing existing ones, and always add a story.
- Never modify `features/migration/steps/*` or the migration execution code — compose/wrap only.
- Backend Go: table-driven unit tests, mock VMware/OpenStack/k8s (CLAUDE.md); run `make generate`
  after any CRD change; never hand-edit `zz_generated.deepcopy.go` or `deploy/installer.yaml`.
- Commit after each task or logical group; stop at checkpoints to validate a story on mock data.
