# Proxy VMs Feature — Technical Specification

**Feature path**: `src/features/proxyvms/`  
**API path**: `src/api/proxyvms/`  
**Query hook**: `src/hooks/api/useProxyVMsQuery.ts`  
**Generated**: 2026-05-27  
**Last updated**: 2026-05-29  
**GitHub Issue**: [#1971](https://github.com/platform9/vjailbreak/issues/1971)  
**Related feature**: See `docs/specs/migration/migration-feature.md` — HotAdd storage copy method extension

---

## 1. Feature Overview

### Purpose

ProxyVM is a Kubernetes CRD that represents a vCenter VM pre-configured to act as a Hot-Add proxy during migration. When a migration uses `HotAdd` storage copy method, the vJailbreak controller attaches the source VM's disks to this proxy VM and reads data through it — avoiding the need for VDDK. The UI provides CRUD management for ProxyVM CRs and integrates the proxy VM selector into the migration form.

> **Beta**: Feature is marked Beta in the navigation sidebar and in the HotAdd radio option in the migration form.

### Key User Journeys

1. **Add Proxy VM** → select VMware credentials → pick existing vCenter VM from searchable dropdown → paste SSH private key (or upload key file) → submit → controller verifies prerequisites → VM becomes `Ready`
2. **Monitor Proxy VM status** → page polls every 5s while any VM is `Pending` or `Verifying`; status filter + search available in table toolbar
3. **Delete Proxy VM** → confirmation dialog → delete CRD + SSH key Secret
4. **Use in migration** → Step 3 of migration form: select `HotAdd via Proxy VM` (Beta) → choose a `Ready` proxy VM from dropdown

---

## 2. Architecture Map

### Module/Folder Structure

```
src/api/proxyvms/
├── model.ts        — ProxyVM TypeScript interfaces (includes sshKeySecretRef)
├── proxyVMs.ts     — CRUD API calls (list/get/post/delete)
└── index.ts        — barrel export

src/hooks/api/
└── useProxyVMsQuery.ts  — React Query hook, auto-polls while Pending/Verifying

src/features/proxyvms/
├── pages/
│   └── ProxyVMsPage.tsx         — Thin page wrapper, renders ProxyVMsTable
└── components/
    ├── ProxyVMsTable.tsx         — Self-contained: DataGrid + toolbar + drawer management
    └── AddProxyVMDrawer.tsx      — Add drawer: VMware creds → VM select → SSH key
```

> `AddProxyVMDialog.tsx` is a dead file (orphaned). It is not imported anywhere and can be deleted.

### Component Hierarchy

```
ProxyVMsPage
└── ProxyVMsTable (self-contained)
    ├── CommonDataGrid
    ├── ConfirmationDialog (delete)
    └── AddProxyVMDrawer (managed internally)
```

### Navigation

`Proxy VMs` nav item under `Credentials` sidebar group in `src/config/navigation.tsx`.  
Badge: `{ label: 'Beta', color: 'warning', variant: 'outlined' }` — same as Storage Array.  
Route: `/dashboard/proxy-vms` registered in `src/App.tsx`.

---

## 3. CRD Spec

**Resource**: `proxyvms.vjailbreak.k8s.pf9.io/v1alpha1`  
**Namespace**: `migration-system`  
**API endpoint**: `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms`

```typescript
// spec (user-provided)
interface ProxyVMSpec {
  vmName: string // vCenter VM display name (selected from VMware machines list)
  vmwareCredsRef: {
    name: string // VMwareCreds CR name (must be Succeeded)
  }
  sshKeySecretRef?: {
    name: string // Kubernetes Secret name holding the SSH private key
  }
}

// status (controller-managed)
interface ProxyVMStatus {
  validationStatus: 'Pending' | 'Verifying' | 'Ready' | 'VerificationFailed'
  validationMessage?: string
  ipAddress?: string
  attachedDiskCount?: number
  componentsVerified?: string[] // lsblk, nbdkit, qemu-nbd, sshd, disk.EnableUUID
  lastValidationTime?: string // RFC3339
}
```

**Kubernetes resource name**: auto-derived from `vmName` — lowercased, non-alphanumeric replaced with `-`, truncated to 63 chars. Used as both the ProxyVM CR name and the SSH key Secret name.

---

## 4. Component Specifications

### ProxyVMsPage (`pages/ProxyVMsPage.tsx`)

**Props**: None.  
**Behavior**: Thin wrapper — renders `<ProxyVMsTable />` with no props. All state lives in ProxyVMsTable.

---

### ProxyVMsTable (`components/ProxyVMsTable.tsx`)

**Props**: None (self-contained).

**Internal state**:

- `useProxyVMsQuery()` — data + loading + refetch
- `addDrawerOpen` — controls `AddProxyVMDrawer`
- `deleteTarget` — ProxyVM selected for deletion
- `statusFilter` — active status filter (`'All' | 'Pending' | 'Verifying' | 'Ready' | 'VerificationFailed'`)

**Toolbar**: `ListingToolbar` + `CustomSearchToolbar` (search by name/VM name, status filter dropdown, refresh) + "Add Proxy VM" button.

**Columns**:

| Column         | Field                                     | Notes                                                         |
| -------------- | ----------------------------------------- | ------------------------------------------------------------- |
| Name           | `metadata.name`                           | Kubernetes resource name                                      |
| VM Name        | `spec.vmName`                             | vCenter display name                                          |
| Status         | `status.validationStatus`                 | Color-coded chip (`borderRadius: 4px`)                        |
| Message        | `status.validationMessage`                | Truncated + tooltip; red on `VerificationFailed`              |
| IP Address     | `status.ipAddress`                        | `-` if absent                                                 |
| Attached Disks | `status.attachedDiskCount`                | `-` if absent                                                 |
| Age            | derived from `metadata.creationTimestamp` | humanized (5m, 3h, 2d)                                        |
| Last Validated | `status.lastValidationTime`               | `toLocaleString()`                                            |
| Actions        | delete `IconButton`                       | `DeleteOutlined`, `stopPropagation`, opens ConfirmationDialog |

**Status chip colors**:

- `Pending` → grey (`default`)
- `Verifying` → yellow (`warning`)
- `Ready` → green (`success`)
- `VerificationFailed` → red (`error`)

**Delete flow**: ConfirmationDialog (`WarningIcon`, `actionVariant="outlined"`) → `deleteProxyVM(name)` + `deleteSecret(name)` (fire-and-forget) → invalidate `['proxyvms']` query cache.

---

### AddProxyVMDrawer (`components/AddProxyVMDrawer.tsx`)

Implemented as `DrawerShell` following the design system pattern (same as `VMwareCredentialsDrawer`, `AddArrayCredentialsDrawer`).

**Props**:

| Name      | Type         | Required |
| --------- | ------------ | -------- |
| `open`    | `boolean`    | Yes      |
| `onClose` | `() => void` | Yes      |

**Form**: `useForm` with `mode: 'onChange'`. Submit button disabled until `isValid`.

**Layout**:

```
DrawerHeader (title + subtitle + close)
  SurfaceCard
    Section "Proxy VM"
      SectionHeader (title + subtitle)
      VMware Credentials  [RHFSelect]
        helperText: "Only validated credentials are shown"
      VM Name             [RHFSelect, searchable, disabled until cred selected]
        helperText: "Search or select the vCenter VM name of the Proxy VM"
    Section "SSH Access"
      SectionHeader (title + subtitle)
      Alert info: "Add the public key … to /root/.ssh/authorized_keys before registering."
      [Upload key file] [Paste only the OpenSSH private key content…]
      SSH Private Key     [RHFTextField, multiline, minRows=10]
DrawerFooter (Cancel | Add Proxy VM)
```

**VM Name field behavior**:

- Disabled until `vmwareCredsRef` selected
- On cred change: resets `vmName` to `''`; fetches VMs via `getVMwareMachines(namespace, vmwareCredsRef)`
- Query key: `['vmwaremachines-for-proxy', vmwareCredsRef]`, `staleTime: 30_000`
- **Only powered-on VMs shown** — `queryFn` filters `m.status?.powerState === 'running'` before mapping options
- Options: `{ label: vm.spec.vms.name, value: vm.spec.vms.name }`
- Placeholder chains: `"Select VMware credentials first"` → `"Loading VMs..."` → `"No VMs found"` → `"Search and select a VM"`
- After VM list loads, info Alert shown: "Only powered on VMs can be added as a Proxy VM. If the VM is powered on but not listed, please revalidate the credentials."

**SSH Private Key field**:

- Upload key file: reads file as text → `setValue('sshPrivateKey', text)` with `shouldValidate: true`; max file size 1 MB
- Validation: must contain `-----BEGIN OPENSSH PRIVATE KEY-----` and `-----END OPENSSH PRIVATE KEY-----`
- `sx={{ '& textarea': { wordBreak: 'break-all', overflowWrap: 'break-word', overflowX: 'hidden' } }}` prevents horizontal scroll on long keys

**Error handling**:

- HTTP 409 Conflict → `"A Proxy VM with the name '{name}' already exists."`
- Other errors → inline dismissible error Alert

**Submit flow**:

1. `createSecret(proxyVmName, { 'ssh-privatekey': sshPrivateKey }, 'migration-system')`
2. `postProxyVM({ ..., spec: { vmName, vmwareCredsRef, sshKeySecretRef: { name: proxyVmName } } })`
3. On ProxyVM POST failure: `deleteSecret(proxyVmName)` (rollback)
4. On success: invalidate `['proxyvms']` → close drawer

**Kubernetes resource name**: derived internally — `toK8sName(data.vmName)`. Not displayed to user. Used as both ProxyVM CR name and SSH Secret name.

---

## 5. API Contract

### ProxyVM CRUD

| Operation | Endpoint                                                                    | Method |
| --------- | --------------------------------------------------------------------------- | ------ |
| List      | `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms` | GET    |
| Get       | `…/proxyvms/{name}`                                                         | GET    |
| Create    | `…/proxyvms`                                                                | POST   |
| Delete    | `…/proxyvms/{name}`                                                         | DELETE |

All calls use the shared `axios` client from `src/api/axios.ts` with `VJAILBREAK_DEFAULT_NAMESPACE = 'migration-system'`.

### SSH Key Secret

| Operation | API                                                        | Notes                                                         |
| --------- | ---------------------------------------------------------- | ------------------------------------------------------------- |
| Create    | `createSecret(name, { 'ssh-privatekey': key }, namespace)` | On drawer submit, before ProxyVM POST                         |
| Delete    | `deleteSecret(name, namespace)`                            | On ProxyVM delete (fire-and-forget) + rollback on failed POST |

Secret name = ProxyVM CR name = `toK8sName(vmName)`.

### VMware Machines (VM picker)

| Operation               | API                                            | Notes                      |
| ----------------------- | ---------------------------------------------- | -------------------------- |
| List VMs for credential | `getVMwareMachines(namespace, vmwareCredName)` | Filtered by label selector |

Returns `VMwareMachineList`. VM name shown = `machine.spec.vms.name`.

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

useQuery (VM picker, inline in AddProxyVMDrawer)
  queryKey: ['vmwaremachines-for-proxy', vmwareCredsRef]
  queryFn: getVMwareMachines(namespace, vmwareCredsRef)
  enabled: Boolean(vmwareCredsRef)
  staleTime: 30_000
```

**Cache invalidation**: After `postProxyVM` or `deleteProxyVM`, invalidate `['proxyvms']`.

### Data Flow

```
ProxyVMsPage mounts
  → ProxyVMsTable mounts
  → useProxyVMsQuery polls /proxyvms
  → CommonDataGrid renders rows (filtered by statusFilter)
  → If any Pending/Verifying: re-polls every 5s

User clicks "Add Proxy VM"
  → AddProxyVMDrawer opens
  → User selects VMware creds → fetches VMs for that cred
  → User picks VM from searchable dropdown
  → User pastes / uploads SSH private key
  → Submit:
      1. createSecret(proxyVmName, { ssh-privatekey })
      2. postProxyVM({ vmName, vmwareCredsRef, sshKeySecretRef })
      → invalidate ['proxyvms'] → table refetches → new VM appears as Pending
      → Controller validates → Verifying → Ready | VerificationFailed

User clicks Delete
  → ConfirmationDialog
  → deleteProxyVM(name) + deleteSecret(name)
  → invalidate ['proxyvms']
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

**Beta chip**: `HotAdd` radio label shows `<Chip label="Beta" color="warning" variant="outlined" />` (same styling as `StorageAcceleratedCopy` beta chip). Applies to both standard and rolling forms since `NetworkAndStorageMappingStep` is shared.

### NetworkAndStorageMappingStep Behavior (HotAdd)

When `storageCopyMethod === 'HotAdd'`:

- **Proxy VM selector** shown (with `FieldLabel` above):
  - Source: `useProxyVMsQuery()` filtered to `status.validationStatus === 'Ready'`
  - Option label: `metadata.name (status.ipAddress)` or just `metadata.name`
  - `onChange('proxyVMRef')(selectedName)`
  - If no Ready VMs: warning Alert shown
- **VMware Datastore → PCD Volume Type mapping table** also shown (same as `normal` mode)
  - Uses `params.storageMappings` and `openstackStorage`
  - All datastores must be mapped to proceed
- Switching away from HotAdd clears `proxyVMRef` (RadioGroup `onChange`)
- `unmappedStorage` calculated the same as `normal` mode (uses `params.storageMappings`)

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
      && !isNilOrEmpty(params.storageMappings)
      && all datastores mapped via params.storageMappings
    : storageCopyMethod === 'StorageAcceleratedCopy' ? ...arrayCredsMappings check
    : ...storageMappings check
```

HotAdd requires **both** `proxyVMRef` AND fully-mapped `storageMappings` to proceed.

### Form Submit

**Standard** (`useMigrationFormSubmit.ts`) — `handleSubmit`:

```
HotAdd → createStorageMapping(params.storageMappings)   // POST /storagemappings
       → updateMigrationTemplate: set spec.proxyVMRef = { name: params.proxyVMRef }
                                   AND spec.storageMapping = storageMappings.metadata.name

StorageAcceleratedCopy → createArrayCredsMapping → set spec.arrayCredsMapping
normal → createStorageMapping → set spec.storageMapping
```

**Rolling** (`useRollingFormSubmit.ts`) — same branching logic applies.

### MigrationTemplate Spec Field

Added to `MigrationTemplateSpec` (`src/api/migration-templates/model.ts`):

```ts
proxyVMRef?: { name: string }
storageCopyMethod?: string
```

---

## 8. Validation Rules

| Rule                     | Condition                                                     | Error                                                                                    |
| ------------------------ | ------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| proxyVMRef required      | `storageCopyMethod === 'HotAdd'` and `!params.proxyVMRef`     | "Please select a Proxy VM to use for Hot-Add data copy"                                  |
| storageMappings required | `storageCopyMethod === 'HotAdd'` and storage not fully mapped | Storage mapping required (same as `normal` mode)                                         |
| VM required              | Add drawer submit                                             | "VM is required"                                                                         |
| vmwareCredsRef required  | Add drawer submit                                             | "VMware credentials are required"                                                        |
| sshPrivateKey required   | Add drawer submit                                             | "SSH private key is required"                                                            |
| sshPrivateKey format     | Must contain OpenSSH headers                                  | "Invalid key format. Expected OpenSSH private key (-----BEGIN OPENSSH PRIVATE KEY-----)" |
| SSH key file size        | Upload > 1 MB                                                 | "File too large. SSH private key must be under 1 MB."                                    |

---

## 9. Prerequisites for Proxy VM

The VM must have these configured before adding it as a Proxy VM (documented externally — not shown in the UI form):

1. `lsblk` — block device lister
2. `nbdkit` — network block device kit
3. `qemu-nbd` — QEMU NBD server
4. `sshd` — SSH daemon running and accessible
5. `disk.EnableUUID` — VMware disk UUID enabled on VM
6. SSH key authorization — vJailbreak public key in `~/.ssh/authorized_keys` (the user must do this manually before submitting the form — the drawer's info Alert reminds them)

---

## 10. Known Constraints

- ProxyVM must be in `migration-system` namespace — hardcoded via `VJAILBREAK_DEFAULT_NAMESPACE`
- Only `Ready` proxy VMs appear in migration form dropdown
- HotAdd forces cold copy; warm (hot) migration not supported with HotAdd
- SSH Secret name = ProxyVM CR name — both derived from `toK8sName(vmName)`. Collision possible if two different VM names normalize to the same K8s-safe string.
- `AddProxyVMDialog.tsx` — orphaned dead file; safe to delete
- ProxyVM CRD is not session-scoped — persists across migrations and must be explicitly deleted
- VM picker in drawer fetches from `/vmwaremachines?labelSelector=...` — requires VMware creds to be already validated and VMwareMachine CRs to exist in the cluster
