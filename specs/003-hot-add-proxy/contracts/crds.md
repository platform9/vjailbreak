# CRD Contracts: Hot-Add Proxy Migration

**Feature**: Hot-Add Proxy Migration  
**Branch**: `1944-hot-add-proxy`

---

## New CRD: ProxyVM

### kubectl examples

```yaml
# Register a Proxy VM
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: ProxyVM
metadata:
  name: ha-proxy-vm
  namespace: migration-system
spec:
  vmName: "dnd-ha-proxy-vm"
  vmwareCredsRef:
    name: vcenter-creds
```

```yaml
# Status after successful verification
status:
  validationStatus: Ready
  validationMessage: "All components verified. disk.EnableUUID=True."
  ipAddress: "10.96.2.75"
  attachedDiskCount: 0
  lastValidationTime: "2026-05-19T12:00:00Z"
  componentsVerified:
    - name: lsblk
      present: true
    - name: qemu-nbd
      present: true
    - name: nbdkit
      present: true
    - name: sshd
      present: true
```

```yaml
# Status after failed verification
status:
  validationStatus: VerificationFailed
  validationMessage: "Missing required component: qemu-nbd. Install with: apt-get install qemu-utils"
  componentsVerified:
    - name: lsblk
      present: true
    - name: qemu-nbd
      present: false
      message: "qemu-nbd not found in PATH. Install with: apt-get install qemu-utils"
    - name: nbdkit
      present: true
    - name: sshd
      present: true
```

### kubectl columns (printer columns)

| Column | JSON Path | Description |
|--------|-----------|-------------|
| NAME | `.metadata.name` | Resource name |
| VM-NAME | `.spec.vmName` | vCenter VM display name |
| STATUS | `.status.validationStatus` | Ready / VerificationFailed / Pending / Verifying |
| IP | `.status.ipAddress` | Discovered IP address |
| ATTACHED-DISKS | `.status.attachedDiskCount` | Current disk attachment count |
| AGE | `.metadata.creationTimestamp` | Age |

---

## Modified CRD: MigrationTemplate

### Hot-Add example

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: MigrationTemplate
metadata:
  name: template-hotadd
  namespace: migration-system
spec:
  sourceEnvironment:
    datacenter: prison
    vmwareCredsRef:
      name: vcenter-creds
  destinationEnvironment:
    openstackCredsRef:
      name: pcd-creds
    targetPCDClusterName: cluster-01
  networkMapping: default-network-mapping
  storageMapping: default-storage-mapping
  storageCopyMethod: HotAdd
  proxyVMRef:
    name: ha-proxy-vm
```

### Validation rules

- When `storageCopyMethod: HotAdd`, `proxyVMRef` MUST be set.
- When `storageCopyMethod: HotAdd`, the referenced ProxyVM MUST have `status.validationStatus: Ready`.
- `proxyVMRef` is ignored (and may be absent) for `storageCopyMethod: normal` and `StorageAcceleratedCopy`.

---

## ConfigMap contract (v2v-helper)

The migrationplan_controller populates a ConfigMap consumed by the v2v-helper pod. New keys for Hot-Add:

| Key | Type | Example | Notes |
|-----|------|---------|-------|
| `STORAGE_COPY_METHOD` | string | `HotAdd` | Triggers Hot-Add code path |
| `PROXY_VM_IP` | string | `10.96.2.75` | IP discovered from ProxyVM.Status |
| `PROXY_VM_NAME` | string | `dnd-ha-proxy-vm` | vCenter display name — used to locate VM via govmomi |
| `PROXY_VM_K8S_NAME` | string | `ha-proxy-vm` | Kubernetes resource name (`metadata.name`) — used by v2v-helper to patch `status.attachedDiskCount` |

Existing keys are unchanged. The `STORAGE_COPY_METHOD` = `"HotAdd"` value is exclusive with `"StorageAcceleratedCopy"` — no overlap.

---

## REST API (UI ↔ backend)

The UI communicates with the Kubernetes API server via the existing vjailbreak REST proxy. ProxyVM resources follow the same CRUD pattern as other CRDs.

| Operation | Path | Body |
|-----------|------|------|
| List ProxyVMs | `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms` | — |
| Create ProxyVM | `POST /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms` | ProxyVM CR |
| Get ProxyVM | `GET /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms/{name}` | — |
| Delete ProxyVM | `DELETE /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms/{name}` | — |

No custom verbs needed. The controller reacts to Create events to start verification.

### Retry Verification API

Trigger re-verification for a ProxyVM in `VerificationFailed` state without deleting the resource:

| Operation | Path | Content-Type | Body |
|-----------|------|--------------|------|
| Retry verification | `PATCH /apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms/{name}` | `application/merge-patch+json` | `{"metadata":{"annotations":{"vjailbreak.k8s.pf9.io/retry-at":"<ISO-8601-timestamp>"}}}` |

The controller watches for annotation changes and re-runs the full verification flow when this annotation is set or updated.
