# Quickstart: Cinder Fallback for Storage-Accelerated-Copy LUN Mapping

**Feature**: [spec.md](./spec.md) | **Date**: 2026-07-06

## Prerequisites

- The storage array's Cinder backend is configured in cinder.conf and its `cinder-volume` service is up (already required by the existing Cinder-manage step — the fallback adds nothing new).
- ArrayCreds validated (`status.phase: Validated`) with `openstackMapping` (volume type + backend) set.
- ESXi host has SAN connectivity to the array (it must — the source datastore lives there).

## 1. Default behavior (auto)

Do nothing. `mappingMode` unset ≡ `auto`:

- Pure / NetApp → vendor-native mapping (existing fast path, unchanged).
- Any vendor whose provider has no native mapping → Cinder fallback, automatically.

```bash
kubectl -n migration-system get arraycreds <name> -o jsonpath='{.spec.mappingMode}'  # empty = auto
```

## 2. Force the Cinder path on Pure/NetApp (regression gate)

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: ArrayCreds
metadata:
  name: pure-01
  namespace: migration-system
spec:
  vendorType: pure
  mappingMode: cinder
  # ... existing secretRef / openstackMapping unchanged
```

Run a normal Storage-Accelerated-Copy migration, then:

```bash
# v2v-helper pod logs
kubectl -n migration-system logs <migration>-v2v-helper | grep selectMapper
#   selectMapper: using cinder fallback (pure)
kubectl -n migration-system logs <migration>-v2v-helper | grep "Cinder connector"
#   full connector dict (keep for manual cleanup if the pod ever dies mid-copy)

# cinder-volume log on the PCD side: os-initialize_connection / os-terminate_connection
# array side (Pure UI): host shows a connection to volume-<cinder-id>-cinder during the copy window
```

Do **not** use `cinder show` to verify — `os-initialize_connection` creates no attachment record; the volume stays `available` the whole time.

## 3. Require vendor-native mapping (guardrail)

```yaml
spec:
  vendorType: <vendor-without-native-mapping>
  mappingMode: native
```

The reconciler marks the CR Failed before any migration:

```bash
kubectl -n migration-system get arraycreds <name> -o jsonpath='{.status.arrayValidationMessage}'
#   MappingMode=native unsupported by vendor <type>
```

## 4. Onboarding a brand-new array vendor (the point of this feature)

1. Implement the core provider only (~200 LOC): `Connect`, `Disconnect`, `ValidateCredentials`, `CreateVolume`, `DeleteVolume`, `GetVolumeInfo`, `ListAllVolumes`, `GetAllVolumeNAAs`, `ResolveCinderVolumeToLUN`, `WhoAmI` in `pkg/vpwned/sdk/storage/<vendor>/`. No mapping methods.
2. Register it in `pkg/vpwned/sdk/storage/providers/providers.go` (blank import).
3. Ensure the array's Cinder backend exists in cinder.conf; set `openstackMapping` on the ArrayCreds.
4. Leave `mappingMode` unset. Mapping is handled by the array's Cinder driver.

### Per-backend knobs worth setting

| Backend | Knob | Why |
|---|---|---|
| HPE 3PAR/Alletra | volume-type extra spec `hpe3par:persona` (11 = VMware) | Auto-created hosts otherwise get Generic-ALUA persona |
| Pure | `pure_host_personality = esxi` | Correct host personality if the driver ever creates the host |
| NetApp | igroup `os_type` | Only relevant if using the Cinder path on NetApp |
| Any FC + Zone Manager | zoning policy | initialize_connection will zone every WWPN in the connector |
| iSCSI + CHAP | pre-register ESXi initiator | Fresh driver-created host entries may get generated CHAP creds |

## 5. Developer verification

```bash
# Unit — connector building + mapper plumbing (pure Go)
cd pkg/vpwned && go test ./sdk/storage/cinder/...

# Unit — selector matrix (execute on Linux/Docker; CGO module)
cd v2v-helper && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./migrate/... -run 'TestSelectMapper|TestVendorMapperAdapter'
# or: make test-v2v-helper

# Controller — native-mode enforcement
cd k8s/migration && make test

# Regeneration after the CRD change
cd k8s/migration && make generate && make manifests   # deepcopy no-op + CRD YAML
cd v2v-helper && go generate ./openstack/...          # OpenstackOperations mock
make generate-manifests                               # deploy/installer.yaml (repo root; needs built images)
```

## Troubleshooting

| Symptom | Check |
|---|---|
| `selectMapper: MappingMode=native but provider ... has no vendor-native mapper` | Vendor lacks native mapping; use `auto` or `cinder` |
| `no usable iSCSI or FC initiators` | `esxcli storage core adapter list` on the ESXi host; adapters must be link-up/online |
| initialize succeeds but device never appears | Same-array placement (datastore ↔ ArrayCreds mapping), zoning, and that the NAA in the log matches the created LUN |
| Leaked export after v2v-helper crash | Grep pod log for `Cinder connector:`; replay `os-terminate_connection` with that connector via cinderclient |
