# Cinder Fallback for Storage-Accelerated-Copy LUN Mapping

## Context

vJailbreak's Storage-Accelerated-Copy (XCOPY) path needs to map a target LUN to the ESXi host before vmkfstools can offload the clone to the array. Today every vendor implements this mapping in `pkg/vpwned/sdk/storage/<vendor>/` — host discovery (matching ESXi WWPNs/IQNs to array host objects), initiator group management, and LUN attach/detach. Pure (`pkg/vpwned/sdk/storage/pure/pure.go:159-273`) and NetApp (`pkg/vpwned/sdk/storage/netapp/netapp.go`, ~1020 LOC) work, but the FC/SCSI mapping step is the most complex per-vendor code we maintain — every new array (HPE, Dell, IBM, …) currently requires reimplementing it from scratch.

Cinder's drivers already implement this exact operation for every supported array. We can offload **the mapping step only** to Cinder via `os-initialize_connection` / `os-terminate_connection`. The user has verified upstream that Cinder accepts an arbitrary connector dict — drivers don't care whether the host is a Nova compute node or an ESXi host.

The remaining vendor-specific work (Connect, ValidateCredentials, CreateVolume, DeleteVolume, GetVolumeInfo, NAA construction, ResolveCinderVolumeToLUN) stays vendor-native so we keep:
- **Placement guarantee**: direct `CreateVolume` on the array's REST API forces the LUN onto the same physical array as the source datastore (Cinder's scheduler does not).
- **NAA without driver assumptions**: `BuildNAA(serial)` from the array's create response works universally; we don't depend on Cinder drivers returning a serial in `connection_info` (NetApp's FC driver doesn't).

Outcome: adding a new array drops from ~1000 LOC (NetApp scale) to ~200 LOC of REST wrappers (Connect/CreateVolume/DeleteVolume/Resolve), with mapping handled by the existing Cinder driver for that array.

## Shape of the change

```
                         copyDiskViaStorageAcceleratedCopy (per-disk)
                                       │
                                       ▼
           ┌──────────────────────────────────────────────┐
           │  1. Load ArrayCreds for this disk's          │
           │     datastore (was inline in                 │
           │     manageVolumeToCinder)                    │
           │  2. selectMapper(provider, mode, osClients)  │
           └────────────────────────┬─────────────────────┘
                                    ▼
                        ┌───────────────────────┐
                        │ MappingMode = "auto"  │
                        │ + provider implements │
                        │   VendorMapper?       │
                        └───────────┬───────────┘
                                    ▼
                  ┌─────────────────┴─────────────────┐
                  │                                   │
              YES │                               NO  │
                  ▼                                   ▼
         ┌─────────────────┐               ┌────────────────────┐
         │ vendor-native   │               │   CinderMapper     │
         │ (Pure / NetApp) │               │ (REST wrapper over │
         │                 │               │  os-initialize_    │
         │ unchanged code  │               │  connection)       │
         └────────┬────────┘               └──────────┬─────────┘
                  │                                   │
                  │  Mapper interface (same 4 methods)│
                  │                                   │
                  ▼                                   ▼
            ESXi sees LUN via NAA → vmkfstools XCOPY runs
```

## Design

### Interface split (backward compatible)

In `pkg/vpwned/sdk/storage/storage.go`, split today's monolithic `StorageProvider` into:

- `StorageProvider` (Core, **unchanged name**) — Connect, Disconnect, ValidateCredentials, CreateVolume, DeleteVolume, GetVolumeInfo, ListAllVolumes, GetAllVolumeNAAs, ResolveCinderVolumeToLUN, WhoAmI. Required for every vendor.
- `VendorMapper` (new, **optional**) — CreateOrUpdateInitiatorGroup, MapVolumeToGroup, UnmapVolumeFromGroup, GetMappedGroups. Vendors with native mapping implement this; new "Cinder-only" vendors skip it.

```go
type VendorMapper interface {
    CreateOrUpdateInitiatorGroup(name string, hbaIDs []string) (MappingContext, error)
    MapVolumeToGroup(name string, vol Volume, ctx MappingContext) (Volume, error)
    UnmapVolumeFromGroup(name string, vol Volume, ctx MappingContext) error
    GetMappedGroups(vol Volume, ctx MappingContext) ([]string, error)
}
```

