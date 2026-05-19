# Data Model: Hot-Add Proxy Migration

**Feature**: Hot-Add Proxy Migration  
**Branch**: `1944-hot-add-proxy`  
**Date**: 2026-05-19

---

## New Entity: ProxyVM

**File**: `k8s/migration/api/v1alpha1/proxyvm_types.go`  
**Kind**: `ProxyVM` | **Group**: `vjailbreak.k8s.pf9.io` | **Version**: `v1alpha1`  
**Scope**: Namespaced

### Spec

```go
type ProxyVMSpec struct {
    // VMName is the display name of the Proxy VM in vCenter.
    VMName string `json:"vmName"`

    // VMwareCredsRef references the VMwareCreds used to locate and connect to the Proxy VM.
    VMwareCredsRef corev1.LocalObjectReference `json:"vmwareCredsRef"`
}
```

### Status

```go
type ProxyVMStatus struct {
    // ValidationStatus is one of: Pending, Verifying, Ready, VerificationFailed.
    // +optional
    ValidationStatus string `json:"validationStatus,omitempty"`

    // ValidationMessage contains a human-readable summary of the last validation result.
    // +optional
    ValidationMessage string `json:"validationMessage,omitempty"`

    // IPAddress is the IP address discovered from vCenter guest info.
    // +optional
    IPAddress string `json:"ipAddress,omitempty"`

    // AttachedDiskCount is the number of source-snapshot disks currently attached
    // to this Proxy VM across all active Hot-Add migrations. Max 60.
    // +optional
    AttachedDiskCount int `json:"attachedDiskCount,omitempty"`

    // ComponentsVerified lists components checked during verification.
    // +optional
    ComponentsVerified []ProxyVMComponentCheck `json:"componentsVerified,omitempty"`

    // LastValidationTime records when the last verification completed.
    // +optional
    LastValidationTime *metav1.Time `json:"lastValidationTime,omitempty"`
}

type ProxyVMComponentCheck struct {
    // Name is the component name (e.g., "lsblk", "qemu-nbd", "nbdkit", "sshd").
    Name string `json:"name"`
    // Present indicates whether the component was found.
    Present bool `json:"present"`
    // Message provides detail for missing components.
    // +optional
    Message string `json:"message,omitempty"`
}
```

### Validation Status States

| Status | Meaning |
|--------|---------|
| `Pending` | Registered but verification not yet started |
| `Verifying` | Verification in progress |
| `Ready` | All checks passed; Proxy VM can serve Hot-Add migrations |
| `VerificationFailed` | One or more checks failed; see ValidationMessage |

### Verification Checks (in order)

