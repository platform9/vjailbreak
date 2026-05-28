# Migration Feature — Technical Specification

**Feature path**: `src/features/migration/`  
**Generated**: 2026-05-20  
**Updated**: 2026-05-27 (HotAdd proxy VM integration)  
**Coverage**: 73 files — api, components, context, hooks, pages, steps, utils

---

## Quick-Start Context (read this first in a new session)

> **Purpose**: This spec gives an LLM or new developer full context on the migration feature. Read §1–2 for architecture orientation, §3 for specific component APIs, §4 for API contracts, §5 for validation rules.

### What this feature does
Orchestrates VMware → PCD (OpenStack) VM migrations. Two modes: **standard** (one-time, per-VM) and **rolling** (cluster-wide with ESXi host lifecycle). Both modes go through a multi-step drawer form that creates `MigrationPlan` + `MigrationTemplate` Kubernetes CRDs.

### Key files to know

| What | File |
|------|------|
| Standard form (6 steps) | `src/features/migration/pages/MigrationForm.tsx` |
| Rolling form (8 steps) | `src/features/migration/pages/RollingMigrationForm.tsx` |
| Migrations list page | `src/features/migration/pages/MigrationsPage.tsx` |
| All form types/interfaces | `src/features/migration/types.ts` |
| Constants (enums, options) | `src/features/migration/constants.ts` |
| Step 3: Network+Storage | `src/features/migration/steps/NetworkAndStorageMappingStep.tsx` |
| Step 5: Advanced options | `src/features/migration/steps/MigrationOptionsAlt.tsx` |
| Standard validation | `src/features/migration/hooks/useFormValidation.ts` |
| Rolling validation | `src/features/migration/hooks/useRollingFormValidation.ts` |
| Standard submit | `src/features/migration/hooks/useMigrationFormSubmit.ts` |
| Rolling submit | `src/features/migration/hooks/useRollingFormSubmit.ts` |
| MigrationTemplate model | `src/api/migration-templates/model.ts` |
| MigrationPlan model | `src/api/migration-plans/model.ts` |

### Storage copy methods (Step 3)

| Value | Storage UI | Submit behavior |
|-------|-----------|-----------------|
| `normal` | Datastore → Volume Type mapping | POST `/storage-mappings`, patch template `storageMapping` |
| `StorageAcceleratedCopy` | Datastore → Array Creds mapping | POST `/arraycreds-mapping`, patch template `arrayCredsMapping` |
| `HotAdd` | ProxyVM selector dropdown | No storage mapping POST; patch template `proxyVMRef: { name }` |

`HotAdd` added in May 2026. Related spec: `docs/specs/proxyvms/proxyvms-feature.md`.

### Form state architecture (critical to understand)

- Primary state: `params` (plain `useState` object) — holds all field values
- Secondary state: `react-hook-form` manages a subset of fields (`securityGroups`, `serverGroup`, time fields)
- Sync: `useFormSync` / `useRollingFormSync` keep both in sync bidirectionally
- **Do not add new fields to RHF unless they need RHF validation.** Add to `FormValues` in `types.ts` and access via `params`.

### Common pitfalls for new developers

1. **Two VM types**: `VmDataWithFlavor` (standard) vs `VM` (rolling) — bridged by `vmAdapters.ts → CanonicalVM`. Always update both types when changing VM shape.
2. **Session-scoped K8s resources**: VMwareCreds, OpenstackCreds, MigrationTemplate are created per form session and must be deleted on close (`handleClose`). Orphans persist if browser crashes.
3. **StorageCopyMethod type**: Lives in `types.ts`. When adding new copy methods, update: `StorageCopyMethod` type, `STORAGE_COPY_METHOD_OPTIONS` constant, `useFormValidation`, `useRollingFormValidation`, `useMigrationFormSubmit`, `useRollingFormSubmit`, `NetworkAndStorageMappingStep`, `MigrationOptionsAlt`.
4. **Namespace**: `VJAILBREAK_DEFAULT_NAMESPACE = 'migration-system'` in `src/api/constants.ts`.
5. **Template polling**: `MigrationTemplate` is polled every 3s after creation. VMs/networks/storage only available after controller populates `status`. Form cannot proceed until then.

---

---

## 1. Feature Overview

### Purpose and Business Context

The Migration feature is the core of vJailbreak: it orchestrates the migration of VMware VMs to Platform9 Private Cloud Director (PCD) running on OpenStack. Users configure source (VMware cluster/credentials), destination (PCD cluster/OpenStack credentials), select VMs, map networks and storage, configure advanced options, then submit a migration plan that creates Kubernetes CRDs (`MigrationPlan`, `MigrationTemplate`) which the backend controller processes.

Two migration modes exist:

- **Standard migration** — Migrate selected VMs from a VMware cluster to a PCD tenant. Individual one-time operation.
- **Rolling migration** — Migrate an entire VMware cluster incrementally, including ESXi host lifecycle management via bare metal (MAAS) config and PCD host configs.

### High-Level User Journey

```
1. Open migration drawer (standard or rolling)
2. Select source VMware cluster + destination PCD cluster
3. (Rolling only) View bare metal config + assign host configs to ESXi hosts
4. Select VMs → assign OS family, flavor, IP addresses
5. Map source networks → PCD networks
6. Map source datastores → PCD volume types (or array creds for accelerated copy)
7. (Optional) Configure security groups, server groups, image profiles
8. (Optional) Configure data copy method, cutover window, post-migration scripts
9. Review preview summary
10. Submit → MigrationPlan + MigrationTemplate created → controller executes
11. Monitor progress in MigrationsPage (polling, phase steps, pod logs)
```