Pure and NetApp already implement the four methods on their concrete types — no edits needed in `pure.go` / `netapp.go`. They satisfy `VendorMapper` automatically.

### Mapper selection (auto + override flag)

Add `MappingMode` to `ArrayCredsSpec` in `k8s/migration/api/v1alpha1/arraycreds_types.go`:

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

Selector logic (lives in v2v-helper):

| MappingMode    | Provider satisfies VendorMapper? | Choice          |
|----------------|----------------------------------|-----------------|
| "" / "auto"    | yes                              | vendor-native   |
| "" / "auto"    | no                               | CinderMapper    |
| "native"       | yes                              | vendor-native   |
| "native"       | no                               | error at validate |
| "cinder"       | any                              | CinderMapper    |

### CinderMapper

New file `pkg/vpwned/sdk/storage/cinder/mapper.go` (~200 LOC). Lives in the storage SDK because vpwned already imports `gophercloud/v2`.

```go
type CinderMapper struct {
    Client CinderActionClient   // small interface, see below
    Host   string                // optional; static identifier in connector["host"]
}

// CinderActionClient is the minimal Cinder surface CinderMapper needs.
// v2v-helper's *utils.OpenStackClients implements it (see "Plumbing" below).
type CinderActionClient interface {
    InitializeVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) (map[string]any, error)
    TerminateVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) error
}

// CreateOrUpdateInitiatorGroup builds the os-brick connector dict from the
// ESXi HBA identifiers and stashes it in the MappingContext. No array call.
func (m *CinderMapper) CreateOrUpdateInitiatorGroup(_ string, hbaIDs []string) (storage.MappingContext, error) {
    connector, err := buildConnectorFromHBAs(hbaIDs, m.Host)
    if err != nil {
        return nil, err
    }
    return storage.MappingContext{"connector": connector}, nil
}

func (m *CinderMapper) MapVolumeToGroup(_ string, vol storage.Volume, mctx storage.MappingContext) (storage.Volume, error) {
    connector, _ := mctx["connector"].(map[string]any)
    _, err := m.Client.InitializeVolumeConnection(context.Background(), vol.OpenstackVol.ID, connector)
    return vol, err
}

func (m *CinderMapper) UnmapVolumeFromGroup(_ string, vol storage.Volume, mctx storage.MappingContext) error {
    connector, _ := mctx["connector"].(map[string]any)
    return m.Client.TerminateVolumeConnection(context.Background(), vol.OpenstackVol.ID, connector)
}

func (m *CinderMapper) GetMappedGroups(_ storage.Volume, _ storage.MappingContext) ([]string, error) {
    return nil, nil
}
```

`buildConnectorFromHBAs` consumes the same `[]string` returned by `esxiClient.GetAllHostAdapters()` (`v2v-helper/esxi-ssh/disk_ops.go:740`, format `"iqn..."` or `"fc.WWNN:WWPN"` lowercase) and emits an os-brick connector dict:

- All entries iSCSI → `{initiator, host, platform: "x86_64", os_type: "esxi", multipath: true}` (use first iqn as initiator)
- All entries FC → `{wwpns: [...], wwnns: [...], host, platform: "x86_64", os_type: "esxi", multipath: true}` (parse via `fcutil.ParseFCUID` at `pkg/vpwned/sdk/storage/fcutil/fcutil.go:22`; emit colon-stripped uppercase via `fcutil.StripWWNFormatting`).
- Mixed → emit both sets in one connector; Cinder drivers ignore unused keys.
- `host` defaults to `"vjailbreak-xcopy"` when caller passes `""`.

### Plumbing the BlockStorageClient through v2v-helper

`migobj.Openstackclients` is typed as the `openstack.OpenstackOperations` interface (`v2v-helper/openstack/openstackops.go:31`). The interface today exposes `ManageExistingVolume(...)` which posts to Cinder's manage endpoint. Add two siblings:

```go
// In v2v-helper/openstack/openstackops.go OpenstackOperations interface:
InitializeVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) (map[string]any, error)
TerminateVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) error
```

Implement both on `*utils.OpenStackClients` (`v2v-helper/pkg/utils/openstackopsutils.go`) using the same gophercloud pattern as `ManageExistingVolume` at `openstackopsutils.go:957-1014`:

