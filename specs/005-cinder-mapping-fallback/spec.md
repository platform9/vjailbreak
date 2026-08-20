# Feature Specification: Cinder Fallback for Storage-Accelerated-Copy LUN Mapping

**Feature Branch**: `private/main/cinder-optimisation`
**Created**: 2026-07-06
**Status**: Draft
**Input**: User description: "Offload the ESXi LUN-mapping step of Storage-Accelerated-Copy (XCOPY) to Cinder's os-initialize_connection / os-terminate_connection so new storage arrays only need ~200 LOC of core REST wrappers instead of ~1000 LOC including host/initiator-group mapping."

## Overview

vJailbreak's Storage-Accelerated-Copy (XCOPY) path must expose a freshly created target LUN to the ESXi host before `vmkfstools` can offload the clone to the array. Today that mapping step (host discovery, initiator-group management, LUN attach/detach) is implemented per vendor in `pkg/vpwned/sdk/storage/<vendor>/` and is the most complex per-vendor code we maintain. Cinder's volume drivers already implement exactly this operation for every supported array.

This feature splits the storage SDK interface into a required core (`StorageProvider`) and an optional `VendorMapper`, and adds a `CinderMapper` that performs the mapping via Cinder's `os-initialize_connection` / `os-terminate_connection` volume actions using a connector dict built from the ESXi host's HBAs. Vendor-native mapping (Pure, NetApp) remains the default fast path; the Cinder path is selected automatically for vendors without native mapping, or forced via a new `mappingMode` field on ArrayCreds.

Volume creation, deletion, NAA construction, and Cinder-volume-to-LUN resolution stay vendor-native to preserve the placement guarantee (target LUN lands on the same physical array as the source datastore) and NAA derivation independent of Cinder driver behavior.

---

## Clarifications

### Session 2026-07-06

