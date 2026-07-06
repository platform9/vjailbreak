# Research: Cinder Fallback for Storage-Accelerated-Copy LUN Mapping

**Feature**: [spec.md](./spec.md) | **Date**: 2026-07-06

Decisions below were validated against the vJailbreak codebase (branch `private/main/cinder-optimisation`) and upstream OpenStack sources.

## D1 — Offload only the mapping step to Cinder; keep create/delete/NAA vendor-native

**Decision**: Cinder handles `os-initialize_connection`/`os-terminate_connection` only. `CreateVolume`, `DeleteVolume`, NAA construction (`BuildNAA(serial)` from the array's create response), and `ResolveCinderVolumeToLUN` stay on the vendor's own REST API.

**Rationale**: (a) Placement guarantee — direct CreateVolume on the array forces the target LUN onto the same physical array as the source datastore; Cinder's scheduler does not. (b) NAA without driver assumptions — some Cinder drivers (NetApp FC) do not return a serial in `connection_info`, so deriving the NAA from the vendor create response is the only universal path. The ESXi rescan (`RescanStorageForDevice`, keyed by NAA) is untouched.

**Alternatives considered**: Full Cinder lifecycle (create via volume type) — rejected: loses placement guarantee and NAA derivation. os-brick on the appliance — rejected: the LUN must be visible to the *ESXi host*, not to the appliance.

## D2 — Every FC/iSCSI Cinder driver supports this by contract

**Decision**: Treat `initialize_connection`/`terminate_connection` as universally available across SAN drivers.

**Evidence**: They are declared in Cinder's `VolumeDriverCore` (`cinder/interface/volume_driver.py`) — "Core backend driver required interface... all backend drivers should support this interface as a bare minimum." Every in-tree vendor driver runs mandatory third-party CI where attach/detach (which exercises these methods) is baseline. Scope boundary: non-SAN backends (Ceph/RBD, NFS, PowerFlex/SDC) return connection info ESXi cannot consume — but they also cannot back VMFS datastores doing XCOPY, so the limitation is congruent with feature eligibility. NVMe-oF is excluded (vJailbreak's NAA/rescan flow is SCSI-only today).

## D3 — Arbitrary (non-Nova) connectors are accepted

**Decision**: Build the connector dict ourselves from ESXi HBA identifiers; call the volume actions directly.

**Evidence**: The driver interface defines the connector as an opaque "dictionary containing information about what is being connected to." Established non-Nova callers: Glance's Cinder image store and cinderclient's local-attach. Team additionally verified upstream that drivers don't care whether the host is a Nova compute or an ESXi host. Policy: the volume-action APIs default to admin-or-owner; vJailbreak owns the managed volume (managed into its own project).

## D4 — Connector `host` is per-ESXi, not a static constant

**Decision**: `connector["host"] = "vjailbreak-" + sanitized(esxiHostIP)` (fallback `vjailbreak-xcopy` when the IP is unknown).

**Evidence & rationale**: Driver-source review showed SAN drivers resolve array host objects **initiator-first** (HPE 3PAR FC `_create_host` queries `queryHost(wwns=...)` and reuses any host owning the WWPNs; Pure searches hosts by IQN/WWN; NetApp matches igroups by initiator set). Because XCOPY requires the source datastore on the same array, the ESXi's initiators are already registered, so the production host object is reused and the connector host name is usually never materialized. The per-ESXi value is cheap hardening for the residual cases: name-first driver paths (3PAR matches by name *before* falling back to WWN query and merges new WWNs into a name-matched host via `_add_new_wwn_to_host`), duplicate-name create conflicts on drivers that use `connector['host']` verbatim, and cleanup races on a shared host object between concurrent migrations from different ESXi hosts. It also restores the os-brick invariant every driver is tested under: one connector host value = one physical host.

## D5 — Connector shape mimics os-brick exactly

**Decision**: `wwpns`/`wwnns` lowercase colon-stripped hex; `initiator` = first IQN; include `ip` (ESXi host IP) when known; `platform: "x86_64"`; `os_type: "linux"`; `multipath: true`. Mixed-transport hosts emit both key sets.

