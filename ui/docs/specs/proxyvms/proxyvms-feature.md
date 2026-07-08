# Proxy VMs Feature — Technical Specification

**Feature path**: `src/features/proxyvms/`  
**API path**: `src/api/proxyvms/`  
**Query hook**: `src/hooks/api/useProxyVMsQuery.ts`  
**Generated**: 2026-05-27  
**Last updated**: 2026-07-07  
**GitHub Issue**: [#1971](https://github.com/platform9/vjailbreak/issues/1971) (base feature), [#2067](https://github.com/platform9/vjailbreak/issues/2067) (dup-registration guard), [#2088](https://github.com/platform9/vjailbreak/issues/2088) (Windows VM block)  
**Related feature**: See `docs/specs/migration/migration-feature.md` — HotAdd storage copy method extension (UI label: "vJailbreak Accelerated Copy")

---

## 1. Feature Overview

### Purpose

ProxyVM is a Kubernetes CRD that represents a vCenter VM pre-configured to act as a Hot-Add proxy during migration. When a migration uses `HotAdd` storage copy method, the vJailbreak controller attaches the source VM's disks to this proxy VM and reads data through it — avoiding the need for VDDK. The UI provides CRUD management for ProxyVM CRs and integrates the proxy VM selector into the migration form.

> **Beta**: Feature is marked Beta in the navigation sidebar and in the HotAdd radio option in the migration form.

### Key User Journeys

1. **Deploy new Proxy VM** → select method "Deploy a new vJailbreak Proxy VM" → enter VM name + pick VMware creds → pick datacenter/datastore/network → submit → controller deploys OVA, injects SSH keys, registers ProxyVM automatically → drawer shows progress and auto-closes once the VM appears in the list
2. **Register existing VM** → select method "Register an existing VM" → pick VMware creds → pick VM from searchable dropdown (shows IP + vCPU; Windows VMs and already-registered VMs are greyed out/disabled) → generate key pair (copy public key to VM's `authorized_keys`) OR upload existing private key → submit → controller verifies prerequisites → VM becomes `Ready`
3. **Monitor Proxy VM status** → table's `useProxyVMsQuery()` polls every 5s while any VM is `Pending` or `Verifying` (a `Deploying` VM is tracked separately by the Add drawer's own deploy-poll, see §6); status filter + search available in table toolbar
4. **Retry failed verification** → row in `VerificationFailed` status shows a retry icon → patches a `force-reconcile` annotation → controller re-verifies
5. **Delete Proxy VM(s)** → single-row delete or multi-select "Delete Selected (n)" bulk delete → confirmation dialog → delete CRD(s) + SSH key Secret(s)
6. **Use in migration** → Step 3 of migration form: select `vJailbreak Accelerated Copy` (Beta) → choose a `Ready` proxy VM from dropdown

---

## 2. Architecture Map

### Module/Folder Structure

```
src/api/proxyvms/
├── model.ts        — ProxyVM TypeScript interfaces
├── proxyVMs.ts     — CRUD + OVA deploy + vCenter resources API calls
└── index.ts        — barrel export

src/hooks/api/
└── useProxyVMsQuery.ts  — React Query hook, auto-polls while any item is Pending/Verifying

src/features/proxyvms/
├── pages/
│   └── ProxyVMsPage.tsx         — Thin page wrapper, renders ProxyVMsTable
└── components/
    ├── types.ts                 — Shared types (FormMode, SSHKeySource, VMOption, SelectFormData incl. authorizedKeysConfirmed, CreateFormData, GeneratedKey)
    ├── MethodCard.tsx           — Radio-style method selection card component
    ├── VMAutocomplete.tsx       — Searchable VM dropdown with IP/vCPU display + selected VM chip
    ├── SSHAccessSection.tsx     — SSH key section (generate key pair / upload private key toggle)
    ├── RegisterVMForm.tsx       — Form for "Register existing VM" mode (uses form context)
    ├── DeployVMForm.tsx         — Form for "Deploy new VM from OVA" mode (uses form context)
    ├── AddProxyVMDrawer.tsx     — Main drawer: state, queries, handlers, method card rendering
    ├── ProxyVMsTable.tsx        — Self-contained: DataGrid + toolbar + drawer management
    └── ProxyVMDetailDrawer.tsx  — Detail view drawer
```


### Component Hierarchy

```
ProxyVMsPage
└── ProxyVMsTable (self-contained)
    ├── CommonDataGrid
    ├── ConfirmationDialog (delete)
    └── AddProxyVMDrawer
        ├── MethodCard × 2          (Deploy / Register)
        ├── RegisterVMForm          (select mode)
        │   ├── VMAutocomplete
        │   └── SSHAccessSection
        └── DeployVMForm            (create mode)
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
  vmName: string               // vCenter VM display name
  vmwareCredsRef: { name: string }  // VMwareCreds CR name (must be Succeeded)
  sshKeySecretRef?: { name: string }   // Secret holding SSH private key (manual upload path)
  sshKeyPairRef?: { name: string }     // Secret holding generated key pair (generate path)
}

// status (controller-managed)
interface ProxyVMStatus {
  validationStatus: 'Deploying' | 'DeployFailed' | 'Pending' | 'Verifying' | 'Ready' | 'VerificationFailed'
  validationMessage?: string
  ipAddress?: string
  attachedDiskCount?: number
  componentsVerified?: { name: string; present: boolean; message?: string }[]
  lastValidationTime?: string    // RFC3339
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
- `deleteTarget` — single ProxyVM selected for row delete
- `rowSelectionModel` / `bulkDeleteDialogOpen` — multi-select bulk delete
- `retryingNames` — set of names currently retrying (spinner on retry icon)
- `detailVM` / `detailOpen` — controls `ProxyVMDetailDrawer` (opened by clicking the Name cell)
- `statusFilter` — active status filter (`'All' | 'Deploying' | 'DeployFailed' | 'Pending' | 'Verifying' | 'Ready' | 'VerificationFailed'`)

**Toolbar**: `ListingToolbar` + `CustomSearchToolbar` (search by name/VM name, status filter dropdown, refresh) + "Add vJailbreak Proxy VM" button. When rows are selected, a "Delete Selected (n)" button appears next to the search bar.

**Columns**:

| Column         | Field                                     | Notes                                                                 |
| -------------- | ----------------------------------------- | ---------------------------------------------------------------------|
| Name           | `metadata.name`                           | Kubernetes resource name; clickable, opens `ProxyVMDetailDrawer`     |
| VM Name        | `spec.vmName`                             | vCenter display name                                                  |
| Status         | `status.validationStatus`                 | Color-coded chip (`borderRadius: 4px`)                                |
| IP Address     | `status.ipAddress`                        | `-` if absent                                                          |
| Attached Disks | `status.attachedDiskCount`                | `-` if absent                                                          |
| Age            | derived from `metadata.creationTimestamp` | humanized (5m, 3h, 2d)                                                 |
| Last Validated | `status.lastValidationTime`               | `toLocaleString()`                                                     |
| Actions        | retry (conditional) + delete `IconButton` | retry only on `VerificationFailed` rows; both `stopPropagation`        |

> `checkboxSelection` is enabled on the grid for bulk delete; there is no separate "Message" column — `status.validationMessage` is only shown in the detail drawer's Status card.

**Status chip colors**:

- `Deploying` → blue (`info`)
- `DeployFailed` → red (`error`)
- `Pending` → grey (`default`)
- `Verifying` → yellow (`warning`)
- `Ready` → green (`success`)
- `VerificationFailed` → red (`error`)

**Retry flow** (`VerificationFailed` rows only): retry `IconButton` (`Refresh`, spinner while in-flight) → `retryProxyVMVerification(name)` → 1s delay → invalidate `['proxyvms']` query cache.

**Delete flow**: ConfirmationDialog (`WarningIcon`, `actionVariant="outlined"`) → `deleteProxyVM(name)`, tolerating 404 (already gone) → `deleteSecret(spec.sshKeySecretRef.name)` (fire-and-forget, manual-upload path only) → invalidate `['proxyvms']` query cache + `refetch()`.

**Bulk delete flow**: select rows via checkboxes → "Delete Selected (n)" → ConfirmationDialog listing selected names → deletes all in parallel (`Promise.all`, tolerating 404 per item) → same secret cleanup + cache invalidation.

---

### AddProxyVMDrawer (`components/AddProxyVMDrawer.tsx`)

Implemented as `DrawerShell` following the design system pattern.

**Props**:

| Name      | Type         | Required |
| --------- | ------------ | -------- |
| `open`    | `boolean`    | Yes      |
| `onClose` | `() => void` | Yes      |

**Form modes** (`FormMode = 'select' | 'create'`):

Both forms use `useForm` with `mode: 'onChange'`. Submit button disabled until form `isValid`.

**Layout**:

```
DrawerHeader
  title: "Add vJailbreak Proxy VM"
  subtitle: varies by formMode

SurfaceCard
  [error Alert — dismissible]

  Method (overline label)
    MethodCard: "Deploy a new vJailbreak Proxy VM" [RECOMMENDED]
    MethodCard: "Register an existing VM"

  ─── divider ───

  [formMode === 'select'] → RegisterVMForm
  [formMode === 'create' && !deploymentStarted] → DeployVMForm
  [formMode === 'create' && deploymentStarted] → deployment progress UI (Alert + LinearProgress)

DrawerFooter
  Cancel | Register vJailbreak Proxy VM   (select mode)
  Done | Deploy & Register VM             (create mode, "Done" once deploymentStarted)
```

**Subtitle**:

- `select`: `"Register a powered-on VM you have already prepared as a vJailbreak Accelerated Copy proxy."`
- `create`: `"A new vJailbreak Proxy VM is deployed from the OVA template and registered automatically."`

**Deployment progress UI** (`formMode === 'create' && deploymentStarted`): success Alert naming the VM, `LinearProgress`, body text ("...typically takes 3–5 minutes"), and an info Alert noting the panel can be closed — the drawer auto-closes itself once the deployed VM name appears in the polled ProxyVM list (see §6).

---

### MethodCard (`components/MethodCard.tsx`)

Outlined `Paper` with radio button + icon + title + optional RECOMMENDED chip + description. Blue border + tinted background when selected.

---

### RegisterVMForm (`components/RegisterVMForm.tsx`)

Rendered inside `DesignSystemForm id="add-proxy-vm-form"`. Sections:

**VMware Environment**
- VMware Credentials `RHFSelect` — only validated creds shown

**Select VM**
- `VMAutocomplete` — searchable dropdown of powered-on VMs

**SSH Access**
- `SSHAccessSection` — toggle between generate / upload

**Internal**: watches `form.watch('sshPrivateKey')` and passes `hasPrivateKey={Boolean(value.trim())}` to `SSHAccessSection`.

---

### VMAutocomplete (`components/VMAutocomplete.tsx`)

`FieldLabel` above + MUI `Autocomplete<VMOption>` + selected VM summary card.

**VMOption shape**:
```typescript
interface VMOption {
  name: string
  ipAddress?: string   // from spec.vms.ipAddress || spec.vms.assignedIp
  cpu: number          // from spec.vms.cpu
  powerState: string
  osFamily?: string    // from spec.vms.osFamily; 'linuxGuest' | 'windowsGuest' | ...
}
```

**Behavior**:
- Disabled until VMware creds selected
- Only powered-on VMs listed (`powerState === 'running'`)
- Filter by name or IP
- **Option disabling** (`getOptionDisabled`) — an option is disabled (greyed out, unselectable) when either:
  - `registeredVMNames.has(option.name)` — already registered as a vJailbreak Proxy VM (GHI #2067; prevents secret mismatch on re-registration)
  - `option.osFamily !== 'linuxGuest'` — non-Linux guest (GHI #2088; Windows VMs cannot be proxy VMs)
  - Disabled options show a trailing caption instead of IP/vCPU: `"Windows not supported"` (Windows), `"Requires Linux OS"` (other non-Linux), or `"Already registered"`
- Each enabled option shows: green/grey dot + name + IP + vCPU count
- On creds change: clears selection and resets `vmName` form field
- Helper text shown when no VM selected: "Only powered-on Linux VMs from the selected vCenter are listed."
- Selected VM chip shows: green dot + name + IP + vCPU

**Query**:
```
queryKey: ['vmwaremachines-for-proxy', credsRef]
queryFn:  getVMwareMachines(namespace, credsRef) → filter running → map to VMOption
staleTime: 30_000
```

---

### SSHAccessSection (`components/SSHAccessSection.tsx`)

Uses `useFormContext` (must render inside `DesignSystemForm`).

**Props**: `sshKeySource`, `generatedKey`, `hasPrivateKey` (bool — whether manual key is pasted/uploaded), plus callbacks.

**Toggle**: `ToggleButtonGroup` — `"Generate Key Pair"` | `"Upload Private Key"`

**Generate Key Pair tab**:
1. Info Alert: "Generate a key pair, then add the public key to the VM's `/root/.ssh/authorized_keys` before registering." (shown before key generated)
2. "Generate Key Pair" `ActionButton` — disabled until VM selected. Calls `generateSSHKeyPair(secretName)`.
3. On success: shows Warning Alert ("Copy the public key below and add it to `/root/.ssh/authorized_keys`…"), public key in read-only `TextField` with copy button, "Regenerate" button, then `<AuthorizedKeysConfirmation />`.
4. Regenerate: calls `deleteSSHKeyPair(secretName)` then clears state (also unregisters confirmation checkbox via `shouldUnregister`).
5. Cleanup: on SSH source change or VM deselect, deletes the generated key secret.

**Upload Private Key tab**:
1. Info Alert ("Upload or paste your SSH private key…") when no key present. Warning Alert ("Before clicking Register, ensure public key is added to `authorized_keys`…") once key present (`hasPrivateKey`).
2. "Upload key file" button — reads file text → `setValue('sshPrivateKey', text)`.
3. Paste textarea `RHFTextField name="sshPrivateKey"` — validated via shared `validateSshPrivateKey` util (accepts OpenSSH, RSA, EC, PKCS#8 headers, not just OpenSSH).
4. `{hasPrivateKey && <AuthorizedKeysConfirmation />}` — shown after key pasted/uploaded.

**`AuthorizedKeysConfirmation`** (internal sub-component):
- `Controller name="authorizedKeysConfirmed"` with `shouldUnregister` + `rules={{ validate: v => v === true || 'Required' }}`
- Renders bordered Box containing `FormControlLabel` + `Checkbox`, with bold red `FormHelperText` on error
- Label: "I've added this public key to the proxy VM's `authorized_keys`."
- Unregisters (and resets validation) when unmounted (key cleared or tab switched)

**SSH key secret naming**: `${toK8sName(vmName)}-keypair` (generated), `${toK8sName(vmName)}-hot-add-ssh-key` (manual).

---

### DeployVMForm (`components/DeployVMForm.tsx`)

Rendered inside `DesignSystemForm id="create-proxy-vm-form"`. Sections:

**Info Alert** (`severity="info"`, always visible at top): "The OVA is deployed, powered on, and registered automatically. SSH keys are injected at boot — nothing else to configure."

**VMware Environment**
- VMware Credentials `RHFSelect`

**New VM**
- VM Name `RHFTextField` — validation: required, no blank/whitespace, lowercase alphanumeric + hyphens only (`/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/`), max 63 chars, no leading/trailing hyphens, must not collide with an existing vCenter VM (`vmOptions`) **or** an already-registered/deploying ProxyVM (`registeredVMNames` — same guard as the Register form's dropdown; blocks entering the name of a proxy VM that is still `Deploying`/`Pending`, not just ones already `Ready`, see GHI #2067).
- Caption: "Becomes both the vSphere VM name and the vJailbreak Proxy VM record. Must be unique."

**Deployment Target** (2-column grid)
- Datacenter `RHFSelect` — loaded from `getVCenterResources(creds)`
- Datastore `RHFSelect` — loaded from `getVCenterResources(creds, datacenter)`
- Network `RHFSelect` — same scoped query
- Cluster/Host (optional) `RHFSelect` — same scoped query

**Scoped resource queries**:
```
queryKey: ['vcenter-datacenters', vmwareCredsRef]
queryFn:  getVCenterResources(vmwareCredsRef)

queryKey: ['vcenter-scoped-resources', vmwareCredsRef, datacenter]
queryFn:  getVCenterResources(vmwareCredsRef, datacenter)
staleTime: 60_000
```

**Deployment polling** (in `AddProxyVMDrawer`):
- After `createProxyVMFromOVA` succeeds: sets `deploymentStarted = true`, records `deployedVMName`.
- Polls `getProxyVMList()` every 5s until the new VM appears in the list, then auto-closes the drawer.

---

### ProxyVMDetailDrawer (`components/ProxyVMDetailDrawer.tsx`)

Opens from clicking the Name cell in `ProxyVMsTable`. Width 760px, `requireCloseConfirmation={false}`.

**Status card**: `StatusChip` (filled) showing `validationStatus`, tone per `statusTone()` (`Ready`→success, `Deploying`→info, `Verifying`→warning, `DeployFailed`/`VerificationFailed`→error, else default). A spinner (`CircularProgress`) shows next to the chip while `Deploying` or `Verifying`. `status.validationMessage` shown below in red for `DeployFailed`/`VerificationFailed`, muted otherwise.

**General section** — `KeyValueGrid` items:

| Label | Value |
|-------|-------|
| VM name | `spec.vmName` |
| VMware credentials | `spec.vmwareCredsRef.name` |
| IP address | `status.ipAddress` |
| Attached disks | `status.attachedDiskCount` |
| Last validated | `status.lastValidationTime` |
| Components verified | `status.componentsVerified` mapped to `name ✓/✗` joined (always shown, `—` if absent) |
| Created | `metadata.creationTimestamp` |

> Note: `vJailbreak Proxy VM name` (K8s metadata.name) is NOT shown in the grid — it is displayed as the drawer title.

**SSH Access section**:

`isOVADeployed = !spec.sshKeyPairRef && !spec.sshKeySecretRef`

| Condition | Content |
|-----------|---------|
| `isOVADeployed` | "SSH access is configured automatically during OVA deployment." |
| `sshKeyPairRef` set, key loading | Spinner + "Loading public key…" |
| `sshKeyPairRef` set, load error | Warning alert naming the secret |
| `sshKeyPairRef` set, loaded | Read-only public key `TextField` with copy button |
| `sshKeySecretRef` set (manual upload) | Info alert: manually uploaded key, no public key stored |
| Not OVA-deployed, no key ref, `Ready` | Warning alert: "No SSH key configured for this vJailbreak Proxy VM." |
| Not OVA-deployed, no key ref, not yet `Ready` | "SSH key will be available once the VM is ready." |

Subtitle "SSH public key used by vJailbreak to access this Proxy VM." shown for all non-`isOVADeployed` cases.

---

## 5. API Contract

### ProxyVM CRUD

| Operation | Endpoint                                                                    | Method |
| --------- | --------------------------------------------------------------------------- | ------ |
| List      | `/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/proxyvms` | GET    |
| Get       | `…/proxyvms/{name}`                                                         | GET    |
| Create    | `…/proxyvms`                                                                | POST   |
| Delete    | `…/proxyvms/{name}`                                                         | DELETE |
| Retry     | `…/proxyvms/{name}` PATCH                                                   | PATCH — adds `force-reconcile` annotation |

### OVA Deploy

| Operation        | Endpoint                              | Method | Notes                          |
| ---------------- | ------------------------------------- | ------ | ------------------------------ |
| Deploy VM        | `/dev-api/sdk/vpw/v1/create-proxy-vm` | POST   | Returns `{ status, message }`  |

**Request body** (`CreateProxyVMFromOVARequest`):
```typescript
{
  vmName: string
  vmwareCredsRef: string
  datacenter: string
  datastore: string
  network: string
  cluster?: string
}
```

### vCenter Resources

| Operation        | Endpoint                                   | Method | Notes                                       |
| ---------------- | ------------------------------------------ | ------ | ------------------------------------------- |
| List DCs         | `/dev-api/sdk/vpw/v1/vcenter-resources`    | GET    | `?vmwareCredsRef=X`                         |
| List scoped      | `/dev-api/sdk/vpw/v1/vcenter-resources`    | GET    | `?vmwareCredsRef=X&datacenter=Y`            |

**Response** (`VCenterResources`):
```typescript
{
  datacenters: string[]
  clusters: string[]
  datastores: string[]
  networks: string[]
}
```

### SSH Key Pairs

| Operation    | API                                      | Notes                                          |
| ------------ | ---------------------------------------- | ---------------------------------------------- |
| Generate     | `generateSSHKeyPair(secretName)`         | Returns `{ publicKey }`; stores keypair secret |
| Delete       | `deleteSSHKeyPair(secretName)`           | Cleanup on cancel / VM change / regenerate     |

### SSH Key Secret (manual path)

| Operation | API                                                        | Notes                                                         |
| --------- | ---------------------------------------------------------- | ------------------------------------------------------------- |
| Create    | `createSecret(name, { 'ssh-privatekey': key }, namespace)` | Before ProxyVM POST                                           |
| Delete    | `deleteSecret(name, namespace)`                            | On ProxyVM delete + rollback on failed POST                   |

Secret name = `${toK8sName(vmName)}-hot-add-ssh-key` (manual) or `${toK8sName(vmName)}-keypair` (generated).

### VMware Machines (VM picker)

| Operation               | API                                            | Notes                      |
| ----------------------- | ---------------------------------------------- | -------------------------- |
| List VMs for credential | `getVMwareMachines(namespace, vmwareCredName)` | Filtered by label selector |

Returns `VMwareMachineList`. Drawer maps to `VMOption` (name + ipAddress + cpu + powerState + osFamily). Only `powerState === 'running'` items are fetched into the option list; non-Linux and already-registered options remain in the list but are disabled/greyed out client-side (see §4 VMAutocomplete).

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
  queryKey: ['vmwaremachines-for-proxy', credsRef]
  queryFn: getVMwareMachines(namespace, credsRef) → filter running → VMOption[]
  enabled: Boolean(credsRef) && open
  staleTime: 30_000

useQuery (deploy poll, inline in AddProxyVMDrawer)
  queryKey: [...PROXY_VMS_QUERY_KEY, 'deploy-poll']
  queryFn: getProxyVMList()
  enabled: deploymentStarted
  refetchInterval: deploymentStarted ? 5000 : false
  → auto-closes drawer when deployedVMName appears in list

useQuery (vCenter DCs)
  queryKey: ['vcenter-datacenters', vmwareCredsRef]
  queryFn: getVCenterResources(vmwareCredsRef)
  enabled: Boolean(vmwareCredsRef) && formMode === 'create' && open
  staleTime: 60_000

useQuery (vCenter scoped resources)
  queryKey: ['vcenter-scoped-resources', vmwareCredsRef, datacenter]
  queryFn: getVCenterResources(vmwareCredsRef, datacenter)
  enabled: Boolean(vmwareCredsRef) && Boolean(datacenter) && formMode === 'create' && open
  staleTime: 60_000
```

**Cache invalidation**: After `postProxyVM`, `createProxyVMFromOVA`, `deleteProxyVM`, or `retryProxyVMVerification`, invalidate `['proxyvms']`.

**`registeredVMNames`**: `useMemo(() => new Set(existingProxyVMs.map(vm => vm.spec.vmName)), [existingProxyVMs])` in `AddProxyVMDrawer`, built from `useProxyVMsQuery()` (enabled only while drawer `open`). Includes ProxyVMs in every status — `Deploying`/`Pending` included, not just `Ready` — so it also blocks re-registering/re-deploying a proxy VM name that hasn't finished provisioning yet. Passed to both `RegisterVMForm` (→ `VMAutocomplete`, disables the option) and `DeployVMForm` (→ blocks the name field's `validate` rule).