### Key Actors and Entry Points

| Actor | Entry Point |
|-------|-------------|
| Standard migration | "Start Migration" button in toolbar → opens `MigrationFormDrawer` |
| Rolling migration | Menu/nav action → opens `RollingMigrationFormDrawer` |
| Monitor migrations | `/migrations` route → `MigrationsPage` |
| Open form programmatically | `useMigrationFormActions()` context hook |

---

## 2. Architecture Map

### Module/Folder Structure

```
src/features/migration/
├── api/
│   ├── migration-plans/
│   │   ├── helpers.ts          — Build MigrationPlan CRD JSON from form params
│   │   ├── migrationPlans.ts   — CRUD API calls for MigrationPlan CRDs
│   │   └── model.ts            — MigrationPlan TypeScript interfaces
│   ├── migration-templates/
│   │   ├── helpers.ts          — Build MigrationTemplate CRD JSON from form params
│   │   ├── migrationTemplates.ts — CRUD API calls for MigrationTemplate CRDs
│   │   └── model.ts            — MigrationTemplate TypeScript interfaces
│   ├── migrationPlans.ts       — Re-export barrel
│   ├── migrations.ts           — Re-export barrel (migrations API + model)
│   └── useMigrationPlanDestinationsQuery.ts — React Query hook: resolve cluster+tenant per migration
├── components/
│   ├── cells/
│   │   ├── OsFamilyCell.tsx    — OS family selector table cell
│   │   ├── RollingFlavorCell.tsx — Flavor selector for rolling mode
│   │   ├── RollingIpAddressCell.tsx — IP display for rolling mode
│   │   ├── StandardIpAddressCell.tsx — IP display for standard mode
│   │   └── index.ts            — Cell exports
│   ├── BaseLogsDrawer.tsx      — Reusable log viewer (search, filter, copy, download)
│   ├── BulkIPEditDialog.tsx    — Bulk IP + preserve IP/MAC dialog
│   ├── ControllerLogsDrawer.tsx — Controller pod logs drawer
│   ├── FlavorAssignmentDialog.tsx — Bulk flavor assignment dialog
│   ├── HostConfigAssignmentDialog.tsx — Bulk host config assignment dialog
│   ├── LogLine.tsx             — Syntax-highlighted log line renderer
│   ├── MaasConfigDetailDialog.tsx — MAAS config details (styled dialog, syntax highlighted)
│   ├── MigrationProgress.tsx   — Status icon + text for migration phases
│   ├── MigrationProgressWithPopover.tsx — Progress icon + stepper popover
│   ├── MigrationsTable.tsx     — Main DataGrid for migration list
│   ├── MissingInterfaceIpWarningAlert.tsx — Alert for VMs with missing IPs
│   ├── missingInterfaceIpWarnings.ts — Logic to detect missing IP warnings
│   ├── PodLogsDrawer.tsx       — Pod logs with download bundle support
│   ├── RdmDiskConfigurationPanel.tsx — RDM disk → Cinder backend config
│   ├── ResourceMappingTable.tsx — Read-only mapping table with delete
│   ├── ResourceMappingTableNew.tsx — RHF-integrated mapping table
│   ├── TriggerAdminCutover/
│   │   └── TriggerAdminCutoverButton.tsx — Trigger cutover with confirmation
│   ├── UpgradeModal.tsx        — vJailbreak version upgrade flow
│   └── index.ts                — Component exports
├── context/
│   └── MigrationFormContext.tsx — Context to open migration forms globally
├── hooks/
│   ├── useBulkIPEdit.ts        — Standard mode bulk IP state + validation
│   ├── useBulkIPHandlers.ts    — Rolling mode bulk IP state + API patching
│   ├── useClusterData.ts       — VMware + PCD cluster data fetching
│   ├── useCredentialFetching.ts — Credential fetch + template creation/polling
│   ├── useFilteredMappings.ts  — Filter/auto-map network+storage mappings
│   ├── useFlavorAssignment.ts  — Standard flavor dialog + bulk assignment
│   ├── useFlavorHandlers.ts    — Rolling flavor individual + bulk handlers
│   ├── useFormSync.ts          — Bidirectional RHF ↔ params sync (standard)
│   ├── useFormValidation.ts    — Step completion + error flags (standard)
│   ├── useHostConfigHandlers.ts — ESXi host → PCD host config assignment
│   ├── useMigrationFormSubmit.ts — Standard form submit + cleanup
│   ├── useMigrationsQuery.ts   — React Query for migrations list
│   ├── useMigrationStatusMonitor.ts — Phase change tracking + analytics
│   ├── useNetworkIPsMap.ts     — Aggregate IPs per network from selected VMs
│   ├── useNetworkSubnetCompatibility.ts — Validate VM IPs vs subnet CIDRs
│   ├── useOsAssignment.ts      — OS family assignment with API persistence
│   ├── useRdmConfiguration.ts  — RDM disk config dialog + apply flow
│   ├── useRollingColumns.tsx   — Rolling mode DataGrid column definitions
│   ├── useRollingFormData.ts   — Rolling mode data fetching (hosts, VMs, MAAS)
│   ├── useRollingFormSubmit.ts — Rolling form submit orchestration
│   ├── useRollingFormSync.ts   — Bidirectional RHF ↔ params sync (rolling)
│   ├── useRollingFormValidation.ts — Step completion + error flags (rolling)
│   ├── useSectionTracking.ts   — IntersectionObserver active section tracking
│   ├── useStandardColumns.tsx  — Standard mode DataGrid column definitions
│   ├── useToast.ts             — Toast notification state management
│   ├── useVmSelection.ts       — VM selection state + form sync (standard)
│   └── useVmsSelectionState.ts — VmsSelectionStep state orchestrator
├── pages/
│   ├── MigrationForm.tsx       — Standard migration drawer (full 6-step flow)
│   ├── MigrationsPage.tsx      — Migrations list page with delete + monitor
│   └── RollingMigrationForm.tsx — Rolling migration drawer (7-step flow)
├── steps/
│   ├── MigrationOptionsAlt.tsx — Advanced options step (copy, cutover, scripts)
│   ├── NetworkAndStorageMappingStep.tsx — Network + storage mapping step
│   ├── SecurityGroupAndServerGroup.tsx — Security groups + server groups step
│   ├── SourceDestinationClusterSelection.tsx — Cluster + cred selection step
│   └── VmsSelectionStep.tsx    — Dual-mode VM selection DataGrid step
├── utils/
│   ├── ipValidation.ts         — IP address parsing and validation utilities
│   ├── migrationTableUtils.ts  — Phase → step number + progress text
│   ├── vmAdapters.ts           — Convert between VmDataWithFlavor ↔ VM ↔ CanonicalVM
│   └── vmNetworking.ts         — VM network interface existence checks
├── constants.ts                — Enums, option arrays, default values
└── types.ts                    — Feature-wide TypeScript interfaces
```