**Rationale**: os-brick emits lowercase WWNs and some drivers do exact string comparison — `fcutil.StripWWNFormatting` returns uppercase, so the mapper lowers it. Several iSCSI drivers read `connector["ip"]`. `multipath: true` is load-bearing: the 3PAR FC driver truncates `connector['wwpns']` to a single WWPN when multipath is falsy. `os_type: "linux"` (not "vmware") keeps us inside the value space drivers are tested with; array-side host personas are controlled by backend config (`hpe3par:persona`, `pure_host_personality`), not the connector. Idempotency: re-running `initialize_connection` is idempotent in SAN drivers (returns the same connection info), which preserves vaai_copy's existing already-mapped tolerance.

## D6 — v2v-helper Mapper interface is context-aware; vendors bridged by an adapter

**Decision**: The local `Mapper` interface takes `context.Context`; `vendorMapperAdapter` (~15 LOC) drops ctx when delegating to `VendorMapper` implementations; `CinderMapper` is natively ctx-aware. The deferred unmap uses a fresh `context.WithTimeout(context.Background(), 2m)`.

**Rationale**: Vendor methods are ctx-less today and must not change (zero-edit guarantee). Hard-coding `context.Background()` inside CinderMapper would hide a real bug: on failure paths the migration ctx is already canceled when the deferred cleanup runs, which would abort the terminate call. The adapter isolates that decision at the seam and future mappers slot in without another interface change.

## D7 — Interface split is safe repo-wide

**Decision**: Remove the four mapping methods from `StorageProvider`; add optional `VendorMapper`.

**Evidence**: Repo-wide grep shows the only non-generated consumers are `v2v-helper/migrate/vaai_copy.go`, `pkg/vpwned/server/storage.go`, and the two providers. No UI code calls the mapping RPCs. `BaseStorageProvider` has no mapping defaults. The codebase already uses the optional-interface + type-assert pattern (`BackendTargetDiscoverer`, asserted in arraycreds_controller.go). Go structural typing means Pure/NetApp satisfy `VendorMapper` with zero edits, and `*utils.OpenStackClients` satisfies `cinder.CinderActionClient` once the two methods exist on `OpenstackOperations`.

## D8 — gRPC proto frozen; handlers degrade gracefully

**Decision**: Keep the four mapping RPCs; each type-asserts `VendorMapper` right after provider lookup (before Connect) and returns `Success: false` + message (or an error for GetMappedGroups, which has no Success flag).

**Rationale**: Schema stability for external consumers; fail-fast avoids a pointless array round-trip.

## D9 — Tooling facts (corrections to the original draft spec)

- `ManageExistingVolume` lives at `v2v-helper/pkg/utils/openstackopsutils.go:1133-1177` (not :957) — same `BlockStorageClient.Post` + `ServiceURL` + `OkCodes` pattern to copy. `os-initialize_connection` → 200 with `connection_info`; `os-terminate_connection` → 202, no body; no microversion header needed (manage's `volume 3.8` header is specific to manageable_volumes).
- Mocks: `github.com/golang/mock v1.6.0` via `//go:generate mockgen` in `v2v-helper/openstack/openstackops.go:29`, output `v2v-helper/openstack/openstackops_mock.go`. There is no `migrate/mocks/` directory and no go.uber.org/mock.
- `k8s/migration/Makefile`: `generate` runs controller-gen `object` (deepcopy only); `manifests` produces the CRD YAML — run both. Adding a scalar string field produces **no** deepcopy diff (`ArrayCredsSpec.DeepCopyInto` is `*out = *in` for scalars).
- v2v-helper migrate tests require Linux to *execute* (CGO/libnbd); cross-compiling from macOS only builds.
- `pkg/vpwned` uses a `vendor/` directory; the new cinder package imports only stdlib, the storage SDK, fcutil, and klog/v2 (already vendored via pure.go) — no vendor updates required.

## D10 — Operational caveats to document (quickstart)

Auto-created array host objects may get generic personas/OS types — set backend knobs where it matters (`hpe3par:persona`, `pure_host_personality`, NetApp igroup os_type). CHAP-enforced iSCSI backends work on the initiator-reuse path; fresh driver-created host entries with generated CHAP may need ESXi-side config. FC Zone Manager deployments will zone every WWPN present in the connector. Crash between map and unmap leaks the export invisibly (no attachment record) — the connector is logged at map time so support can hand-run `os-terminate_connection`.
