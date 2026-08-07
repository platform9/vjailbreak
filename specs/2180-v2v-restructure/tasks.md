# Tasks: v2v-helper Module Restructure — Phase 1

**Input**: Design documents from `specs/2180-v2v-restructure/`
**Branch**: `2180-v2v-restructure`

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- No test tasks — this is a pure mechanical refactor; all new logic is zero; existing tests validate correctness
- **HARD CONSTRAINT**: No function body content may be modified — functions move verbatim

---

## Phase 1: Setup (Pre-flight Verification)

**Purpose**: Verify current state and pre-conditions before any files are touched.

- [x] T001 Record current `migrate.go` method count: `grep -c "^func" v2v-helper/migrate/migrate.go` — save as baseline for post-split verification
- [x] T002 [P] Verify no circular import risk: `grep -r "pkg/utils" v2v-helper/openstack/` — must return empty
- [x] T003 [P] Verify `*reporter.Reporter` satisfies `reporter.ReporterOps`: inspect `v2v-helper/reporter/reporter.go` and confirm all interface methods are implemented on the struct
- [x] T004 [P] Verify current build passes before any change: `cd v2v-helper && go build ./...`

**Checkpoint**: All pre-conditions confirmed — proceed to user story phases

---

## Phase 2: Foundational (Blocking Prerequisites)

No foundational infrastructure needed — this is a pure file reorganization. Proceed directly to user story phases after Phase 1.

---

## Phase 3: User Story 1 — Split migrate/migrate.go (Priority: P1) 🎯 MVP

**Goal**: Reduce `migrate.go` from 2,723 LOC to ~550 LOC by extracting 4 concern-based files within the same `package migrate`. Zero import changes required anywhere.

**Independent Test**: After each extraction step, `go build ./...` passes. After all extractions, `wc -l v2v-helper/migrate/migrate.go` reports ≤600 lines.

### Implementation for User Story 1

> **Note**: Steps T005–T008 must run sequentially — each one edits `migrate.go` to remove extracted methods. Build-check after each step catches omissions immediately.

- [x] T005 [US1] Create `v2v-helper/migrate/nbd_copy.go` — move verbatim: `LiveReplicateDisks`, `EnableCBTWrapper`, `SyncCBT`, `WaitForAdminCutover`. Remove these methods from `v2v-helper/migrate/migrate.go`. File header: `package migrate`
- [x] T006 [US1] Build check after T005: `cd v2v-helper && go build ./...` — must pass before continuing
- [x] T007 [US1] Create `v2v-helper/migrate/network.go` — move verbatim: `ReservePortsForVM`, `reuseExistingPorts`, `createPortsForNetworks`, `createPort`, `resolveNICOverride`, `logAndCollectDetectedIPs`, `applyPreserveIPOverride`, `applyPreserveMACOverride`, `syncIPperMacFromPort`, `configureWindowsNetwork`, `configureLinuxNetwork`, `configureUbuntuNetwork`, `configureRHELNetwork`, `addUdevRulesForUbuntu`, `DetectAndHandleNetwork`. Remove from `migrate.go`. File header: `package migrate`
- [x] T008 [US1] Build check after T007: `cd v2v-helper && go build ./...` — must pass before continuing
- [x] T009 [US1] Create `v2v-helper/migrate/conversion.go` — move verbatim: `ConvertVolumes`, `attachAllVolumes`, `detectBootVolume`, `handleLinuxOSDetection`, `handleWindowsBootDetection`, `validateLinuxOS`, `getBootCommand`, `performDiskConversion`, and standalone functions `parseVersionID`, `isNetplanSupported`, `blockDriverFromMetadata`. Remove from `migrate.go`. File header: `package migrate`
- [x] T010 [US1] Build check after T009: `cd v2v-helper && go build ./...` — must pass before continuing
- [x] T011 [US1] Create `v2v-helper/migrate/vm_ops.go` — move verbatim: `CreateVolumes`, `AttachVolume`, `DetachVolume`, `DetachAllVolumes`, `DetachAllVolumesWithCleanup`, `DeleteAllVolumes`, `applyImageMetadataForXCOPYVolumes`, `verifyVMCreatedDespiteTimeout`, `InitializeStorageProvider`, `LoadESXiSSHKey`, `buildProviderOptions`, `reportStagedVolumeIDs`, `CreateTargetInstance`, `HealthCheck`, `pingVM`, `checkHTTPGet`, `tryConnection`, and standalone functions `extractFileName`, `logDiskCopyPlan`, `validateDiskMapping`, `logMessage`, `LogMessage`. Remove from `migrate.go`. File header: `package migrate`
- [x] T012 [US1] Build check after T011: `cd v2v-helper && go build ./...` — must pass before continuing
- [x] T013 [US1] Verify `migrate.go` now contains ONLY: `Migrate` struct definition, `MigrateVM()`, `cleanup()`, `WaitforCutover`, `CheckIfAdminCutoverSelected`, `CheckCutoverOptions`, `gracefulTerminate`. Run `grep -c "^func" v2v-helper/migrate/migrate.go` — count must match baseline minus extracted methods. Run `wc -l v2v-helper/migrate/migrate.go` — must be ≤600

**Checkpoint**: User Story 1 complete — `migrate.go` is ≤600 LOC; all 4 new files compile; `go build ./...` passes

---

## Phase 4: User Story 2 — Fix openstack Interface/Implementation Split (Priority: P2)

**Goal**: Remove `pkg/utils/openstackopsutils.go` and place its content in `openstack/clients.go` so interface and implementation live in the same package.