### Component Hierarchy

```
MigrationsPage
└── MigrationsTable
    ├── MigrationProgressWithPopover (per row)
    ├── TriggerAdminCutoverButton (per eligible row)
    └── PodLogsDrawer → BaseLogsDrawer → LogLine[]

MigrationFormDrawer (standard)
├── SourceDestinationClusterSelection (step 1)
├── VmsSelectionStep mode="standard" (step 2)
│   ├── FlavorAssignmentDialog
│   ├── BulkIPEditDialog
│   └── RdmDiskConfigurationPanel
├── NetworkAndStorageMappingStep (step 3)
│   ├── ResourceMappingTableNew (networks)
│   └── ResourceMappingTableNew (storage / array creds)
├── SecurityGroupAndServerGroup (step 4)
├── MigrationOptionsAlt (step 5)
└── Preview Card (step 6)

RollingMigrationFormDrawer (rolling)
├── SourceDestinationClusterSelection (step 1)
├── Bare Metal Config view (step 2) → MaasConfigDetailDialog
├── ESXi Hosts DataGrid (step 3) → HostConfigAssignmentDialog
├── VmsSelectionStep mode="rolling" (step 4)
│   ├── FlavorAssignmentDialog
│   └── BulkIPEditDialog
├── NetworkAndStorageMappingStep (step 5)
├── SecurityGroupAndServerGroup (step 6)
├── MigrationOptionsAlt (step 7)
└── Preview Card (step 8)
```

### Data Flow

```
User selects cluster
  → useClusterData (VMware clusters + PCD clusters)
  → useCredentialFetching (VMwareCreds, OpenstackCreds, MigrationTemplate)
    → MigrationTemplate polled every 3s until status populated
    → Template status.vmware[] = available VMs
    → Template status.openstack = { networks[], volumeTypes[] }

User selects VMs
  → useVmsSelectionState
    → standard: useVMwareMachinesQuery → VmDataWithFlavor[]
    → rolling: useRollingFormData → VM[]
  → useVmSelection / useVmsSelectionState → selectedVMs (Set/GridRowSelectionModel)
  → useStandardColumns / useRollingColumns → DataGrid column defs
  → useOsAssignment → vmOSAssignments (patchVMwareMachine on change)
  → useFlavorAssignment / useFlavorHandlers → targetFlavorId (patchVMwareMachine on apply)
  → useBulkIPEdit / useBulkIPHandlers → preserveIp / preserveMac / IP overrides

User maps networks/storage
  → useFilteredMappings → filteredNetworkMappings, filteredStorageMappings
  → useNetworkSubnetCompatibility → subnetWarnings (debounced API call)

User submits
  → useMigrationFormSubmit / useRollingFormSubmit
    1. POST /network-mappings
    2. POST /storage-mappings or /arraycreds-mapping
    3. PATCH /migration-templates/{name} (attach mappings)
    4. POST /migration-plans → navigate to MigrationsPage

Migration monitoring
  → useMigrationsQuery (staleTime: Infinity, refetchOnWindowFocus)
  → MigrationsPage adaptive refetch: 5s (pending) / 30s (stable)
  → useMigrationStatusMonitor → Amplitude events on phase changes
  → useMigrationPlanDestinationsQuery → resolve cluster+tenant per plan
```

### Shared Dependencies

| Dependency | Used by |
|-----------|---------|
| `MigrationFormContext` | Any component needing to open the form |
| `useAmplitude()` | useMigrationFormSubmit, useMigrationStatusMonitor, useRollingFormSubmit |
| `useErrorHandler()` / `reportError` | All submit hooks, useBulkIPEdit, useOsAssignment |
| `@tanstack/react-query` | All query hooks, UpgradeModal |
| `react-hook-form` | MigrationForm, RollingMigrationForm, ResourceMappingTableNew, all *Sync hooks |
| `@mui/x-data-grid` | VmsSelectionStep, MigrationsTable |
| `VJAILBREAK_DEFAULT_NAMESPACE` | All API calls requiring namespace |

---

## 3. Component Specifications

### MigrationFormDrawer (`pages/MigrationForm.tsx`)