```go
func (osclient *OpenStackClients) InitializeVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) (map[string]any, error) {
    body := map[string]any{
        "os-initialize_connection": map[string]any{"connector": connector},
    }
    var result struct {
        ConnectionInfo map[string]any `json:"connection_info"`
    }
    _, err := osclient.BlockStorageClient.Post(
        ctx,
        osclient.BlockStorageClient.ServiceURL("volumes", volumeID, "action"),
        body, &result,
        &gophercloud.RequestOpts{OkCodes: []int{200}},
    )
    if err != nil {
        return nil, fmt.Errorf("os-initialize_connection failed for volume %s: %w", volumeID, err)
    }
    return result.ConnectionInfo, nil
}

func (osclient *OpenStackClients) TerminateVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) error {
    body := map[string]any{
        "os-terminate_connection": map[string]any{"connector": connector},
    }
    _, err := osclient.BlockStorageClient.Post(
        ctx,
        osclient.BlockStorageClient.ServiceURL("volumes", volumeID, "action"),
        body, nil,
        &gophercloud.RequestOpts{OkCodes: []int{202}},
    )
    if err != nil {
        return fmt.Errorf("os-terminate_connection failed for volume %s: %w", volumeID, err)
    }
    return nil
}
```

`*utils.OpenStackClients` then satisfies `cinder.CinderActionClient`. Run `mockgen` regeneration for the OpenstackOperations interface (existing `go.uber.org/mock` setup; see `v2v-helper/migrate/migrate_test.go` mock usage).

### v2v-helper integration in vaai_copy.go

Refactor `copyDiskViaStorageAcceleratedCopy` (`v2v-helper/migrate/vaai_copy.go:150-296`):

1. **Load ArrayCreds before any mapping calls.** The current `manageVolumeToCinder` (`vaai_copy.go:384-457`) loads `arrayCreds.Spec` inline at line 406, but we now need it before `CreateOrUpdateInitiatorGroup` at line 175 (to know the MappingMode). Hoist the lookup: extract a small helper `resolveArrayCreds(ctx, vmDisk) (vjailbreakv1alpha1.ArrayCreds, error)` containing lines 388-409, call it once at the top of `copyDiskViaStorageAcceleratedCopy`, and pass the resolved object into `manageVolumeToCinder` as a new parameter so we don't double-fetch.

2. **Define the local Mapper interface and selector.** In a new `v2v-helper/migrate/mapper.go`:

   ```go
   type Mapper interface {
       CreateOrUpdateInitiatorGroup(name string, hbaIDs []string) (storage.MappingContext, error)
       MapVolumeToGroup(name string, vol storage.Volume, ctx storage.MappingContext) (storage.Volume, error)
       UnmapVolumeFromGroup(name string, vol storage.Volume, ctx storage.MappingContext) error
   }

   func selectMapper(provider storage.StorageProvider, osClients openstack.OpenstackOperations, mode string) (Mapper, error) {
       vendor, ok := provider.(storage.VendorMapper)
       switch mode {
       case "", "auto":
           if ok { return vendor, nil }
           return &cinder.CinderMapper{Client: osClients}, nil
       case "native":
           if ok { return vendor, nil }
           return nil, fmt.Errorf("MappingMode=native but provider %s has no vendor-native mapper", provider.WhoAmI())
       case "cinder":
           return &cinder.CinderMapper{Client: osClients}, nil
       default:
           return nil, fmt.Errorf("unknown MappingMode: %s", mode)
       }
   }
   ```

3. **Replace direct `migobj.StorageProvider.<map-method>` calls.** Specifically:
   - `vaai_copy.go:175` `CreateOrUpdateInitiatorGroup` → `mapper.CreateOrUpdateInitiatorGroup`
   - `vaai_copy.go:229` `MapVolumeToGroup` → `mapper.MapVolumeToGroup`
   - `vaai_copy.go:245` `UnmapVolumeFromGroup` (in defer) → `mapper.UnmapVolumeFromGroup`

   The Pure/NetApp paths get exactly the same behavior because the selector returns the existing provider (which already implements the four methods).

