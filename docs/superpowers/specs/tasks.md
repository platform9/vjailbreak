# Tasks: Cluster Conversion Redesign

**Input**: Design documents from `docs/superpowers/specs/`
**Prerequisites**: plan.md ✅ | spec.md ✅ | research.md ✅ | data-model.md ✅

**Organization**: Grouped by user story — each story independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1–US4 from spec.md
- Exact file paths in all descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: New CRD type definitions and constants. No behavior change.

- [ ] T001 Create `k8s/migration/api/v1alpha1/clusterconversionbatch_types.go` with all structs: `ClusterConversionBatch`, `ClusterConversionBatchList`, `ClusterConversionBatchSpec`, `ClusterConversionBatchStatus`, `HostEntry`, `HostConversionStatus`, and all phase/status type aliases (`AutoStartMode`, `ClusterConversionBatchPhase`, `HostConversionPhase`, `EligibilityStatus`). Register in `SchemeBuilder`. Add `+kubebuilder:` markers for status subresource and printcolumns.
- [ ] T002 Modify `k8s/migration/api/v1alpha1/esximigration_types.go` — make `RollingMigrationPlanRef` optional (`omitempty`), add optional `BMConfigRef *corev1.LocalObjectReference` and `ClusterConversionBatchRef *corev1.LocalObjectReference` fields to `ESXIMigrationSpec`
- [ ] T003 [P] Add constants to the existing constants file in `k8s/migration/pkg/common/constants/`: `ClusterConversionBatchFinalizer`, `ClusterConversionBatchLabel`, `ClusterConversionBatchControllerName`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: CRD generation, scope, ESXIMigration controller changes, and UI API layer. Must complete before any user story.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T004 Run `cd k8s/migration && make generate` to regenerate `zz_generated.deepcopy.go` and CRD YAML in `config/crd/bases/`. Verify `vjailbreak.k8s.pf9.io_clusterconversionbatches.yaml` appears and no compile errors.
- [ ] T005 [P] Create `k8s/migration/pkg/scope/clusterconversionbatchscope.go` following exact pattern of `esximigrationscope.go`: `ClusterConversionBatchScopeParams`, `ClusterConversionBatchScope` struct, `NewClusterConversionBatchScope`, `Close()` (calls `client.Update`), `Name()`, `Namespace()`
- [ ] T006 Modify `k8s/migration/internal/controller/esximigration_controller.go` — wrap the `rollingMigrationPlan` fetch (lines ~75–88) in `if esxiMigration.Spec.RollingMigrationPlanRef.Name != ""`. Add `resolveBMConfigName(esxiMig, rmp)` helper that returns `spec.bmConfigRef.Name` first, falls back to `rmp.Spec.BMConfigRef.Name`. Replace the existing BMConfig name lookup (~line 136) with `resolveBMConfigName`.
- [ ] T007 Add test cases to `k8s/migration/internal/controller/esximigration_controller_test.go` for: (1) no `RollingMigrationPlanRef` + `BMConfigRef` set → BMConfig fetched from `spec.bmConfigRef`, (2) both refs absent → error, (3) existing old-flow behavior unchanged
- [ ] T008 Create `ui/src/api/cluster-conversion-batches/model.ts` with TypeScript interfaces from `data-model.md`: `ClusterConversionBatch`, `ClusterConversionBatchSpec`, `ClusterConversionBatchStatus`, `HostEntry`, `HostConversionStatus`, `ClusterConversionBatchPhase`, `HostConversionPhase`. Import `ItemMetadata`, `NameReference` from `rolling-migration-plans/model.ts`.
- [ ] T009 [P] Create `ui/src/api/cluster-conversion-batches/clusterConversionBatches.ts` mirroring `rollingMigrationPlans.ts`: `getClusterConversionBatches`, `getClusterConversionBatch`, `postClusterConversionBatch`, `deleteClusterConversionBatch`, `patchClusterConversionBatch` (with `Content-Type: application/merge-patch+json`). Base path: `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/clusterconversionbatches`
- [ ] T010 [P] Create `ui/src/api/cluster-conversion-batches/index.ts` (re-export all) and `ui/src/hooks/api/useClusterConversionBatchesQuery.ts` mirroring `useRollingMigrationPlansQuery.ts` with `CLUSTER_CONVERSION_BATCHES_QUERY_KEY`