**Purpose**: Top-level drawer managing the complete 6-step standard migration workflow.

**Props**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `open` | `boolean` | Yes | Controls drawer visibility |
| `onClose` | `() => void` | Yes | Close callback |
| `onSuccess` | `() => void` | Yes | Post-submission success callback |

**Internal State**:

| Name | Type | Initial | Triggers Change |
|------|------|---------|----------------|
| `sessionId` | `string` | `uuid()` | Never (stable per open) |
| `params` | `FormValues` | `{}` | Every field update via `onChange` |
| `fieldErrors` | `FieldErrors` | `{}` | Validation hook side effects |
| `selectedMigrationOptions` | `SelectedMigrationOptionsType` | all false | User toggles checkboxes in step 5 |
| `touchedSections` | `{ options: boolean }` | false | User interacts with step 5 |
| `activeSectionId` | `string` | `'source-destination'` | Scroll via `useSectionTracking` |

**User Interactions → Effects**:

| Action | Effect |
|--------|--------|
| Select VMware cluster | Populates `params.vmwareCreds`, triggers credential fetching |
| Select PCD cluster | Populates `params.openstackCreds`, triggers credential fetching |
| Select VMs | Populates `params.vms[]` |
| Add network mapping | Updates `params.networkMappings[]` |
| Toggle migration option checkbox | Updates `selectedMigrationOptions` |
| Click Submit | Calls `handleSubmit()`, shows spinner, navigates on success |
| Click Close/Cancel | Calls `handleClose()` which cleans up temp credentials/templates |

**Rendered Variants**:
- Default: All steps visible, section nav on left
- Mobile (`isSmallNav`): Section nav hidden, tab selector shown at top
- Submitting: Submit button shows spinner + disabled
- Step error: Section nav icon shows error badge

**Edge Cases**:
- Session ID ensures temp K8s resources are unique per form open
- If user closes mid-flow, `handleClose` deletes temp VMwareCreds, OpenstackCreds, MigrationTemplate

---

### RollingMigrationFormDrawer (`pages/RollingMigrationForm.tsx`)

**Purpose**: Drawer for cluster conversion with ESXi host lifecycle management.

**Props**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `open` | `boolean` | Yes | Controls drawer visibility |
| `onClose` | `() => void` | Yes | Close callback |

**Internal State**: 15+ state variables spanning selection, form params, section tracking, and dialog states. See `RollingMigrationForm.tsx`.

**Step Structure**:

| # | Section ID | Component | Notes |
|---|-----------|-----------|-------|
| 1 | `source-destination` | SourceDestinationClusterSelection | |
| 2 | `baremetal` | Inline (MAAS view) | Opens MaasConfigDetailDialog |
| 3 | `hosts` | Inline DataGrid | Opens HostConfigAssignmentDialog |
| 4 | `vms` | VmsSelectionStep mode="rolling" | |
| 5 | `map-resources` | NetworkAndStorageMappingStep | |
| 6 | `security` | SecurityGroupAndServerGroup | |
| 7 | `options` | MigrationOptionsAlt | |
| 8 | `preview` | Inline summary card | |

---

### MigrationsPage (`pages/MigrationsPage.tsx`)

**Purpose**: Lists all migrations with delete, bulk admin cutover, and real-time monitoring.

**Props**: None (page-level route component)

**Adaptive Refetch**:
- Has pending migration → refetch every 5s
- No pending → refetch every 30s
- Also `refetchOnWindowFocus: true`

**Delete Flow**:
1. User selects migrations → clicks Delete
2. Confirmation dialog shown
3. On confirm: `deleteMigration()` for each → `getMigrationPlan()` → `patchMigrationPlan()` to remove VM from plan
4. Success snackbar

---

### MigrationsTable (`components/MigrationsTable.tsx`)

**Purpose**: DataGrid with filtering, selection, bulk cutover, pod log viewer.

**Props**:

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `migrations` | `Migration[]` | Yes | Migrations to display |
| `onDeleteMigration` | `(name: string) => void` | No | Single delete handler |
| `onDeleteSelected` | `(migrations: Migration[]) => void` | No | Bulk delete handler |
| `refetchMigrations` | `() => void` | Yes | Trigger refetch |
| `loading` | `boolean` | No | Show loading overlay |

**Columns**: Name, Status, Agent, Time Elapsed, Destination (cluster + tenant), Progress, Actions

**Filter Options**:
- Status: All / Succeeded / Failed / In Progress
- Date: All Time / Last 24h / Last 7 days / Last 30 days

**Internal State**:

| Name | Type | Initial |
|------|------|---------|
| `selectedRows` | `GridRowSelectionModel` | `[]` |
| `statusFilter` | `string` | `'All'` |
| `dateFilter` | `string` | `'All Time'` |
| `logsDrawerOpen` | `boolean` | `false` |
| `selectedPod` | `object \| null` | `null` |
| `bulkCutoverDialogOpen` | `boolean` | `false` |

---

### VmsSelectionStep (`steps/VmsSelectionStep.tsx`)

**Purpose**: Dual-mode VM selection grid — standard (hook-managed) or rolling (parent-owned state).

**Props**: See `VmsSelectionStepProps` in `types.ts` — 24 props total, most optional and mode-specific.

**Key Behaviors**:
- Standard mode: Fetches own VMs, manages selection/flavor/IP/RDM state via hooks
- Rolling mode: Accepts parent-owned `vmsWithAssignments` + `selectedVMs`, parent callbacks for changes
- `isRowSelectable()`: Prevents selecting already-migrated VMs or VMs without required flavor
- RDM alert appears when selected VMs contain RDM disks

