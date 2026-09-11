# Tasks: Cinder Fallback for Storage-Accelerated-Copy LUN Mapping

**Input**: Design documents from `/specs/005-cinder-mapping-fallback/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/interfaces.md

**Tests**: Included — constitution IV (Test-First) is NON-NEGOTIABLE for this repo.

**Organization**: Foundational SDK/plumbing first (blocks everything), then user stories: US1 = auto fallback path (MVP), US2 = cinder override, US3 = native guardrail.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 (auto fallback), US2 (cinder override), US3 (native guardrail)

## Phase 1: Setup

- [X] T001 Feature branch `private/main/cinder-optimisation` checked out; spec-kit artifacts generated under `specs/005-cinder-mapping-fallback/`
- [ ] T002 `make setup-hooks` run once on the developer clone (pre-commit validation)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Interface split, CinderMapper, OpenStack client plumbing, CRD field — everything the stories build on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T003 Split `StorageProvider` and add `VendorMapper` (same 4 signatures) in `pkg/vpwned/sdk/storage/storage.go`; Pure/NetApp/fcutil untouched
- [X] T004 [P] Write failing-first unit tests for connector building and mapper plumbing in `pkg/vpwned/sdk/storage/cinder/mapper_test.go` (FC-only, iSCSI-only, mixed, malformed `fc.X:Y`, empty input, host/ip defaults, multipath pin, map/unmap via fake `CinderActionClient`, missing volume-ID/connector errors)
- [X] T005 Implement `CinderMapper`, `CinderActionClient`, `BuildConnectorFromHBAs` in `pkg/vpwned/sdk/storage/cinder/mapper.go` (ctx-aware; lowercase WWNs; per-ESXi host; `MappingContext{"connector": ...}`; connector logged at map time)
- [X] T006 [P] Extend `OpenstackOperations` with `InitializeVolumeConnection`/`TerminateVolumeConnection` in `v2v-helper/openstack/openstackops.go`
- [X] T007 [P] Implement both volume actions on `*OpenStackClients` in `v2v-helper/pkg/utils/openstackopsutils.go` (pattern from `ManageExistingVolume` :1133; 200/`connection_info` and 202/no-body)
- [X] T008 Extend `v2v-helper/openstack/openstackops_mock.go` with the two methods (hand-added in golang/mock style) — regenerate to confirm: `cd v2v-helper && go generate ./openstack/...`
- [X] T009 [P] Add `MappingMode` enum field + `MappingModeAuto/Native/Cinder` constants to `k8s/migration/api/v1alpha1/arraycreds_types.go`
- [ ] T010 Regenerate CRD artifacts: `cd k8s/migration && make generate && make manifests` (deepcopy expected no-op; CRD YAML gains the enum — hand-staged in this change, target must produce zero further diff)

**Checkpoint**: SDK + plumbing compile; `cd pkg/vpwned && go test ./sdk/storage/cinder/...` green.

---

## Phase 3: User Story 1 — Migrate via auto Cinder fallback (Priority: P1) 🎯 MVP

**Goal**: A vendor with only the core provider migrates end-to-end; mapping goes through Cinder automatically.

**Independent Test**: Migration on a core-only vendor logs `selectMapper: using cinder fallback (<vendor>)` and completes; deferred unmap fires.

### Tests for User Story 1

- [X] T011 [P] [US1] Selector-matrix + adapter unit tests in `v2v-helper/migrate/mapper_test.go` (each mode × provider shape incl. native-failure and unknown-mode; adapter arg/result forwarding; CinderMapper host derivation from ESXi IP)

### Implementation for User Story 1

- [X] T012 [US1] Add `Mapper` interface, `vendorMapperAdapter`, `selectMapper(provider, osClients, mode, esxiHostIP)` in `v2v-helper/migrate/mapper.go`
- [X] T013 [US1] Hoist ArrayCreds resolution into `resolveArrayCreds(ctx, vmDisk)` and change `manageVolumeToCinder` to `(ctx, volumeName, arrayCreds)` in `v2v-helper/migrate/vaai_copy.go` (no logic change; drops the double-fetch and dead `vmDisk` param)
- [X] T014 [US1] Wire the mapper into `copyDiskViaStorageAcceleratedCopy`: check the previously ignored `InitializeStorageProvider` error (:163), select mapper after init, replace the three direct mapping calls, log `selectMapper: using <desc>`, deferred unmap under fresh 2-minute timeout ctx in `v2v-helper/migrate/vaai_copy.go`

**Checkpoint**: `cd v2v-helper && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./migrate/... -run 'TestSelectMapper|TestVendorMapperAdapter'` (Linux/Docker) green; Pure auto-mode behavior unchanged by inspection of selector tests.

---

## Phase 4: User Story 2 — Force Cinder path on Pure/NetApp (Priority: P2)

**Goal**: `mappingMode: cinder` exercises the fallback on existing hardware — the regression gate.

**Independent Test**: Pure E2E with `mappingMode: cinder` passes; logs + array UI confirm the Cinder path.

- [X] T015 [US2] `cinder` mode handled in `selectMapper` (forces CinderMapper for any provider) — covered by T012 code + T011 tests
- [ ] T016 [US2] Lab E2E: existing Pure migration with `mappingMode: cinder`; archive `selectMapper: using cinder fallback (pure)` + `Cinder connector:` log lines and cinder-volume `os-initialize_connection` entries (SC-003, CHK025)
- [ ] T017 [US2] Lab E2E control: same array, `mappingMode` unset → `selectMapper: using vendor-native (pure)`, zero mapping-time Cinder API calls (SC-002)

**Checkpoint**: Fallback path proven on real hardware without a third-party array.

---

## Phase 5: User Story 3 — Native-mode guardrail (Priority: P3)

**Goal**: `mappingMode: native` fails fast at CR validation for vendors without native mapping; gRPC preflight degrades gracefully.

**Independent Test**: ArrayCreds with `mappingMode: native` on a non-mapper vendor → `status.phase: Failed` with the exact message; mapping RPCs return failure responses instead of compile-time coupling.

- [X] T018 [P] [US3] Enforce native mode beside the NetApp gate in `k8s/migration/internal/controller/arraycreds_controller.go` (assert `storagesdk.VendorMapper`; message `MappingMode=native unsupported by vendor <type>`)
- [X] T019 [P] [US3] Type-assert `VendorMapper` in the 4 mapping RPCs in `pkg/vpwned/server/storage.go` (fail fast before Connect; `Success:false` responses, error for GetMappedGroups)
- [ ] T020 [US3] Controller test for the native-mode rejection in `k8s/migration` (envtest): `cd k8s/migration && make test`

**Checkpoint**: All three stories implemented; guardrail verified by controller suite.

---

## Phase 6: Polish & Cross-Cutting

- [X] T021 [P] Operator documentation: `specs/005-cinder-mapping-fallback/quickstart.md` (modes, per-backend knobs, troubleshooting, leak recovery)
- [ ] T022 Run full unit suites on Linux: `cd pkg/vpwned && go test ./sdk/storage/...`; `make test-v2v-helper`; `cd k8s/migration && make test`
- [ ] T023 `make generate-manifests` from repo root (requires vjail-controller + ui built) → `deploy/installer.yaml`
- [ ] T024 `GRAPHIFY_NO_BACKUP=1 graphify update .` to refresh the knowledge graph
- [ ] T025 Quickstart validation pass on the appliance (walk §1–§3 as written)

---

## Phase 7: First Cinder-mapped vendor — Hitachi Vantara (VSP)

**Goal**: Prove the ~200-LOC-core-provider promise: Vantara implements only the core `StorageProvider` (auth/session, CreateVolume, Delete, GetInfo, List, NAA, Resolve) — LUN mapping is delegated to the Hitachi Cinder driver (HBSD) via the CinderMapper under `auto`.

- [X] T026 [P] Implement `pkg/vpwned/sdk/storage/vantara/vantara.go`: Configuration Manager REST (session auth w/ 25-min refresh, API ≥1.9 gate, async job polling), CreateVolume (DP LDEV + 32-char label + naaId→NAA), Delete/GetInfo/List/GetAllVolumeNAAs, ResolveCinderVolumeToLUN by HBSD relabel convention (cinder UUID sans dashes), single-DP-pool auto-pick
- [X] T027 [P] Add `storage.CinderManageRefBuilder` optional interface in `pkg/vpwned/sdk/storage/storage.go`; Vantara returns `{"source-id": <LDEV id>}` (HBSD's source-name lookup requires dash-free labels); `manageVolumeToCinder` consults the builder and now takes the full `storage.Volume`
- [X] T028 [P] Unit tests `pkg/vpwned/sdk/storage/vantara/vantara_test.go` (httptest fake GUM): version gate, create w/ pool + block rounding + label truncation + NAA lowercasing, ambiguous-pool error, single-pool auto-pick, resolve-by-relabel (case-insensitive), idempotent delete, manage-ref, NOT-a-VendorMapper assertion
- [X] T029 Register vantara in `pkg/vpwned/sdk/storage/providers/providers.go`
- [X] T030 Config plumbing (mirrors NetApp): `VantaraConfig{poolId,restPort}` on ArrayCreds + CRD YAML; `VANTARA_POOL_ID`/`VANTARA_REST_PORT` in `migrationplan_controller.go` configmap; `buildProviderOptionsFromSpec` (arraycreds_controller.go); v2v-helper `vcenterutils.go` → `main.go` → `migrate.go` `buildProviderOptions`
- [X] T034 Auto-derive the DP pool from the Cinder backend mapping: `storage.CinderBackendPoolAware` optional interface + `ApplyCinderPoolHint` (numeric-ID or pool-name resolution, explicit `vantaraConfig.poolId` wins) in vantara; hint extracted in `vaai_copy.go` from `openstackMapping.cinderBackendPool` or the `#pool` suffix of `cinderHost`, applied before CreateVolume. Pool resolution order: explicit poolId → Cinder backend pool hint → sole-DP-pool auto-pick → fail with guidance
- [ ] T031 **`cd k8s/migration && make generate`** — REQUIRED: `VantaraConfig` is a pointer struct; until regen, `ArrayCredsSpec` deepcopies silently drop it. Then `make manifests` (must be zero-diff vs. staged YAML)
- [ ] T032 cinder.conf: configure the HBSD backend (`hitachi_storage_id`, `hitachi_pools`, FC/iSCSI driver) + volume type with `volume_backend_name`; set ArrayCreds `openstackMapping` accordingly
- [ ] T033 Lab E2E on VSP: migration with `mappingMode` unset → logs `selectMapper: using cinder fallback (vantara)`, HBSD `os-initialize_connection` maps the LDEV, XCOPY clone at array speed, terminate on cleanup

---

## Dependencies & Execution Order

- Phase 2 blocks everything; within it T004→T005 (test-first), T003 blocks T005/T012/T018/T019; T006→T007→T008.
- US1 (T011→T012→T013→T014) blocks US2 lab runs; US3 tasks T018/T019 depend only on Phase 2 and run parallel to US1.
- T010/T022–T025 require a Go 1.24 toolchain / lab and are left for the developer machine (sandbox had no toolchain; all code tasks above are complete and awaiting those runs).

## Notes

- Generated-file discipline: CRD YAML staged to match controller-gen output — `make manifests` must produce zero diff (verify in T010). `zz_generated.deepcopy.go` needs no change for a scalar field.
- Pure/NetApp provider files and `migrate.go` intentionally untouched — verify with `git diff --stat`.
