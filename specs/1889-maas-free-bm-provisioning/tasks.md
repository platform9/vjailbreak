# Tasks: Per-ESXi Cluster Conversion + MAAS-Free BM Provisioning

**Input**: Design documents from `specs/1889-maas-free-bm-provisioning/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅

**Organization**: Backend phases first (1–7), UI phases after (8–11).
Backend is fully testable independently; UI phases begin only after Phase 7 validates.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete task dependencies)
- **[Story]**: User story label (US1–US4)
- Each file is touched by at most one `[P]` task at a time

---

## ── BACKEND PHASES ──────────────────────────────────────────────────────────

---

## Phase 1: Setup

**Purpose**: Verify toolchain and branch readiness before any code changes.

- [X] T001 Verify `cd k8s/migration && make generate` runs cleanly (toolchain baseline — no code changes)
- [X] T002 [P] Verify `cd pkg/vpwned && go build ./...` runs cleanly (baseline)
- [X] T003 [P] Verify `cd ui && yarn build` runs cleanly (baseline)

---

## Phase 2: Foundational — CRD Type Changes (Milestone 1)

**Purpose**: CRD type changes that ALL user stories depend on. MUST complete before any controller or UI work.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete and `make generate` passes.

- [X] T004 Add to `k8s/migration/api/v1alpha1/vmwarecluster_types.go`: (a) `HostStatus` struct with `Name string`, `VMCount int`, `InMaintenanceMode bool`; (b) `Hosts []HostStatus` and `Conditions []metav1.Condition` to `VMwareClusterStatus`; (c) `VMwareCredsRef corev1.LocalObjectReference` (required), `BMConfigRef *corev1.LocalObjectReference` (optional), `PCDClusterRef *corev1.LocalObjectReference` (optional) to `VMwareClusterSpec`
- [X] T005 [P] Add to `k8s/migration/api/v1alpha1/esximigration_types.go`: change `RollingMigrationPlanRef` from value to pointer (`*corev1.LocalObjectReference`) with `omitempty`; add `BMConfigRef *corev1.LocalObjectReference` and `PCDClusterRef *corev1.LocalObjectReference` with `omitempty` to `ESXIMigrationSpec`
- [X] T006 [P] Change `RollingMigrationPlan` field to pointer (`*vjailbreakv1alpha1.RollingMigrationPlan`) in `ESXIMigrationScope` in `k8s/migration/pkg/scope/esximigrationscope.go` (was already a pointer — no change needed)
- [X] T007 Run `make generate` in `k8s/migration/` and verify `zz_generated.deepcopy.go` and CRD YAML files updated
- [X] T008 [P] Write table-driven unit tests for deepcopy of new structs in `k8s/migration/api/v1alpha1/types_test.go`: cover `HostStatus`, `ESXIMigrationSpec` new fields, `VMwareClusterSpec` new fields — all 5 tests pass
- [X] T009 Run `cd k8s/migration && make test` — pre-existing go vet failure in bmprovisionerutils.go unrelated to our changes; `go test ./api/v1alpha1/...` and `go build ./...` both pass cleanly
- [X] T009b [US2] Add controller-level validation in `k8s/migration/internal/controller/esximigration_controller.go`: when `RollingMigrationPlanRef` is nil, verify `BMConfigRef` and `PCDClusterRef` are both non-nil; if either missing, set ESXIMigration status to Failed with descriptive message and return without requeue

**Checkpoint**: CRD types updated, deepcopy regenerated, tests pass. User story phases can now begin.

---

## Phase 3: US1 Backend — VMwareCluster Controller + ListVMs API

**Goal**: Controller polls vCenter every 30s, populates per-host VM counts + maintenance state in VMwareCluster status. ListVMs accepts optional host filter.

**Backend Test**: `kubectl -n migration-system get vmwarecluster <name> -o yaml` shows `status.hosts[]` with VM counts within 30s. `curl vpwned/listVMs?host_name=esxi-01` returns only that host's VMs.

- [ ] T010 [US1] Search existing controllers for govmomi session/client pattern before writing new code: `grep -r "govmomi.NewClient\|govmomi.NewURL" k8s/migration/internal/controller/`
- [ ] T011 [US1] Create `VMwareClusterReconciler` in `k8s/migration/internal/controller/vmwarecluster_controller.go`: fetch VMwareCluster CR; connect via `Spec.VMwareCredsRef`; iterate hosts; populate `Status.Hosts[]` with VM count + maintenance state; requeue after 30s; handle EC-001 (unreachable host → set `LastPollError` condition on `Status.Conditions`, do NOT overwrite last-known status); handle EC-003 (deletion timestamp present → remove finalizer if set, return without requeue)
- [ ] T012 [US1] Register `VMwareClusterReconciler` in `k8s/migration/cmd/main.go` (reuse pattern from other controller registrations)
- [ ] T013 [P] [US1] Write table-driven unit tests for `VMwareClusterReconciler` in `k8s/migration/internal/controller/vmwarecluster_controller_test.go`: cases — 0 VMs, multiple VMs, host in maintenance, host unreachable (assert `LastPollError` condition set, last-known status preserved), CR deletion (assert reconcile exits cleanly); assert `ctrl.Result.RequeueAfter <= 30*time.Second`; mock govmomi client via interface
- [ ] T014 [US1] Add `host_name string` field to `ListVMsRequest` in `pkg/vpwned/sdk/proto/v1/api.proto`
- [ ] T015 [US1] Regenerate proto bindings in `pkg/vpwned/` after api.proto change
- [ ] T016 [US1] Implement `host_name` filter in `ListVMs` in `pkg/vpwned/sdk/targets/vcenter/vcenter.go`: after fetching all VMs, filter by `vm.Summary.Runtime.Host` name when `host_name` non-empty; reuse existing govmomi property retrieval pattern
- [ ] T017 [P] [US1] Write unit tests for ListVMs host filter in `pkg/vpwned/sdk/targets/vcenter/vcenter_test.go`: empty filter returns all VMs, non-empty filter returns only matching host's VMs, host not found returns empty list (not error)

**Checkpoint**: `VMwareCluster.status.hosts[]` populated within 30s. ListVMs host filter returns correct subset. All tests pass.

---

## Phase 4: US2 Backend — ESXIMigration Decoupling + Maintenance API

**Goal**: ESXIMigration controller works without RollingMigrationPlanRef. EnterMaintenanceMode and ExitMaintenanceMode API endpoints functional.

**Backend Test**: Create ESXIMigration CR with `rollingMigrationPlanRef: null` and valid `bmConfigRef`/`pcdClusterRef` — controller proceeds to ConvertingToPCDHost phase. `curl vpwned/enter_maintenance_mode` succeeds against test vCenter.

- [ ] T025 [US2] Guard plan fetch with nil check in `k8s/migration/internal/controller/esximigration_controller.go` at lines 75–88: wrap in `if esxiMigration.Spec.RollingMigrationPlanRef != nil`; set `scope.RollingMigrationPlan` only inside the guard; guard every downstream `scope.RollingMigrationPlan` field access with nil check (nil dereference is primary regression risk)
- [ ] T026 [US2] Add `GetBMProviderFromBMConfigRef` helper in `k8s/migration/pkg/utils/bmprovisionerutils.go`: instantiates correct `BMCProvider` from a direct `corev1.LocalObjectReference` (sequential after T025 — depends on guarded scope)
- [ ] T027 [US2] Refactor `ConvertESXiToPCDHost` in `k8s/migration/pkg/utils/bmprovisionerutils.go`: extract VMware creds retrieval from `GetVMwareCredsFromRollingMigrationPlan` into a helper accepting either `*RollingMigrationPlan` or direct `VMwareCredsRef`; standalone path uses `ESXIMigrationSpec.VMwareCredsRef` and `PCDClusterRef` directly
- [ ] T028 [US2] Wire standalone path in `handleESXiCordoned` in `k8s/migration/internal/controller/esximigration_controller.go`: when `scope.RollingMigrationPlan` is nil, call `GetBMProviderFromBMConfigRef` + `GetCredsFromSpec`; when non-nil, existing code path unchanged
- [ ] T029 [P] [US2] Write unit tests for ESXIMigration standalone path in `k8s/migration/internal/controller/esximigration_controller_test.go`: with plan ref (existing path unchanged), without plan ref (uses BMConfigRef + PCDClusterRef), EC-004 (missing BMConfig → Failed phase); mock k8s client
- [ ] T030 [US2] Add `EnterMaintenanceMode` and `ExitMaintenanceMode` RPCs to `VCenter` service in `pkg/vpwned/sdk/proto/v1/api.proto` with HTTP bindings `/vpw/v1/enter_maintenance_mode` and `/vpw/v1/exit_maintenance_mode`
- [ ] T031 [US2] Regenerate proto bindings in `pkg/vpwned/` after maintenance RPC additions
- [ ] T032 [US2] Implement `EnterMaintenanceMode` handler in `pkg/vpwned/server/target_vcenter.go` using `govmomi object.HostSystem.EnterMaintenanceMode(ctx, timeout, evacuatePoweredOffVms, spec)` — consult govmomi docs before implementing
- [ ] T033 [US2] Implement `ExitMaintenanceMode` handler in `pkg/vpwned/server/target_vcenter.go` using `govmomi object.HostSystem.ExitMaintenanceMode` (same file as T032 — run after T032 completes)
- [ ] T034 [P] [US2] Write unit tests for EnterMaintenanceMode and ExitMaintenanceMode handlers in `pkg/vpwned/server/target_vcenter_test.go` (mock HostSystem)

**Checkpoint**: Standalone ESXIMigration path works end-to-end without RollingMigrationPlan. EnterMaintenanceMode / ExitMaintenanceMode API endpoints tested.

---

## Phase 5: US3 Backend — BMConfig CRD + IronicProvider

**Goal**: ESXi→PCD conversion uses OpenStack Ironic when BMConfig.ProviderType=ironic.

**Backend Test**: Configure BMConfig with ProviderType=ironic + endpoint + credentials. Create ESXIMigration CR. Verify Ironic API used for provisioning; MAAS not required.

- [ ] T040 [US3] Add `IronicProvider BMCProviderName = "ironic"` and `IPMIProvider BMCProviderName = "ipmi"` constants to `k8s/migration/api/v1alpha1/bmconfig_types.go`
- [ ] T041 [US3] Add `IronicConfig` struct (Endpoint, Username, Password, ProjectID, DomainName) and `IPMIConfig` struct (BMCAddress, Username, Password, Interface, UseRedfish) to `k8s/migration/api/v1alpha1/bmconfig_types.go`
- [ ] T042 [US3] Add `IronicConfig *IronicConfig` and `IPMIConfig *IPMIConfig` optional fields to `BMConfigSpec` in `k8s/migration/api/v1alpha1/bmconfig_types.go`
- [ ] T043 [US3] Run `make generate` in `k8s/migration/` and verify BMConfig CRD YAML updated with new fields
- [ ] T044 [US3] Add `gophercloud` dependency: `cd pkg/vpwned && go get github.com/gophercloud/gophercloud && go mod tidy`
- [ ] T045 [US3] Extend existing stub `pkg/vpwned/sdk/providers/ironic/ironic.go` to implement `BMCProvider`: `Connect` (Keystone auth → Ironic client), `ListResources` (GET /v1/nodes), `GetResourceInfo` (GET /v1/nodes/{id}), `DeployMachine` (set instance_info + provision → active with EC-005 retry: up to 3 attempts with exponential backoff), `ReclaimBM` (provision → available), `SetBM2PXEBoot` (PATCH boot_interface: pxe), `StartBM`/`StopBM` (power state); consult [Ironic API docs](https://docs.openstack.org/ironic/latest/api/) before implementing; unsupported MAAS-specific methods return `ErrNotSupported`
- [ ] T046 [US3] Add blank import `_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/ironic"` to `k8s/migration/pkg/utils/bmprovisionerutils.go` — triggers IronicProvider.init() self-registration; verify with `providers.GetProvider("ironic")` in unit test
- [ ] T047 [P] [US3] Write unit tests for IronicProvider in `pkg/vpwned/sdk/providers/ironic/ironic_test.go`: mock gophercloud HTTP client; test Connect, DeployMachine, ReclaimBM, SetBM2PXEBoot, StartBM, StopBM; test EC-005 (unreachable → 3 retries with backoff → error)