**Toolbar Actions** (visible when VMs selected):
- Assign Flavor → opens `FlavorAssignmentDialog`
- Assign IP (powered-off only) → opens `BulkIPEditDialog`
- Configure RDM (standard, if RDM VMs) → opens `RdmDiskConfigurationPanel`

---

### SourceDestinationClusterSelection (`steps/SourceDestinationClusterSelection.tsx`)

**Purpose**: Searchable cluster pickers for source VMware cluster and destination PCD cluster.

**Props**:

| Name | Type | Required |
|------|------|----------|
| `onChange` | `(id: string) => (value: unknown) => void` | Yes |
| `errors` | `FieldErrors` | Yes |
| `vmwareCluster` | `string` | No |
| `pcdCluster` | `string` | No |
| `loadingVMware` / `loadingPCD` | `boolean` | No |

**VM Cluster ID Format**: `"credName:clusterName"` (colon-separated compound key)

---

### NetworkAndStorageMappingStep (`steps/NetworkAndStorageMappingStep.tsx`)

**Purpose**: Map VMware networks → PCD networks; VMware datastores → PCD volume types, array credentials, or select a Proxy VM.

**Storage Copy Method**:
- `normal` → storage mappings (datastore → volume type)
- `StorageAcceleratedCopy` → array creds mappings (datastore → array cred)
- `HotAdd` → ProxyVM selector dropdown (no datastore mapping; select Ready ProxyVM CRD)

**HotAdd behavior**:
- Fetches `useProxyVMsQuery()` filtered to `status.validationStatus === 'Ready'`
- Each option label: `metadata.name (status.ipAddress)`
- Selection stored as `params.proxyVMRef`
- If no Ready VMs exist: warning Alert shown
- `unmappedStorageCount` returns 0 (storage mapping not applicable)

**Subnet Warnings**: Shown per source network when VM IPs don't match target subnet CIDR.

---

### SecurityGroupAndServerGroup (`steps/SecurityGroupAndServerGroup.tsx`)

**Purpose**: Optional security groups, server groups, volume image profiles.

**Profile Conflict Validation**: If two selected profiles define the same key with different values, shows error and blocks submission.

---

### MigrationOptionsAlt (`steps/MigrationOptionsAlt.tsx`)

**Purpose**: Advanced migration settings — data copy method, cutover scheduling, post-migration actions and scripts.

**Key Constraints**:
- `StorageAcceleratedCopy` hides data copy + cutover (not applicable)
- `HotAdd` forces `dataCopyMethod = 'cold'`, disables `hot` and `mock` options, shows helper note
- `PowerOffThenCopy` (cold) disables cutover options (cutover automatic)
- Mixed OS VMs require OS tags in post-migration script
- `hasL2Network` disables fallback to DHCP

---

### BaseLogsDrawer (`components/BaseLogsDrawer.tsx`)

**Purpose**: Generic reusable log viewer with fuzzy search, level filtering, auto-scroll, copy, and download.

**Props**: 10 props including `logs: string[]`, `isLoading`, `error`, `isPaused`, `onPausedChange`, `onReconnect`, `onDownload`.

**Search**: Powered by `fuse.js` (fuzzy search across filtered logs).

**Log Levels**: ALL, ERROR, WARN, INFO, DEBUG

---

### ResourceMappingTableNew (`components/ResourceMappingTableNew.tsx`)

**Purpose**: RHF-integrated mapping table with auto-add on selection and delete.

---

## 4. API Contract

### MigrationPlan CRUD

| Operation | Endpoint | Method | Payload | Response |
|-----------|----------|--------|---------|---------|
| List | `/api/v1/namespaces/{ns}/migrationplans` | GET | — | `GetMigrationPlansList` |
| Get | `/api/v1/namespaces/{ns}/migrationplans/{name}` | GET | — | `MigrationPlan` |
| Create | `/api/v1/namespaces/{ns}/migrationplans` | POST | `MigrationPlan` CRD JSON | `MigrationPlan` |
| Delete | `/api/v1/namespaces/{ns}/migrationplans/{name}` | DELETE | — | `MigrationPlan` |
| Patch | `/api/v1/namespaces/{ns}/migrationplans/{name}` | PATCH (merge) | Partial `MigrationPlan` | `MigrationPlan` |

**MigrationPlan spec fields** (built by `createMigrationPlanJson`):
```typescript
{
  migrationTemplate: string           // Template name reference
  migrationStrategy: { type: string } // "cold" | "warm" | ...
  virtualMachines: string[][]         // Array of VM name arrays
  retry: boolean
  advancedOptions?: { ... }
  postMigrationActions?: { ... }
  securityGroups?: string[]
  serverGroup?: string
}
```

---

### MigrationTemplate CRUD

| Operation | Endpoint | Method |
|-----------|----------|--------|
| List | `/api/v1/namespaces/{ns}/migrationtemplates` | GET |
| Get | `/api/v1/namespaces/{ns}/migrationtemplates/{name}` | GET |
| Create | `/api/v1/namespaces/{ns}/migrationtemplates` | POST |
| Update | `/api/v1/namespaces/{ns}/migrationtemplates/{name}` | PUT |
| Patch | `/api/v1/namespaces/{ns}/migrationtemplates/{name}` | PATCH (merge) |
| Delete | `/api/v1/namespaces/{ns}/migrationtemplates/{name}` | DELETE |

