# Tasks: Per-ESXi Cluster Conversion + MAAS-Free BM Provisioning

**Input**: Design documents from `specs/1889-maas-free-bm-provisioning/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete task dependencies)
- **[Story]**: User story label (US1–US4)
- Each file is touched by at most one `[P]` task at a time

---

## Phase 1: Setup

**Purpose**: Verify toolchain and branch readiness before any code changes.

- [ ] T001 Verify `cd k8s/migration && make generate` runs cleanly (toolchain baseline — no code changes)
- [ ] T002 [P] Verify `cd pkg/vpwned && go build ./...` runs cleanly (baseline)
- [ ] T003 [P] Verify `cd ui && yarn build` runs cleanly (baseline)

---

## Phase 2: Foundational — CRD Type Changes (Milestone 1)

**Purpose**: CRD type changes that ALL user stories depend on. MUST complete before any controller or UI work.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete and `make generate` passes.

- [ ] T004 Add to `k8s/migration/api/v1alpha1/vmwarecluster_types.go`: (a) `HostStatus` struct with `Name string`, `VMCount int`, `InMaintenanceMode bool`; (b) `Hosts []HostStatus` to `VMwareClusterStatus`; (c) `VMwareCredsRef corev1.LocalObjectReference` (required), `BMConfigRef *corev1.LocalObjectReference` (optional), and `PCDClusterRef *corev1.LocalObjectReference` (optional) to `VMwareClusterSpec`
- [ ] T005 [P] Add to `k8s/migration/api/v1alpha1/esximigration_types.go`: change `RollingMigrationPlanRef` from value to pointer (`*corev1.LocalObjectReference`) with `omitempty`; add `BMConfigRef *corev1.LocalObjectReference` and `PCDClusterRef *corev1.LocalObjectReference` fields with `omitempty` to `ESXIMigrationSpec`
- [ ] T006 [P] Change `RollingMigrationPlan` field to pointer (`*vjailbreakv1alpha1.RollingMigrationPlan`) in `ESXIMigrationScope` in `k8s/migration/pkg/scope/esximigrationscope.go`
- [ ] T007 Run `make generate` in `k8s/migration/` and verify `zz_generated.deepcopy.go` and CRD YAML files updated
- [ ] T008 [P] Write table-driven unit tests for deepcopy of new structs in `k8s/migration/api/v1alpha1/vmwarecluster_types_test.go` and `esximigration_types_test.go`: cover `HostStatus`, `ESXIMigrationSpec` new fields, `VMwareClusterSpec` new fields
- [ ] T009 Run `cd k8s/migration && make test` — all tests must pass before proceeding
- [ ] T009b [US2] Add controller-level validation in `k8s/migration/internal/controller/esximigration_controller.go`: when `RollingMigrationPlanRef` is nil, verify `BMConfigRef` and `PCDClusterRef` are both non-nil; if either missing, set ESXIMigration status to Failed with descriptive message and return without requeue

**Checkpoint**: CRD types updated, deepcopy regenerated, tests pass. User story phases can now begin.

---

## Phase 3: User Story 1 — Per-ESXi VM Migration (Priority: P1) 🎯 MVP

**Goal**: Show per-host VM counts in UI; allow selecting VMs on one ESXi host and opening migration form pre-populated.

**Independent Test**: Open Cluster Conversions page → expand a cluster → see ESXi host rows with VM counts. Select VMs on one host → "Migrate Selected" → migration form opens pre-populated with those VMs.

### Implementation — VMwareCluster Controller (Milestone 2)

- [ ] T010 [US1] Search existing controllers for govmomi session/client pattern before writing new code: `grep -r "govmomi.NewClient\|govmomi.NewURL" k8s/migration/internal/controller/`
- [ ] T011 [US1] Create `VMwareClusterReconciler` in `k8s/migration/internal/controller/vmwarecluster_controller.go`: fetch VMwareCluster CR; connect via `Spec.VMwareCredsRef`; iterate hosts; populate `Status.Hosts[]` with VM count + maintenance state; requeue after 30s; handle EC-001 (unreachable host → set `LastPollError` condition, do NOT overwrite last-known status); handle EC-003 (check for deletion timestamp; if CR is being deleted, remove finalizer if set and return without requeue)
- [ ] T012 [US1] Register `VMwareClusterReconciler` in `k8s/migration/cmd/main.go` (reuse pattern from other controller registrations)
- [ ] T013 [P] [US1] Write table-driven unit tests for `VMwareClusterReconciler` in `k8s/migration/internal/controller/vmwarecluster_controller_test.go`: cases — 0 VMs, multiple VMs, host in maintenance, host unreachable (assert `LastPollError` set, last-known status preserved), CR deletion (assert reconcile exits cleanly); assert `ctrl.Result.RequeueAfter <= 30*time.Second`; mock govmomi client via interface

### Implementation — ListVMs Host Filter (Milestone 4, partial)

- [ ] T014 [US1] Add `host_name string` field to `ListVMsRequest` in `pkg/vpwned/sdk/proto/v1/api.proto`
- [ ] T015 [US1] Regenerate proto bindings in `pkg/vpwned/` after api.proto change
- [ ] T016 [US1] Implement `host_name` filter in `ListVMs` in `pkg/vpwned/sdk/targets/vcenter/vcenter.go`: after fetching all VMs, filter by `vm.Summary.Runtime.Host` name when `host_name` non-empty; reuse existing govmomi property retrieval
- [ ] T017 [P] [US1] Write unit tests for ListVMs host filter in `pkg/vpwned/sdk/targets/vcenter/vcenter_test.go`: empty filter returns all VMs, non-empty filter returns only matching host's VMs

### Implementation — UI Components (Milestone 5, partial)

- [ ] T018 [P] [US1] Update `ui/src/api/vmware-clusters/vmwareClusters.ts` to ensure `status.hosts`, `spec.bmConfigRef`, and `spec.pcdClusterRef` are read from VMwareCluster API response
- [ ] T019 [P] [US1] Create `useVMwareClustersQuery` hook in `ui/src/hooks/api/useVMwareClustersQuery.ts` (reuse pattern from existing `useESXIMigrationsQuery.ts`)
- [ ] T020 [US1] Create `ESXiClusterAccordion` component in `ui/src/features/clusterConversions/components/ESXiClusterAccordion.tsx`: fetches VMwareCluster list via `useVMwareClustersQuery`; fetches ESXIMigration list via `useESXIMigrationsQuery`; groups hosts by cluster; renders `ESXiHostRow` per host
- [ ] T021 [US1] Create `ESXiHostRow` component in `ui/src/features/clusterConversions/components/ESXiHostRow.tsx`: props `host: HostStatus`, `esxiMigration?: ESXIMigration`, `vmwareCluster: VMwareCluster`; MUI Accordion; header shows host name + VM count progress bar + state chip; "Migrate VMs" button visible only when `vmCount > 0`; state chip derives from `ESXIMigration.status.phase` if CR exists else from `HostStatus`; stub "Put in Maintenance", "Exit Maintenance", "Convert to PCD Host" buttons (wired in Phase 4)
- [ ] T022 [US1] Create `ESXiVMTable` component in `ui/src/features/clusterConversions/components/ESXiVMTable.tsx`: props `hostName: string`, `vmwareCredsRef: string`; calls `listVMs` with `host_name` filter on accordion expand; MUI checkbox table; "Migrate Selected (N) →" button opens existing migration form with pre-selected VMs
- [ ] T023 [US1] Update `ui/src/features/clusterConversions/pages/ClusterConversionsPage.tsx` to add `useVMwareClustersQuery` and render `<ESXiClusterAccordion />` above `<RollingMigrationsTable />`; `RollingMigrationsTable` props unchanged
- [ ] T024 [P] [US1] Write unit tests for `ESXiHostRow` in `ui/src/features/clusterConversions/components/ESXiHostRow.test.tsx`: "Migrate VMs" visible when vmCount > 0, hidden when 0; mock `useVMwareClustersQuery`

**Checkpoint**: VMwareCluster controller populates per-host VM counts within 30s. ESXi accordion shows in UI with VM counts and Migrate VMs button. Migration form opens pre-populated.

---

## Phase 4: User Story 2 — Maintenance Mode + PCD Host Conversion (Priority: P2)

**Goal**: "Put in Maintenance" calls vCenter API. Empty+maintenance hosts show "Convert to PCD Host" which creates standalone ESXIMigration CR.

**Independent Test**: Click "Put in Maintenance" → host enters maintenance in vCenter → row state updates. Empty+maintenance host: "Convert to PCD Host" appears → click → ESXIMigration CR created → row shows conversion phases.

### Implementation — ESXIMigration Controller Decoupling (Milestone 3)

- [ ] T025 [US2] Guard plan fetch with nil check in `k8s/migration/internal/controller/esximigration_controller.go` at lines 75–88: wrap in `if esxiMigration.Spec.RollingMigrationPlanRef != nil`; set `scope.RollingMigrationPlan` only inside the guard; guard every downstream `scope.RollingMigrationPlan` field access with nil check (nil dereference is primary regression risk)
- [ ] T026 [US2] Add `GetBMProviderFromBMConfigRef` helper in `k8s/migration/internal/controller/bmprovisionerutils.go`: instantiates correct `BMCProvider` from a direct `corev1.LocalObjectReference` (sequential after T025 — same controller package)
- [ ] T027 [US2] Refactor `ConvertESXiToPCDHost` in `k8s/migration/internal/controller/bmprovisionerutils.go`: extract VMware creds retrieval from `GetVMwareCredsFromRollingMigrationPlan` into a helper accepting either `*RollingMigrationPlan` or direct `VMwareCredsRef`; standalone path uses `ESXIMigrationSpec.VMwareCredsRef` and `PCDClusterRef` directly
- [ ] T028 [US2] Wire standalone path in `handleESXiCordoned` in `k8s/migration/internal/controller/esximigration_controller.go`: when `scope.RollingMigrationPlan` is nil, call `GetBMProviderFromBMConfigRef` + `GetCredsFromSpec`; when non-nil, existing code path unchanged
- [ ] T029 [P] [US2] Write unit tests for ESXIMigration standalone path in `k8s/migration/internal/controller/esximigration_controller_test.go`: with plan ref (existing path unchanged), without plan ref (uses BMConfigRef + PCDClusterRef), EC-004 (missing BMConfig → Failed phase); mock k8s client

### Implementation — Maintenance Mode API (Milestone 4, remaining)

- [ ] T030 [US2] Add `EnterMaintenanceMode` and `ExitMaintenanceMode` RPCs to `VCenter` service in `pkg/vpwned/sdk/proto/v1/api.proto` with HTTP bindings `/vpw/v1/enter_maintenance_mode` and `/vpw/v1/exit_maintenance_mode`
- [ ] T031 [US2] Regenerate proto bindings in `pkg/vpwned/` after maintenance RPC additions
- [ ] T032 [US2] Implement `EnterMaintenanceMode` handler in `pkg/vpwned/server/target_vcenter.go` using `govmomi object.HostSystem.EnterMaintenanceMode(ctx, timeout, evacuatePoweredOffVms, spec)` — consult govmomi docs before implementing
- [ ] T033 [US2] Implement `ExitMaintenanceMode` handler in `pkg/vpwned/server/target_vcenter.go` using `govmomi object.HostSystem.ExitMaintenanceMode` (same file as T032 — run after T032 completes)
- [ ] T034 [P] [US2] Write unit tests for EnterMaintenanceMode and ExitMaintenanceMode handlers in `pkg/vpwned/server/target_vcenter_test.go` (mock HostSystem)

### Implementation — UI Maintenance + Conversion Actions (Milestone 5, remaining)

- [ ] T035 [P] [US2] Create maintenance API client in `ui/src/api/vcenter/maintenance.ts`: `enterMaintenanceMode` and `exitMaintenanceMode` functions passing access info via same pattern as existing `listVMs` calls in `vcenter.ts`
- [ ] T036 [US2] Wire "Put in Maintenance" button in `ui/src/features/clusterConversions/components/ESXiHostRow.tsx`: always visible; calls `enterMaintenanceMode`; shows loading state; warns user (EC-002) if migration is running on host but allows; updates row state on confirmation
- [ ] T037 [US2] Wire "Exit Maintenance" button in `ui/src/features/clusterConversions/components/ESXiHostRow.tsx`: visible only when `inMaintenanceMode === true`; calls `exitMaintenanceMode`; shows loading state; updates row state on confirmation
- [ ] T038 [US2] Wire "Convert to PCD Host" button in `ui/src/features/clusterConversions/components/ESXiHostRow.tsx`: visible only when `vmCount === 0 && inMaintenanceMode`; use `vmwareCluster.spec.bmConfigRef` and `vmwareCluster.spec.pcdClusterRef` as defaults; if either is nil, show a selection dialog prompting user to pick an existing BMConfig CR and PCD cluster; on confirm, create standalone `ESXIMigration` CR via k8s API; row shows conversion progress phases
- [ ] T039 [P] [US2] Update `ESXiHostRow` unit tests in `ui/src/features/clusterConversions/components/ESXiHostRow.test.tsx`: cover "Put in Maintenance" always visible, "Exit Maintenance" visible only when inMaintenanceMode, "Convert to PCD Host" gate (vmCount=0 AND inMaintenance), standalone ESXIMigration CR creation

**Checkpoint**: Full per-ESXi workflow functional end-to-end. Maintenance mode enter/exit works. Standalone ESXIMigration CR creates and progresses without ClusterMigration parent.

---

## Phase 5: User Story 3 — Ironic Provider (Priority: Phase 2 - P1)

**Goal**: ESXi→PCD conversion uses OpenStack Ironic when BMConfig.ProviderType=ironic.

**Independent Test**: Configure BMConfig with ProviderType=ironic + endpoint + credentials. Trigger ESXi→PCD. Verify Ironic API used for provisioning; MAAS not required.

### Implementation — BMConfig CRD Extension (Milestone 6, part 1)

- [ ] T040 [US3] Add `IronicProvider BMCProviderName = "ironic"` and `IPMIProvider BMCProviderName = "ipmi"` constants to `k8s/migration/api/v1alpha1/bmconfig_types.go`
- [ ] T041 [US3] Add `IronicConfig` struct (Endpoint, Username, Password, ProjectID, DomainName) and `IPMIConfig` struct (BMCAddress, Username, Password, Interface, UseRedfish) to `k8s/migration/api/v1alpha1/bmconfig_types.go`
- [ ] T042 [US3] Add `IronicConfig *IronicConfig` and `IPMIConfig *IPMIConfig` optional fields to `BMConfigSpec` in `k8s/migration/api/v1alpha1/bmconfig_types.go`
- [ ] T043 [US3] Run `make generate` in `k8s/migration/` and verify BMConfig CRD YAML updated

### Implementation — IronicProvider (Milestone 6, part 2)

- [ ] T044 [US3] Add `gophercloud` dependency: `cd pkg/vpwned && go get github.com/gophercloud/gophercloud && go mod tidy`
- [ ] T045 [US3] Extend existing stub `pkg/vpwned/sdk/providers/ironic/ironic.go` to implement `BMCProvider`: `Connect` (Keystone auth → Ironic client), `ListResources` (GET /v1/nodes), `GetResourceInfo` (GET /v1/nodes/{id}), `DeployMachine` (set instance_info + provision → active with EC-005 retry: up to 3 attempts with exponential backoff), `ReclaimBM` (provision → available), `SetBM2PXEBoot` (PATCH boot_interface: pxe), `StartBM`/`StopBM` (power state); consult [Ironic API docs](https://docs.openstack.org/ironic/latest/api/) before implementing; unsupported MAAS-specific methods return `ErrNotSupported`
- [ ] T046 [US3] Add blank import `_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/ironic"` to `k8s/migration/pkg/utils/bmprovisionerutils.go` — triggers IronicProvider.init() self-registration; verify with `providers.GetProvider("ironic")` in unit test
- [ ] T047 [P] [US3] Write unit tests for IronicProvider in `pkg/vpwned/sdk/providers/ironic/ironic_test.go`: mock gophercloud HTTP client; test Connect, DeployMachine, ReclaimBM, SetBM2PXEBoot, StartBM, StopBM; test EC-005 (unreachable → 3 retries with backoff → error)