**Checkpoint**: Foundation ready — user story implementation can begin.

---

## Phase 3: User Story 1 — Auto-Convert (Priority: P1) 🎯 MVP

**Goal**: Operator creates a batch with `AutoStart=Auto`; eligible hosts automatically get an `ESXIMigration` created and convert. One host's failure does not affect siblings.

**Independent Test**: Create `ClusterConversionBatch` with 3 hosts, `AutoStart=Auto`. Mock eligibility to return Ready for all. Verify 3 `ESXIMigration` resources are created, host phases become `Converting`, batch phase becomes `Running`. Verify mock-failed host retries while sibling phases are unaffected.

### Implementation for User Story 1

- [ ] T013 [US1] Create `k8s/migration/pkg/utils/clusterconversionbatchutils_test.go` **FIRST** (TDD): table-driven tests for `ComputeRetryBackoff` (60s base: retry 1→60s, 2→120s, 3→240s), `CreateESXIMigrationForBatch` (correct labels, `spec.bmConfigRef` set, no ownerRef present), `ProcessBatchAnnotations` (trigger/retry/skip/empty). Run `go test` — expect compilation failure (functions not yet defined). Proceed to T011 only after tests are written.
- [ ] T011 [US1] Create `k8s/migration/pkg/utils/clusterconversionbatchutils.go` with `CreateESXIMigrationForBatch`, `GetESXIMigrationForBatch`, `ComputeRetryBackoff(baseSeconds, retryCount int) time.Duration`, `buildTemporaryRMPScope` (synthetic `RollingMigrationPlan` with only the fields `EnsureESXiInMass` and `CanEnterMaintenanceMode` actually read — verify by reading those functions), and `ProcessBatchAnnotations` (reads trigger/retry/skip annotations, returns `[]BatchAction`, removes processed annotations). No owner reference on ESXIMigration (no GC cascade). CloudInitConfigRef intentionally not passed to ESXIMigration — deferred to future work.
- [ ] T012 [US1] Implement `CheckPerHostEligibility` in `k8s/migration/pkg/utils/clusterconversionbatchutils.go`: BMConfig valid → PCD cluster exists → MAAS match via `EnsureESXiInMass` → DRS+capacity via `CanEnterMaintenanceMode`. All existing helper functions called unchanged. Returns `(EligibilityStatus, reason, error)`.
- [ ] T016 [US1] Create `k8s/migration/internal/controller/clusterconversionbatch_controller_test.go` **FIRST** (TDD): table-driven tests: initialize host statuses (3 hosts → 3 entries all CheckingEligibility), auto-start eligible host (eligibility=Ready, AutoStart=Auto → ESXIMigration created, phase=Converting), sibling isolation (host A failed, host B Ready → B advances to Converting), batch phase aggregation (all Succeeded→Succeeded, mix Succeeded+NeedsAttention/Skipped→PartialFail, all NeedsAttention→Failed; `Failed` phase is NOT terminal — only Succeeded/NeedsAttention/Skipped are terminal), batch delete (finalizer removed, ESXIMigrations NOT deleted). Define `EligibilityChecker` interface for mocking. Run `go test` — expect compilation failure. Proceed to T014 only after tests are written.
- [ ] T014 [US1] Create `k8s/migration/internal/controller/clusterconversionbatch_controller.go` with: `ClusterConversionBatchReconciler` struct (inject `EligibilityChecker` interface), `Reconcile` (get batch → scope → defer Close → handle deletion → reconcileNormal), `reconcileNormal` (initialize status.hosts, call `ProcessBatchAnnotations`, call `applyAction` per action, call `processHost` per host, `updateBatchAggregates` — terminal phases = Succeeded/NeedsAttention/Skipped only, requeue 30s), `processHost` (auto-start path only), `esxiMigrationToBatch` Watch handler, `reconcileDelete` (remove finalizer, do NOT delete child ESXIMigrations), `SetupWithManager` with `Watches` on ESXIMigration via label mapper
- [ ] T015 [US1] Register `ClusterConversionBatchReconciler` in `k8s/migration/cmd/main.go` following existing controller registration pattern
- [ ] T017 [P] [US1] Create `ui/src/features/clusterConversions/components/HostStatusChip.tsx` mapping `HostConversionPhase` → MUI Chip color: CheckingEligibility→default, NotReady→warning, Ready→info, Converting→info, Succeeded→success, Failed→warning, NeedsAttention→error, Skipped→default
- [ ] T018 [US1] Create `ui/src/features/clusterConversions/components/BatchesTable.tsx` mirroring `RollingMigrationsTable.tsx` structure: `CommonDataGrid` with columns (Cluster, Status chip, Progress bar, Running/NeedsAttention counts, AutoStart chip, Age, Details button opening `BatchDetailDrawer`). Multi-select delete with `ConfirmationDialog`. "Create Conversion Batch" button in toolbar (disabled without VMware+PCD creds).
- [ ] T019 [US1] Modify `ui/src/features/clusterConversions/pages/ClusterConversionsPage.tsx` — add `useClusterConversionBatchesQuery` (refetchInterval=30s), render `<BatchesTable>` as primary section above existing `<RollingMigrationsTable>` (legacy section only shown if `rollingMigrationPlans.length > 0`)

