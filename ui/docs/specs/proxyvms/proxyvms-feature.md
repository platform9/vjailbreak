# Proxy VMs Feature — Technical Specification

**Feature path**: `src/features/proxyvms/`  
**API path**: `src/api/proxyvms/`  
**Query hook**: `src/hooks/api/useProxyVMsQuery.ts`  
**Generated**: 2026-05-27  
**GitHub Issue**: [#1971](https://github.com/platform9/vjailbreak/issues/1971)  
**Related feature**: See `docs/specs/migration/migration-feature.md` — HotAdd storage copy method extension

---

## 1. Feature Overview

### Purpose

ProxyVM is a Kubernetes CRD that represents a vCenter VM pre-configured to act as a Hot-Add proxy during migration. When a migration uses `HotAdd` storage copy method, the vJailbreak controller attaches the source VM's disks to this proxy VM and reads data through it — avoiding the need for VDDK. The UI provides CRUD management for ProxyVM CRs and integrates the proxy VM selector into the migration form.

### Key User Journeys

1. **Add Proxy VM** → provide vCenter VM name + VMware credentials → controller verifies prerequisites → VM becomes `Ready`
2. **Monitor Proxy VM status** → page polls every 5s while any VM is `Pending` or `Verifying`
3. **Delete Proxy VM** → confirmation dialog → delete CRD
4. **Use in migration** → Step 3 of migration form: select `HotAdd via Proxy VM` → choose a `Ready` proxy VM from dropdown

---

## 2. Architecture Map

### Module/Folder Structure

```
src/api/proxyvms/
├── model.ts        — ProxyVM TypeScript interfaces
├── proxyVMs.ts     — CRUD API calls (list/get/post/delete)
└── index.ts        — barrel export

src/hooks/api/
└── useProxyVMsQuery.ts  — React Query hook, auto-polls while Pending/Verifying

src/features/proxyvms/
├── pages/
│   └── ProxyVMsPage.tsx         — Page component: toolbar + table
└── components/
    ├── ProxyVMsTable.tsx         — DataGrid with status chips + delete
    └── AddProxyVMDialog.tsx      — Add dialog: vmName, VMware creds, prereqs accordion
```

### Component Hierarchy

```
ProxyVMsPage
└── ProxyVMsTable (DataGrid)
    └── ConfirmationDialog (delete)

ProxyVMsPage → AddProxyVMDialog (modal)
```

### Navigation

`Proxy VMs` added under `Credentials` sidebar group in `src/config/navigation.tsx`.  
Route: `/dashboard/proxy-vms` registered in `src/App.tsx`.

---

## 3. CRD Spec

**Resource**: `proxyvms.vjailbreak.k8s.pf9.io/v1alpha1`  
**Namespace**: `migration-system`  
**API endpoint**: `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms`

```typescript
// spec (user-provided)
interface ProxyVMSpec {
  vmName: string              // vCenter VM display name
  vmwareCredsRef: {
    name: string              // VMwareCreds CR name (must be Succeeded)
  }
}

// status (controller-managed)
interface ProxyVMStatus {
  validationStatus: 'Pending' | 'Verifying' | 'Ready' | 'VerificationFailed'
  validationMessage?: string
  ipAddress?: string
  attachedDiskCount?: number
  componentsVerified?: string[]   // lsblk, nbdkit, qemu-nbd, sshd, disk.EnableUUID
  lastValidationTime?: string     // RFC3339
}
```

**Kubernetes resource name**: auto-derived from `vmName` — lowercased, non-alphanumeric replaced with `-`, truncated to 63 chars.

---

## 4. Component Specifications

### ProxyVMsPage (`pages/ProxyVMsPage.tsx`)

**Props**: None (page-level route component)

**Behavior**:
- Calls `useProxyVMsQuery()` — auto-polls 5s while any item is Pending/Verifying
- "Add Proxy VM" button → opens `AddProxyVMDialog`
- Passes `toolbar` node as prop to `ProxyVMsTable`

---

### ProxyVMsTable (`components/ProxyVMsTable.tsx`)

**Props**:

| Name | Type | Required |
|------|------|----------|
| `proxyVMs` | `ProxyVM[]` | Yes |
| `loading` | `boolean` | No |
| `toolbar` | `React.ReactNode` | Yes |

**Columns**:

| Column | Field | Notes |
|--------|-------|-------|
| Name | `metadata.name` | Kubernetes resource name |
| VM Name | `spec.vmName` | vCenter display name |
| Status | `status.validationStatus` | Color-coded chip |
| IP Address | `status.ipAddress` | `-` if absent |
| Attached Disks | `status.attachedDiskCount` | `-` if absent |
| Age | derived from `metadata.creationTimestamp` | humanized (5m, 3h, 2d) |
| Last Validated | `status.lastValidationTime` | `toLocaleString()` |
| Delete | icon button | Opens ConfirmationDialog |

**Status chip colors**:
- `Pending` → grey (`default`)
- `Verifying` → yellow (`warning`)
- `Ready` → green (`success`)
- `VerificationFailed` → red (`error`)

**Delete flow**: ConfirmationDialog → `deleteProxyVM(name)` → invalidate `['proxyvms']` query cache.

---

### AddProxyVMDialog (`components/AddProxyVMDialog.tsx`)

**Props**:

| Name | Type | Required |
|------|------|----------|
| `open` | `boolean` | Yes |
| `onClose` | `() => void` | Yes |

**Form fields**:
- `vmName` (TextField, required) — vCenter VM display name
- Derived K8s name shown as read-only helper field (auto-computed from vmName)
- `vmwareCredsRef` (Select dropdown, required) — VMware creds filtered to `vmwareValidationStatus === 'Succeeded'`

**Collapsible prerequisites accordion** (always shown, collapsed by default):
- lsblk installed
- nbdkit installed
- qemu-nbd installed
- sshd running and accessible
- disk.EnableUUID enabled on VM
- vJailbreak public key in authorized_keys

**Error handling**:
- HTTP 409 Conflict → `"A Proxy VM with the name '{name}' already exists."`
- Other errors → inline error alert with `err.response.data.message`

**Submit**: `postProxyVM()` → invalidate `['proxyvms']` → close dialog

---

## 5. API Contract

### ProxyVM CRUD

| Operation | Endpoint | Method |
|-----------|----------|--------|
| List | `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms` | GET |
| Get | `…/proxyvms/{name}` | GET |
| Create | `…/proxyvms` | POST |
| Delete | `…/proxyvms/{name}` | DELETE |

All calls use the shared `axios` client from `src/api/axios.ts` with `VJAILBREAK_DEFAULT_NAMESPACE = 'migration-system'`.

---

## 6. State and Data Flow

### React Query

```
useProxyVMsQuery
  queryKey: ['proxyvms', namespace?]
  queryFn: getProxyVMList(namespace)
  staleTime: 0
  refetchOnWindowFocus: true
  refetchInterval: (query) =>
    any item Pending|Verifying ? 5000 : false
```

**Cache invalidation**: After `postProxyVM` or `deleteProxyVM`, invalidate `['proxyvms']`.

### Data Flow

```
ProxyVMsPage mounts
  → useProxyVMsQuery polls /proxyvms
  → ProxyVMsTable renders rows
  → If any Pending/Verifying: re-polls every 5s
  → Ready status reached: polling stops

User clicks "Add Proxy VM"
  → AddProxyVMDialog opens
  → Fetches VMware creds (useVmwareCredentialsQuery, filter Succeeded)
  → User submits → POST /proxyvms
  → invalidate ['proxyvms'] → table refetches → new VM appears as Pending
  → Controller validates → status transitions to Verifying → Ready|VerificationFailed
```

---

## 7. Integration with Migration Form

ProxyVM is consumed in Step 3 (Network and Storage Mapping) of both standard and rolling migration forms.

### Storage Copy Method: `HotAdd`

Added to `STORAGE_COPY_METHOD_OPTIONS` in `src/features/migration/constants.ts`:
```ts
{ value: 'HotAdd', label: 'Hot-Add via Proxy VM' }
```

**`StorageCopyMethod` type** (`src/features/migration/types.ts`):
```ts
type StorageCopyMethod = 'normal' | 'StorageAcceleratedCopy' | 'HotAdd'
```

**Form params**: `proxyVMRef?: string` added to `FormValues` and `RollingFormParams`.

### NetworkAndStorageMappingStep Behavior (HotAdd)

When `storageCopyMethod === 'HotAdd'`:
- Storage mapping table hidden (no volume type mapping needed)
- Proxy VM Select dropdown shown:
  - Source: `useProxyVMsQuery()` filtered to `status.validationStatus === 'Ready'`
  - Option label: `metadata.name (status.ipAddress)` or just `metadata.name`
  - `onChange('proxyVMRef')(selectedName)`
- If no Ready VMs: Alert warning shown — "No Ready Proxy VM found. Add and verify a Proxy VM on the Proxy VMs page before starting a Hot-Add migration."
- `unmappedStorageCount` returns 0 for HotAdd (no storage to map)

### MigrationOptionsAlt Behavior (HotAdd)

When `storageCopyMethod === 'HotAdd'`:
- `dataCopyMethod` forced to `'cold'` via useEffect
- `hot` and `mock` options disabled in the Select
- Helper text shown: "Hot-Add migration only supports Cold copy. Hot copy is disabled."

### Validation

**Standard** (`useFormValidation.ts`) and **Rolling** (`useRollingFormValidation.ts`) — Step 3:
```ts
storageValidation =
  storageCopyMethod === 'HotAdd'
    ? Boolean(params.proxyVMRef)
    : storageCopyMethod === 'StorageAcceleratedCopy' ? ...arrayCredsMappings check
    : ...storageMappings check
```

Error if HotAdd selected but `proxyVMRef` empty: "Please select a Proxy VM to use for Hot-Add data copy"

### Form Submit

**Standard** (`useMigrationFormSubmit.ts`) — `handleSubmit`:
```
HotAdd → skip createStorageMapping + createArrayCredsMapping
       → updateMigrationTemplate: set spec.proxyVMRef = { name: params.proxyVMRef }

StorageAcceleratedCopy → createArrayCredsMapping → set spec.arrayCredsMapping
normal → createStorageMapping → set spec.storageMapping
```

**Rolling** (`useRollingFormSubmit.ts`) — same branching logic in both the mapping creation and the `patchMigrationTemplate` call.

### MigrationTemplate Spec Field

Added to `MigrationTemplateSpec` (`src/api/migration-templates/model.ts`):
```ts
proxyVMRef?: { name: string }
storageCopyMethod?: string
```

---

## 8. Validation Rules

| Rule | Condition | Error |
|------|-----------|-------|
| proxyVMRef required | `storageCopyMethod === 'HotAdd'` and `!params.proxyVMRef` | "Please select a Proxy VM to use for Hot-Add data copy" |
| vmName required | Add dialog submit | "VM name is required" |
| vmwareCredsRef required | Add dialog submit | "VMware credentials are required" |

---

## 9. Prerequisites for Proxy VM

The VM must have these configured before adding it as a Proxy VM:
1. `lsblk` — block device lister
2. `nbdkit` — network block device kit
3. `qemu-nbd` — QEMU NBD server
4. `sshd` — SSH daemon running and accessible
5. `disk.EnableUUID` — VMware disk UUID enabled on VM
6. SSH key authorization — vJailbreak public key in `~/.ssh/authorized_keys`

---

## 10. Known Constraints

- ProxyVM must be in `migration-system` namespace — hardcoded via `VJAILBREAK_DEFAULT_NAMESPACE`
- Only `Ready` proxy VMs appear in migration form dropdown
- HotAdd forces cold copy; warm (hot) migration not supported with HotAdd
- If browser crashes after selecting HotAdd, no orphan K8s resources — ProxyVM CRD persists (not session-scoped)
- ProxyVM CRD is not created as a session resource — it persists across migrations and must be explicitly deleted
