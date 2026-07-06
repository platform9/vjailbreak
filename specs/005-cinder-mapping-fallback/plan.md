# Implementation Plan: Cinder Fallback for Storage-Accelerated-Copy LUN Mapping

**Branch**: `private/main/cinder-optimisation` | **Date**: 2026-07-06 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-cinder-mapping-fallback/spec.md`

## Summary

Split the storage SDK's monolithic `StorageProvider` into a required core interface and an optional `VendorMapper` interface (the four ESXi mapping methods). Add a `CinderMapper` that performs the mapping via Cinder `os-initialize_connection` / `os-terminate_connection` with an os-brick-style connector built from the ESXi host's HBAs. v2v-helper selects the mapper per disk from the ArrayCreds' new `mappingMode` field (`auto` default: vendor-native when available, else Cinder). Volume creation/deletion/NAA/resolution stay vendor-native, preserving the placement guarantee and NAA derivation. Pure and NetApp code is untouched and remains the default fast path.

## Technical Context

**Language/Version**: Go 1.24 (module directives: v2v-helper `go 1.24.10`)
**Primary Dependencies**: gophercloud/v2 v2.9.0 (both pkg/vpwned and v2v-helper), controller-runtime (k8s/migration), k8s.io/klog/v2, github.com/golang/mock v1.6.0 (v2v-helper mocks)
**Storage**: Kubernetes CRs (ArrayCreds) for configuration; no new persistent state
**Testing**: `go test` table-driven units with mocked externals; envtest for controller (`cd k8s/migration && make test`); v2v-helper tests require `CGO_ENABLED=1 GOOS=linux GOARCH=amd64` (run on Linux/Docker — cross-compiling from macOS builds but cannot execute)
**Target Platform**: vJailbreak appliance (k3s on Linux); v2v-helper migration worker pod
**Project Type**: Multi-module Go monorepo (4 independent modules; 3 touched: `pkg/vpwned`, `k8s/migration`, `v2v-helper`)
**Performance Goals**: No change to copy throughput — mapping is a per-disk control-plane operation (2 Cinder API calls per disk on the fallback path)
**Constraints**: gRPC proto schema frozen; Pure/NetApp provider files must not change; generated files only via make targets
**Scale/Scope**: ~200 LOC new SDK code + ~120 LOC v2v-helper glue + CRD field; new vendors drop from ~1000 LOC (NetApp scale) to ~200 LOC core wrappers

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Kubernetes-Native Architecture | PASS | Mode selection lives on the ArrayCreds CR (`spec.mappingMode`); no external state. |
| II. External Documentation First | PASS | Cinder `VolumeDriverCore` contract, os-brick connector conventions, and 3PAR/Pure driver sources reviewed (research.md D2–D5). |
| III. Generated Code Protection | PASS | `zz_generated.deepcopy.go` unchanged by a scalar field (`*out = *in`); CRD YAML via `make manifests`; `deploy/installer.yaml` via `make generate-manifests`. |
| IV. Test-First Development | PASS | Unit tests for connector building, selector matrix, Cinder action plumbing; mocked externals only. |
| V. Module Independence | PASS | v2v-helper already imports `pkg/vpwned` via module path + replace directive; new `cinder` package depends only on stdlib + storage SDK + klog (no vendor/ additions in pkg/vpwned). |
| VII. Code Reuse and Simplicity | PASS | Logic-preserving hoist of the ArrayCreds lookup (`resolveArrayCreds`); no behavior change to `manageVolumeToCinder` beyond signature. |

## Project Structure

### Documentation (this feature)

```text
specs/005-cinder-mapping-fallback/
├── plan.md              # This file
├── research.md          # Phase 0 output — decisions & upstream evidence
├── data-model.md        # Phase 1 output — CRD field, selector table, connector schema
├── quickstart.md        # Phase 1 output — operator walkthrough & verification
├── contracts/
│   └── interfaces.md    # Phase 1 output — Go interfaces, connector dict, gRPC behavior
├── checklists/
│   └── requirements.md  # Requirements quality checklist
└── tasks.md             # Phase 2 output — implementation tasks
```

### Source Code (repository root)

```text
pkg/vpwned/
├── sdk/storage/
│   ├── storage.go               # MODIFY: split StorageProvider / add VendorMapper
│   ├── cinder/
│   │   ├── mapper.go            # NEW: CinderMapper, CinderActionClient, BuildConnectorFromHBAs
│   │   └── mapper_test.go       # NEW: connector + mapper unit tests
│   ├── pure/pure.go             # NO CHANGES (satisfies VendorMapper as-is)
│   ├── netapp/netapp.go         # NO CHANGES (satisfies VendorMapper as-is)
│   └── fcutil/fcutil.go         # NO CHANGES (reuse ParseFCUID/StripWWNFormatting)
└── server/storage.go            # MODIFY: type-assert VendorMapper in 4 mapping RPCs

