# vjailbreak UI Migration Feature — Refactoring Handoff

## Project
**Repo:** `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
**Branch:** `private/main/sarika/add-more-to-logs`
**Goal:** Break 6 large files (4000+ lines total) in `src/features/migration/` into focused modules. Zero behavior change.

---

## Files — Current Line Counts

| File | Before | After |
|------|--------|-------|
| `RollingMigrationForm.tsx` | 4,264 | 2,678 ✅ |
| `VmsSelectionStep.tsx` | 2,687 | ~2,633 (hooks pending) |
| `MigrationForm.tsx` | 1,827 | ~1,737 (hooks pending) |
| `MigrationOptionsAlt.tsx` | 947 | ~940 |
| `components/MigrationsTable.tsx` | 760 | ~750 |
| `NetworkAndStorageMappingStep.tsx` | 556 | 304 ✅ |

---

## Constraints (hard rules)
- No logic/props/behavior changes during extraction
- No component or hook renames that break external imports
- One concern per output file
- Every moved export re-exports from original path until Step 4 import cleanup
- Preserve all TS types exactly as written
- Node version: v18.20.7 — do NOT change. TypeScript check command:
  `/home/abhijeet/.nvm/versions/node/v18.20.7/bin/node /home/abhijeet/Projects/Platform9/vjailbreak/ui/node_modules/.bin/tsc --noEmit`

---

## 4-Phase Strategy

**Phase 1 — Audit** ✅ Done

**Phase 2 — Extract zero-risk items** ✅ Done
Order: types → constants → pure utils

**Phase 3 — Custom hook extraction** ⏳ In Progress (2 of 4 files done)
- `RollingMigrationForm.tsx` ✅ all 6 hooks done
- `NetworkAndStorageMappingStep.tsx` ✅ all 3 hooks done
- `VmsSelectionStep.tsx` ⏳ 5 hooks pending
- `MigrationForm.tsx` ⏳ 5 hooks pending

**Phase 4 — Update imports + clean re-exports** ⏳ Pending
Update all cross-file imports, remove temporary re-exports.

---

## Step 2 — Completed Work

### New files created

| File | Contents |
|------|----------|
| `src/features/migration/types.ts` | All interfaces/types from all 6 files consolidated |
| `src/features/migration/utils/ipValidation.ts` | `extractFirstIPv4`, `hasMultipleIPv4`, `parseIpList`, `hasMultipleIpEntries`, `isValidIPAddressList`, `isValidIPAddress`, `IPV4_MATCH_REGEX`, `IPV4_FULL_REGEX` |
| `src/features/migration/utils/migrationTableUtils.ts` | `getProgressText`, `PHASE_STEPS`, `IN_PROGRESS_PHASES` |

### `constants.ts` expanded with
- `STORAGE_COPY_METHOD_OPTIONS` (from `NetworkAndStorageMappingStep`)
- `STATUS_ORDER` (from `MigrationsTable`)
- `DEFAULT_MIGRATION_OPTIONS`, `DRAWER_WIDTH`
- `MIGRATED_TOOLTIP_MESSAGE`, `FLAVOR_NOT_FOUND_MESSAGE`, `DEFAULT_PAGINATION_MODEL`
- `NEXT_SCRIPT_DELIMITER`

---

## Step 3 — Hook Extraction

### Hooks directory: `src/features/migration/hooks/`

```
hooks/
├── useBulkIPHandlers.ts          (RollingMigrationForm) ✅
├── useFilteredMappings.ts        (NetworkAndStorageMappingStep) ✅
├── useFlavorHandlers.ts          (RollingMigrationForm) ✅
├── useHostConfigHandlers.ts      (RollingMigrationForm) ✅
├── useMigrationsQuery.ts         (pre-existing)
├── useMigrationStatusMonitor.ts  (pre-existing)
├── useNetworkIPsMap.ts           (NetworkAndStorageMappingStep) ✅
├── useNetworkSubnetCompatibility.ts (NetworkAndStorageMappingStep) ✅
├── useRollingFormData.ts         (RollingMigrationForm) ✅
├── useRollingFormSubmit.ts       (RollingMigrationForm) ✅
└── useRollingFormValidation.ts   (RollingMigrationForm) ✅
```

---

### `RollingMigrationForm.tsx` — ✅ All 6 hooks done

**`hooks/useBulkIPHandlers.ts`** — all bulk IP state + handlers (~400 lines)

**`hooks/useFlavorHandlers.ts`** — flavor dialog state + handlers

**`hooks/useHostConfigHandlers.ts`** — host config dialog state + handlers

**`hooks/useRollingFormData.ts`** — `fetchMaasConfigs`, `fetchClusterHosts`, `fetchClusterVMs`, `fetchOpenstackCredentialDetails` + effects

**`hooks/useRollingFormValidation.ts`** — owns `vmIpValidationError`, `esxHostConfigValidationError`, `osValidationError` state; memos: `esxHostMappingStatus`, `vmIpValidation`, `esxHostConfigValidation`, `osValidation`, `isSubmitDisabled`
- Imports `SelectedMigrationOptionsType` from `'../types'` (not from `'../RollingMigrationForm'` — avoids circular dep)
- `params` type is `RollingFormParams` (not `Record<string, unknown>`) to enable spread operators

**`hooks/useRollingFormSubmit.ts`** — owns `submitting` state; exports `handleSubmit`, `handleClose`; all API submission calls
- All API imports brought in directly
- `params` type is `RollingFormParams`

**Key type added to `types.ts`:**
```typescript
export interface RollingFormParams extends Record<string, unknown> {
  dataCopyMethod?: string
  dataCopyStartTime?: string
  cutoverOption?: string
  cutoverStartTime?: string
  cutoverEndTime?: string
  postMigrationScript?: string
  osFamily?: string
  storageCopyMethod?: StorageCopyMethod
  postMigrationAction?: { suffix?: string; folderName?: string; renameVm?: boolean; moveToFolder?: boolean }
  useGPU?: boolean
  useFlavorless?: boolean
  disconnectSourceNetwork?: boolean
  fallbackToDHCP?: boolean
  networkPersistence?: boolean
}
```

**Hook call order in RollingMigrationForm (matters for deps):**
```typescript
const { submitting, handleSubmit, handleClose } = useRollingFormSubmit({ ... })
// useRollingFormValidation needs `submitting` from above
const { vmIpValidationError, esxHostConfigValidationError, osValidationError,
        esxHostMappingStatus, vmIpValidation, esxHostConfigValidation,
        osValidation, isSubmitDisabled } = useRollingFormValidation({ ... })