### Data Flow — Register Existing VM

```
User clicks "Add Proxy VM"
  → AddProxyVMDrawer opens (formMode = 'select')
  → Selects VMware creds → fetches VMwareMachines for that cred
  → Picks VM from VMAutocomplete (shows name + IP + vCPU)
  → SSH Access:
      Generate Key Pair path:
        → generateSSHKeyPair(secretName) → get publicKey
        → User copies publicKey to VM's authorized_keys
        → Submit → postProxyVM({ ..., sshKeyPairRef: { name: secretName } })
      Upload Private Key path:
        → User uploads/pastes private key
        → Submit → createSecret(name, { ssh-privatekey: key })
                 → postProxyVM({ ..., sshKeySecretRef: { name: secretName } })
                 → on failure: deleteSecret(name) rollback
  → invalidate ['proxyvms'] → close drawer → table shows new VM as Pending
  → Controller validates → Verifying → Ready | VerificationFailed
```

### Data Flow — Deploy New VM from OVA

```
User selects "Deploy a new vJailbreak Proxy VM" method
  → DeployVMForm shown
  → Selects creds → fetches datacenters
  → Selects datacenter → fetches datastores + networks + clusters
  → Enters VM name (validated: lowercase alnum + hyphens, no spaces, ≤63 chars)
  → Submit → createProxyVMFromOVA({ vmName, vmwareCredsRef, datacenter, datastore, network, cluster? })
  → deploymentStarted = true → polls getProxyVMList() every 5s
  → When deployedVMName appears in list → auto-closes drawer
  → (OVA deploy injects SSH keys automatically — no key setup required)
```