**MigrationTemplate spec fields** (built by `createMigrationTemplateJson`):
```typescript
{
  source: { vmwareRef: string }
  destination: { openstackRef: string }
  networkMapping: string      // NetworkMapping CRD name
  storageMapping: string      // StorageMapping CRD name
  targetPCDClusterName?: string
  useFlavorless?: boolean
}
```

**Template status** (populated by controller, polled every 3s):
```typescript
{
  openstack: { networks: string[], volumeTypes: string[] }
  vmware: VmData[]  // Available VMs with metadata
}
```

---

### IP Validation

| Endpoint | Method | Payload | Response |
|----------|--------|---------|---------|
| `/validateOpenstackIPs` | POST | `{ credentials, ips: string[] }` | `{ valid: boolean, conflicts: string[] }` |

**Error handling**: Shows per-IP validation status; blocks Apply button on errors.

---

### VMware Machine Patch (OS, Flavor, IP Assignment)

| Endpoint | Method | Payload |
|----------|--------|---------|
| `/vmware-machines/{vmId}` | PATCH | `{ osFamily? } \| { targetFlavorId? } \| { networkInterfaces? }` |

Used by: `useOsAssignment`, `useFlavorAssignment`, `useFlavorHandlers`, `useBulkIPEdit`, `useBulkIPHandlers`

---

### Network/Storage Mappings

| Resource | Endpoint | Method | When used |
|----------|----------|--------|-----------|
| Network Mapping | `/network-mappings` | POST | Always (if networks exist) |
| Storage Mapping | `/storage-mappings` | POST | `storageCopyMethod === 'normal'` |
| Array Creds Mapping | `/arraycreds-mapping` | POST | `storageCopyMethod === 'StorageAcceleratedCopy'` |
| ProxyVM (patch template) | template PATCH only | — | `storageCopyMethod === 'HotAdd'` — no mapping POST; `proxyVMRef: { name }` set directly on template |

**HotAdd template patch** (`useMigrationFormSubmit.ts` + `useRollingFormSubmit.ts`):
```typescript
// When storageCopyMethod === 'HotAdd'
spec: {
  storageCopyMethod: 'HotAdd',
  proxyVMRef: { name: params.proxyVMRef },
  networkMapping: networkMappings.metadata.name
}
```

---

### Migration Submissions

| Type | Endpoint | Method |
|------|----------|--------|
| Standard plan | `/api/v1/namespaces/{ns}/migrationplans` | POST |
| Rolling plan | `/api/v1/namespaces/{ns}/rollingmigrationplans` | POST |

---

### Admin Cutover

| Endpoint | Method | Payload |
|----------|--------|---------|
| `triggerAdminCutover(namespace, migrationName)` | POST | `{ namespace, name }` |

---

### Upgrade

| Endpoint | Method |
|----------|--------|
| `getAvailableTags()` | GET |
| `initiateUpgrade(version, flag)` | POST |
| `getUpgradeProgress()` | GET (polled every 3s) |
| `cleanupApiCall()` | POST |

---

## 5. Validation Rules

### Step 1 — Source and Destination

| Field | Rule | Error |
|-------|------|-------|
| `vmwareCluster` | Required | "Source cluster is required" |
| `pcdCluster` | Required | "Destination cluster is required" |

Trigger: On submit + when parent checks `isStep1Complete`.

---

### Step 2 — VM Selection

| Rule | Condition | Error |
|------|-----------|-------|
| At least 1 VM selected | `params.vms.length === 0` | "Select at least one VM" |
| No powered-off VMs without OS assignment | Powered-off VM + no osFamily | "OS family required for powered-off VMs" |
| No IP validation errors | Any VM IP `ipValidationStatus === 'invalid'` | "Fix IP validation errors before continuing" |
| RDM disks configured | RDM VMs selected + incomplete config | "Configure RDM disk settings" |

Trigger: On selection change + on submit.

---

### Step 3 — Network and Storage Mapping

| Rule | Condition | Error |
|------|-----------|-------|
| All networks mapped | `unmappedNetworksCount > 0` | Network mapping required (count shown) |
| All storage mapped | `unmappedStorageCount > 0` (not applicable for HotAdd) | Storage mapping required |
| Array creds present | `StorageAcceleratedCopy` + no validated creds | Warning shown |
| Proxy VM selected | `HotAdd` + `!proxyVMRef` | "Please select a Proxy VM to use for Hot-Add data copy" |

Trigger: On mapping change + on submit.

---

### Step 4 — Security Groups (Optional)

| Rule | Condition | Error |
|------|-----------|-------|
| No profile conflicts | Two profiles same key, different values | "Conflicting profile properties" |

Trigger: On profile selection change.

---

### Step 5 — Migration Options (Optional, when selected)

| Field | Rule | Error |
|-------|------|-------|
| `dataCopyStartTime` | Valid datetime, not in past | "Invalid date/time" |
| `cutoverStartTime` | Valid datetime | "Invalid date/time" |
| `cutoverEndTime` | Valid datetime, after start | "End must be after start" |
| `postMigrationScript` | If mixed OS: must contain OS tags | "Script must include OS-specific tags" |
| `periodicSyncInterval` | Positive integer | "Invalid interval" |

Trigger: On field blur + on submit.

---

### Rolling Mode — Additional Validation

| Rule | Condition | Error |
|------|-----------|-------|
| All ESXi hosts have host config | Host without PCD host config | "All ESXi hosts must have host config assigned" |
| All powered-off VMs have IP | Powered-off VM + no IP | "Assign IPs to all powered-off VMs" |
| All VMs have OS assigned | VM without osFamily | "Assign OS family to all VMs" |