**Checkpoint**: ESXi→PCD conversion works with Ironic provider. MAAS regression verified (existing BMConfig unchanged).

---

## Phase 6: US4 Backend — Direct IPMI/Redfish Provider

**Goal**: ESXi→PCD conversion uses direct IPMI/Redfish when BMConfig.ProviderType=ipmi.

**Backend Test**: Configure BMConfig with ProviderType=ipmi + BMC credentials. Create ESXIMigration CR. Verify IPMI sets PXE boot + power cycle; cloud-init served from vJailbreak HTTP endpoint.

- [ ] T048 [US4] Add `gofish` dependency: `cd pkg/vpwned && go get github.com/stmcginnis/gofish && go mod tidy`
- [ ] T049 [US4] Extract IPMI boot/power helpers from `pkg/vpwned/sdk/providers/maas/maas.go` into `pkg/vpwned/sdk/providers/ipmi/ipmi_helpers.go` without changing logic — extract functions `ChassisControlPowerUp`, `ChassisControlPowerDown`, and the PXE bootdev sequence (refactor only — logic-preserving per constitution VII)
- [ ] T050 [US4] Create `pkg/vpwned/sdk/providers/ipmi/ipmi.go` implementing `BMCProvider` — IPMI path: `StartBM` (ChassisControlPowerUp), `StopBM` (ChassisControlPowerDown), `SetBM2PXEBoot` (chassis bootdev pxe + power cycle via helpers from T049); `ListResources` returns single-element list from `IPMIConfig.BMCAddress`; MAAS-specific methods return `ErrNotSupported`
- [ ] T051 [US4] Add Redfish path to `IPMIProvider` in `pkg/vpwned/sdk/providers/ipmi/ipmi.go` (same file as T050 — run after T050): when `IPMIConfig.UseRedfish=true`, use `gofish` for power management (ComputerSystem.Reset) and boot override (BootSourceOverride PATCH)
- [ ] T052 [US4] Implement `DeployMachine` in `IPMIProvider` in `pkg/vpwned/sdk/providers/ipmi/ipmi.go`: serve cloud-init user-data from vJailbreak VM's local HTTP endpoint; after PXE boot machine fetches cloud-init and self-enrolls in PCD
- [ ] T053 [US4] Add blank import `_ "github.com/platform9/vjailbreak/pkg/vpwned/sdk/providers/ipmi"` to `k8s/migration/pkg/utils/bmprovisionerutils.go` — triggers IPMIProvider.init() self-registration (match maas and base pattern); verify with `providers.GetProvider("ipmi")` in unit test
- [ ] T054 [P] [US4] Write unit tests for IPMIProvider in `pkg/vpwned/sdk/providers/ipmi/ipmi_test.go`: IPMI path (boot device set + power cycle sequence), Redfish path (mock gofish power + boot override), `ListResources` single-element return