---

## 7. Integration with Migration Form

ProxyVM is consumed in Step 3 (Network and Storage Mapping) of both standard and rolling migration forms.

### Storage Copy Method: `HotAdd`

The internal enum value is still `'HotAdd'`, but the user-facing label is now **"vJailbreak Accelerated Copy"** (renamed from "Hot-Add via Proxy VM"). `STORAGE_COPY_METHOD_OPTIONS` in `src/features/migration/constants.ts`:

```ts
export const STORAGE_COPY_METHOD_OPTIONS = [
  { value: 'normal', label: 'Standard Copy' },
  { value: 'StorageAcceleratedCopy', label: 'Storage Accelerated Copy' },
  { value: 'HotAdd', label: 'vJailbreak Accelerated Copy' }
] as const
```

**`StorageCopyMethod` type** (`src/features/migration/types.ts`):

```ts
type StorageCopyMethod = 'normal' | 'StorageAcceleratedCopy' | 'HotAdd'
```

**Form params**: `proxyVMRef?: string` added to `FormValues` and `RollingFormParams`.

**Beta chip**: both `StorageAcceleratedCopy` and `HotAdd` radio labels show `<Chip label="Beta" color="warning" variant="outlined" />` (not `HotAdd`-only).

**Migration detail/list display**: `MigrationDetailsTab` labels `HotAdd` migrations as `"vJailbreak Accelerated Copy"` and shows an extra "vJailbreak Proxy VM" field with the proxy's name. `Phase.HotAddTransferInProgress` / `Phase.HotAddCleanup` are surfaced as distinct progress phases in `MigrationProgress`, `MigrationProgressWithPopover`, `MigrationsTable`, and `MigrationNextActionBanner`.

