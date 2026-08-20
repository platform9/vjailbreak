# Contracts: Interfaces & Wire Formats

**Feature**: [spec.md](../spec.md) | **Date**: 2026-07-06

## 1. Storage SDK — `pkg/vpwned/sdk/storage/storage.go`

### StorageProvider (core, required — post-split)

```go
type StorageProvider interface {
    Connect(ctx context.Context, accessInfo StorageAccessInfo) error
    Disconnect() error
    ValidateCredentials(ctx context.Context) error
    CreateVolume(volumeName string, size int64) (Volume, error)
    DeleteVolume(volumeName string) error
    GetVolumeInfo(volumeName string) (VolumeInfo, error)
    ListAllVolumes() ([]VolumeInfo, error)
    GetAllVolumeNAAs() ([]string, error)
    ResolveCinderVolumeToLUN(volumeName string) (Volume, error)
    WhoAmI() string
}
```

### VendorMapper (optional — signatures identical to the pre-split methods)

```go
type VendorMapper interface {
    CreateOrUpdateInitiatorGroup(initiatorGroupName string, hbaIdentifiers []string) (MappingContext, error)
    MapVolumeToGroup(initiatorGroupName string, targetVolume Volume, context MappingContext) (Volume, error)
    UnmapVolumeFromGroup(initiatorGroupName string, targetVolume Volume, context MappingContext) error
    GetMappedGroups(targetVolume Volume, context MappingContext) ([]string, error)
}
```

**Contract**: `*PureStorageProvider` and `*NetAppStorageProvider` satisfy both interfaces with zero source changes. New "Cinder-only" vendors implement `StorageProvider` only (and register in `pkg/vpwned/sdk/storage/providers/providers.go`).

## 2. CinderMapper — `pkg/vpwned/sdk/storage/cinder/mapper.go`

```go
// Minimal Cinder surface; *utils.OpenStackClients (v2v-helper) satisfies it structurally.
type CinderActionClient interface {
    InitializeVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) (map[string]any, error)
    TerminateVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) error
}

type CinderMapper struct {
    Client CinderActionClient
    Host   string // connector["host"]; default DefaultConnectorHost ("vjailbreak-xcopy")
    IP     string // optional connector["ip"] — ESXi host IP
}

func (m *CinderMapper) CreateOrUpdateInitiatorGroup(ctx context.Context, name string, hbaIdentifiers []string) (storage.MappingContext, error)
func (m *CinderMapper) MapVolumeToGroup(ctx context.Context, name string, vol storage.Volume, mctx storage.MappingContext) (storage.Volume, error)
func (m *CinderMapper) UnmapVolumeFromGroup(ctx context.Context, name string, vol storage.Volume, mctx storage.MappingContext) error

func BuildConnectorFromHBAs(hbaIdentifiers []string, host, ip string) (map[string]any, error)
```

**Contracts**:
- `CreateOrUpdateInitiatorGroup` performs no array/Cinder call; it builds the connector and returns `MappingContext{"connector": connector}`. The `name` parameter is ignored (kept for interface symmetry).
- `MapVolumeToGroup`/`UnmapVolumeFromGroup` require `vol.OpenstackVol.ID != ""` and a `connector` entry in the MappingContext; both error descriptively otherwise. The identical connector is sent on both calls.
- Methods are ctx-aware (deliberately different from `VendorMapper` — CinderMapper does not satisfy `VendorMapper` and is never registered as a provider).

### Connector dict examples

iSCSI-only host:

```json
{"initiator": "iqn.1998-01.com.vmware:esx01-4aa9d624", "host": "vjailbreak-10-4-2-17",
 "ip": "10.4.2.17", "platform": "x86_64", "os_type": "linux", "multipath": true}
```

FC-only host (`fc.20000025b510a086:21000025b510a086` → wwnn/wwpn, lowercase, colon-stripped):

```json
{"wwpns": ["21000025b510a086", "21000025b510a087"],
 "wwnns": ["20000025b510a086", "20000025b510a087"],
 "host": "vjailbreak-10-4-2-17", "ip": "10.4.2.17",
 "platform": "x86_64", "os_type": "linux", "multipath": true}
```

Mixed transport: union of both examples' keys. Drivers ignore keys for transports they do not serve.

## 3. v2v-helper Mapper seam — `v2v-helper/migrate/mapper.go`

```go
type Mapper interface {
    CreateOrUpdateInitiatorGroup(ctx context.Context, initiatorGroupName string, hbaIdentifiers []string) (storage.MappingContext, error)
    MapVolumeToGroup(ctx context.Context, initiatorGroupName string, targetVolume storage.Volume, mappingContext storage.MappingContext) (storage.Volume, error)
    UnmapVolumeFromGroup(ctx context.Context, initiatorGroupName string, targetVolume storage.Volume, mappingContext storage.MappingContext) error
}

// selectMapper returns the mapper, a log description ("vendor-native (pure)" /
// "cinder fallback (pure)"), or an error for native-without-VendorMapper and
// unknown modes. esxiHostIP feeds the per-ESXi connector host and ip fields.
func selectMapper(provider storage.StorageProvider, osClients openstack.OpenstackOperations,
    mode string, esxiHostIP string) (Mapper, string, error)
```

`vendorMapperAdapter` bridges `storage.VendorMapper` to `Mapper` by dropping ctx. `GetMappedGroups` is intentionally absent — the XCOPY flow never calls it.

## 4. OpenStack client — `v2v-helper/openstack/openstackops.go`

Two additions to `OpenstackOperations` (implemented on `*utils.OpenStackClients`, mocked via golang/mock `go generate ./openstack/...`):

```go
InitializeVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) (map[string]any, error)
TerminateVolumeConnection(ctx context.Context, volumeID string, connector map[string]any) error
```

### Cinder wire contract

`POST /v3/{project_id}/volumes/{volume_id}/action`

| Action | Request body | Success | Response |
|---|---|---|---|
| os-initialize_connection | `{"os-initialize_connection": {"connector": {…}}}` | 200 | `{"connection_info": {…}}` (returned but unused — ESXi discovers by NAA) |
| os-terminate_connection | `{"os-terminate_connection": {"connector": {…}}}` | 202 | empty |

No microversion header required. No attachment record is created; volume status remains `available`.

## 5. gRPC — `pkg/vpwned/server/storage.go` (proto unchanged)

| RPC | Provider lacks VendorMapper → |
|---|---|
| CreateOrUpdateInitiatorGroup | `{Success: false, Message: "vendor <type> does not implement native mapping; use the Cinder fallback"}` |
| MapVolumeToGroup | same shape |
| UnmapVolumeFromGroup | same shape |
| GetMappedGroups | Go error with the same message (response has no Success flag) |

Assertion happens immediately after `NewStorageProvider`, before `Connect` (fail fast, no array round-trip). Vendors with native mapping: behavior unchanged.

## 6. ArrayCreds CRD — `spec.mappingMode`

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: ArrayCreds
spec:
  vendorType: pure
  mappingMode: cinder   # optional: auto (default) | native | cinder
```

Admission: enum-validated. Reconciler: `native` + non-mapper vendor ⇒ status Failed, message `MappingMode=native unsupported by vendor <type>`.
