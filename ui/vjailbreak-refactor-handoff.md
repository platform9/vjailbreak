# vjailbreak UI Migration Feature — Refactoring Handoff

## Project
**Repo:** `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
**Branch:** `587-ui-refactor-migration`
**Goal:** Break 6 large files (4000+ lines total) in `src/features/migration/` into focused modules. Zero behavior change.

---

## Files — Line Counts

| File | Before | After |
|------|--------|-------|
| `RollingMigrationForm.tsx` | 4,264 | 2,678 ✅ |
| `VmsSelectionStep.tsx` | 2,687 | 1,497 ✅ |
| `MigrationForm.tsx` | 1,827 | 510 ✅ |
| `NetworkAndStorageMappingStep.tsx` | 556 | 322 ✅ |
| `MigrationOptionsAlt.tsx` | 947 | 929 ✅ |
| `components/MigrationsTable.tsx` | 760 | 674 ✅ |

---

## Constraints (hard rules)
- No logic/props/behavior changes during extraction
- No component or hook renames that break external imports
- One concern per output file
- Preserve all TS types exactly as written
- Node version: v18.20.7 — do NOT change. TypeScript check command:
  `/home/abhijeet/.nvm/versions/node/v18.20.7/bin/node /home/abhijeet/Projects/Platform9/vjailbreak/ui/node_modules/.bin/tsc --noEmit`

---

## 4-Phase Strategy

**Phase 1 — Audit** ✅ Done

**Phase 2 — Extract zero-risk items** ✅ Done
Order: types → constants → pure utils

**Phase 3 — Custom hook extraction** ✅ Done (all 4 files)

**Phase 4 — Update imports + clean re-exports** ✅ Done
- No temporary re-exports remain in component files
- `MigrationOptions.tsx` imports types from `./types` (not `./MigrationForm`)
- No hooks import from component files (no circular deps)

---

## Phase 2 — Completed Work

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

## Phase 3 — Hook Extraction

### Hooks directory: `src/features/migration/hooks/`

```
hooks/
├── useBulkIPEdit.ts              (VmsSelectionStep) ✅
├── useBulkIPHandlers.ts          (RollingMigrationForm) ✅
├── useCredentialFetching.ts      (MigrationForm) ✅
├── useFilteredMappings.ts        (NetworkAndStorageMappingStep) ✅
├── useFlavorAssignment.ts        (VmsSelectionStep) ✅
├── useFlavorHandlers.ts          (RollingMigrationForm) ✅
├── useFormSync.ts                (MigrationForm) ✅
├── useFormValidation.ts          (MigrationForm) ✅
├── useHostConfigHandlers.ts      (RollingMigrationForm) ✅
├── useMigrationFormSubmit.ts     (MigrationForm) ✅
├── useMigrationsQuery.ts         (pre-existing re-export)
├── useMigrationStatusMonitor.ts  (pre-existing re-export)
├── useNetworkIPsMap.ts           (NetworkAndStorageMappingStep) ✅
├── useNetworkSubnetCompatibility.ts (NetworkAndStorageMappingStep) ✅
├── useOsAssignment.ts            (VmsSelectionStep) ✅
├── useRdmConfiguration.ts        (VmsSelectionStep) ✅
├── useRollingFormData.ts         (RollingMigrationForm) ✅
├── useRollingFormSubmit.ts       (RollingMigrationForm) ✅
├── useRollingFormValidation.ts   (RollingMigrationForm) ✅
├── useSectionTracking.ts         (MigrationForm) ✅
└── useVmSelection.ts             (VmsSelectionStep) ✅
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

**Key type in `types.ts`:**
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

**Note:** `RollingMigrationForm` keeps local `SelectedMigrationOptionsType` (adds `osFamily: boolean`, extends `Record<string, unknown>`) — structurally compatible with `types.ts` version that has `[key: string]: unknown`.

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

### `VmsSelectionStep.tsx` — ✅ All 5 hooks done

**`hooks/useBulkIPEdit.ts`** — 30+ bulk IP state vars + all bulk IP handlers

**`hooks/useFlavorAssignment.ts`** — flavor dialog state + handlers

**`hooks/useRdmConfiguration.ts`** — RDM dialog flow + handlers

**`hooks/useVmSelection.ts`** — `handleVmSelection`, `isRowSelectable`, selection state

**`hooks/useOsAssignment.ts`** — `handleOSAssignment` + OS state

---

### `MigrationForm.tsx` — ✅ All 5 hooks done

**`hooks/useFormSync.ts`** — URL param ↔ RHF sync effects

**`hooks/useCredentialFetching.ts`** — credential + template fetching effects

**`hooks/useMigrationFormSubmit.ts`** — `createNetworkMapping`, `createStorageMapping`, `createArrayCredsMapping`, `updateMigrationTemplate`, `createMigrationPlan`, `handleSubmit`

**`hooks/useFormValidation.ts`** — step completion memos + `disableSubmit`

**`hooks/useSectionTracking.ts`** — IntersectionObserver logic

---

## Key Design Notes
- `RollingMigrationForm` keeps own `FormValues` (simpler: no vmwareCreds/openstackCreds) and `SelectedMigrationOptionsType` (adds `osFamily: boolean`) — both differ from canonical `types.ts` versions
- `normalizeNetworkInterfaces` in `VmsSelectionStep` not shared — stays local
- Spread operator on `params` requires typed interface (`RollingFormParams`), not `Record<string, unknown>`
- Circular import rule: hooks import types from `'../types'`, never from component files
- `useRollingFormSubmit` and `useRollingFormData` import `SourceDataItem`/`PcdDataItem` from `'../useClusterData'` — OK, not a component file

---

## Pitfalls Hit

**Spread type error:** `params: Record<string, unknown>` → `params.field` has type `unknown` → `TS2698: Spread types may only be created from object types`. Fix: add typed `RollingFormParams` interface to `types.ts`.

**Circular import:** `useRollingFormValidation` initially imported `SelectedMigrationOptionsType` from `'../RollingMigrationForm'`. Fix: import from `'../types'` instead.

**Orphaned code after Edit:** Edit tool matched only partial old string, leaving dead code block. Fix: Python `content.replace(old_block, new_block, 1)` for exact single-occurrence replacement.

**Unused destructured vars:** `filteredNetworkMappings` etc. returned by hook but not used in JSX (JSX reads `params.x` directly). Fix: only destructure what JSX actually uses (`handleArrayCredsMappingsChange`).

---

## Current Status
**All 4 phases complete. tsc --noEmit: 0 errors.**

Remaining: commit + PR.