### NetworkAndStorageMappingStep Behavior (HotAdd)

When `storageCopyMethod === 'HotAdd'`:

- **VMware Datastore → PCD Volume Type mapping table** shown first (same table as `normal` mode)
- **Proxy VM selector** shown below it (with `FieldLabel` above):
  - Source: `useProxyVMsQuery()` (enabled only while `storageCopyMethod === 'HotAdd'`) filtered to `status.validationStatus === 'Ready'`
  - Option label: `metadata.name (status.ipAddress)` or just `metadata.name`
  - If no Ready VMs: warning Alert, select disabled
- Switching away from HotAdd clears `proxyVMRef`

### MigrationOptionsAlt Behavior (HotAdd)

When `storageCopyMethod === 'HotAdd'` (`isHotAdd`):

- `dataCopyMethod` forced to `'cold'` **only if it isn't already `'cold'` or `'mock'`** — i.e. HotAdd allows **Cold or Mock** copy, not cold-only
- Only the `hot` data-copy option is disabled in the dropdown (`disabled={isHotAdd && item.value !== 'cold' && item.value !== 'mock'}`)
- Info alert: "vJailbreak Accelerated Copy requires Cold or Mock copy. Other data copy methods are not available."

### Validation

Step 3 storage validation for HotAdd requires **both** `proxyVMRef` AND fully-mapped `storageMappings`.