**Checkpoint**: All four backend user stories fully functional. No MAAS required for Ironic or IPMI paths.

---

## Phase 7: Backend Validation

**Purpose**: Verify all backend changes as a whole before UI work begins.

- [ ] T055 [P] Run `cd k8s/migration && make test` — all controller tests pass
- [ ] T056 [P] Run `cd pkg/vpwned && go test ./...` — all provider + handler tests pass
- [ ] T058 Verify MAAS regression: create BMConfig with ProviderType=MAAS and trigger ESXIMigration with RollingMigrationPlanRef set — existing orchestrated path works unchanged (SC-003)

**Checkpoint**: All backend tests pass. MAAS regression confirmed. Ready for UI phases.

---

## ── UI PHASES ────────────────────────────────────────────────────────────────

---

## Phase 8: UI — US1 (VMware Cluster Display)

**Goal**: Show VMware clusters + per-host VM counts and Migrate VMs action.

**UI Test**: Open Cluster Conversions page → VMware Clusters table shows hosts + VM counts. Click "Details" → drawer opens with ESXi host rows. Click "Migrate VMs" → migration form opens pre-populated.

- [ ] T018 [P] [US1] Update `ui/src/api/vmware-clusters/vmwareClusters.ts` to ensure `status.hosts`, `spec.bmConfigRef`, and `spec.pcdClusterRef` are read from VMwareCluster API response
- [ ] T019 [P] [US1] Create `useVMwareClustersQuery` hook in `ui/src/hooks/api/useVMwareClustersQuery.ts` (reuse pattern from existing `useESXIMigrationsQuery.ts`)
- [ ] T020 [US1] Create `VMwareClustersTable` component in `ui/src/features/clusterConversions/components/VMwareClustersTable.tsx`: mirrors `RollingMigrationsTable.tsx` structure — `CommonDataGrid` with columns (cluster name with `<cds-icon shape="cluster">`, host count, VM count, `StatusChip`, `LinearProgress` hosts-converted, Details action button); `ListingToolbar` with title="VMware Clusters"; Details button opens `ESXiClusterDetailsDrawer`
- [ ] T021 [US1] Create `ESXiClusterDetailsDrawer` component in `ui/src/features/clusterConversions/components/ESXiClusterDetailsDrawer.tsx`: mirrors `ClusterDetailsDrawer` from `RollingMigrationsTable.tsx` — `StyledDrawer` (right-anchored 1200px, gridTemplateRows header/content/footer), `DrawerHeader`/`DrawerContent`/`DrawerFooter` styled divs; `StatusSummary` at top; `CommonDataGrid` for ESXi host rows with columns: host name (`<cds-icon shape="host">`), `StatusChip` (add maintenance→warning, converting→info to existing switch), VM count, time elapsed, Actions column with `Box sx={{ display:'flex', gap:1 }}` containing stub "Migrate VMs" / "Put in Maintenance" / "Exit Maintenance" / "Convert to PCD Host" `Button variant="text" size="small"` buttons (wired in Phase 9); state chip from `ESXIMigration.status.phase` if CR exists else from `HostStatus`
- [ ] T022 [US1] Create `ESXiVMTable` component in `ui/src/features/clusterConversions/components/ESXiVMTable.tsx`: props `hostName: string`, `vmwareCredsRef: string`; calls `listVMs` with `host_name` filter when "Migrate VMs" clicked in drawer; `CommonDataGrid` with checkbox selection; "Migrate Selected (N) →" opens existing migration form with pre-selected VMs
- [ ] T023 [US1] Update `ui/src/features/clusterConversions/pages/ClusterConversionsPage.tsx` to add `useVMwareClustersQuery` and render `<VMwareClustersTable />` above `<RollingMigrationsTable />`; `RollingMigrationsTable` props unchanged
- [ ] T024 [P] [US1] Write unit tests for `ESXiClusterDetailsDrawer` in `ui/src/features/clusterConversions/components/ESXiClusterDetailsDrawer.test.tsx`: "Migrate VMs" visible when vmCount > 0, hidden when 0; mock `useVMwareClustersQuery`