**Checkpoint**: User Story 1 fully testable — create batch, watch auto-conversion, verify sibling isolation.

---

## Phase 4: User Story 2 — Manual-Trigger Mode (Priority: P2)

**Goal**: With `AutoStart=Manual`, hosts reach `Ready` but no `ESXIMigration` is created until operator explicitly triggers per host. AutoStart mode can be changed mid-flight.

**Independent Test**: Create `ClusterConversionBatch` with `AutoStart=Manual`. Mock eligibility Ready for 2 hosts. Verify no `ESXIMigration` exists. Set trigger annotation for host A. Verify only host A gets `ESXIMigration`. Verify AutoStart patch to `Auto` causes host B to start.

### Implementation for User Story 2

- [ ] T020 [US2] Add `applyAction` to `k8s/migration/internal/controller/clusterconversionbatch_controller.go` for `trigger` type: host must be `Ready`; creates ESXIMigration via `CreateESXIMigrationForBatch`, sets phase=Converting, sets StartedAt. (`ProcessBatchAnnotations` already implemented in T011; `reconcileNormal` already calls it in T014 — T020 only adds the trigger case to `applyAction`.)
- [ ] T021 [US2] Verify `k8s/migration/internal/controller/clusterconversionbatch_controller.go` annotation flow end-to-end: `ProcessBatchAnnotations` called at start of `reconcileNormal`, each returned action dispatched to `applyAction`, cleared annotations persisted via `r.Update` before status update. Add unit test with fake client: patch trigger annotation on batch, call Reconcile, verify ESXIMigration created and annotation absent from batch metadata.
- [ ] T022 [US2] Add test cases to `k8s/migration/internal/controller/clusterconversionbatch_controller_test.go`: trigger annotation on Ready host → ESXIMigration created + annotation removed, trigger on non-Ready host → no-op, manual mode no auto-start (eligibility=Ready, AutoStart=Manual → no ESXIMigration), AutoStart switch Manual→Auto → all Ready hosts start
- [ ] T023 [US2] Create `ui/src/features/clusterConversions/components/BatchDetailDrawer.tsx` with: per-host `DataGrid` (ESX Host, Phase chip via HostStatusChip, Eligibility+reason tooltip, Retries counter, Duration, Actions column), "Start" button rendered only when `hostStatus.phase === 'Ready'` and `batch.spec.autoStart === 'Manual'` (calls `patchClusterConversionBatch` with trigger annotation), AutoStart `Switch` in drawer header (patches `spec.autoStart`). Reuse `StyledDrawer`/`DrawerHeader` styled components from `RollingMigrationsTable.tsx`.

**Checkpoint**: User Story 2 testable — create Manual batch, verify hosts wait at Ready, trigger individual hosts via UI.

