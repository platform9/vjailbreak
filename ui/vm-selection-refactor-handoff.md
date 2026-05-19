# VmsSelectionStep Merge Refactor — Handoff

## Context

**Repo:** `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
**Branch:** `587-ui-refactor-migration`
**Goal:** Merge `VmsSelectionStep` and `RollingVmsSelectionStep` into one parameterized component. Zero behavior change.

**TypeScript check:**
```
/home/abhijeet/.nvm/versions/node/v18.20.7/bin/node node_modules/.bin/tsc --noEmit
```
Node: v18.20.7 — do NOT change.

---

## Current Status: PHASE 3 COMPLETE ✅

**tsc --noEmit: 0 errors**

Work is **uncommitted** — all changes in working tree.

---

## Why Two Components Were Redundant

`VmsSelectionStep` (`steps/VmsSelectionStep.tsx`) and `RollingVmsSelectionStep` (`steps/RollingVmsSelectionStep.tsx`) share duplicated logic:
- Bulk IP Edit dialog (~200 lines identical)
- Flavor Assignment dialog (~80 lines near-identical)
- OS assignment dropdown renderCell (~60 lines near-identical)
- `renderValidationAdornment` (identical — now deleted from both)
- `MissingInterfaceIpWarningAlert` usage
- DataGrid structure + pagination

Key differences (features unique to each):

| Concern | VmsSelectionStep | RollingVmsSelectionStep |
|---------|-----------------|------------------------|
| VM type | `VmDataWithFlavor` (`vmState`, `ipAddress`, `cpuCount`) | `VM` (`powerState`, `ip`, `cpu`) |
| VM loading | Internal (`useVMwareMachinesQuery` + migrated VMs fetch) | External (parent passes `vmsWithAssignments`) |
| RDM support | Full (dialog + validation + warning) | None |
| Flavor column | Dialog only | Inline `Select` per row + dialog |
| IP assignment | All selected VMs | Powered-off only |
| OS dropdown trigger | Any selected VM | Selected + powered-off only |
| `isMigrated` display | Yes (chip, disabled row) | No |
| GPU warning | Yes | No |
| Amplitude tracking | Yes | No |
| Error display | Internal snackbar | Props (`vmIpValidationError`, `osValidationError`) |
| `React.memo` | Yes (custom comparator) | No |

---

## Refactor Plan (5 Phases)

### Phase 1 ✅ — Normalize VM type

**`src/features/migration/types.ts`** — 3 changes:
1. `VmNetworkInterface` extended with `preserveIP?: boolean` and `preserveMAC?: boolean`
2. `VmDataWithFlavor` extended with `flavor?: string`
3. New `CanonicalVM` interface added (see below)

**`src/features/migration/utils/vmAdapters.ts`** — new file, 4 converters:
- `fromVmDataWithFlavor(vm: VmDataWithFlavor): CanonicalVM` — maps `vmState`→`powerState` ('running'→'powered-on'), `ipAddress`→`ip`, `cpuCount`→`cpu`, `flavor ?? flavorName`→`flavor`
- `toVmDataWithFlavor(vm: CanonicalVM): VmDataWithFlavor` — reverse, for standard hooks
- `fromVM(vm: VM): CanonicalVM` — type-lift (VM already close to canonical)
- `toVM(vm: CanonicalVM): VM` — reverse, for rolling hooks

**`CanonicalVM` interface** (in `types.ts`):
```typescript
export interface CanonicalVM {
  id: string
  name: string
  powerState: 'powered-on' | 'powered-off'
  ip: string
  cpu?: number
  memory?: number
  osFamily?: string
  flavor?: string
  targetFlavorId?: string
  networkInterfaces?: VmNetworkInterface[]
  esxHost?: string
  networks?: string[]
  datastores?: string[]
  preserveIp?: Record<number, boolean>
  preserveMac?: Record<number, boolean>
  ipValidationStatus?: 'pending' | 'valid' | 'invalid' | 'validating'
  ipValidationMessage?: string
  // Standard-migration-only (absent in rolling mode)
  isMigrated?: boolean
  flavorNotFound?: boolean
  hasSharedRdm?: boolean
  vmKey?: string
  vmid?: string
  labels?: Record<string, string>
  disks?: string[]
  vmWareMachineName?: string
}
```

---

### Phase 2 ✅ — Extract shared sub-components

**New files created in `src/features/migration/components/`:**

#### `BulkIPEditDialog.tsx`
- Accepts `CanonicalVM[]` for VM list
- Uses `vm.powerState !== 'powered-on'` for `isPoweredOff` (canonical)
- Optional `bulkCurrentIPs?: Record<string, Record<number, string>>` — standard-mode only (used for `currentIp` fallback display)
- Optional `duplicateNames?: Set<string>` — standard-mode only (shows `vmKey` when VM name is duplicate)
- `helperText` logic: `status === 'invalid' ? message || 'Invalid IP' : preserveIp && !existingIp ? message : ''`
- `renderValidationAdornment` now lives inside this component only
- Props interface:
  ```typescript
  export interface BulkIPEditDialogProps {
    open: boolean
    selectedVMCount: number
    vms: CanonicalVM[]
    bulkEditIPs: Record<string, Record<number, string>>
    bulkPreserveIp: Record<string, Record<number, boolean>>
    bulkPreserveMac: Record<string, Record<number, boolean>>
    bulkExistingIPs: Record<string, Record<number, string>>
    bulkCurrentIPs?: Record<string, Record<number, string>>
    bulkValidationStatus: Record<string, Record<number, string>>
    bulkValidationMessages: Record<string, Record<number, string>>
    assigningIPs: boolean
    hasBulkIpsToApply: boolean
    hasBulkIpValidationErrors: boolean
    duplicateNames?: Set<string>
    onClose: () => void
    onApply: () => void
    onClearAll: () => void
    onPreserveIpChange: (vmId: string, ifIdx: number, val: boolean) => void
    onPreserveMacChange: (vmId: string, ifIdx: number, val: boolean) => void
    onIpChange: (vmId: string, ifIdx: number, val: string) => void
  }
  ```

#### `FlavorAssignmentDialog.tsx`
- Uses `Autocomplete` + `SharedTextField` (standard approach, better UX than rolling's `Select`)
- Props interface:
  ```typescript
  export interface FlavorAssignmentDialogProps {
    open: boolean
    selectedVMCount: number
    flavors: OpenStackFlavor[]
    selectedFlavor: string
    updating: boolean
    onClose: () => void
    onApply: () => void
    onFlavorChange: (flavorId: string) => void
  }
  ```

**Other changes in Phase 2:**

`src/features/migration/hooks/useFlavorHandlers.ts`:
- Added `setSelectedFlavor` to return object

`src/features/migration/components/index.ts`:
- Added exports for `BulkIPEditDialog`, `BulkIPEditDialogProps`, `FlavorAssignmentDialog`, `FlavorAssignmentDialogProps`

`src/features/migration/steps/VmsSelectionStep.tsx`:
- Replaced inline dialogs with `<BulkIPEditDialog>` and `<FlavorAssignmentDialog>` calls
- Removed `renderValidationAdornment`, ~250 lines of dialog JSX

`src/features/migration/steps/RollingVmsSelectionStep.tsx`:
- Same dialog extraction; removed ~320 lines of dialog JSX + `renderValidationAdornment`

---

### Phase 3 ✅ — Merge into unified VmsSelectionStep

**Completed in this session.** tsc: 0 errors.

**What changed:**

`src/features/migration/types.ts`:
- Added imports: `Dispatch`, `SetStateAction` from react; `GridRowSelectionModel` from `@mui/x-data-grid`; `ErrorContext` from `src/services/errorReporting`
- `VmsSelectionStepProps` replaced with unified interface (see below)

`src/features/migration/steps/VmsSelectionStep.tsx`:
- Full rewrite — single component with `mode?: 'standard' | 'rolling'`
- Both hook sets (`useBulkIPEdit`/`useBulkIPHandlers`, `useFlavorAssignment`/`useFlavorHandlers`) called unconditionally; `isRolling` gates which results are used
- `handleRollingOSAssignment` inlined (patches VMwareMachine, updates `vmsWithAssignments`)
- Rolling flavor sync `useEffect` added (syncs flavor names from `openstackFlavors` to `vmsWithAssignments`), gated by `isRolling`
- `standardColumns` and `rollingColumns` defined separately; passed via `columns={isRolling ? rollingColumns : standardColumns}`
- `StandardToolbarWithActions` and `RollingToolbarWithActions` defined as separate local components
- Two `<DataGrid>` branches in JSX (standard vs rolling) — avoids mixing incompatible props
- `React.memo` retained; `arePropsEqual` updated: rolling mode always returns `false` (parent owns state), standard mode uses original comparator
- `canonicalVMs` = `isRolling ? vmsWithAssignments.map(fromVM) : vmsWithFlavor.map(fromVmDataWithFlavor)`
- `vmOSAssignments` = `isRolling ? vmOSAssignmentsProp : standardVmOSAssignments`
- `reportError` = `isRolling && reportErrorProp ? reportErrorProp : internalReportError`
- RDM dialogs, GlobalStyles, flavor Snackbar all behind `{!isRolling && ...}`

**Unified `VmsSelectionStepProps`:**
```typescript
export interface VmsSelectionStepProps {
  mode?: 'standard' | 'rolling'