- Q: Will `os-initialize_connection` work for every storage array? → A: For every array with an FC or iSCSI Cinder driver, yes by contract: `initialize_connection`/`terminate_connection` are part of Cinder's `VolumeDriverCore` required interface, exercised by mandatory third-party CI per vendor. Non-SAN backends (Ceph/RBD, NFS, PowerFlex) return connection info ESXi cannot consume, but those cannot back a VMFS datastore participating in XCOPY, so the boundary matches the feature's eligibility exactly. NVMe-oF backends are out of scope (vJailbreak's NAA/rescan flow is SCSI-only today).
- Q: Do Cinder drivers accept a connector dict that does not come from a Nova compute node? → A: Yes. The connector is defined as an opaque dictionary; Glance's Cinder image store and cinderclient's local-attach already call these APIs with non-Nova connectors. Verified upstream by the team.
- Q: Static connector `host` ("vjailbreak-xcopy") vs. per-ESXi host name? → A: Per-ESXi (`vjailbreak-<esxi-ip>`). Drivers look up existing array host objects initiator-first (verified in 3PAR/Pure/NetApp driver sources), and the XCOPY precondition means the ESXi's initiators are already registered — so the production host object is reused in the common case. The per-ESXi name is cheap hardening for name-first driver paths and for concurrent migrations from different ESXi hosts, and restores the os-brick invariant "one connector host = one physical host."
- Q: How to verify a Cinder-path mapping happened? → A: NOT via `cinder show` — `os-initialize_connection` creates no attachment record and the volume stays `available`. Verify array-side (e.g., Pure host connections) or in cinder-volume logs.
- Q: Mock tooling for the new OpenstackOperations methods? → A: `github.com/golang/mock` via the existing `//go:generate mockgen` directive in `v2v-helper/openstack/openstackops.go`; the mock lives at `v2v-helper/openstack/openstackops_mock.go`. (Not go.uber.org/mock.)
- Q: Which make targets regenerate CRD artifacts? → A: `make generate` (deepcopy — a no-op for this scalar field) AND `make manifests` (CRD YAML) inside `k8s/migration/`, then `make generate-manifests` from the repo root for `deploy/installer.yaml`.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Migrate from an array with no vendor-native mapping code (Priority: P1)

An operator needs to migrate VMs whose datastores live on a storage array (e.g., HPE 3PAR/Alletra, Dell) for which vJailbreak ships only the core provider (Connect, CreateVolume, DeleteVolume, GetVolumeInfo, ResolveCinderVolumeToLUN) — no host-mapping code. With `mappingMode` unset (auto), the migration runs end-to-end: the target LUN is created vendor-natively, managed into Cinder, exposed to the ESXi host via Cinder `os-initialize_connection`, cloned at array speed with `vmkfstools`, and unmapped via `os-terminate_connection`.

**Why this priority**: This is the entire value of the feature — new array support drops from ~1000 LOC to ~200 LOC of REST wrappers because the mapping step is delegated to the array's existing Cinder driver.

**Independent Test**: Configure ArrayCreds for a vendor whose provider does not implement `VendorMapper`; run a migration; confirm the log line `selectMapper: using cinder fallback (<vendor>)` and a successful XCOPY clone.

**Acceptance Scenarios**:

1. **Given** an ArrayCreds with `mappingMode` unset for a vendor whose provider lacks native mapping, **When** a Storage-Accelerated-Copy migration runs, **Then** the target LUN is exposed to the ESXi host via Cinder, the `vmkfstools` clone completes, and the LUN is unmapped afterwards via Cinder.
2. **Given** the same setup, **When** the mapping step runs, **Then** v2v-helper logs `selectMapper: using cinder fallback (<vendor>)` and logs the connector dict used, and cinder-volume logs show `os-initialize_connection` landing on the correct backend.
3. **Given** the ESXi host reports both iSCSI and FC adapters, **When** the connector is built, **Then** it contains both `initiator` and `wwpns`/`wwnns` keys and the backend driver uses whichever transport it serves.
4. **Given** the mapping step fails on the array side, **When** `os-initialize_connection` returns an error, **Then** the migration fails fast with the Cinder error surfaced, and any partial state is cleaned up by the deferred unmap.

---

### User Story 2 - Force the Cinder path on a natively supported array (Priority: P2)

An operator or CI pipeline sets `mappingMode: cinder` on a Pure/NetApp ArrayCreds to exercise the Cinder fallback on hardware that normally uses vendor-native mapping — validating the fallback path without needing a third-party array.

**Why this priority**: This is the only practical regression gate for the fallback path (the P1 array may not exist in CI), and it de-risks every future vendor added through this mechanism.

**Independent Test**: Set `mappingMode: cinder` on an existing Pure ArrayCreds; run the existing Pure migration E2E; confirm the cinder-path log lines and array-side host connection.

**Acceptance Scenarios**:

1. **Given** a Pure ArrayCreds with `mappingMode: cinder`, **When** a migration runs, **Then** it succeeds and v2v-helper logs `selectMapper: using cinder fallback (pure)`; the Pure UI shows the host connection during the copy window.
2. **Given** a Pure ArrayCreds with `mappingMode` unset or `auto`, **When** a migration runs, **Then** behavior is byte-for-byte the existing native path: logs show `selectMapper: using vendor-native (pure)` and no `os-initialize_connection` calls reach the Cinder API.
3. **Given** the volume is already mapped from a previous interrupted attempt, **When** the mapping step re-runs, **Then** the operation is treated as idempotent and the migration continues.

---

### User Story 3 - Operator requires vendor-native mapping (Priority: P3)

A storage administrator who does not want out-of-band Cinder connections on a given array sets `mappingMode: native`. For vendors with native mapping this changes nothing; for vendors without it, the ArrayCreds is marked Failed at validation time — before any migration is attempted — with an actionable message.

**Why this priority**: Guardrail/policy control; valuable but not required for the core capability.

**Independent Test**: Create an ArrayCreds with `mappingMode: native` for a vendor without `VendorMapper`; observe status.phase = Failed with the message `MappingMode=native unsupported by vendor <type>`, and no migration runs.

**Acceptance Scenarios**:

1. **Given** `mappingMode: native` on a Pure/NetApp ArrayCreds, **When** the reconciler validates it, **Then** validation succeeds and migrations use the vendor-native path.
2. **Given** `mappingMode: native` on an ArrayCreds whose vendor lacks native mapping, **When** the reconciler validates it, **Then** the CR is marked Failed with `MappingMode=native unsupported by vendor <type>` and the migration selector would also refuse with the same reason if reached.
3. **Given** any ArrayCreds, **When** `mappingMode` is set to a value outside `auto|native|cinder`, **Then** the API server rejects the update at admission (CRD enum validation).
4. **Given** a vendor without native mapping, **When** the UI/API preflight calls the mapping gRPC endpoints, **Then** they return a graceful failure response (`vendor <type> does not implement native mapping; use the Cinder fallback`) instead of crashing.

---

### Edge Cases

- ESXi host reports no active iSCSI or FC adapters → connector build fails with a descriptive error before any array/Cinder call.
- A malformed FC adapter UID (not `fc.WWNN:WWPN` hex) is skipped with a warning; if no usable initiators remain, the connector build fails.
- v2v-helper pod dies between initialize and terminate → the export leaks silently (no Cinder attachment record exists). Mitigation: the connector dict is logged at map time so support can hand-run `os-terminate_connection`; parity with the native path, which has the same failure mode.
- Two concurrent migrations from **different** ESXi hosts → distinct connector `host` values (`vjailbreak-<ip>`) keep driver-created host objects separate; initiator-first drivers reuse each ESXi's production host object anyway.
- Two concurrent migrations from the **same** ESXi host → identical connector; drivers share one host object per physical host, which is the os-brick invariant they are tested under.
- Migration ctx already canceled when cleanup runs → the deferred unmap uses a fresh timeout context so terminate still executes.
- Array host object auto-created by a Cinder driver may carry a generic (non-VMware) host persona/OS type → acceptable for a short-lived XCOPY target; per-backend cinder.conf knobs (e.g., `hpe3par:persona`, `pure_host_personality`) documented in quickstart.
- Fibre Channel Zone Manager deployments: `initialize_connection` may trigger zoning for every WWPN in the connector, including ones not previously zoned to this array.
- NVMe-oF backends: out of scope; the NAA-based ESXi rescan and `vmkfstools` flow is SCSI-only.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The storage SDK MUST split into a required core interface (`StorageProvider`: Connect, Disconnect, ValidateCredentials, CreateVolume, DeleteVolume, GetVolumeInfo, ListAllVolumes, GetAllVolumeNAAs, ResolveCinderVolumeToLUN, WhoAmI) and an optional `VendorMapper` interface (CreateOrUpdateInitiatorGroup, MapVolumeToGroup, UnmapVolumeFromGroup, GetMappedGroups) with signatures identical to today's methods, such that Pure and NetApp satisfy `VendorMapper` with zero code changes.
- **FR-002**: Mapper selection in v2v-helper MUST follow: `""`/`auto` → vendor-native if the provider satisfies `VendorMapper`, else Cinder fallback; `native` → vendor-native or error; `cinder` → Cinder fallback always; unknown → error.
- **FR-003**: The Cinder fallback MUST map/unmap the target LUN using `os-initialize_connection` / `os-terminate_connection` on the managed volume's ID, with the same connector dict for both calls, and MUST NOT create Cinder attachment records.
- **FR-004**: The connector dict MUST be built from the ESXi HBA identifiers returned by `esxiClient.GetAllHostAdapters()` and MUST mimic os-brick conventions: lowercase colon-stripped `wwpns`/`wwnns`, `initiator` = first IQN, `host` = `vjailbreak-<esxi-host-ip>` (fallback `vjailbreak-xcopy`), `ip` = ESXi host IP when known, `platform: x86_64`, `os_type: linux`, `multipath: true`. Mixed-transport hosts emit both key sets.
- **FR-005**: ArrayCreds MUST gain an optional `mappingMode` field validated at admission to `auto|native|cinder`; the reconciler MUST mark the CR Failed when `mappingMode: native` is set for a vendor whose provider does not satisfy `VendorMapper`.
- **FR-006**: The four mapping gRPC RPCs in vpwned MUST return a graceful failure (Success=false or error, message naming the vendor) when the provider does not satisfy `VendorMapper`; the proto schema MUST NOT change.
- **FR-007**: Volume creation, deletion, NAA construction, and `ResolveCinderVolumeToLUN` MUST remain vendor-native; the ESXi-side NAA-based rescan MUST be unchanged.
- **FR-008**: Order of operations MUST be preserved: vendor CreateVolume → Cinder manage → resolve renamed LUN → map (native or Cinder) → rescan by NAA → vmkfstools clone → deferred unmap.
- **FR-009**: v2v-helper MUST log the selected mapper as `selectMapper: using vendor-native (<vendor>)` or `selectMapper: using cinder fallback (<vendor>)`, and MUST log the connector dict at map time for supportability.
- **FR-010**: All new code MUST have unit tests with mocked external dependencies (constitution IV): connector building (FC-only, iSCSI-only, mixed, malformed, empty), selector matrix (each mode × provider shape), and Cinder action plumbing.
- **FR-011**: The deferred unmap MUST run under a fresh timeout context so cleanup executes even when the migration context is canceled.

### Key Entities

- **ArrayCreds.spec.mappingMode**: Optional enum (`auto|native|cinder`, empty ≡ auto) selecting how target LUNs are exposed to ESXi during Storage-Accelerated-Copy.
- **VendorMapper**: Optional storage-SDK interface marking providers with vendor-native mapping (Pure, NetApp).
- **Mapper**: v2v-helper-local, context-aware interface abstracting the three mapping operations used by the XCOPY flow; implemented by a thin adapter over `VendorMapper` and by `CinderMapper`.
- **CinderMapper**: Maps/unmaps via Cinder volume actions through a minimal `CinderActionClient` interface satisfied by v2v-helper's OpenStack client.
- **MappingContext**: Existing flexible map; the Cinder path stores the os-brick-style connector under key `connector`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new array vendor can be enabled for Storage-Accelerated-Copy by implementing only the core provider (~200 LOC of REST wrappers) — zero mapping code — and configuring its Cinder backend.
- **SC-002**: Pure and NetApp migrations under `auto` are behaviorally unchanged: selector logs vendor-native and zero `os-initialize_connection` calls reach the Cinder API during mapping.
- **SC-003**: The existing Pure migration E2E passes with `mappingMode: cinder`, proving the fallback path on real hardware.
- **SC-004**: `mappingMode: native` on an unsupported vendor is rejected at ArrayCreds validation (CR Failed with actionable message) — never mid-migration.
- **SC-005**: Unit suites pass: `cd pkg/vpwned && go test ./sdk/storage/cinder/...`, v2v-helper migrate tests (Linux/Docker), `cd k8s/migration && make test`.

## Assumptions

- The array's Cinder backend is configured in cinder.conf and healthy — already a hard requirement of the existing `manageVolumeToCinder` step, so the fallback adds no new deployment prerequisites.
- Scope is FC and iSCSI SAN backends; NVMe-oF and non-SAN backends (Ceph, NFS, PowerFlex) are out of scope.
- The OpenStack policy for `os-initialize_connection`/`os-terminate_connection` permits the volume owner (vJailbreak manages the volume into its own project); default admin-or-owner policy suffices.
- ESXi hosts participating in XCOPY already have SAN connectivity to the array (source datastore lives there), so their initiators are typically pre-registered and Cinder drivers reuse the existing array host object initiator-first.
- The v2v-helper pod runs one migration; per-disk mapping operations are sequential within it.