**Checkpoint**: VMware Clusters table renders. ESXi host drawer opens. Migrate VMs flow functional.

---

## Phase 9: UI — US2 (Maintenance + Conversion Actions)

**Goal**: "Put in Maintenance", "Exit Maintenance", "Convert to PCD Host" buttons wired in drawer.

**UI Test**: Click "Put in Maintenance" → `ConfirmationDialog` shown → confirm → `Snackbar` success + row state updates. Empty+maintenance host: "Convert to PCD Host" appears → click → ESXIMigration CR created → row shows phases.

- [ ] T035 [P] [US2] Create maintenance API client in `ui/src/api/vcenter/maintenance.ts`: `enterMaintenanceMode` and `exitMaintenanceMode` functions passing access info via same pattern as existing `listVMs` calls in `vcenter.ts`
- [ ] T036 [US2] Wire "Put in Maintenance" button in `ui/src/features/clusterConversions/components/ESXiClusterDetailsDrawer.tsx`: always visible; shows `ConfirmationDialog` (from `src/components/dialogs`) with `icon={<WarningIcon color="warning" />}` before calling `enterMaintenanceMode`; shows loading state; warns user (EC-002) if migration running on host but allows; updates row state on confirmation via `Snackbar`+`Alert`
- [ ] T037 [US2] Wire "Exit Maintenance" button in `ui/src/features/clusterConversions/components/ESXiClusterDetailsDrawer.tsx`: visible only when `inMaintenanceMode === true`; calls `exitMaintenanceMode`; shows loading state; updates row state via `Snackbar`+`Alert`
- [ ] T038 [US2] Wire "Convert to PCD Host" button in `ui/src/features/clusterConversions/components/ESXiClusterDetailsDrawer.tsx`: visible only when `vmCount === 0 && inMaintenanceMode`; read `vmwareCluster.spec.bmConfigRef` + `pcdClusterRef` as defaults; if either nil, show `ConfirmationDialog` with embedded MUI `Select` fields for BMConfig and PCD cluster before creating CR; on confirm, create standalone `ESXIMigration` CR via k8s API; row shows conversion progress phases
- [ ] T039 [P] [US2] Update `ESXiClusterDetailsDrawer` unit tests in `ui/src/features/clusterConversions/components/ESXiClusterDetailsDrawer.test.tsx`: cover "Put in Maintenance" always visible + ConfirmationDialog shown, "Exit Maintenance" visible only when inMaintenanceMode, "Convert to PCD Host" gate (vmCount=0 AND inMaintenance), standalone ESXIMigration CR creation