k8s/migration/
├── api/v1alpha1/arraycreds_types.go            # MODIFY: MappingMode field + constants
├── internal/controller/arraycreds_controller.go # MODIFY: native-mode enforcement
└── config/crd/bases/vjailbreak.k8s.pf9.io_arraycreds.yaml # REGENERATED (make manifests)

v2v-helper/
├── openstack/
│   ├── openstackops.go          # MODIFY: +InitializeVolumeConnection/+TerminateVolumeConnection
│   └── openstackops_mock.go     # REGENERATED (go generate ./openstack/...)
├── pkg/utils/openstackopsutils.go # MODIFY: implement the two volume actions
└── migrate/
    ├── mapper.go                # NEW: Mapper interface, vendorMapperAdapter, selectMapper
    ├── mapper_test.go           # NEW: selector matrix + adapter tests
    └── vaai_copy.go             # MODIFY: hoist resolveArrayCreds, wire mapper, ctx-safe unmap

deploy/installer.yaml            # REGENERATED (make generate-manifests, requires built images)
```

**Structure Decision**: Three of the four Go modules are touched at their existing seams. The new `cinder` package lives in the storage SDK (`pkg/vpwned/sdk/storage/cinder/`) beside the vendors it substitutes for; the selector lives in v2v-helper because that is where the ArrayCreds context and OpenStack clients meet.

## Design

### 1. Interface split (backward compatible for implementations)

In `pkg/vpwned/sdk/storage/storage.go`, remove the four mapping methods from `StorageProvider` and declare them on a new optional interface:

```go
type VendorMapper interface {
    CreateOrUpdateInitiatorGroup(initiatorGroupName string, hbaIdentifiers []string) (MappingContext, error)
    MapVolumeToGroup(initiatorGroupName string, targetVolume Volume, context MappingContext) (Volume, error)
    UnmapVolumeFromGroup(initiatorGroupName string, targetVolume Volume, context MappingContext) error
    GetMappedGroups(targetVolume Volume, context MappingContext) ([]string, error)
}
```

Signatures are byte-identical to today's methods, so `*PureStorageProvider` (pure.go:162-281) and `*NetAppStorageProvider` (netapp.go:487-706) satisfy `VendorMapper` with **zero edits**. Repo-wide, only three non-generated files consume these methods (`v2v-helper/migrate/vaai_copy.go`, `pkg/vpwned/server/storage.go`, and the providers themselves) — all updated by this plan. `BaseStorageProvider` (base.go) carries no mapping defaults. Precedent for the optional-interface pattern: `BackendTargetDiscoverer` (storage.go:108, asserted at arraycreds_controller.go:357).

### 2. Mapper selection (auto + override)

New field on `ArrayCredsSpec` (k8s/migration/api/v1alpha1/arraycreds_types.go):

```go
// MappingMode selects how target LUNs are exposed to the ESXi host during
// Storage-Accelerated-Copy. "auto" (default) uses vendor-native mapping when
// the provider implements it, falling back to Cinder os-initialize_connection
// otherwise. "native" hard-requires vendor-native (validation fails for
// providers that don't implement it). "cinder" forces the Cinder fallback
// even on Pure/NetApp (useful for testing the fallback path).
// +kubebuilder:validation:Enum=auto;native;cinder
// +optional
MappingMode string `json:"mappingMode,omitempty"`
```

plus exported constants `MappingModeAuto|Native|Cinder`. Selector truth table:

| MappingMode | Provider satisfies VendorMapper? | Choice |
|---|---|---|
| "" / "auto" | yes | vendor-native |
| "" / "auto" | no | CinderMapper |
| "native" | yes | vendor-native |
| "native" | no | error at validate/select |
| "cinder" | any | CinderMapper |

### 3. Local Mapper interface in v2v-helper (context-aware)

`v2v-helper/migrate/mapper.go` defines the seam the XCOPY flow consumes. Unlike the vendor methods, it takes `context.Context` so the Cinder calls get cancellation/timeouts; a ~15-line adapter bridges vendor providers without touching them:

```go
type Mapper interface {
    CreateOrUpdateInitiatorGroup(ctx context.Context, name string, hbaIdentifiers []string) (storage.MappingContext, error)
    MapVolumeToGroup(ctx context.Context, name string, vol storage.Volume, mctx storage.MappingContext) (storage.Volume, error)
    UnmapVolumeFromGroup(ctx context.Context, name string, vol storage.Volume, mctx storage.MappingContext) error
}