  // Standard-mode props
  onChange?: (id: string) => (value: unknown) => void
  error?: string
  open?: boolean
  vmwareCredsValidated?: boolean
  openstackCredsValidated?: boolean
  sessionId?: string
  openstackFlavors?: OpenStackFlavor[]
  vmwareCredName?: string
  openstackCredName?: string
  openstackCredentials?: OpenstackCreds
  vmwareCluster?: string
  useGPU?: boolean
  showHeader?: boolean

  // Rolling-mode props
  vmsWithAssignments?: VM[]
  setVmsWithAssignments?: Dispatch<SetStateAction<VM[]>>
  vmOSAssignments?: Record<string, string>
  setVmOSAssignments?: Dispatch<SetStateAction<Record<string, string>>>
  selectedVMs?: GridRowSelectionModel
  onSelectionChange?: (ids: GridRowSelectionModel) => void
  loadingVMs?: boolean
  vmIpValidationError?: string
  osValidationError?: string
  fetchClusterVMs?: () => Promise<void>
  openstackCredData?: OpenstackCreds | null
  reportError?: (error: Error, additionalContext?: ErrorContext) => void
}
```

**Hook bridging (call both sets, gate which results used):**

| Hook | Mode | Called with |
|------|------|-------------|
| `useBulkIPEdit` | both | standard state; empty in rolling mode (no-op) |
| `useBulkIPHandlers` | both | rolling props; empty in standard mode (no-op) |
| `useFlavorAssignment` | both | standard state; `onChange ?? (() => () => {})` fallback |
| `useFlavorHandlers` | both | rolling props; `fetchClusterVMs ?? noOpAsync` fallback |
| `useVmSelection` | both | standard `vmsWithFlavor` (empty in rolling) |
| `useOsAssignment` | both | standard `vmsWithFlavor` (empty in rolling) |
| `useRdmConfiguration` | both | standard state (empty `Set<string>()` in rolling) |
| `useVMwareMachinesQuery` | both | `enabled: !isRolling && open` |

---

### Phase 4 — Update callers

**`pages/MigrationForm.tsx`** — add `mode="standard"` (or omit — default is `'standard'`):
```tsx
<VmsSelectionStep
  mode="standard"   // or omit — default
  onChange={getParamsUpdater}
  error={fieldErrors['vms']}
  open={open}
  vmwareCredsValidated={vmwareCredsValidated}
  openstackCredsValidated={openstackCredsValidated}
  sessionId={sessionId}
  openstackFlavors={openstackCredentials?.spec?.flavors}
  vmwareCredName={params.vmwareCreds?.existingCredName}
  openstackCredName={params.openstackCreds?.existingCredName}
  openstackCredentials={openstackCredentials}
  vmwareCluster={params.vmwareCluster}
  useGPU={params.useGPU}
  showHeader={false}