**Checkpoint**: All per-host actions functional. Standalone ESXIMigration CR created from UI.

---

## Phase 10: UI — BMConfig Form (Ironic + IPMI Provider Types)

**Goal**: BMConfigForm supports provider type selection. Ironic and IPMI credential fields shown per selection.

**UI Test**: Open BM Config page → select ProviderType=ironic → Ironic credential fields appear, MAAS fields hidden. Save → BMConfig CR has ProviderType=ironic and IronicConfig populated. Switch back to MAAS → MAAS fields restore, existing behavior unchanged.

- [ ] T060 [US3] Add MUI `Select` for `ProviderType` (`MAAS` | `ironic` | `ipmi`) at top of `Section` in `ui/src/features/baremetalConfig/components/BMConfigForm.tsx`; default selection = MAAS; keep `SurfaceCard`/`Section`/`ActionButton` structure unchanged
- [ ] T061 [US3] Add conditional `Section` for Ironic fields in `BMConfigForm.tsx` (shown when ProviderType=ironic): Endpoint URL, Username, Password, Project ID, Domain Name — each as `FieldBlock` + `FieldLabel` + `TextField size="small" variant="outlined"` matching existing field layout; wire to form submit to create `BMConfig` with `IronicConfig` populated
- [ ] T062 [US4] Add conditional `Section` for IPMI fields in `BMConfigForm.tsx` (shown when ProviderType=ipmi): BMC Address, Username, Password, Interface — as `FieldBlock`/`FieldLabel`/`TextField`; UseRedfish as `ToggleField`; wire to form submit to create `BMConfig` with `IPMIConfig` populated
- [ ] T063 [P] Write unit tests for `BMConfigForm` provider switching in `ui/src/features/baremetalConfig/components/BMConfigForm.test.tsx`: MAAS fields visible when ProviderType=MAAS, Ironic fields visible/MAAS hidden when ironic selected, IPMI fields visible/MAAS hidden when ipmi selected