type vendorMapperAdapter struct{ vendor storage.VendorMapper } // drops ctx

func selectMapper(provider storage.StorageProvider, osClients openstack.OpenstackOperations,
    mode string, esxiHostIP string) (Mapper, string, error)
```

`selectMapper` returns a human-readable description (`vendor-native (pure)` / `cinder fallback (pure)`) which the caller logs as `selectMapper: using <desc>`. `GetMappedGroups` is not part of `Mapper` — the XCOPY flow never calls it (gRPC preflight does, via `VendorMapper` directly).

### 4. CinderMapper

New `pkg/vpwned/sdk/storage/cinder/mapper.go` (~200 LOC). No gophercloud import needed — the client surface is an interface:

```go
type CinderActionClient interface {
    InitializeVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) (map[string]any, error)
    TerminateVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) error
}

type CinderMapper struct {
    Client CinderActionClient // *utils.OpenStackClients satisfies this
    Host   string             // connector["host"]; default "vjailbreak-xcopy"
    IP     string             // optional connector["ip"] (ESXi host IP)
}
```

`CreateOrUpdateInitiatorGroup` builds the connector from the HBA list (no array call) and stashes it in `MappingContext{"connector": ...}`. `MapVolumeToGroup`/`UnmapVolumeFromGroup` call the two volume actions with `vol.OpenstackVol.ID` and the stored connector (same dict both directions — required for drivers to identify the export).

**Connector contract** (`BuildConnectorFromHBAs(hbaIDs, host, ip)`) — mimics os-brick, consuming the exact `[]string` from `esxiClient.GetAllHostAdapters()` (v2v-helper/esxi-ssh/disk_ops.go:740; lowercase `iqn...` / `fc.WWNN:WWPN`):

- iSCSI entries → `initiator` = first IQN.
- FC entries → `wwpns`/`wwnns` as **lowercase**, colon-stripped hex (`fcutil.ParseFCUID` + `StripWWNFormatting`, then lowered — os-brick convention; some drivers exact-string-match). Malformed FC UIDs are skipped with a warning.
- Mixed → both key sets; drivers ignore unused keys.
- Always: `host` (per-ESXi: `"vjailbreak-" + sanitized(esxiHostIP)`, fallback `vjailbreak-xcopy`), `ip` when known, `platform: "x86_64"`, `os_type: "linux"`, `multipath: true` (3PAR truncates to one WWPN without it).
- No usable initiators → error before any API call.

Rationale for per-ESXi `host` and connector fidelity: research.md D4/D5.

### 5. Plumbing the volume actions through v2v-helper

Extend the `OpenstackOperations` interface (v2v-helper/openstack/openstackops.go:31) with `InitializeVolumeConnection` / `TerminateVolumeConnection` and implement both on `*utils.OpenStackClients` (v2v-helper/pkg/utils/openstackopsutils.go) using the same `BlockStorageClient.Post(ctx, ServiceURL("volumes", id, "action"), body, &result, &RequestOpts{OkCodes})` pattern as `ManageExistingVolume` (**openstackopsutils.go:1133-1177**). `os-initialize_connection` expects 200 and returns `connection_info`; `os-terminate_connection` expects 202 and returns no body. No microversion header required. `*OpenStackClients` then satisfies `cinder.CinderActionClient` structurally, and any `OpenstackOperations` value can be passed straight into `CinderMapper{Client: ...}`.

Regenerate the mock with the **existing golang/mock setup**: `//go:generate mockgen -source=../openstack/openstackops.go -destination=../openstack/openstackops_mock.go -package=openstack` (openstackops.go:29) → `go generate ./openstack/...`.