/>
```

**`pages/RollingMigrationForm.tsx`** — replace `<RollingVmsSelectionStep ...>` with:
```tsx
import VmsSelectionStep from '../steps/VmsSelectionStep'
// remove: import RollingVmsSelectionStep from '../steps/RollingVmsSelectionStep'

<VmsSelectionStep
  mode="rolling"
  vmsWithAssignments={vmsWithAssignments}
  setVmsWithAssignments={setVmsWithAssignments}
  selectedVMs={selectedVMs}
  onSelectionChange={(ids) => {
    markTouched('vms')
    setSelectedVMs(ids)
  }}
  vmOSAssignments={vmOSAssignments}
  setVmOSAssignments={setVmOSAssignments}
  openstackCredData={openstackCredData}
  loadingVMs={loadingVMs}
  reportError={reportError}
  fetchClusterVMs={fetchClusterVMs}
  vmIpValidationError={vmIpValidationError}
  osValidationError={osValidationError}
/>
```

---

### Phase 5 — Delete RollingVmsSelectionStep.tsx

Delete `src/features/migration/steps/RollingVmsSelectionStep.tsx`.
Check `src/features/migration/steps/index.ts` (if it exists) for barrel exports referencing it.

---

## Hook Types Reference

| Hook | VM type | SelectedVMs type | Notes |
|------|---------|-----------------|-------|
| `useBulkIPEdit` | `VmDataWithFlavor[]` | `Set<string>` | Standard form |
| `useBulkIPHandlers` | `VM[]` | `GridRowSelectionModel` | Rolling form |
| `useFlavorAssignment` | `VmDataWithFlavor[]` | `Set<string>` | Standard form, needs `onChange('vms')` |
| `useFlavorHandlers` | `VM[]` | `GridRowSelectionModel` | Rolling form, needs `fetchClusterVMs`; returns `setSelectedFlavor` |
| `useOsAssignment` | — | — | Standard form only |
| `useRdmConfiguration` | — | — | Standard form only |
| `useVmSelection` | `VmDataWithFlavor[]` | `Set<string>` | Standard form |

---

## Key Pitfalls

| Problem | Fix |
|---------|-----|
| `flavor` field untyped on `VmDataWithFlavor` | Fixed in Phase 1 — added `flavor?: string` |
| `VmNetworkInterface` defined twice (model vs types.ts) | Fixed in Phase 1 — types.ts version now has `preserveIP`/`preserveMAC` |
| `[].every(condition)` vacuous truth → false 'complete' | Already fixed in `useRollingFormValidation.ts` (prior session) |
| `VmData.vmState` = 'running'/'stopped'; `VM.powerState` = 'powered-on'/'powered-off' | `fromVmDataWithFlavor` adapter handles mapping |
| `useBulkIPEdit` uses `Set<string>`, `useBulkIPHandlers` uses `GridRowSelectionModel` | Keep hooks native; convert at boundary |
| `useFlavorAssignment` needs `onChange('vms')` callback; `useFlavorHandlers` needs `fetchClusterVMs` | Mode-gate which hook results to use; both called unconditionally with fallbacks |
| Rolling `selectedVMs` is `GridRowSelectionModel` (array); standard is `Set<string>` | Use `.length` for rolling, `.size` for standard; BulkIPEditDialog takes `selectedVMCount: number` |
| `VmsSelectionStep` uses `React.memo` with custom comparator | Rolling mode returns `false` from `arePropsEqual` (parent owns state); standard uses original comparator |
| Calling hooks in inactive mode | All hooks safe with empty/no-op fallbacks — no side effects from inactive-mode hook calls |

---

## Files Modified Summary (All Phases)

| File | Change |
|------|--------|
| `src/features/migration/types.ts` | Phase 1: `VmNetworkInterface.preserveIP/preserveMAC`, `VmDataWithFlavor.flavor`, `CanonicalVM` interface. Phase 3: new imports + unified `VmsSelectionStepProps` |
| `src/features/migration/utils/vmAdapters.ts` | Phase 1: new file — `fromVmDataWithFlavor`, `toVmDataWithFlavor`, `fromVM`, `toVM` |
| `src/features/migration/components/BulkIPEditDialog.tsx` | Phase 2: new file — extracted + unified dialog |
| `src/features/migration/components/FlavorAssignmentDialog.tsx` | Phase 2: new file — extracted + unified dialog (Autocomplete style) |
| `src/features/migration/components/index.ts` | Phase 2: added exports for new dialog components |
| `src/features/migration/hooks/useFlavorHandlers.ts` | Phase 2: added `setSelectedFlavor` to return |
| `src/features/migration/steps/VmsSelectionStep.tsx` | Phase 2: dialog extraction. Phase 3: full rewrite — unified component with `mode` prop |
| `src/features/migration/steps/RollingVmsSelectionStep.tsx` | Phase 2: dialog extraction. **Phase 5 target: delete this file** |

---

## Resume Prompt

> Branch: `587-ui-refactor-migration`
> Working dir: `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
>
> **Goal:** Merge `VmsSelectionStep` and `RollingVmsSelectionStep` into one parameterized component.
>
> **Phases 1, 2, 3 COMPLETE. tsc --noEmit: 0 errors. All changes UNCOMMITTED.**
>
> Phase 1: Added `CanonicalVM` type to `types.ts` + `utils/vmAdapters.ts` (4 converters).
> Phase 2: Extracted `BulkIPEditDialog` and `FlavorAssignmentDialog` into `components/`. Both step files now import shared components.
> Phase 3: `VmsSelectionStep.tsx` fully rewritten as unified component with `mode?: 'standard' | 'rolling'`. Both hook sets (`useBulkIPEdit`/`useBulkIPHandlers`, `useFlavorAssignment`/`useFlavorHandlers`) called unconditionally; `isRolling` gates which results are used in render. `VmsSelectionStepProps` unified in `types.ts`.
>
> **Next: Phase 4** — update callers.
> - `pages/MigrationForm.tsx`: add `mode="standard"` (or omit — default)
> - `pages/RollingMigrationForm.tsx`: replace `<RollingVmsSelectionStep ...>` with `<VmsSelectionStep mode="rolling" ...>`, swap import
>
> **Then Phase 5** — delete `src/features/migration/steps/RollingVmsSelectionStep.tsx`.
> Check `src/features/migration/steps/index.ts` for barrel exports.
>
> See `vm-selection-refactor-handoff.md` for full Phase 4 JSX snippets and complete context.