4. **Order of operations preserved.** `manageVolumeToCinder` still happens at line 198 (between `CreateVolume` and `MapVolumeToGroup`), so the Cinder volume_id is available before `MapVolumeToGroup` for both native and cinder paths. The CinderMapper relies on `vol.OpenstackVol.ID` which is set in the `targetVol := storage.Volume{...}` literal at lines 221-228.

### gRPC surface

`pkg/vpwned/server/storage.go` exposes `CreateOrUpdateInitiatorGroup` / `MapVolumeToGroup` / `UnmapVolumeFromGroup` / `GetMappedGroups` as RPCs (lines 73-296). Update each to type-assert the provider to `storagesdk.VendorMapper`; if the assertion fails, return `Success: false, Message: "vendor X does not implement native mapping; use Cinder fallback"` (or the equivalent for the non-Success-flag RPCs). UI preflight calls keep working for Pure/NetApp; new vendors fail loud and the UI can fall back to skipping the preflight or display a more graceful message.

### ArrayCreds reconciler validation

In `k8s/migration/internal/controller/arraycreds_controller.go` at `validateArrayCredentials` (~line 289): after a successful Connect, if `spec.MappingMode == "native"`, type-assert the provider to `storagesdk.VendorMapper`; if it fails, set status `Failed` with message `"MappingMode=native unsupported by vendor <type>"` and short-circuit. `auto`/`cinder`/empty modes pass through.

## Critical files

| File | Change |
|------|--------|
| `pkg/vpwned/sdk/storage/storage.go` | Remove the four mapping methods from the `StorageProvider` interface; add new `VendorMapper` interface with the same four signatures. Keep `MappingContext`, `Volume`, helper funcs unchanged. |
| `pkg/vpwned/sdk/storage/cinder/mapper.go` | **New.** ~150-200 LOC. `CinderMapper`, `CinderActionClient` interface, `buildConnectorFromHBAs`. |
| `pkg/vpwned/sdk/storage/cinder/mapper_test.go` | **New.** Connector building unit tests (FC-only, iSCSI-only, mixed, malformed `fc.X:Y`, empty input). |
| `pkg/vpwned/sdk/storage/pure/pure.go` | No changes. (Already satisfies both `StorageProvider` and `VendorMapper`.) |
| `pkg/vpwned/sdk/storage/netapp/netapp.go` | No changes. |
| `pkg/vpwned/sdk/storage/fcutil/fcutil.go` | Re-use `ParseFCUID`, `StripWWNFormatting`. No changes. |
| `pkg/vpwned/server/storage.go` | In each mapping RPC, type-assert provider to `storagesdk.VendorMapper`; return failure response when not satisfied. |
| `k8s/migration/api/v1alpha1/arraycreds_types.go` | Add `MappingMode` enum field to `ArrayCredsSpec`. |
| `k8s/migration/internal/controller/arraycreds_controller.go` | At ~line 320 (after `ValidateCredentials`), enforce `MappingMode == "native"` against `VendorMapper` type-assertion. |
| `k8s/migration/api/v1alpha1/zz_generated.deepcopy.go` | Regenerated by `make generate`. |
| `k8s/migration/config/crd/bases/vjailbreak.k8s.pf9.io_arraycreds.yaml` | Regenerated by `make manifests`. |
| `deploy/installer.yaml` | Regenerated by `make generate-manifests` from repo root. |
| `v2v-helper/openstack/openstackops.go` | Extend `OpenstackOperations` interface with `InitializeVolumeConnection` / `TerminateVolumeConnection`. |
| `v2v-helper/pkg/utils/openstackopsutils.go` | Implement the two new methods on `*OpenStackClients` using the `BlockStorageClient.Post` pattern from `ManageExistingVolume` at line 957. |
| `v2v-helper/migrate/migrate.go` | Pass `MappingMode` through (read from ArrayCreds when constructing the mapper — via `vaai_copy.go`, no field on Migrate needed). |
| `v2v-helper/migrate/mapper.go` | **New.** Local `Mapper` interface + `selectMapper(provider, osClients, mode)`. |
| `v2v-helper/migrate/vaai_copy.go` | Hoist ArrayCreds load to the top of `copyDiskViaStorageAcceleratedCopy`; thread it (and its `MappingMode`) into both `selectMapper` and `manageVolumeToCinder` (new signature: `manageVolumeToCinder(ctx, volumeName, vmDisk, arrayCreds)` to avoid re-fetching). Replace 3 mapping calls with the selected mapper. |
| `v2v-helper/migrate/mocks/*` | Regenerate the OpenstackOperations mock (existing `go.uber.org/mock`). |
| `v2v-helper/migrate/mapper_test.go` | **New.** Selector tests covering each `MappingMode` × provider-shape matrix. |