---

## Phase 5: User Story 3 — Pre-flight Eligibility in Create Dialog (Priority: P2)

**Goal**: Before creating a batch, operator sees per-host eligibility status in the Create dialog (sourced from existing `VMwareHost` objects). Operator can filter to only eligible hosts or select all.

**Independent Test**: Open Create Batch dialog, select a cluster. Verify host list displays with eligibility indicators from `VMwareHost.status.state`. Verify selecting only eligible hosts submits batch with those hosts only.

### Implementation for User Story 3

- [ ] T024 [P] [US3] Create `ui/src/features/clusterConversions/components/CreateBatchDialog.tsx` Step 1 — MUI `Stepper` inside `Dialog`: cluster `Autocomplete` (fetches via `getVMwareClusters`), host list with checkboxes showing `VMwareHost.spec.hostConfigId` and `VMwareHost.status.state` as eligibility indicator, `Switch` for AutoStart, `Select` for BMConfig (`useBMConfigQuery`), `Select` for OpenStack creds filtered to PCD type (`useOpenstackCredentialsQuery`), collapsible Advanced section for `maxRetries` + `retryBackoffSeconds`
- [ ] T025 [US3] Add Step 2 (review summary table + submit) to `ui/src/features/clusterConversions/components/CreateBatchDialog.tsx` — calls `postClusterConversionBatch`, on success invalidates `CLUSTER_CONVERSION_BATCHES_QUERY_KEY` and closes dialog. Wire "Create Conversion Batch" toolbar button in `BatchesTable.tsx` to open `CreateBatchDialog`.

**Checkpoint**: User Story 3 testable — open create dialog, see host eligibility indicators, submit batch.

---

## Phase 6: User Story 4 — Retry and Skip Stuck Hosts (Priority: P3)

**Goal**: After retry exhaustion, host enters `NeedsAttention`. Operator can Retry (resets counter, new ESXIMigration) or Skip (marks Skipped, batch advances). Automatic exponential backoff between retries.

**Independent Test**: Simulate host always failing. After `maxRetries` failures, verify `NeedsAttention`. Verify Retry resets count + creates new ESXIMigration. Verify Skip marks host Skipped and batch phase recalculates.

### Implementation for User Story 4

- [ ] T026 [US4] Add `retry` and `skip` cases to `applyAction` in `k8s/migration/internal/controller/clusterconversionbatch_controller.go` — retry: only if `NeedsAttention`, delete existing ESXIMigration (ignore NotFound), reset `RetryCount=0`/`NextRetryAt=nil`/phase=`CheckingEligibility`. Skip: set `SkippedAt`, phase=`Skipped`, do NOT delete ESXIMigration if Converting.
- [ ] T027 [US4] Update `processHost` in `clusterconversionbatch_controller.go` for failed ESXIMigration path: increment `RetryCount`; if `RetryCount > MaxRetries` → `NeedsAttention`; else → set `NextRetryAt = now + ComputeRetryBackoff(RetryBackoffSeconds, RetryCount)`, phase=`Failed`. Add retry timer check at top of `processHost` (skip processing if `NextRetryAt` not yet elapsed).
- [ ] T028 [US4] Add test cases to `k8s/migration/internal/controller/clusterconversionbatch_controller_test.go`: host failure increments RetryCount (Failed→RetryCount=1, NextRetryAt set), retry exhaustion→NeedsAttention (RetryCount=MaxRetries), retry timer active→no new ESXIMigration, retry annotation on NeedsAttention→RetryCount=0+CheckingEligibility, skip annotation→Skipped+ESXIMigration not deleted, batch PartialFail when some Succeeded+some Skipped
- [ ] T029 [US4] Add Retry and Skip buttons to per-host Actions column in `ui/src/features/clusterConversions/components/BatchDetailDrawer.tsx` — rendered only when `hostStatus.phase === 'NeedsAttention'`; Retry calls `patchClusterConversionBatch` with `retry-host` annotation; Skip calls with `skip-host` annotation. Add `ProcessBatchAnnotations` test cases for retry/skip to `k8s/migration/pkg/utils/clusterconversionbatchutils_test.go`.