**Checkpoint**: ESXi→PCD conversion works with Ironic provider. MAAS regression verified (existing BMConfig unchanged).

---

## Phase 6: User Story 4 — Direct IPMI/Redfish Provider (Priority: Phase 2 - P2)

**Goal**: ESXi→PCD conversion uses direct IPMI/Redfish when BMConfig.ProviderType=ipmi (no MAAS, no Ironic required).

**Independent Test**: Configure BMConfig with ProviderType=ipmi + BMC credentials. Trigger conversion. Verify IPMI sets PXE boot + power cycle; cloud-init served from vJailbreak HTTP endpoint.

### Implementation — IPMIProvider (Milestone 7)

- [ ] T048 [US4] Add `gofish` dependency: `cd pkg/vpwned && go get github.com/stmcginnis/gofish && go mod tidy`
- [ ] T049 [US4] Extract IPMI boot/power helpers from `pkg/vpwned/sdk/providers/maas/maas.go` into `pkg/vpwned/sdk/providers/ipmi/ipmi_helpers.go` without changing logic — extract functions `ChassisControlPowerUp`, `ChassisControlPowerDown`, and the PXE bootdev sequence (refactor only — logic-preserving per constitution VII)
- [ ] T050 [US4] Create `pkg/vpwned/sdk/providers/ipmi/ipmi.go` implementing `BMCProvider` — IPMI path: `StartBM` (ChassisControlPowerUp), `StopBM` (ChassisControlPowerDown), `SetBM2PXEBoot` (chassis bootdev pxe + power cycle via helpers from T049); `ListResources` returns single-element list from `IPMIConfig.BMCAddress`; MAAS-specific methods return `ErrNotSupported`
- [ ] T051 [US4] Add Redfish path to `IPMIProvider` in `pkg/vpwned/sdk/providers/ipmi/ipmi.go` (same file as T050 — run after T050): when `IPMIConfig.UseRedfish=true`, use `gofish` for power management (ComputerSystem.Reset) and boot override (BootSourceOverride PATCH)
- [ ] T052 [US4] Implement `DeployMachine` in `IPMIProvider` in `pkg/vpwned/sdk/providers/ipmi/ipmi.go`: serve cloud-init user-data from vJailbreak VM's local HTTP endpoint; after PXE boot machine fetches cloud-init and self-enrolls in PCD
- [ ] T053 [US4] Add blank import `_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/ipmi"` to `k8s/migration/pkg/utils/bmprovisionerutils.go` — triggers IPMIProvider.init() self-registration (match maas and base pattern); verify with `providers.GetProvider("ipmi")` in unit test
- [ ] T054 [P] [US4] Write unit tests for IPMIProvider in `pkg/vpwned/sdk/providers/ipmi/ipmi_test.go`: IPMI path (boot device set + power cycle sequence), Redfish path (mock gofish power + boot override), `ListResources` single-element return