**Checkpoint**: BMConfig form supports all three provider types. Existing MAAS flow unchanged.

---

## Phase 11: Full Build Validation

**Purpose**: All tests pass end-to-end. Full build confirms no integration regressions.

- [ ] T057 [P] Run `cd ui && yarn test` — all UI component tests pass
- [ ] T059 [P] Run `make generate-manifests` — full build (backend + UI) succeeds

**Checkpoint**: All phases complete. Feature ready for PR.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup — **BLOCKS all other phases**
- **US1 Backend (Phase 3)**: Depends on Foundational
- **US2 Backend (Phase 4)**: Depends on Foundational
- **US3 Backend (Phase 5)**: Depends on Foundational — independent of US1/US2
- **US4 Backend (Phase 6)**: Depends on Foundational — can run in parallel with US3
- **Backend Validation (Phase 7)**: Depends on all desired backend phases
- **UI US1 (Phase 8)**: Depends on Phase 3 (VMwareCluster API must be stable)
- **UI US2 (Phase 9)**: Depends on Phase 4 (maintenance API) and Phase 8 (extends existing drawer)
- **UI BMConfig (Phase 10)**: Depends on Phase 5 + Phase 6 (IronicConfig/IPMIConfig in CRD)
- **Full Validation (Phase 11)**: Depends on Phases 8–10

