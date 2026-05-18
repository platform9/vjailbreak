# RollingMigrationForm Refactor Handoff

## Goal
Make 5 shared sections in `RollingMigrationForm.tsx` structurally identical to `MigrationForm.tsx`.
Extract rolling-specific inline code into proper components/hooks.
Zero behavior change.

**Working directory:** `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
**Branch:** `587-ui-refactor-migration`

**TypeScript check:**
```
/home/abhijeet/.nvm/versions/node/v18.20.7/bin/node node_modules/.bin/tsc --noEmit
```
Node: v18.20.7 — do NOT change.

---

## Current Status: ALL PHASES COMPLETE ✅

**tsc --noEmit: 0 errors**

Work is **uncommitted** — all changes staged in working tree, not yet in a commit.

Pending: commit + PR.

---

## What Was Done

### Pre-Work — types.ts extended
Added to `RollingFormParams`:
```typescript
vmwareCluster?: string  // "credName:datacenter:clusterName"
pcdCluster?: string
networkMappings?: ResourceMap[]
storageMappings?: ResourceMap[]
arrayCredsMappings?: ResourceMap[]
securityGroups?: string[]
serverGroup?: string
```

Added to `RollingMigrationRHFValues`:
```typescript
securityGroups: string[]
serverGroup: string
```

### Phase 1 ✅ — Source And Destination aligned
- Removed `sourceCluster`, `destinationPCD`, `selectedVMwareCredName`, `selectedPcdCredName` useState
- Removed `handleSourceClusterChange`, `handleDestinationPCDChange`
- `SourceDestinationClusterSelection` now receives `onChange={getParamsUpdater}`, reads from `params.vmwareCluster`/`params.pcdCluster`
- `useRollingFormData.ts` — accepts `vmwareCluster`/`pcdCluster` from params, returns `selectedVMwareCredName`/`selectedPcdCredName`
- `useRollingFormSubmit.ts` — reads cred names from `useRollingFormData` return, reads cluster from params
- `useRollingFormValidation.ts` — reads cluster from params

### Phase 2 ✅ — RollingVmsSelectionStep extracted
**New file:** `steps/RollingVmsSelectionStep.tsx` (~1029 lines)
- All `vmColumns` renderCell logic
- `CustomToolbarWithActions` component
- `handleOSAssignment`
- `renderValidationAdornment`
- Bulk IP Edit Dialog JSX
- Flavor Assignment Dialog JSX
- `vmOSAssignments` state
- `missingInterfaceIpWarnings` memo
- `useBulkIPHandlers` and `useFlavorHandlers` are now internal to this step

Form uses: `<RollingVmsSelectionStep vmsWithAssignments={...} selectedVMs={...} openstackFlavors={...} ... />`

### Phase 3 ✅ — Map Networks And Storage aligned
- Removed `networkMappings`, `storageMappings`, `arrayCredsMappings` local state
- Removed `networkMappingError`, `storageMappingError` local state (moved to `useRollingFormValidation` return)
- Removed `handleMappingsChange` wrapper
- `NetworkAndStorageMappingStep` now receives `params={params}` + `onChange={getParamsUpdater}`
- `useRollingFormSubmit.ts` reads from `params.networkMappings` etc.
- `useRollingFormValidation.ts` returns `networkMappingError`, `storageMappingError`

### Phase 4 ✅ — Security Groups section added
- `SecurityGroupAndServerGroupStep` added to Rolling form
- `securityRef` added, section nav updated
- `useSectionTracking` sections array includes `security`
- `touchedSections` includes `security: false`

### Phase 5 ✅ — Migration Options aligned
- Removed `onOptionsChange`/`onOptionsSelectionChange` wrappers
- `MigrationOptions` receives `getParamsUpdater`/`updateSelectedMigrationOptions` directly

### Phase 6a ✅ — useSectionTracking wired
Replaced 65-line inline IntersectionObserver with:
```typescript
useSectionTracking({ open, contentRootRef, sections: [...], setActiveSectionId })
```
Refs renamed to semantic names: `sourceDestRef`, `baremetalRef`, `hostsRef`, `vmsRef`, `mapResourcesRef`, `securityRef`, `optionsRef`.

### Phase 6b ✅ — useRollingFormSync extracted
**New file:** `hooks/useRollingFormSync.ts` (~166 lines)
- Moved 6 manual RHF↔params `useEffect`s from inline form
- Includes `securityGroups`/`serverGroup` sync for Phase 4

### Phase 6c ✅ — sectionNavItems moved to useRollingFormValidation
`step1HasErrors`…`stepNHasErrors`, `hasAnyMigrationOptionSelected`, `areSelectedMigrationOptionsConfigured`, `sectionNavItems` useMemo now computed in `useRollingFormValidation`.
Returned alongside existing validation values.

### Phase 7 ✅ — Inline dialogs extracted
**New files in `components/`:**
- `MaasConfigDetailDialog.tsx` (~216 lines) — was inline `MaasConfigDialog`
- `HostConfigAssignmentDialog.tsx` (~93 lines) — was inline dialog

---

## New Files Created

| File | Lines | Contents |
|------|-------|----------|
| `steps/RollingVmsSelectionStep.tsx` | ~1029 | VM DataGrid, OS/flavor/bulkIP dialogs, column defs |
| `hooks/useRollingFormSync.ts` | ~166 | RHF↔params sync effects |
| `components/MaasConfigDetailDialog.tsx` | ~216 | Extracted MaaS config dialog |
| `components/HostConfigAssignmentDialog.tsx` | ~93 | Extracted host config assignment dialog |

---

## Modified Files

| File | Change |
|------|--------|
| `types.ts` | +7 fields to `RollingFormParams`, +2 to `RollingMigrationRHFValues` |
| `pages/RollingMigrationForm.tsx` | 2678 → 1027 lines (extracted ~1600 lines) |
| `hooks/useRollingFormData.ts` | Accept cluster from params; return cred names |
| `hooks/useRollingFormSubmit.ts` | Read cluster/mappings from params |
| `hooks/useRollingFormValidation.ts` | Read mappings from params; return errors + sectionNavItems |

---

## Shared Context

Prior refactor on this branch:
- 21 hooks extracted across 4 large component files
- `types.ts`, `constants.ts`, `utils/` extracted
- Directory reorganized: `pages/`, `steps/`, `hooks/` structure
- See `vjailbreak-refactor-handoff.md` for full prior-phase context

---

## Resume Prompt

> All rolling refactor phases complete on branch `587-ui-refactor-migration`.
>
> Working dir: `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
>
> **What was done:**
> - `RollingMigrationForm.tsx` reduced from 2678 → 1027 lines
> - Aligned 5 sections to match `MigrationForm.tsx` structure
> - Extracted `RollingVmsSelectionStep`, `useRollingFormSync`, `MaasConfigDetailDialog`, `HostConfigAssignmentDialog`
> - Added Security Groups section (was missing from Rolling form)
> - All rolling-specific hooks updated to read from `params` instead of separate state
>
> **tsc --noEmit: 0 errors**
>
> **Status: ALL UNCOMMITTED.** Changes staged in working tree but no commit yet.
>
> Next step: commit all staged changes + open PR against `main`.
>
> See `rolling-migration-refactor-handoff.md` for full context.
> See `vjailbreak-refactor-handoff.md` for prior hook-extraction + directory-reorg context.