### 6. v2v-helper integration in vaai_copy.go

Refactor `copyDiskViaStorageAcceleratedCopy` (vaai_copy.go:150-296):

1. **Check the previously ignored error** from `migobj.InitializeStorageProvider(ctx)` (line 163).
2. **Hoist the ArrayCreds load**: extract `resolveArrayCreds(ctx, vmDisk)` from manageVolumeToCinder's lines 388-409 (mapping CR lookup by datastore + `k8sutils.GetArrayCreds`, k8sutils.go:259/:267); call it once at the top so `spec.MappingMode` is known before mapping, and pass the resolved object down — no double-fetch.
3. **Select the mapper** after provider init: `mapper, desc, err := selectMapper(migobj.StorageProvider, migobj.Openstackclients, arrayCreds.Spec.MappingMode, hostIP)`; log `selectMapper: using <desc>`.
4. Replace the three direct calls: line 175 → `mapper.CreateOrUpdateInitiatorGroup(ctx, ...)`, line 229 → `mapper.MapVolumeToGroup(ctx, ...)`, line 245 (defer) → `mapper.UnmapVolumeFromGroup(cleanupCtx, ...)` where `cleanupCtx` is a fresh 2-minute timeout context (the migration ctx may already be canceled on failure paths).
5. `manageVolumeToCinder` signature becomes `(ctx, volumeName string, arrayCreds vjailbreakv1alpha1.ArrayCreds)` — the `vmDisk` parameter is dropped (it existed only for the datastore lookup now hoisted). Body logic from line 411 onward is unchanged.

Order of operations preserved: manage (line 198) still precedes MapVolumeToGroup (line 229), so `vol.OpenstackVol.ID` (set in the targetVol literal, lines 221-228) is available to the CinderMapper. The existing already-mapped idempotency tolerance (lines 234-239) is kept. No changes to `migrate.go` — MappingMode flows entirely through vaai_copy.go.

### 7. gRPC surface (pkg/vpwned/server/storage.go:73-296)

Each of the four mapping RPCs type-asserts the provider to `storagesdk.VendorMapper` immediately after `NewStorageProvider` (fail fast, before Connect). On failure: `Success: false, Message: "vendor <type> does not implement native mapping; use the Cinder fallback"` (GetMappedGroups, which has no Success flag, returns an error). Proto schema unchanged. Note: no UI code calls these RPCs today (verified by repo grep) — this is a compile fix plus graceful behavior for external REST/gRPC consumers.

### 8. ArrayCreds reconciler validation

In `arraycreds_controller.go`, after credential validation succeeds and alongside the existing NetApp selection gate (~lines 218-235): if `spec.MappingMode == MappingModeNative`, get the provider via `storagesdk.NewStorageProvider(vendorType)` and type-assert `VendorMapper` (no array call needed — the assertion is type-based). On failure set phase/status Failed with `MappingMode=native unsupported by vendor <type>`. `auto`/`cinder`/empty pass through.

## Critical files