### Form Submit

```
HotAdd → createStorageMapping(params.storageMappings)
       → updateMigrationTemplate: spec.proxyVMRef = { name: params.proxyVMRef }
                                   AND spec.storageMapping = storageMappings.metadata.name
```

---

## 8. Validation Rules

| Rule                          | Condition                                                                       | Error                                                                                       |
| ----------------------------- | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| proxyVMRef required           | `storageCopyMethod === 'HotAdd'` and `!params.proxyVMRef`                       | "Please select a Proxy VM to use for Hot-Add data copy"                                      |
| storageMappings required      | `storageCopyMethod === 'HotAdd'` and storage not fully mapped                   | Storage mapping required (same as `normal` mode)                                             |
| VM required (register)        | Add drawer, select mode, submit                                                 | "VM is required"                                                                              |
| vmwareCredsRef required       | Add drawer submit                                                               | "VMware credentials are required"                                                             |
| vmName required (deploy)      | Add drawer, create mode, submit                                                 | "VM name is required"                                                                          |
| vmName format (deploy)        | Must match `/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/`                                  | "Lowercase letters, numbers, and hyphens only. Cannot start or end with a hyphen."             |
| vmName blank (deploy)         | Whitespace-only input                                                           | "VM name cannot be blank"                                                                      |
| vmName length (deploy)        | > 63 characters                                                                 | "Must be 63 characters or fewer"                                                               |
| vmName vCenter collision (deploy) | Name matches a VM already in the selected vCenter (`vmOptions`)             | "A VM with this name already exists in the selected vCenter."                                 |
| vmName registered collision (deploy) | Name matches any existing ProxyVM CR, incl. `Deploying`/`Pending` ones (`registeredVMNames`) | "A vJailbreak Proxy VM with this name is already registered or deploying." |
| VM option disabled (register) | vCenter VM already registered (`registeredVMNames`) or non-Linux (`osFamily !== 'linuxGuest'`) | Option unselectable in `VMAutocomplete`; caption shows why |
| sshPrivateKey required        | Add drawer, register mode, upload tab, submit                                  | "SSH private key is required"                                                                  |
| sshPrivateKey format          | Must pass `validateSshPrivateKey` (OpenSSH, RSA, EC, or PKCS#8 headers)         | Format-specific message from `validateSshPrivateKey`                                          |
| SSH key file size             | Upload > 1 MB                                                                   | "File too large. SSH private key must be under 1 MB."                                          |
| Generated key required        | Register mode, generate tab, no key generated yet                              | "Generate a key pair first."                                                                    |
| authorizedKeysConfirmed       | Register mode; generate tab after key generated, OR upload tab after key pasted | "Required" (bold red FormHelperText below checkbox)                                            |

---

## 9. Prerequisites for Proxy VM

The VM must have these configured before registering it (documented externally — not shown in form). Applies only to "Register existing VM" path. Deploy-from-OVA path configures these automatically.

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
- HotAdd (vJailbreak Accelerated Copy) allows Cold or Mock data copy only; warm (hot) migration not supported with HotAdd
- SSH Secret name derived from `toK8sName(vmName)`. Collision possible if two VM names normalize to the same K8s-safe string.
- ProxyVM CRD is not session-scoped — persists across migrations and must be explicitly deleted
- VM picker fetches from `/vmwaremachines?labelSelector=...` — requires VMware creds already validated and VMwareMachine CRs to exist
- OVA deploy endpoint is at `/dev-api/sdk/vpw/v1/create-proxy-vm` — proxied through the API server
- vCenter resources endpoint is at `/dev-api/sdk/vpw/v1/vcenter-resources` — scoped by `datacenter` param for second-level resources
- Windows guest VMs (`osFamily === 'windowsGuest'`) and any non-`linuxGuest` VM cannot be selected as a proxy VM (GHI #2088) — enforced client-side only in `VMAutocomplete`, not by the controller
- Duplicate-name guard (`registeredVMNames`, GHI #2067) is also client-side only — a race between two clients (or direct API/kubectl use) could still create two ProxyVM CRs whose `spec.vmName` collide; the controller itself does not reject this
- Retry (`force-reconcile` annotation) only surfaces in the UI for `VerificationFailed` rows; `DeployFailed` rows have no retry action from the table