### Existing utilities to reuse

- `BlockStorageClient.Post(ctx, ServiceURL(...), body, &result, opts)` pattern from `ManageExistingVolume` (`v2v-helper/pkg/utils/openstackopsutils.go:976`). Same shape for `os-initialize_connection`/`os-terminate_connection`.
- `fcutil.ParseFCUID` (`pkg/vpwned/sdk/storage/fcutil/fcutil.go:22`) and `fcutil.StripWWNFormatting` (`fcutil.go:44`) for converting `fc.WWNN:WWPN` HBA strings to connector wwpns/wwnns (Cinder os-brick connectors typically expect colon-stripped uppercase hex).
- `esxiClient.GetAllHostAdapters()` (`v2v-helper/esxi-ssh/disk_ops.go:740`) — already returns the HBA list in the format we need.
- `migobj.Openstackclients` (interface at `v2v-helper/openstack/openstackops.go:31`) — extend with two methods following `ManageExistingVolume`.
- `k8sutils.GetArrayCreds` / `GetArrayCredsMapping` (`v2v-helper/pkg/k8sutils/k8sutils.go:258`, `:266`) — reuse for the hoisted ArrayCreds load.

## Verification

### Unit
- `cd pkg/vpwned && go test ./sdk/storage/cinder/...` — connector building (FC, iSCSI, mixed, malformed input) passes.
- `cd v2v-helper && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./migrate/... -run TestSelectMapper` — selector returns the right mapper for every `MappingMode` × provider-shape combination, including the `"native"` failure path.
- `cd k8s/migration && make test` — controller validation rejects `MappingMode=native` against a non-mapper provider.

### CRD generation
- `cd k8s/migration && make generate` after editing `arraycreds_types.go`. Confirm `zz_generated.deepcopy.go` and `config/crd/bases/vjailbreak.k8s.pf9.io_arraycreds.yaml` reflect the new field.
- `make generate-manifests` from repo root regenerates `deploy/installer.yaml`.

### End-to-end
1. **Pure (auto → native)**: existing migration test against Pure FlashArray. Logs must show `selectMapper: using vendor-native (pure)`. No Cinder `os-initialize_connection` calls hit the API.
2. **Pure (cinder override)**: same array, `mappingMode: cinder` on ArrayCreds. Logs show `selectMapper: using cinder fallback`. Migration succeeds. `cinder show <vol>` reports the connection; Pure UI shows the host connection.
3. **HPE 3PAR / Dell (auto → cinder)**: configure ArrayCreds for an array with only Connect/CreateVolume/Delete/Resolve implemented (no `VendorMapper`). Migration succeeds end-to-end; vmkfstools clone runs at array speed.
4. **Native required, missing**: `mappingMode: native` on the HPE ArrayCreds. Reconciler marks the CR `Failed` with `"MappingMode=native unsupported by vendor hpe"`; no migration runs.

### Logs to grep
- v2v-helper: existing `Cleaning up volume mappings` should still appear in both paths. New: `selectMapper: using vendor-native (<vendor>)` / `selectMapper: using cinder fallback (<vendor>)`.
- Cinder service log: `os-initialize_connection` calls land on the right backend driver and return success.

## What this plan deliberately does NOT do

- Does not touch CreateVolume / DeleteVolume / NAA generation paths — placement guarantee preserved exactly.
- Does not modify `manageVolumeToCinder`'s logic (just its signature, to accept pre-loaded ArrayCreds and avoid the duplicate fetch).
- Does not touch ESXi-side code (`esxi-ssh/`) — NAA-based device lookup keeps working because we still get the NAA from the vendor's `CreateVolume` response, never from Cinder.
- Does not deprecate Pure/NetApp native mapping — they keep their fast path and remain the default under `auto`.
- Does not change the gRPC `.proto` schema. Mapping RPCs stay; the server-side handler now returns a failure response for vendors without `VendorMapper` instead of crashing on a missing method.