| File | Change |
|---|---|
| pkg/vpwned/sdk/storage/storage.go | Remove 4 mapping methods from `StorageProvider`; add `VendorMapper`. `MappingContext`, `Volume`, helpers unchanged. |
| pkg/vpwned/sdk/storage/cinder/mapper.go | NEW. CinderMapper, CinderActionClient, BuildConnectorFromHBAs. |
| pkg/vpwned/sdk/storage/cinder/mapper_test.go | NEW. Connector building (FC-only, iSCSI-only, mixed, malformed, empty) + map/unmap plumbing tests. |
| pkg/vpwned/sdk/storage/pure/pure.go | No changes. |
| pkg/vpwned/sdk/storage/netapp/netapp.go | No changes. |
| pkg/vpwned/sdk/storage/fcutil/fcutil.go | No changes (reuse ParseFCUID :22, StripWWNFormatting :44). |
| pkg/vpwned/server/storage.go | VendorMapper type-assertions in 4 mapping RPCs. |
| k8s/migration/api/v1alpha1/arraycreds_types.go | MappingMode enum field + constants. |
| k8s/migration/internal/controller/arraycreds_controller.go | native-mode enforcement beside the NetApp gate. |
| k8s/migration/api/v1alpha1/zz_generated.deepcopy.go | No diff expected (scalar field; `*out = *in`). Confirm via `make generate`. |
| k8s/migration/config/crd/bases/vjailbreak.k8s.pf9.io_arraycreds.yaml | Regenerated by `make manifests`. |
| deploy/installer.yaml | Regenerated by `make generate-manifests` (repo root; requires vjail-controller + ui built). |
| v2v-helper/openstack/openstackops.go | +2 interface methods. |
| v2v-helper/openstack/openstackops_mock.go | Regenerated via `go generate ./openstack/...` (golang/mock). |
| v2v-helper/pkg/utils/openstackopsutils.go | Implement the two volume actions (pattern from ManageExistingVolume :1133). |
| v2v-helper/migrate/mapper.go | NEW. Mapper interface, vendorMapperAdapter, selectMapper. |
| v2v-helper/migrate/mapper_test.go | NEW. Selector matrix + adapter forwarding tests. |
| v2v-helper/migrate/vaai_copy.go | Init-error check, resolveArrayCreds hoist, mapper wiring, ctx-safe deferred unmap, manageVolumeToCinder(ctx, name, arrayCreds). |
| v2v-helper/migrate/migrate.go | No changes. |

## Verification

### Unit

- `cd pkg/vpwned && go test ./sdk/storage/cinder/...` — connector building and mapper plumbing (pure Go, runs anywhere).
- `cd v2v-helper && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./migrate/... -run 'TestSelectMapper|TestVendorMapperAdapter'` — **executes only on Linux** (cross-compiling from macOS builds but cannot run); use Docker or `make test-v2v-helper`.
- `cd k8s/migration && make test` — controller rejects `mappingMode: native` for non-mapper vendors.

### Generation

- `cd k8s/migration && make generate && make manifests` — deepcopy (expected no-op) + CRD YAML with the new enum property.
- `make generate-manifests` from repo root regenerates `deploy/installer.yaml` (requires vjail-controller and ui built first).
- `cd v2v-helper && go generate ./openstack/...` — refresh the OpenstackOperations mock.

### End-to-end

- **Pure (auto → native)**: existing migration E2E. Logs show `selectMapper: using vendor-native (pure)`; zero `os-initialize_connection` calls in Cinder logs during mapping.
- **Pure (cinder override)**: `mappingMode: cinder`; logs show `selectMapper: using cinder fallback (pure)`; migration succeeds; Pure UI shows the host connection during the copy window. (Do **not** use `cinder show` — initialize_connection creates no attachment record.)
- **New vendor (auto → cinder)**: requires that vendor's ~200 LOC core provider to exist first; until then, the Pure cinder-override run is the regression gate.
- **Native required, missing**: `mappingMode: native` on a non-mapper vendor's ArrayCreds → CR Failed with `MappingMode=native unsupported by vendor <type>`; no migration runs.

### Logs to grep

- v2v-helper: `selectMapper: using vendor-native (<vendor>)` / `selectMapper: using cinder fallback (<vendor>)`; `Cinder connector:` (full dict at map time); existing `Cleaning up volume mappings` in both paths.
- cinder-volume: `os-initialize_connection` / `os-terminate_connection` landing on the right backend.

## What this plan deliberately does NOT do

- Does not touch CreateVolume / DeleteVolume / NAA generation / ResolveCinderVolumeToLUN — placement guarantee preserved exactly.
- Does not modify manageVolumeToCinder's logic (only its signature, to accept the pre-loaded ArrayCreds).
- Does not touch ESXi-side code (esxi-ssh/) — NAA-based device lookup unchanged.
- Does not deprecate Pure/NetApp native mapping — default under auto.
- Does not change the gRPC .proto schema.
- Does not add NVMe-oF support (SCSI-only NAA flow).

## Complexity Tracking

No constitution violations to justify.