### Backend Phases 3–6 — parallel after Phase 2

### Within Each Phase

- Tasks marked `[P]` touch different files and have no incomplete-task dependencies
- Phase 2: T004, T005 [P], T006 [P] run in parallel (different files); T007 follows all three
- Phase 3 API: T014 → T015 → T016 (sequential; same proto flow)
- Phase 4 API: T030 → T031 → T032 → T033 (sequential; proto then same handler file)
- Phase 6: T049 → T050 → T051 → T052 (sequential; same ipmi.go file after T049 extraction)

---

## Parallel Execution Examples

```bash
# Phase 2 — T004, T005, T006 in parallel (different files):
Task A: "vmwarecluster_types.go: HostStatus + Conditions + VMwareCredsRef + BMConfigRef + PCDClusterRef"
Task B: "esximigration_types.go: RollingMigrationPlanRef pointer + BMConfigRef + PCDClusterRef"
Task C: "esximigrationscope.go: RollingMigrationPlan pointer"
# Then T007 (make generate), T008 (tests in parallel), T009 (make test)

# Backend Phases 3–6 in parallel after Phase 2:
Task A: "Phase 3: US1 controller + ListVMs API"
Task B: "Phase 4: US2 controller decoupling + maintenance API"
Task C: "Phase 5: US3 Ironic provider"
Task D: "Phase 6: US4 IPMI provider"

# UI Phases 8–10 in parallel after Phase 7 validates:
Task A: "Phase 8: VMwareClustersTable + ESXiClusterDetailsDrawer (display only)"
Task B: "Phase 10: BMConfigForm Ironic/IPMI extension"
# Phase 9 starts after Phase 8 drawer component exists
```

---

## Implementation Strategy

### Backend-First Delivery

1. Phases 1–2: Toolchain + Foundational CRDs
2. Phases 3–6 in parallel: All backend user stories
3. Phase 7: Backend validation gate
4. **STOP AND VALIDATE** before any UI work begins
5. Phases 8–10 in parallel: UI user stories
6. Phase 11: Full build + regression

### Team Split (Backend)

After Phase 2:

- Developer A: Phase 3 (US1 controller + ListVMs)
- Developer B: Phase 4 (US2 decoupling + maintenance API)
- Developer C: Phase 5 (Ironic provider)
- Developer D: Phase 6 (IPMI provider)

### Team Split (UI)

After Phase 7 validates:

- Developer A: Phase 8 (cluster table + drawer display)
- Developer B: Phase 9 (maintenance action buttons — extends Phase 8 drawer)
- Developer C: Phase 10 (BMConfigForm extension)

---

## Notes

- `[P]` = different files, no blocking in-progress task — same file is always sequential
- All new Go code requires unit tests (`_test.go` alongside impl — constitution IV)
- Run `make generate` after every CRD type change — never hand-edit generated files (constitution III)
- Consult govmomi docs before implementing EnterMaintenanceMode/ExitMaintenanceMode (constitution II)
- Consult Ironic API docs before implementing IronicProvider (constitution II)
- Logic-preserving refactors (T049 IPMI extract) in separate commit from behavioral changes (constitution VII)
- Commit per milestone as specified in plan.md