---

### IP Validation (via OpenStack API)

- Triggered on IP change (debounced)
- Shows per-interface status: `pending` / `validating` / `valid` / `invalid`
- `invalid` blocks bulk IP apply

---

## 6. State and Data Flow

### Global State

| Store/Context | Used For |
|---------------|---------|
| `MigrationFormContext` | Open standard/rolling form from any component |
| `@tanstack/react-query` cache | Migrations list, VMware machines, PCD credentials, clusters |
| React Hook Form (`useForm`) | `securityGroups`, `serverGroup`, scheduling time fields, post-migration action fields |

### Form State Architecture

- **Primary store**: `params` (plain object via `useParams`/`useState`) — all form field values
- **RHF fields**: A subset of fields managed via `react-hook-form` for fields needing validation + controller integration (`MigrationDrawerRHFValues`, `RollingMigrationRHFValues`)
- **Sync**: `useFormSync` / `useRollingFormSync` keep `params` ↔ RHF bidirectionally in sync

### State Lifecycle

| Event | State Change |
|-------|-------------|
| Drawer opens | New `sessionId` generated; `params` reset to defaults |
| Cluster selected | Credential fetch triggered; template created |
| Template status populated | VM list + OpenStack networks/storage available |
| VM selected | `params.vms` updated; form sync triggers |
| Form submitted | Loading state; on success → query invalidation → navigate |
| Form closed | `handleClose` deletes session-scoped K8s resources |

### Derived State (via `useFormValidation` / `useRollingFormValidation`)

All step completion flags, error flags, and section nav items are computed via `useMemo` — no explicit synchronization needed.

### React Query Keys

| Key | Data |
|-----|------|
| `['migrations', namespace]` | All migrations |
| `['migrationTemplates']` | Template list |
| `['migrationPlans']` | Plan list |
| `['vmwareMachines', credName, clusterName]` | VM list per cluster |
| `['rdmDisks', ...]` | RDM disks |
| `['availableTags']` | Available upgrade versions |
| `['proxyvms', namespace?]` | ProxyVM list — polled 5s while Pending/Verifying |

---

## 7. User Flows

### Flow 1 — Happy Path: Standard Migration

**Preconditions**: VMware credentials and PCD credentials exist and are valid.

1. User clicks "Start Migration" → `MigrationFormDrawer` opens
2. Step 1: Selects VMware cluster from dropdown → PCD cluster from dropdown
   - System: Fetches VMwareCreds + OpenstackCreds → creates MigrationTemplate → polls until status populated
3. Step 2: VMs load in DataGrid → User selects 3 VMs
   - System: Sets `params.vms`, updates toolbar count
4. User assigns OS family to powered-off VMs → `patchVMwareMachine` called
5. (Optional) User opens Flavor Assignment dialog → selects flavor → Apply → `patchVMwareMachine` bulk
6. Step 3: Network mappings auto-populated → User maps unmapped networks
7. Step 3: Storage mappings auto-populated → User maps unmapped datastores
8. Step 4: (Skip) No security groups needed
9. Step 5: (Skip) Use default cold migration
10. User clicks Submit → `handleSubmit`:
    - POST /network-mappings
    - POST /storage-mappings
    - PATCH /migration-templates/{name}
    - POST /migration-plans
11. System navigates to MigrationsPage
12. Migrations appear in table with "Pending" status → auto-updates to Running → Succeeded

**Success**: Migration plan CRD created; VMs migrated to PCD.

**Failure Conditions**:
- No VMware/PCD credentials → Step 1 disabled or error shown
- Template polling timeout → Error alert
- Network mapping incomplete → Submit blocked, section nav shows error
- API error on submit → Error toast shown; user can retry

---

### Flow 2 — Happy Path: Rolling Migration

**Preconditions**: VMware cluster with ESXi hosts; PCD cluster with host configs configured.

1. User opens Rolling Migration drawer
2. Step 1: Select source VMware cluster + destination PCD cluster
3. Step 2: View MAAS bare metal config → click config to see details in drawer
4. Step 3: ESXi hosts table loads → User assigns PCD host config to all hosts (individually or bulk)
5. Step 4: VMs grouped by ESXi host → User selects VMs → assigns OS, flavor, IPs
6. Step 5: Map networks and storage
7. Step 6: (Optional) Security groups
8. Step 7: Set cutover time window
9. Submit → creates RollingMigrationPlan CRD
10. Monitor in MigrationsPage

---

### Flow 3 — Validation Failure: Missing Network Mapping

1. User completes steps 1–2
2. Step 3: User adds only some network mappings, leaves one unmapped
3. User clicks Submit → validation fires
4. Section nav highlights Step 3 with error badge
5. Page scrolls to Step 3, error text shows unmapped count
6. User adds remaining mappings → error clears → Submit enabled

---

### Flow 4 — API Error at Submission

1. User completes all steps → clicks Submit
2. `handleSubmit` POSTs /network-mappings → success
3. POSTs /storage-mappings → 500 error
4. Error toast shown: "Failed to create storage mapping"
5. `submitting` resets to false; Submit button re-enabled
6. User can retry without losing form state

---

### Flow 5 — Bulk IP Assignment

1. User selects powered-off VMs in Step 2
2. Clicks "Assign IP" in toolbar → `BulkIPEditDialog` opens
3. For each VM: Enter IP address per network interface
4. System validates IPs via `POST /validateOpenstackIPs` as user types
5. If IP conflicts: Validation status shows "invalid", Apply button disabled
6. User fixes IPs → validation passes → clicks Apply
7. `patchVMwareMachine` called for each VM with updated `networkInterfaces`
8. Dialog closes, IP column updates in DataGrid

