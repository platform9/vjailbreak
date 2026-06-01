# VmsSelectionStep Refactor — Handoff

## Context

**Repo:** `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
**Branch:** `587-ui-refactor-migration`
**TypeScript check:**
```
/home/abhijeet/.nvm/versions/node/v18.20.7/bin/node node_modules/.bin/tsc --noEmit
```
Node: v18.20.7 — do NOT change.

---

## Completed Work (Phases 1–5) ✅

All uncommitted. tsc: 0 errors.

**What was done:**
- Phase 1: Added `CanonicalVM` type to `types.ts` + `utils/vmAdapters.ts` (4 converters: `fromVmDataWithFlavor`, `toVmDataWithFlavor`, `fromVM`, `toVM`)
- Phase 2: Extracted `BulkIPEditDialog` and `FlavorAssignmentDialog` into `components/`
- Phase 3: `VmsSelectionStep.tsx` fully rewritten as unified component with `mode?: 'standard' | 'rolling'`; `VmsSelectionStepProps` unified in `types.ts`
- Phase 4: Updated callers — `MigrationForm.tsx` (`mode="standard"`), `RollingMigrationForm.tsx` (replaced `<RollingVmsSelectionStep>` with `<VmsSelectionStep mode="rolling">`)
- Phase 5: Deleted `RollingVmsSelectionStep.tsx` (no barrel exports referencing it)

**Files modified:**
| File | Change |
|------|--------|
| `src/features/migration/types.ts` | `CanonicalVM` interface, `VmsSelectionStepProps` unified |
| `src/features/migration/utils/vmAdapters.ts` | new file — 4 converters |
| `src/features/migration/components/BulkIPEditDialog.tsx` | new file — extracted dialog |
| `src/features/migration/components/FlavorAssignmentDialog.tsx` | new file — extracted dialog |
| `src/features/migration/components/index.ts` | added exports for both dialogs |
| `src/features/migration/hooks/useFlavorHandlers.ts` | added `setSelectedFlavor` to return |
| `src/features/migration/steps/VmsSelectionStep.tsx` | full rewrite — unified component |
| `src/features/migration/pages/MigrationForm.tsx` | added `mode="standard"` |
| `src/features/migration/pages/RollingMigrationForm.tsx` | swapped import, added `mode="rolling"` |
| `src/features/migration/steps/RollingVmsSelectionStep.tsx` | DELETED |

---

## Current State: `VmsSelectionStep.tsx`

**1646 lines.** Breakdown:

| Section | Lines | Notes |
|---------|-------|-------|
| Imports | 1–61 | |
| Styled components | 66–91 | |
| `StandardToolbarWithActions` | 93–139 | `props: any` — untyped |
| `RollingToolbarWithActions` | 141–168 | `props: any` — untyped |
| Component function body (state/hooks/effects) | 176–559 | |
| `standardColumns` array | 562–843 | 280 lines, defined inside render |
| `rollingColumns` array | 846–1148 | 300 lines, defined inside render |
| JSX return | 1156–1606 | |
| `arePropsEqual` + `React.memo` | 1609–1646 | |

**Column `renderCell` sizes:**
| Column | Mode | Lines |
|--------|------|-------|
| `name` | standard | ~40 |
| `ipAddress` | standard | ~65 |
| `osFamily` | standard | ~95 |
| `name` | rolling | ~15 |
| `ip` | rolling | ~85 |
| `osFamily` | rolling | ~95 |
| `flavor` | rolling | ~50 |

**Key duplication:** `osFamily` renderCell is ~95 lines in both standard and rolling — logic nearly identical, only trigger condition differs:
- Standard: shows `Select` when `isSelected` (any power state)
- Rolling: shows `Select` when `isSelected && powerState === 'powered-off'`

---

## Further Refactor Plan (Phases A–G)

Zero behavior change. tsc must stay at 0 errors after each phase.

---

### Phase A — Type toolbar components

**Files:** `steps/VmsSelectionStep.tsx` only (lines 93–168)

Add typed interfaces to replace `props: any`:

```typescript
interface StandardToolbarWithActionsProps {
  rowSelectionModel: string[]
  onAssignFlavor: () => void
  onAssignIP: () => void
  hasRdmVMs: boolean
  onAssignRdmConfiguration: () => void
  selectedCount: number
  rdmVMsCount: number
  // CustomSearchToolbar passthrough
  onRefresh?: () => void
  disableRefresh?: boolean
  placeholder?: string
}