**Checkpoint**: All four user stories fully functional.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T055 [P] Run `cd k8s/migration && make test` — all controller tests pass
- [ ] T056 [P] Run `cd pkg/vpwned && go test ./...` — all provider + handler tests pass
- [ ] T057 [P] Run `cd ui && yarn test` — all UI component tests pass
- [ ] T058 Verify MAAS regression: create BMConfig with ProviderType=MAAS and trigger ESXIMigration with RollingMigrationPlanRef set — existing orchestrated path works unchanged (SC-003)
- [ ] T059 [P] Run `make generate-manifests` to verify full build succeeds with all changes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup — **BLOCKS all user story phases**
- **US1 (Phase 3)**: Depends on Foundational
- **US2 (Phase 4)**: Depends on Foundational; shares `ESXiHostRow.tsx` with US1 (T036–T039 extend T021)
- **US3 (Phase 5)**: Depends on Foundational only — independent of US1/US2
- **US4 (Phase 6)**: Depends on Foundational only — can run in parallel with US3
- **Polish (Phase 7)**: Depends on all desired user stories complete

### User Story Dependencies

- **US1 (P1)**: Start after Phase 2 — no inter-story deps
- **US2 (P2)**: Start after Phase 2 — extends ESXiHostRow from US1 (T036–T039 add to T021)
- **US3 (Phase 2 - P1)**: Start after Phase 2 — fully independent of US1/US2
- **US4 (Phase 2 - P2)**: Start after Phase 2 — fully independent; parallel with US3