---

### Flow 6 — Cancellation Mid-Flow

1. User opens form, selects clusters (MigrationTemplate created, VMwareCreds and OpenstackCreds created for session)
2. User clicks Close/Cancel at any step
3. `handleClose` fires → deletes session-scoped resources:
   - DELETE /migration-templates/{sessionId-template}
   - DELETE /vmware-creds/{sessionId-cred} (if not persistent)
   - DELETE /openstack-creds/{sessionId-cred} (if not persistent)
4. Drawer closes; no orphan K8s resources

---

### Flow 7 — Admin Cutover Trigger

**Precondition**: Migration in `AwaitingAdminCutOver` phase.

1. User clicks play button on migration row → confirmation dialog
2. User confirms → `triggerAdminCutover(namespace, name)` called
3. Loading state on button
4. On success: `refetchMigrations()` called; migration updates to next phase
5. On error: Error shown in dialog; user can retry

---

### Flow 8 — Delete Migration

1. User selects migrations in table → clicks Delete in toolbar
2. Confirmation dialog: "Delete X migration(s)?"
3. On confirm: For each migration:
   a. `deleteMigration(name)` → DELETE /migrations/{name}
   b. `getMigrationPlan(planId)` → find associated plan
   c. `patchMigrationPlan(planId, { virtualMachines: updatedList })` → remove VM from plan
4. Snackbar: "Migration(s) deleted successfully"

---

### Flow 9 — View Pod Logs

1. User clicks log icon on migration row → `PodLogsDrawer` opens
2. Logs stream via `useDirectPodLogs` hook
3. User can search, filter by level, pause stream
4. User clicks Download → downloads combined bundle:
   - Pod logs (live streamed)
   - Kubernetes resource YAML (`fetchMigrationResourceBundle`)
   - Debug logs from `/var/log/pf9` (`fetchPodDebugLogs`)

---

### Flow 10 — Edge Case: No VMware Credentials

1. User opens migration form
2. Step 1: VMware cluster dropdown shows empty state or error
3. User cannot proceed past step 1
4. `isStep1Complete` = false → Submit disabled

---

### Flow 11 — Edge Case: StorageAcceleratedCopy with Array Creds

1. User selects storage copy method = "StorageAcceleratedCopy" in Step 5
2. Step 3: Storage section switches from volume type mapping to array creds mapping
3. If no validated array creds: Warning shown "No validated array credentials found"
4. Data copy + cutover options hidden in Step 5 (not applicable to accelerated copy)

---

### Flow 12 — RDM Disk Configuration

1. User selects VMs containing RDM (Raw Device Mapping) disks
2. Alert shown: "Selected VMs contain RDM disks"
3. "Configure RDM" button appears in toolbar
4. User clicks → `RdmDiskConfigurationPanel` opens
5. For each RDM disk: Select Cinder backend pool + volume type
6. Warning shown if volume type doesn't match backend type
7. User clicks Apply → `patchRdmDisk` called → query cache invalidated

---

## 8. Known Constraints and Assumptions

### Architecture Constraints

- **Dual VM types**: `VmDataWithFlavor` (standard mode) and `VM` (rolling mode) are separate types bridged by `vmAdapters.ts` → `CanonicalVM`. Changes to VM data model require updating both types and adapters.
- **MigrationTemplate polling**: Template is created immediately on credential selection and polled every 3s. The form cannot proceed with VM/network/storage selection until template status is populated by the controller.
- **Session-scoped K8s resources**: VMwareCreds, OpenstackCreds, MigrationTemplate created during the form session must be cleaned up on close. If the browser crashes mid-session, orphan resources may persist in the cluster.
- **Namespace**: Default `vjailbreak-system` (constant `VJAILBREAK_DEFAULT_NAMESPACE`). Rolling migrations use `migration-system` for pod operations.

### Feature Flags and Conditional Behavior

- `isPCD` flag (derived from OpenstackCreds) enables GPU and flavorless options in MigrationOptionsAlt
- `storageCopyMethod === 'StorageAcceleratedCopy'` hides data copy + cutover scheduling
- `storageCopyMethod === 'HotAdd'` forces cold copy, disables hot/mock, shows ProxyVM selector (Step 3)
- `hasL2Network` disables security groups and fallback to DHCP
- `useGPU` disables flavor selection (GPU instance auto-selected)

### Performance Considerations

- `staleTime: Infinity` on migrations query — only refetched on window focus or explicit trigger
- Adaptive refetch interval in MigrationsPage (5s vs 30s based on pending migrations)
- Subnet compatibility check debounced 350ms with API response caching per target+IPs combo
- MigrationTemplate polling stopped once status fully populated
- DataGrid row virtualization handles large VM lists

### Accessibility

- All dialogs and drawers use `@mui/material` accessibility primitives
- Form fields use `aria-label` / `htmlFor` associations
- Loading states communicated via `aria-busy` on buttons
- Color is not the sole indicator of status (icons + text accompany all color-coded states)

### Browser Requirements

- Modern browsers (Chrome, Firefox, Edge) — no IE support
- `IntersectionObserver` required for section tracking (`useSectionTracking`)
- `Clipboard API` required for log copy functionality

### Known Technical Debt

No outstanding technical debt.

---

## Related Specs

- **ProxyVM feature**: `docs/specs/proxyvms/proxyvms-feature.md` — full spec for the Proxy VMs management page and HotAdd CRD lifecycle