interface RollingToolbarWithActionsProps {
  rowSelectionModel: string[]
  onAssignFlavor: () => void
  onAssignIP: () => void
  hasPoweredOffVMs: boolean
}
```

Replace `(props: any)` with `(props: StandardToolbarWithActionsProps)` / `(props: RollingToolbarWithActionsProps)`.

**Impact:** ~5 lines added, high type-safety value. No behavior change.

---

### Phase B — Extract cell components

**New directory:** `src/features/migration/components/cells/`

**New files:**

#### `OsFamilyCell.tsx`
Unified — replaces ~190 lines of duplicated logic across both column sets.

```typescript
export interface OsFamilyCellProps {
  vmId: string
  powerState: string
  detectedOsFamily?: string
  assignedOsFamily?: string
  // Standard: show select when selected regardless of power state
  // Rolling: show select only when selected AND powered-off
  showSelectWhenSelected: boolean
  onOSAssignment: (vmId: string, osFamily: string) => void
}
```

Internal logic:
- `currentOsFamily = assignedOsFamily ?? detectedOsFamily`
- Select renders when `showSelectWhenSelected` is true
- Display icons/text for Windows/Linux/Unknown/Other

**Callers:**
- Standard `osFamily` renderCell: `<OsFamilyCell showSelectWhenSelected={isSelected} .../>`
- Rolling `osFamily` renderCell: `<OsFamilyCell showSelectWhenSelected={isSelected && powerState === 'powered-off'} .../>`

#### `StandardIpAddressCell.tsx`
Extracts lines 616–679 (~65 lines) from standard `ipAddress` column.

```typescript
export interface StandardIpAddressCellProps {
  vm: VmDataWithFlavor
  isSelected: boolean
  originalIPsPerVM: Record<string, Record<number, string>>
}
```

#### `RollingIpAddressCell.tsx`
Extracts lines 875–956 (~85 lines) from rolling `ip` column.

```typescript
export interface RollingIpAddressCellProps {
  vm: VM
  isSelected: boolean
}
```

#### `RollingFlavorCell.tsx`
Extracts lines 1091–1141 (~50 lines) from rolling `flavor` column.

```typescript
export interface RollingFlavorCellProps {
  vmId: string
  currentFlavor: string
  isSelected: boolean
  openstackFlavors: OpenStackFlavor[]
  onFlavorChange: (vmId: string, flavorId: string) => void
}
```

**Export all from `components/cells/index.ts`**, then re-export from `components/index.ts`.

**Impact:** ~370 lines removed from main file. Cells become independently testable.

---

### Phase C — Extract column definition hooks

**New files:**
- `src/features/migration/hooks/useStandardColumns.ts`
- `src/features/migration/hooks/useRollingColumns.ts`

Each hook returns `GridColDef[]` via `useMemo` with explicit dependency arrays.

**`useStandardColumns` signature:**
```typescript
export function useStandardColumns(params: {
  selectedVMs: Set<string>
  duplicateNames: Set<string>
  vmOSAssignments: Record<string, string>
  originalIPsPerVM: Record<string, Record<number, string>>
  handleOSAssignment: (vmId: string, os: string) => void
}): GridColDef[]
```

**`useRollingColumns` signature:**
```typescript
export function useRollingColumns(params: {
  selectedVMs: GridRowSelectionModel
  vmOSAssignments: Record<string, string>
  openstackFlavors: OpenStackFlavor[]
  handleOSAssignment: (vmId: string, os: string) => void
  handleFlavorChange: (vmId: string, flavorId: string) => void
}): GridColDef[]
```

**In main component:**
```typescript
const standardColumns = useStandardColumns({ selectedVMs: selectedVMsStandard, duplicateNames, vmOSAssignments, originalIPsPerVM: standardBulkIP.originalIPsPerVM, handleOSAssignment: standardHandleOSAssignment })
const rollingColumns = useRollingColumns({ selectedVMs: rollingSelectedVMs, vmOSAssignments, openstackFlavors: rollingOpenstackFlavors, handleOSAssignment: handleRollingOSAssignment, handleFlavorChange: rollingFlavor.handleIndividualFlavorChange })
```

**Requires Phase B done first** (cell components used inside column definitions).

**Impact:** ~580 lines removed from main file (after Phase B already removed ~370). Main file drops from ~1270 to ~690 lines.

---

### Phase D — Move `normalizeNetworkInterfaces` to utils

**Current location:** `steps/VmsSelectionStep.tsx` lines 485–495 (inside component function).

**Target:** `src/features/migration/utils/vmAdapters.ts` (already exists) or new `src/features/migration/utils/networkUtils.ts`.

```typescript
export function normalizeNetworkInterfaces(
  networkInterfaces?: VmData['networkInterfaces']
): VmData['networkInterfaces'] {
  if (!networkInterfaces || networkInterfaces.length === 0) return networkInterfaces
  return networkInterfaces.map((nic) => ({
    ...nic,
    ipAddress: Array.isArray((nic as any).ipAddress)
      ? (nic as any).ipAddress
      : (nic as any).ipAddress
        ? [(nic as any).ipAddress]
        : [],
  }))
}
```

Import in `VmsSelectionStep.tsx`, delete local definition.

**Impact:** 12 lines removed from component, utility is now testable.

---

### Phase E — Extract dialog prop helpers

**Problem:** `BulkIPEditDialog` and `FlavorAssignmentDialog` are each wired with ~25 ternary-per-prop lines in JSX — hard to read.

**Option A: Helper functions (simpler)**

```typescript
function buildFlavorDialogProps(
  isRolling: boolean,
  rollingFlavor: ReturnType<typeof useFlavorHandlers>,
  standardFlavor: ReturnType<typeof useFlavorAssignment>,
  rollingSelectedVMs: GridRowSelectionModel,
  standardSelectedVMs: Set<string>,
  rollingFlavors: OpenStackFlavor[],
  standardFlavors: OpenStackFlavor[],
): FlavorAssignmentDialogProps { ... }
```

**Option B: Hook (if deps need memoization)**

```typescript
const flavorDialogProps = useFlavorDialogProps({ isRolling, rollingFlavor, standardFlavor, ... })
```

**Impact:** JSX shrinks ~50 lines; intent becomes clear.

---

### Phase F — Extract `useToast` hook

**Current:** 5 state items + 2 handlers inline in component (lines 220–239).

```typescript
// src/features/migration/hooks/useToast.ts
export function useToast() {
  const [toastOpen, setToastOpen] = useState(false)
  const [toastMessage, setToastMessage] = useState('')
  const [toastSeverity, setToastSeverity] = useState<'success' | 'error' | 'warning' | 'info'>('success')
  const showToast = useCallback((message: string, severity = 'success') => { ... }, [])
  const handleClose = useCallback((_event?: React.SyntheticEvent | Event, reason?: string) => { ... }, [])
  return { toastOpen, toastMessage, toastSeverity, showToast, handleClose }
}
```

**Impact:** ~20 lines removed from component; reusable across other forms.

---

### Phase G — `useVmsSelectionState` mega-hook (optional)

Extract ALL hook calls, state, and effects from the component function body into one custom hook.

```typescript
// src/features/migration/hooks/useVmsSelectionState.ts
export function useVmsSelectionState(props: VmsSelectionStepProps) {
  // All useState, useEffect, useMemo, useCallback, useXxx calls
  // Returns everything needed for JSX
  return { isRolling, canonicalVMs, standardBulkIP, rollingBulkIP, ... }
}
```

Component becomes:
```tsx
function VmsSelectionStep(props: VmsSelectionStepProps) {
  const state = useVmsSelectionState(props)
  return <VmsSelectionStepContainer>...</VmsSelectionStepContainer>
}
```

**Risk:** Large surface area. Only do after A–F are complete and tsc verified.

**Impact:** Component file drops to ~250–350 lines of pure JSX.

---

## Phase Execution Order

| Phase | Prerequisite | Effort | Impact |
|-------|-------------|--------|--------|
| A — Type toolbars | none | S | Type safety |
| D — Move `normalizeNetworkInterfaces` | none | XS | Testability |
| F — `useToast` hook | none | S | Reusability |
| B — Extract cell components | none | M | -370 lines |
| E — Dialog prop helpers | none | S | Readability |
| C — Column hooks | B | M | -580 lines (cumulative) |
| G — `useVmsSelectionState` | A–F | L | -300 lines |

Phases A, D, F, B, E are independent — can parallelize or do in any order.
Phase C requires Phase B.
Phase G requires all others.

---

## Pitfalls

| Problem | Fix |
|---------|-----|
| `osFamily` renderCell trigger condition differs between modes | `OsFamilyCell` gets `showSelectWhenSelected: boolean` prop — caller computes condition |
| Column hooks need stable callback refs | Pass memoized callbacks; use `useCallback` at call site |
| `standardBulkIP.originalIPsPerVM` not in current public hook API | Check `useBulkIPEdit` return type before referencing in `useStandardColumns` |
| `normalizeNetworkInterfaces` uses `(nic as any)` casts | Keep casts in extracted util — don't change logic |
| Rolling `arePropsEqual` returns `false` always | Preserve this in `React.memo` wrapper; do NOT change during column extraction |

---

## Resume Prompt

> Branch: `587-ui-refactor-migration`
> Working dir: `/home/abhijeet/Projects/Platform9/vjailbreak/ui`
>
> **Goal:** Further refactor `VmsSelectionStep.tsx` (currently 1646 lines). All prior phases (1–5) complete — `VmsSelectionStep` is already a unified `mode?: 'standard' | 'rolling'` component. Callers updated. `RollingVmsSelectionStep.tsx` deleted. All changes UNCOMMITTED. tsc: 0 errors.
>
> **Next refactor phases (A–G):**
> - Phase A: Type `StandardToolbarWithActions` / `RollingToolbarWithActions` (replace `props: any`)
> - Phase B: Extract cell components (`OsFamilyCell`, `StandardIpAddressCell`, `RollingIpAddressCell`, `RollingFlavorCell`) into `components/cells/`
> - Phase C: Extract column definitions into `useStandardColumns` / `useRollingColumns` hooks (requires Phase B)
> - Phase D: Move `normalizeNetworkInterfaces` to `utils/`
> - Phase E: Extract dialog prop helpers for `BulkIPEditDialog` / `FlavorAssignmentDialog`
> - Phase F: Extract `useToast` hook from inline state
> - Phase G: `useVmsSelectionState` mega-hook (optional, do last)
>
> See `vm-selection-refactor-handoff.md` for full interfaces, signatures, line numbers, and pitfalls.
