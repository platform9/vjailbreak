# vJailbreak Data Copy Methods

Three mechanisms move disk bytes from VMware to PCD. They are selected independently of the hot/cold/mock/periodic-sync scheduling covered in [migration-lifecycle.md](migration-lifecycle.md) — this file is about the wire-level transport.

## Comparison

| Method | Transport | Speed | Requires shared array? | VM state | Datastore types |
|---|---|---|---|---|---|
| **Normal (NFC)** | VMware NFC protocol over the network → NBD | ~1 Gbps per VMDK (hard cap) | No | Hot or cold | Any |
| **Storage-Accelerated (XCOPY)** | Array-level XCOPY, offloaded entirely to the storage array | 10–100x NFC (1 TB disk: ~5–30 min vs ~2.5 hrs) | **Yes** — source and destination must be the SAME physical array | Cold only | VMFS only |
| **Hot-Add Proxy** | Snapshot disks attached to a proxy VM, streamed via NBD/`qemu-nbd`/`nbdcopy` | Faster than NFC (no protocol middleman); slower than XCOPY | No | Cold only (source powered off before disk attach) | Any (NFS, VMFS, vSAN) |

## Normal (NFC) — Baseline

The traditional path: VMware exposes disk blocks over its NFC port, `v2v-helper` reads them via NBD and streams into a Cinder volume. This is capped at roughly 1 Gbps per VMDK by design — the reason the other two methods exist. No special prerequisites beyond VDDK (see [architecture.md](architecture.md)).

**DNS/hostname troubleshooting**: if migration fails during `nbdcopy` with DNS resolution errors in the debug log, the cause is usually the ESXi hostname not being resolvable from the vJailbreak VM. The fix is to add ESXi host entries to `/etc/hosts` on the vJailbreak VM — but the entries must cover **every** ESXi host in the cluster, not just vCenter, or the same failure recurs against a different host. This is a config change to the vJailbreak VM itself, so confirm it with the user or document it as the fix rather than applying it silently.

## Storage-Accelerated Copy (XCOPY)

### Supported Arrays
**Pure Storage FlashArray**, **NetApp ONTAP**, and **Hitachi Vantara** today. Storage vendor
code lives at `pkg/vpwned/sdk/storage/<vendor>/` (not `v2v-helper/pkg/storage/`) behind a
pluggable `StorageProvider` interface. A vendor without native host/igroup-mapping code
(Vantara today) automatically falls back to a generic Cinder-based mapper
(`pkg/vpwned/sdk/storage/cinder/mapper.go`, using Cinder's `os-initialize_connection`/
`os-terminate_connection`) — any future vendor gets SAM support through that path without
vendor-specific mapping code. Selectable per-`ArrayCreds` via `spec.mappingMode`
(`auto`/`native`/`cinder`).

### Prerequisites
- Both the VMware datastore and the PCD Cinder backend must be configured against the **same physical array** — this is a hard requirement, not just "compatible arrays." SVMs/pools may still differ between source and destination (NetApp, Vantara).
- Datastores must be **VMFS** (not NFS).
- Cinder volume types must already exist for that array backend.
- SSH enabled on **every** ESXi host in scope, with an **RSA-4096** keypair (Ed25519 is not reliably accepted by ESXi 8.x). Public key must be in `/etc/ssh/keys-root/authorized_keys` on each host.
- Network: TCP 22 (vJailbreak ↔ ESXi SSH), TCP 3260 (iSCSI) or FC fabric connectivity, TCP 443 (vJailbreak ↔ storage array management API).
- NetApp specifically: `ArrayCreds.spec.netAppConfig.{svm,flexVol}` must be set before the credential validates — not optional, no auto-pick fallback (validation fails with an explicit message if either field is empty).

### Setup CRDs: `ArrayCreds` and `ArrayCredsMapping`
Two CRDs drive SAM setup, neither in this skill's general CRD inventory:
- **`ArrayCreds`** — one object per physical array credential + (for NetApp) its fixed target SVM/FlexVol. Must reach `status.arrayValidationStatus: Succeeded` before any migration can use it.
- **`ArrayCredsMapping`** — routes `spec.mappings[].{source: <datastore name>, target: <ArrayCreds name>}`. A `MigrationTemplate` references one by name (`spec.arrayCredsMapping`); the controller blocks reconciliation for the entire plan if any `ArrayCreds` in the mapping hasn't yet reached `Succeeded`. **Known limitation**: today only one array connection is passed to each v2v-helper pod (the last entry wins); per-disk lookup only selects `mappingMode` and Cinder pool hint (see issue #2510).