**Independent Test**: After this phase, `ls v2v-helper/pkg/utils/openstackopsutils.go` returns file-not-found. `ls v2v-helper/openstack/clients.go` returns the file. `go build ./...` passes.

### Implementation for User Story 2

- [x] T014 [US2] Create `v2v-helper/openstack/clients.go` — copy the full content of `v2v-helper/pkg/utils/openstackopsutils.go` verbatim; change only the first line from `package utils` to `package openstack`. No other changes.
- [x] T015 [US2] Delete `v2v-helper/pkg/utils/openstackopsutils.go`
- [x] T016 [US2] Update import in `v2v-helper/main.go` — replace any import of `github.com/platform9/vjailbreak/v2v-helper/pkg/utils` used for `OpenStackClients` with `github.com/platform9/vjailbreak/v2v-helper/openstack`. Update all `utils.OpenStackClients` references to `openstack.OpenStackClients`.
- [x] T017 [US2] Update import in `v2v-helper/migrate/vaai_copy.go` — same import swap as T016 for any `OpenStackClients` usage
- [x] T018 [US2] Update imports in any migrate/ split files from Phase 3 (vm_ops.go or others) that reference `OpenStackClients` — same import swap
- [x] T019 [US2] Build check: `cd v2v-helper && go build ./...` — must pass

**Checkpoint**: User Story 2 complete — `openstackopsutils.go` gone; `clients.go` present; build passes

---

## Phase 5: User Story 3 — Wire ReporterOps Interface (Priority: P3)

**Goal**: Change `Migrate.Reporter` field type from concrete `*reporter.Reporter` to interface `reporter.ReporterOps`, enabling mock injection in future tests.

**Independent Test**: After this phase, `grep "Reporter " v2v-helper/migrate/migrate.go` shows `reporter.ReporterOps` (not `*reporter.Reporter`). `go build ./...` passes.

### Implementation for User Story 3

- [x] T020 [US3] In `v2v-helper/migrate/migrate.go`, change the `Reporter` field in the `Migrate` struct from `*reporter.Reporter` to `reporter.ReporterOps`. This is a single-line struct field type change — no function bodies modified.
- [x] T021 [US3] Build check: `cd v2v-helper && go build ./...` — must pass (verifies `*reporter.Reporter` satisfies the interface)

**Checkpoint**: User Story 3 complete — field type is interface; build passes

---

## Phase 6: Final Validation

**Purpose**: Confirm all success criteria from spec.md are met.

- [x] T022 [P] Run `cd v2v-helper && go build ./...` — exit code must be 0 (SC-004)
- [x] T023 [P] Run `cd v2v-helper && go vet ./...` — exit code must be 0 (SC-005)
- [x] T024 Verify `wc -l v2v-helper/migrate/migrate.go` ≤ 600 (SC-001)
- [x] T025 [P] Verify `ls v2v-helper/pkg/utils/openstackopsutils.go` returns not-found (SC-002)
- [x] T026 [P] Verify `ls v2v-helper/openstack/clients.go` exists and contains `OpenStackClients` type (SC-003)
- [x] T027 Verify no file in `v2v-helper/migrate/` exceeds 800 lines: `wc -l v2v-helper/migrate/*.go` (SC-007)
- [x] T028 Run full test suite on Linux/CGO: `make test-v2v-helper` — all 10 existing test files must pass (SC-006)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately; T002, T003, T004 are parallel
- **Phase 3 (US1)**: Depends on Phase 1; T005→T006→T007→T008→T009→T010→T011→T012→T013 sequential
- **Phase 4 (US2)**: Depends on Phase 1; T014→T015→T016/T017/T018 (T016–T018 parallel after T015)→T019
- **Phase 5 (US3)**: Depends on Phase 1; T020→T021 sequential
- **Phase 6 (Validation)**: Depends on Phase 3 + Phase 4 + Phase 5 complete

### User Story Dependencies

- **US1 (P1)**: No dependency on US2/US3 — can complete independently
- **US2 (P2)**: No dependency on US1/US3 — can complete independently after Phase 1
- **US3 (P3)**: No dependency on US1/US2 — can complete independently after Phase 1

> With one developer: do US1 → US2 → US3 → validate.
> With three developers: all three user story phases can run in parallel after Phase 1.

### Parallel Opportunities (within US1)

Note: T005–T013 are sequential (each edits migrate.go). No parallelism within Phase 3. The 4 new files (nbd_copy.go, network.go, conversion.go, vm_ops.go) are independent of each other but share the source (migrate.go), so extracting sequentially with build checks is safest.

---

## Implementation Strategy

### MVP First (US1 only)

1. Complete Phase 1 (pre-flight, ~15 min)
2. Complete Phase 3 (US1 — split migrate.go, ~1 hour)
3. Build check passes → migrate.go is now navigable
4. **STOP and validate** if needed — US2/US3 are independent improvements

### Full Delivery

1. Phase 1 → Phase 3 → Phase 4 → Phase 5 → Phase 6
2. Each phase independently buildable
3. Commit after each user story phase completes

---

## Notes

- **Zero logic changes**: If you find yourself editing the content of a function, stop — that is out of scope
- **Build check is the test**: `go build ./...` after each extraction catches any missed method moves immediately
- `hotadd_copy.go` and `vaai_copy.go` are NOT touched in this phase
- If a method's location is ambiguous (appears in multiple concern groups), prefer the file where its primary caller lives