**Checkpoint**: All user stories testable end-to-end.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T030 Run `cd k8s/migration && make generate && make test` — verify `vjailbreak.k8s.pf9.io_clusterconversionbatches.yaml` generated, all controller tests pass
- [ ] T031 [P] Update `docs/src/content/docs/cluster-conversion/` overview page — add version callout for new `ClusterConversionBatch` architecture, new section explaining batch workflow (select cluster → select hosts → configure → create), dynamic eligibility, Auto vs Manual start modes
- [ ] T032 [P] Add `:::caution[Deprecated]:::` callout to any `RollingMigrationPlan`-specific doc pages in `docs/src/content/docs/`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Requires Phase 1 complete (T001–T003) — blocks all user stories
- **US1 (Phase 3)**: Requires Foundational complete (T004–T010)
- **US2 (Phase 4)**: Requires US1 complete (controller must exist before adding annotation processing)
- **US3 (Phase 5)**: Requires US1 complete (BatchesTable must exist to wire toolbar button); can run in parallel with US2
- **US4 (Phase 6)**: Requires US2 complete (applyAction must exist before adding retry/skip cases)
- **Polish (Phase 7)**: Requires all stories complete

### Within Each Phase

- **TDD**: T013 (utils tests) before T011+T012 (utils implementation) — Constitution IV
- **TDD**: T016 (controller tests) before T014 (controller implementation) — Constitution IV
- T011 before T012 (CreateESXIMigrationForBatch used by CheckPerHostEligibility)
- T014 before T015 (controller must exist before main.go registration)
- T014 before T018 (controller must exist before UI components)

### Parallel Opportunities

- T003, T005 can run in parallel (different files, no dependencies)
- T008, T009, T010 can run in parallel (UI files, no inter-dependencies)
- T013, T017 can run in parallel within US1 (utils tests and HostStatusChip are independent)
- T024 (CreateBatchDialog Step 1) can start as soon as BatchesTable exists (T018)
- T031, T032 (docs) can run in parallel

---

## Parallel Example: User Story 1

```bash
# Once Foundational complete — TDD sequence:
Task: T013 — clusterconversionbatchutils_test.go (write tests, run to see compile failure)
Task: T017 — HostStatusChip.tsx  [P with T013]

# After T013 written:
Task: T011 — clusterconversionbatchutils.go (make T013 pass)
Task: T012 — CheckPerHostEligibility (make remaining T013 tests pass)

# TDD for controller — write tests first:
Task: T016 — clusterconversionbatch_controller_test.go (write tests, run to see compile failure)
# After T016 written:
Task: T014 — clusterconversionbatch_controller.go (make T016 pass)

# After T014:
Task: T015 — cmd/main.go registration  [P with T018]
Task: T018 — BatchesTable.tsx          [P with T015]
```

---

## Implementation Strategy

### MVP (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T003)
2. Complete Phase 2: Foundational (T004–T010) — CRITICAL, blocks US1
3. Complete Phase 3: User Story 1 (T013→T011→T012→T016→T014→T015→T017→T018→T019)
4. **STOP and VALIDATE**: Create a batch manually via `kubectl`, verify host status progression and ESXIMigration creation
5. Deploy/demo

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 → Auto-convert works, visible in UI
3. US2 → Manual trigger + AutoStart toggle works
4. US3 → Pre-flight host selection in Create dialog
5. US4 → Retry/skip escape hatches
6. Polish → Docs + final test run

---

## Notes

- [P] = different files, no cross-task dependency — safe to parallelize
- [Story] label maps each task to the user story it enables
- Controller tests use `EligibilityChecker` interface mock — no real vCenter calls
- `buildTemporaryRMPScope` is the most brittle piece — read `EnsureESXiInMass` and `CanEnterMaintenanceMode` carefully before implementing to identify exactly which `RollingMigrationPlan` fields they access
- No owner references on ESXIMigration (intentional — prevents GC cascade on batch delete)
- Commit after each phase or logical group; run `make test` before committing backend phases