1. Look up VM by `VMName` in vCenter using referenced `VMwareCreds` → discover IP from guest info
2. SSH connectivity test (root@`IPAddress`, using vJailbreak's own key at `/root/.ssh/id_rsa`)
3. Component presence: `lsblk`, `qemu-nbd`, `nbdkit`, `sshd`
4. `disk.EnableUUID` check via vCenter API → if missing, set it and reboot VM, then re-verify

---

## Modified Entity: MigrationTemplate

**File**: `k8s/migration/api/v1alpha1/migrationtemplate_types.go` (existing, modified)

### Changes

```go
// StorageCopyMethod — extend enum from 2 to 3 values
// +kubebuilder:validation:Enum=normal;StorageAcceleratedCopy;HotAdd
// +kubebuilder:default=normal
StorageCopyMethod string `json:"storageCopyMethod,omitempty"`

// ProxyVMRef references the ProxyVM to use for HotAdd data copy.
// Required when StorageCopyMethod is "HotAdd".
// +optional
ProxyVMRef *corev1.LocalObjectReference `json:"proxyVMRef,omitempty"`
```

---

## Modified Entity: MigrationParams (v2v-helper)

**File**: `v2v-helper/pkg/utils/vcenterutils.go` (existing, modified)

### New Fields Added

```go
// Hot-Add Proxy params (populated only when StorageCopyMethod == "HotAdd")
ProxyVMIP   string
ProxyVMName string
```

### ConfigMap Keys Added

| Key | Value |
|-----|-------|
| `PROXY_VM_IP` | IP address of the Proxy VM |
| `PROXY_VM_NAME` | vCenter display name of the Proxy VM |

---

## New Runtime Entity: HotAddDiskTransfer (in-memory, v2v-helper)

Not persisted as a CRD — used only within `hotadd_copy.go` during a single migration run.

```go
type hotAddDiskTransfer struct {
    label       string // vCenter disk label, e.g. "Hard disk 2"
    uuid        string // normalized vCenter UUID (lowercase hex, no dashes)
    vmdkPath    string // frozen snapshot VMDK path in datastore
    blockDevice string // resolved /dev/sdX on ProxyVM
    port        int    // qemu-nbd port on ProxyVM
    destDevice  string // destination block device on vJailbreak (e.g. /dev/vdc)
}
```

---

## New Constants

**File**: `pkg/common/constants/constants.go` (existing, modified)

```go
// Copy method identifier
HotAddCopyMethod = "HotAdd"

// Migration phases for Hot-Add
MigrationPhaseHotAddSnapshottingVM      = "SnapshottingSourceVM"
MigrationPhaseHotAddAttachingDisks      = "AttachingDisksToProxy"
MigrationPhaseHotAddIdentifyingDevices  = "IdentifyingBlockDevices"
MigrationPhaseHotAddTransferring        = "HotAddTransferInProgress"
MigrationPhaseHotAddCleaningUp          = "HotAddCleanup"

// Event messages
EventMessageHotAddSnapshotCreate  = "Creating source VM snapshot"
EventMessageHotAddAttachDisks     = "Attaching snapshot disks to Proxy VM"
EventMessageHotAddIdentify        = "Identifying block devices on Proxy VM"
EventMessageHotAddServing         = "Serving disk via NBD on Proxy VM"
EventMessageHotAddCopying         = "Copying data via nbdcopy"
EventMessageHotAddCleanup         = "Cleaning up snapshot and disk attachments"

// Port range for qemu-nbd on Proxy VM
HotAddPortRangeMin = 10809
HotAddPortRangeMax = 11808

// ProxyVM validation statuses
ProxyVMStatusPending             = "Pending"
ProxyVMStatusVerifying           = "Verifying"
ProxyVMStatusReady               = "Ready"
ProxyVMStatusVerificationFailed  = "VerificationFailed"

// Max disks a ProxyVM can have attached (vSphere hardware limit)
ProxyVMMaxAttachedDisks = 60

// Required components on Proxy VM
ProxyVMRequiredComponents = []string{"lsblk", "qemu-nbd", "nbdkit", "sshd"}
```

---

## Entity Relationships

```
MigrationTemplate
  └── StorageCopyMethod = "HotAdd"
  └── ProxyVMRef ──────────────────→ ProxyVM
                                          └── VMwareCredsRef → VMwareCreds

Migration (per VM)
  └── refs MigrationTemplate
  └── v2v-helper pod reads ConfigMap with:
        STORAGE_COPY_METHOD = "HotAdd"
        PROXY_VM_IP         = <from ProxyVM.Status.IPAddress>
        PROXY_VM_NAME       = <from ProxyVM.Spec.VMName>
```

---

## State Transition: ProxyVM Verification

```
[Created] ──→ Pending
                │
         (controller reconcile)
                │
              Verifying
               /    \
       (all pass)  (any fail)
           /              \
        Ready        VerificationFailed
           │
    (re-run verification)
           │
        Verifying  (loop back)
```

## State Transition: Hot-Add Migration (data-copy phase only)

```
AwaitingDataCopyStart
       │
  SnapshottingSourceVM     ← creates snapshot on source VM
       │
  AttachingDisksToProxy    ← attaches frozen VMDKs to ProxyVM via govmomi
       │
  IdentifyingBlockDevices  ← SSH: match UUID→wwid→/dev/sdX (retry ×3)
       │
  HotAddTransferInProgress ← qemu-nbd + nbdcopy per disk (retry ×3 on failure)
       │
  HotAddCleanup            ← kill qemu-nbd, detach disks, delete snapshot
       │
  ConvertingDisk           ← standard flow resumes
```