```

**Note:** `RollingMigrationForm` keeps local `SelectedMigrationOptionsType` (adds `osFamily: boolean`, extends `Record<string, unknown>`) — structurally compatible with `types.ts` version that has `[key: string]: unknown`. Passes to hooks fine.

---

### `NetworkAndStorageMappingStep.tsx` — ✅ All 3 hooks done

**`hooks/useNetworkIPsMap.ts`** — single `useMemo` for VM→network IP deduplication
```typescript
export function useNetworkIPsMap(selectedVMs: VmData[]): Map<string, string[]>
```

**`hooks/useFilteredMappings.ts`** — owns `removedAutoArrayCredsSourcesRef`; memos: `filteredNetworkMappings`, `filteredStorageMappings`, `filteredArrayCredsMappings`; 3 sync effects + 1 auto-map ArrayCreds effect
- Returns `handleArrayCredsMappingsChange` (only destructured value used in component)
- JSX uses `params.networkMappings` directly; hooks' sync effects call `onChange` to keep params updated

**`hooks/useNetworkSubnetCompatibility.ts`** — debounce + ref-based API cache + subnet warning state
```typescript
export function useNetworkSubnetCompatibility({ networkMappings, openstackCredentials, selectedVMs, networkIPsMap, openstackNetworks }): Record<string, string>
```

**Intermediate memos kept in component** (not extracted — needed to derive hook params):
- `validatedArrayCreds` (depends on `arrayCredentials` from `useArrayCredentialsQuery`)
- `arrayCredsNames`
- `openstackNetworkNames`

---

### `VmsSelectionStep.tsx` — ⏳ 5 hooks pending

| Hook | Contents |
|------|----------|
| `hooks/useBulkIPEdit.ts` | 30+ bulk IP state vars + all bulk IP handlers |
| `hooks/useFlavorAssignment.ts` | flavor dialog state + handlers |
| `hooks/useRdmConfiguration.ts` | RDM dialog flow + handlers |
| `hooks/useVmSelection.ts` | `handleVmSelection`, `isRowSelectable`, selection state |
| `hooks/useOsAssignment.ts` | `handleOSAssignment` + OS state |

---

### `MigrationForm.tsx` — ⏳ 5 hooks pending

| Hook | Contents |
|------|----------|
| `hooks/useFormSync.ts` | URL param ↔ RHF sync effects (lines 259–385) |
| `hooks/useCredentialFetching.ts` | credential + template fetching effects (lines 404–551) |
| `hooks/useMigrationFormSubmit.ts` | `createNetworkMapping`, `createStorageMapping`, `createArrayCredsMapping`, `updateMigrationTemplate`, `createMigrationPlan`, `handleSubmit` |
| `hooks/useFormValidation.ts` | step completion memos + `disableSubmit` |
| `hooks/useSectionTracking.ts` | IntersectionObserver logic (lines 1470–1531) |

---

## Key Design Notes
- `RollingMigrationForm` keeps own `FormValues` (simpler: no vmwareCreds/openstackCreds) and `SelectedMigrationOptionsType` (adds `osFamily: boolean`) — both differ from canonical `types.ts` versions
- `BulkIpEdit` + `BulkIpClear` in `VmsSelectionStep` defined inside function body — need hoisting to module scope before extraction
- `normalizeNetworkInterfaces` in `VmsSelectionStep` not shared — stays local
- `MigrationOptions.tsx` (legacy, 390 lines) still imports `FieldErrors, FormValues, SelectedMigrationOptionsType` from `./MigrationForm` — resolves in Step 4 import cleanup
- Spread operator on `params` requires typed interface (`RollingFormParams`), not `Record<string, unknown>`
- Circular import pattern: hooks must import types from `'../types'`, never from `'../RollingMigrationForm'`

---

## Pitfalls Hit This Session

**Spread type error:** `params: Record<string, unknown>` → `params.field` has type `unknown` → `TS2698: Spread types may only be created from object types`. Fix: add typed `RollingFormParams` interface to `types.ts`.

**Circular import:** `useRollingFormValidation` initially imported `SelectedMigrationOptionsType` from `'../RollingMigrationForm'`. Fix: import from `'../types'` instead (structurally compatible).

**Orphaned code after Edit:** Edit tool matched only partial old string, leaving dead code block. Fix: Python `content.replace(old_block, new_block, 1)` for exact single-occurrence replacement.

**Unused destructured vars:** `filteredNetworkMappings` etc. returned by hook but not used in JSX (JSX reads `params.x` directly). Fix: only destructure what JSX actually uses (`handleArrayCredsMappingsChange`).

---

## Current Status
**tsc --noEmit: 0 errors**

---

## Resume Prompt

Paste into new conversation:

> Step 2 (types, constants, utils) and partial Step 3 (hook extraction) are complete. `tsc --noEmit` passes with 0 errors. Node v18.20.7 — do not change.
>
> Working directory: `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
> Branch: `private/main/sarika/add-more-to-logs`
>
> Context: refactoring `src/features/migration/` — breaking large files into focused modules without changing runtime behavior. Constraints: no logic changes, no renames of externally imported symbols, re-export from original path until Step 4.
>
> **Completed Step 2:**
> - `types.ts` — all interfaces consolidated (includes `RollingFormParams`)
> - `utils/ipValidation.ts` — shared IP helpers
> - `utils/migrationTableUtils.ts` — table utils
> - `constants.ts` — expanded with all constants
>
> **Completed Step 3 hooks:**
> - `RollingMigrationForm.tsx` (4264→2678 lines): all 6 hooks extracted — `useBulkIPHandlers`, `useFlavorHandlers`, `useHostConfigHandlers`, `useRollingFormData`, `useRollingFormValidation`, `useRollingFormSubmit`
> - `NetworkAndStorageMappingStep.tsx` (556→304 lines): all 3 hooks extracted — `useNetworkIPsMap`, `useFilteredMappings`, `useNetworkSubnetCompatibility`
>
> **Pending Step 3:**
> - `VmsSelectionStep.tsx` (~2633 lines): extract `useBulkIPEdit`, `useFlavorAssignment`, `useRdmConfiguration`, `useVmSelection`, `useOsAssignment`
> - `MigrationForm.tsx` (~1737 lines): extract `useFormSync`, `useCredentialFetching`, `useMigrationFormSubmit`, `useFormValidation`, `useSectionTracking`
>
> TypeScript check: `/home/abhijeet/.nvm/versions/node/v18.20.7/bin/node /home/abhijeet/Projects/Platform9/vjailbreak/ui/node_modules/.bin/tsc --noEmit`
>
> Proceed with `VmsSelectionStep.tsx` hook extraction. Extract `useBulkIPEdit` first (largest savings). For each hook: show new file, show updated original with extraction replaced by hook call, confirm TS still passes.
>
> See `vjailbreak-refactor-handoff.md` in repo root for full context.