### Storage Terminology Primer
- **HBA** (Host Bus Adapter) — connects an ESXi host to the storage fabric. FC-based HBAs are identified by **WWPN**/**WWNN**; SCSI/iSCSI-based HBAs (hardware or software, via a NIC configured as an iSCSI initiator) are identified by an **IQN**.
- **LUN** — a logical block device the array exposes to ESXi. A storage admin creates the LUN, then maps it to specific ESXi host identifiers (WWPN/IQN) so it becomes visible after a storage rescan.

### Pure Storage Model
Two objects: **Host** (represents one ESXi host on the array — holds its WWPN(s)/IQN) and **Host Group** (groups multiple ESXi hosts, typically one per VMware cluster, to simplify volume presentation to the whole cluster at once).

### NetApp ONTAP Model
More layered: cluster → node controllers → **SVM** (Storage Virtual Machine — a logical, isolated storage server; each SVM looks like a complete standalone array to whoever consumes it, providing per-tenant namespace isolation) → **FlexVol** (a container inside an SVM holding LUNs/NAS shares) → LUN. **Igroup** (initiator group) is NetApp's analog of Pure's host group, but scoped **per-SVM** — an igroup defined in SVM-A is invisible to SVM-B.

### The `StorageProvider` Interface
Each vendor package under `pkg/vpwned/sdk/storage/<vendor>/` implements a core interface plus an optional `VendorMapper`:

| Function group | Functions | Purpose |
|---|---|---|
| Connection | `Connect`, `Disconnect`, `ValidateCredentials` | Array session lifecycle, used when adding array credentials. |
| Volume lifecycle | `CreateVolume`, `DeleteVolume`, `GetVolumeInfo`, `ResolveCinderVolumeToLUN` | `ResolveCinderVolumeToLUN` matters because `cinder manage` **renames** the array-native volume (e.g. to `volume-<cinder-id>-cinder` on Pure) — this function re-derives the underlying LUN/serial from the Cinder volume ID after that rename. |
| Host-side mapping (`VendorMapper`, optional) | `CreateOrUpdateInitiatorGroup`, `MapVolumeToGroup`, `UnmapVolumeFromGroup`, `GetMappedGroups` | The genuinely vendor-specific, complicated part — see the asymmetry below. Vendors that don't implement this (Vantara) use the generic `CinderMapper` instead. |

### The Pure-vs-NetApp Host-Group Asymmetry (the single most important fact in this file)

**On Pure, `CreateOrUpdateInitiatorGroup` never creates a new host group — only reuses an existing one.**

Why: a Pure "host" object (representing one ESXi) can belong to **at most one** host group at a time. If vJailbreak created a new `vjailbreak-xcopy` host group and added the ESXi's host object to it, Pure would **evict** that host from whatever production host group it was already in — instantly unmapping every production volume that group was serving to that ESXi. That's a customer-caused outage, not a migration failure.

The code relies on a guarantee instead: because FC zoning and storage-admin onboarding are prerequisites for that ESXi to see the source datastore in the first place, a host object for that ESXi's WWPN/IQN is **guaranteed to already exist** on the array. So vJailbreak always looks up and reuses whatever host group that host object is already in — it never creates one on Pure.

**On NetApp, vJailbreak DOES create its own igroup per SVM when needed** (named `<name>-fcp`/`<name>-iscsi`, protocol-suffixed), because an ESXi's WWPN can legitimately belong to multiple igroups simultaneously — no eviction risk. This matters for the **cross-SVM case**: the only hard requirement is "same physical ONTAP array" — SVMs can differ between source and destination. If the source ESXi has an igroup on SVM-A, but the target volume was provisioned on SVM-B, that WWPN mapping in SVM-A's igroup is **invisible to SVM-B** (SVMs are isolated namespaces). vJailbreak creates a fresh igroup on SVM-B, adds the WWPN to it, and maps the volume there — without touching the existing SVM-A igroup at all. Mixed FC+iSCSI adapters on one host is a hard failure when a *new* igroup must be created (ONTAP igroups are single-protocol).

**If you're debugging an XCOPY mapping failure**: on Pure, ask "does a host object for this ESXi's WWPN/IQN actually exist on the array?" (if not: FC zoning or storage-admin onboarding is incomplete — this is a storage-admin task, not a vJailbreak bug). On NetApp, ask "is the target volume's SVM the same SVM the source ESXi's WWPN is already mapped in?" (if not, and igroup creation on the target SVM failed, check array-side permissions for igroup creation on **that specific SVM**, not just the source SVM).

### XCOPY Workflow
1. Validate credentials and connectivity to the array.
2. Power off the source VM (cold migration only — XCOPY doesn't support hot).
3. Per disk: resolve which `ArrayCreds` applies via `ArrayCredsMapping` (keyed by the disk's datastore) → create a new dedicated target volume on the array (named `vjb-<vmname>-<diskname>`, never reusing the source LUN) → `cinder manage` it into Cinder (renames it) → resolve the Cinder volume back to its LUN via `ResolveCinderVolumeToLUN` → create/reuse the initiator group and map the volume to the ESXi host → rescan ESXi storage adapters (`esxcli storage core adapter rescan`) → wait for the block device to appear at `/vmfs/devices/disks/<NAA-id>` → run `vmkfstools -i <src> -d rdm:<device> <rdm-descriptor>` (the actual XCOPY clone) over SSH on the ESXi host → track clone progress.
4. Unmap the volume from the initiator/host group: runs for any failure after the volume is mapped (clone failure, device-wait timeout), under its own 2-minute timeout; unmap errors are warnings only, not hard failures. Failures before mapping (manage, resolve, map step itself) skip unmap entirely.
5. Proceed to the normal convert/attach flow.

### Troubleshooting

| Symptom | Likely cause | What to check |
|---|---|---|
| SSH connection failure to ESXi | SSH disabled on the host, wrong key type (must be RSA-4096), or network/firewall blocking TCP 22 | Verify SSH is enabled on each ESXi host; confirm RSA-4096 key in `/etc/ssh/keys-root/authorized_keys` |
| Storage array connection failed | Invalid array credentials, TCP 443 blocked, or insufficient array API permissions | Test TCP 443 from vJailbreak VM to array; check `kubectl get arraycreds <name> -o yaml` for `status.arrayValidationMessage` |
| ESXi device/LUN not found after mapping | iSCSI initiator not configured, LUN masking, or VLAN/network misconfiguration | Run `esxcli storage core device list` on the ESXi host after mapping; check iSCSI initiator config |
| `cinder manage` volume failed | Volume naming mismatch, Cinder backend misconfiguration, or wrong `cinderHost` value | Check v2v-helper pod logs for the exact manage error; verify `cinderHost` in the `ArrayCreds` |
| `vmkfstools` clone failed | Datastore connectivity issue, insufficient free space on the array, or source VMDK inaccessible | Check `/tmp/vmkfstools_rdm_clone_<timestamp>.log` on the ESXi host |
| Clone progress stalled | Array performance/load issue, storage adapter problem, or network instability between ESXi and the array | Monitor ESXi storage adapter utilization; check array-side performance counters |
| `ArrayCreds` stuck at `AwaitingCredentials` / `Failed` | No `secretRef` set, wrong array hostname/credentials, or (NetApp) `spec.netAppConfig.svm`/`flexVol` not set | `kubectl get arraycreds <name> -o yaml` and read `status.arrayValidationMessage` for the specific error |
| `"SVM ... not found"` / `"FlexVol ... not found"` (NetApp) | Typo, permissions issue, or (if `status.backendTargets` is empty) backend-target discovery failed silently — the controller logs `"Failed to discover backend targets"` and treats it as non-fatal, leaving targets empty, so validation then reports not-found for an SVM that actually exists | Check `status.backendTargets` on the `ArrayCreds` object; if empty, look for `"Failed to discover backend targets"` in the controller log |
| Migration plan stuck before any `v2v-helper` pod starts | One or more `ArrayCreds` referenced by the `ArrayCredsMapping` haven't reached `Succeeded` — the entire plan is blocked, not just VMs on that datastore | `kubectl get arraycredsmapping <name> -o yaml`, then check each referenced `ArrayCreds`' `status.arrayValidationStatus` |
| XCOPY mapping fails specifically on a NetApp cross-SVM setup | Array-side permissions restrict igroup creation on the **target** SVM for this credential, even though the source SVM looks fully configured | Confirm the credential has igroup-create (not just read) permission scoped to the target SVM |

## Hot-Add Proxy

Bypasses NFC entirely using a different trick: attach the source VM's disks (as a frozen snapshot) directly to a proxy VM via vCenter's disk-attach API, then stream via NBD.

### Mechanics
1. Power off the source VM and take a snapshot.
2. Attach the frozen snapshot's disks to a dedicated Linux **Proxy VM** in the same vCenter.
3. Match each attached disk to its guest block device by **disk UUID**.
4. Expose each disk as an NBD export via `qemu-nbd` on the Proxy VM.
5. Run `nbdcopy` from the Proxy VM's NBD export straight into the destination Cinder volume.

### Prerequisites
- Proxy VM: Linux (Ubuntu/Alpine/Debian recommended), `openssh-server` + `qemu-nbd` installed, `disk.EnableUUID = TRUE` set in vCenter, root SSH enabled, VMware Tools running (needed so vJailbreak can fetch the proxy's guest IP).
- Proxy VM and source VM must be managed by the **identical** vCenter instance.

### Key Limitations
- **Cold-copy only** — the source VM is powered off before disk attachment; there is no live/hot copy of a running VM's active disks with this method.
- No shared-storage-array requirement — this is the main reason to pick it over XCOPY: works with **any** datastore type (NFS, VMFS, vSAN).

## References
- [migration-lifecycle.md](migration-lifecycle.md), [support-bundle.md](support-bundle.md)
- Docs: `/vjailbreak/concepts/storage-accelerated-copy/`, `/vjailbreak/concepts/hot-add-proxy/`
- SAM deep-dive transcript (internal): storage provider interface + Pure/NetApp initiator-group model