### Within Each Phase

- Tasks marked `[P]` touch different files and have no incomplete-task dependencies
- Phase 2: T004, T005 [P], T006 [P] can run in parallel (different files); T007 follows all three
- Phase 3 API: T014 → T015 → T016 (sequential; same proto flow)
- Phase 4 API: T030 → T031 → T032 → T033 (sequential; proto then same handler file)
- Phase 6: T049 → T050 → T051 → T052 (sequential; same ipmi.go file after T049 extraction)

---

## Parallel Execution Examples

```bash
# Phase 2 — T004, T005, T006 in parallel (different files):
Task: "vmwarecluster_types.go: HostStatus + VMwareCredsRef + BMConfigRef + PCDClusterRef"
Task: "esximigration_types.go: RollingMigrationPlanRef pointer + BMConfigRef + PCDClusterRef"
Task: "esximigrationscope.go: RollingMigrationPlan pointer"
# Then T007 (make generate), T008 (tests in parallel), T009 (make test)

# Phase 3 — T013, T018, T019 in parallel:
Task: "Write VMwareClusterReconciler tests (with requeue assertion)"
Task: "Update vmwareClusters.ts to read status.hosts + spec refs"
Task: "Create useVMwareClustersQuery hook"

# Phase 5 + Phase 6 in parallel after Phase 2:
Task: "US3: BMConfig CRD + gophercloud + IronicProvider"
Task: "US4: gofish + extract IPMI helpers + IPMIProvider"
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all)
3. Complete Phase 3: User Story 1 (per-ESXi VM view)
4. **STOP and VALIDATE**: accordion shows, VM counts update ≤30s, "Migrate Selected" pre-populates
5. Demo to stakeholders

### Incremental Delivery

1. Setup + Foundational → CRDs ready
2. US1 → per-ESXi VM view (MVP)
3. US2 → maintenance mode + PCD conversion
4. US3 → Ironic provider
5. US4 → IPMI/Redfish provider

### Parallel Team Strategy

After Phase 2 completes:

- Developer A: US1 (controller + API + UI display)
- Developer B: US2 (controller decoupling + maintenance API + UI actions)
- Developer C: US3 (Ironic provider)
- Developer D: US4 (IPMI provider)

---

## Notes

- `[P]` = different files, no blocking in-progress dependencies — same file = always sequential
- All new Go code requires unit tests (`_test.go` alongside impl — constitution IV)
- Run `make generate` after every CRD type change — never hand-edit generated files (constitution III)
- Consult govmomi docs before implementing EnterMaintenanceMode/ExitMaintenanceMode (constitution II)
- Consult Ironic API docs before implementing IronicProvider (constitution II)
- Logic-preserving refactors (T049 IPMI extract) in separate commit from behavioral changes (constitution VII)
- Commit per milestone as specified in plan.md
