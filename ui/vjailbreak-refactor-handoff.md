# vjailbreak UI Migration Feature — Refactoring Handoff

## Project
**Repo:** `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
**Branch:** `587-ui-refactor-migration`
**Goal:** Break 6 large files (4000+ lines total) in `src/features/migration/` into focused modules + reorganize directory structure. Zero behavior change.

**TypeScript check:**
```
/home/abhijeet/.nvm/versions/node/v18.20.7/bin/node node_modules/.bin/tsc --noEmit
```
Node: v18.20.7 — do NOT change.

---

## Current Status: ALL WORK COMPLETE ✅

**tsc --noEmit: 0 errors**

Pending: commit + PR (all changes staged/committed on branch `587-ui-refactor-migration`).

---

## Final Directory Structure

```
src/features/migration/
├── api/                          # API calls + models (unchanged)
│   ├── migration-plans/
│   ├── migration-templates/
│   ├── migrations.ts
│   ├── migrationPlans.ts
│   └── useMigrationPlanDestinationsQuery.ts
├── components/                   # Reusable UI components (unchanged)
│   ├── index.ts
│   ├── BaseLogsDrawer.tsx
│   ├── ControllerLogsDrawer.tsx
│   ├── LogLine.tsx
│   ├── MaasConfigDetailsModal.tsx
│   ├── MigrationProgress.tsx
│   ├── MigrationProgressWithPopover.tsx
│   ├── MigrationsTable.tsx
│   ├── MissingInterfaceIpWarningAlert.tsx
│   ├── missingInterfaceIpWarnings.ts
│   ├── PodLogsDrawer.tsx
│   ├── RdmDiskConfigurationPanel.tsx
│   ├── ResourceMapping.tsx
│   ├── ResourceMappingTable.tsx
│   ├── ResourceMappingTableNew.tsx
│   ├── TriggerAdminCutover/
│   └── UpgradeModal.tsx
├── context/                      # React contexts (unchanged)
│   └── MigrationFormContext.tsx
├── hooks/                        # ALL custom hooks (22 files)
│   ├── useBulkIPEdit.ts          (VmsSelectionStep)
│   ├── useBulkIPHandlers.ts      (RollingMigrationForm)
│   ├── useClusterData.ts         ← moved from root
│   ├── useCredentialFetching.ts  (MigrationForm)
│   ├── useFilteredMappings.ts    (NetworkAndStorageMappingStep)
│   ├── useFlavorAssignment.ts    (VmsSelectionStep)
│   ├── useFlavorHandlers.ts      (RollingMigrationForm)
│   ├── useFormSync.ts            (MigrationForm)
│   ├── useFormValidation.ts      (MigrationForm)
│   ├── useHostConfigHandlers.ts  (RollingMigrationForm)
│   ├── useMigrationFormSubmit.ts (MigrationForm)
│   ├── useMigrationsQuery.ts     (re-export from src/hooks/api)
│   ├── useMigrationStatusMonitor.ts (re-export from src/hooks)
│   ├── useNetworkIPsMap.ts       (NetworkAndStorageMappingStep)
│   ├── useNetworkSubnetCompatibility.ts (NetworkAndStorageMappingStep)
│   ├── useOsAssignment.ts        (VmsSelectionStep)
│   ├── useRdmConfiguration.ts    (VmsSelectionStep)
│   ├── useRollingFormData.ts     (RollingMigrationForm)
│   ├── useRollingFormSubmit.ts   (RollingMigrationForm)
│   ├── useRollingFormValidation.ts (RollingMigrationForm)
│   ├── useSectionTracking.ts     (MigrationForm)
│   └── useVmSelection.ts         (VmsSelectionStep)
├── pages/                        # Page-level views
│   ├── MigrationForm.tsx         ← moved from root (510 lines, was 1,827)
│   ├── MigrationsPage.tsx        (pre-existing)
│   └── RollingMigrationForm.tsx  ← moved from root (2,678 lines, was 4,264)
├── steps/                        # Multi-step form step components ← NEW dir
│   ├── MigrationOptionsAlt.tsx   (929 lines, was 947)
│   ├── NetworkAndStorageMappingStep.tsx (322 lines, was 556)
│   ├── SecurityGroupAndServerGroup.tsx
│   ├── SourceAndDestinationEnvStep.tsx
│   ├── SourceDestinationClusterSelection.tsx
│   └── VmsSelectionStep.tsx      (1,497 lines, was 2,687)
├── utils/
│   ├── ipValidation.ts           ← extracted (IP helpers)
│   ├── migrationTableUtils.ts    ← extracted (table utils)
│   └── vmNetworking.ts
├── constants.ts                  ← expanded with all shared constants
└── types.ts                      ← all interfaces consolidated
```

---

## What Was Done (This Session)

### Resolved merge conflicts
`MigrationForm.tsx` + `RollingMigrationForm.tsx` had 3-stage index entries (conflict started, never staged). Working tree was clean. Ran `git add` to mark resolved.

### Confirmed Phase 3 complete
All 10 pending hooks (for VmsSelectionStep + MigrationForm) were already extracted. Verified by checking imports in component files.

### Directory reorganization
Deleted `MigrationOptions.tsx` (zero imports, legacy unused file).

Moved via `git mv`:
- `useClusterData.ts` → `hooks/useClusterData.ts`
- `MigrationForm.tsx` → `pages/MigrationForm.tsx`
- `RollingMigrationForm.tsx` → `pages/RollingMigrationForm.tsx`
- 6 step components → `steps/`

Updated import paths in 10 files:
- `src/App.tsx` — 2 imports to `pages/`
- `src/features/onboarding/pages/Onboarding.tsx` — 1 import to `pages/`
- `hooks/useRollingFormData.ts` — `../useClusterData` → `./useClusterData`
- `hooks/useRollingFormSubmit.ts` — same
- `pages/MigrationForm.tsx` — all `./` relative paths updated to `../` or `../steps/` or `../hooks/`
- `pages/RollingMigrationForm.tsx` — same pattern
- `steps/VmsSelectionStep.tsx` — `./` → `../`
- `steps/NetworkAndStorageMappingStep.tsx` — same
- `steps/SourceDestinationClusterSelection.tsx` — `./useClusterData` → `../hooks/useClusterData`
- `steps/MigrationOptionsAlt.tsx` — `./` → `../`

---

## Phase Summary

| Phase | Work | Status |
|-------|------|--------|
| 1 | Audit | ✅ |
| 2 | Extract types, constants, utils | ✅ |
| 3 | Extract 21 custom hooks | ✅ |
| 4 | Import cleanup, remove re-exports | ✅ |
| 5 | Directory reorganization | ✅ |

---

## Key Design Notes

**Hook import rule:** hooks import types from `'../types'`, never from component files (avoids circular deps).

**`RollingMigrationForm` local types:** keeps own `FormValues` (simpler, no vmwareCreds/openstackCreds) and `SelectedMigrationOptionsType` (adds `osFamily: boolean`). Both structurally compatible with `types.ts` canonical versions.

**Hook call order in RollingMigrationForm matters:**
```typescript
const { submitting, handleSubmit, handleClose } = useRollingFormSubmit({ ... })
// useRollingFormValidation needs `submitting` from above
const { isSubmitDisabled, ... } = useRollingFormValidation({ ...submitting... })
```

**`RollingFormParams` type** in `types.ts` needed because spread operators fail on `Record<string, unknown>` (TS2698). Explicit interface extends it.

**`useClusterData.ts`** exports `SourceDataItem`, `PcdDataItem` — imported by `hooks/useRollingFormData.ts` and `hooks/useRollingFormSubmit.ts` as `'./useClusterData'` (same dir).

---

## Pitfalls Reference

| Problem | Fix |
|---------|-----|
| `TS2698: Spread types may only be created from object types` | `params: Record<string, unknown>` → add typed `RollingFormParams` interface |
| Circular import in hook | Hook imported type from component file; fix: import from `'../types'` |
| Edit tool partial match → orphaned dead code | Use exact full-block replacement, not partial string match |
| Hook returns values unused in JSX (JSX reads `params.x` directly) | Only destructure what JSX actually uses |

---

## Resume Prompt

> All refactoring complete on branch `587-ui-refactor-migration`.
>
> Working directory: `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
>
> **What was done:**
> - Extracted 21 custom hooks from 4 large component files
> - Extracted shared `types.ts`, `constants.ts`, `utils/ipValidation.ts`, `utils/migrationTableUtils.ts`
> - Reorganized `src/features/migration/` into: `hooks/`, `pages/`, `steps/`, `utils/`, `components/`, `api/`, `context/`
> - Deleted unused `MigrationOptions.tsx`
>
> **tsc --noEmit: 0 errors**
>
> Pending: commit + open PR against `main`.
>
> See `vjailbreak-refactor-handoff.md` in `ui/` for full context.
