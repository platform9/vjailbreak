---
description: "UI task list for Hot-Add Proxy Migration feature"
---

# Tasks: Hot-Add Proxy Migration — UI

**Feature**: `1944-hot-add-proxy`  
**Scope**: UI only — API client, ProxyVM management page, migration form changes, sidebar/routing  
**Reference patterns**: ESXi SSH Keys (`ui/src/features/esxiSshKeys/`) · SAM copy method in `NetworkAndStorageMappingStep.tsx`

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US4)
- Exact file paths are in every description

---

## Phase 1: Setup — API Client (Shared Infrastructure)

**Purpose**: ProxyVM TypeScript interfaces and CRUD client that all UI components depend on.

- [ ] T001 Create `ui/src/api/proxy-vm/model.ts` with `ProxyVM`, `ProxyVMList`, and `ProxyVMComponentCheck` TypeScript interfaces matching the CRD (spec: `vmName`, `vmwareCredsRef`; status: `validationStatus`, `validationMessage`, `ipAddress`, `attachedDiskCount`, `componentsVerified`, `lastValidationTime`)
- [ ] T002 Create `ui/src/api/proxy-vm/proxyVm.ts` with `getProxyVMs`, `getProxyVM`, `createProxyVM`, and `deleteProxyVM` functions following the pattern in `ui/src/api/esxi-ssh-creds/esxiSshCreds.ts` — resource path is `proxyvms`, API version `vjailbreak.k8s.pf9.io/v1alpha1`, kind `ProxyVM`
- [ ] T003 Create `ui/src/api/proxy-vm/index.ts` barrel file with `export * from './model'` and `export * from './proxyVm'`

---

## Phase 2: Foundational — Routing & Navigation

**Purpose**: Register the route and sidebar entry so the page is reachable.  
**⚠️ CRITICAL**: Must be done before any ProxyVM page work.

- [ ] T004 Add `<Route path="proxy-vms" element={<ProxyVMPage />} />` to the `/dashboard` nested routes in `ui/src/App.tsx` (alongside the existing `esxi-ssh-keys` route at line 441) and add the import for `ProxyVMPage`
- [ ] T005 Add a "Proxy VMs" child nav entry to the Credentials group in `ui/src/config/navigation.tsx` (after the `esxi-ssh-keys` entry at line 65) with `id: 'proxy-vms'`, `label: 'Proxy VMs'`, `path: '/dashboard/proxy-vms'`, and a `DnsIcon` or `ComputerIcon` from `@mui/icons-material`

**Checkpoint**: Navigating to `/dashboard/proxy-vms` should render (even an empty shell) before proceeding.

---

## Phase 3: User Story 1 — Register and Verify Proxy VM (Priority: P1) 🎯 MVP

**Goal**: Operator can register a Proxy VM, see SSH key prerequisite instructions, and watch status transition from Verifying → Ready or Verification Failed with per-component detail.

**Independent Test**: Register a ProxyVM CR via the UI → confirm status chip updates (polling) → see Ready or VerificationFailed with component list. No migration needed.

### Implementation for User Story 1

- [ ] T006 [P] [US1] Create `ui/src/features/proxyVM/components/AddProxyVMDialog.tsx`: MUI Dialog with (1) an info Alert explaining the SSH key prerequisite — "Before registering, copy the vJailbreak appliance public key (`/root/.ssh/id_rsa.pub`) to the Proxy VM's `/root/.ssh/authorized_keys`"; (2) a "VM Name" TextField bound to `vmName` state; (3) a "VMware Credentials" Select populated by `useQuery` calling `getVMwareCreds()`, showing only validated creds; (4) Submit calls `createProxyVM({ vmName, vmwareCredsRef: { name: selectedCreds } })` and closes the dialog on success
- [ ] T007 [US1] Create `ui/src/features/proxyVM/pages/ProxyVMPage.tsx`: MUI DataGrid listing ProxyVM resources with columns Name, VM Name, Status (Chip coloured by status: `success`=Ready, `warning`=Verifying/Pending, `error`=VerificationFailed), IP Address, Attached Disks, Age; toolbar has an "Add Proxy VM" button that opens `AddProxyVMDialog`; delete action per row calls `deleteProxyVM`; poll via `refetchInterval: 5000` when any item has `validationStatus` of `Pending` or `Verifying` (stop polling when all are `Ready` or `VerificationFailed`); use `useQuery` from `@tanstack/react-query` and follow the pattern in `ui/src/features/esxiSshKeys/pages/EsxiSshKeysPage.tsx`
- [ ] T008 [US1] Add a `ComponentsVerifiedTooltip` sub-component inside `ProxyVMPage.tsx`: when hovering the Status chip of a VerificationFailed row, show a Tooltip listing each component from `status.componentsVerified` with a ✓ or ✗ per item and the per-component `message` for missing ones

**Checkpoint**: ProxyVM page fully usable — add, poll, see Ready / VerificationFailed with component detail, delete.

---

## Phase 4: User Story 2 — Select Hot-Add in Migration Form (Priority: P1)

**Goal**: "Hot-Add via Proxy VM" appears in the copy method dropdown; when selected, the ProxyVM selector replaces the StorageMapping/ArrayCredsMapping sections; form validates that a Ready ProxyVM is chosen before submission.

**Independent Test**: Open migration creation, select Hot-Add → confirm ProxyVM selector appears, StorageMapping section disappears → submit with no ProxyVM selected → validation error shown → select a Ready ProxyVM → form submits and MigrationTemplate CR contains `storageCopyMethod: HotAdd` and `proxyVMRef.name`.

### Implementation for User Story 2

- [ ] T009 [P] [US2] In `ui/src/features/migration/MigrationForm.tsx` (line 120): extend the `storageCopyMethod` union type to `'normal' | 'StorageAcceleratedCopy' | 'HotAdd'`; add `proxyVMRef?: string` field to `FormValues`; in the MigrationTemplate build function (around line 660), add an `else if (storageCopyMethod === 'HotAdd')` branch that sets `spec.storageCopyMethod = 'HotAdd'` and `spec.proxyVMRef = { name: params.proxyVMRef }` on the template spec; add form validation that when `storageCopyMethod === 'HotAdd'` the `proxyVMRef` field must be set
- [ ] T010 [US2] In `ui/src/features/migration/NetworkAndStorageMappingStep.tsx`: (1) append `{ value: 'HotAdd', label: 'Hot-Add via Proxy VM' }` to the `storageCopyMethodOptions` array (around line 40); (2) add a `useQuery` that fetches `getProxyVMs()` with `enabled: storageCopyMethod === 'HotAdd'`; (3) when `storageCopyMethod === 'HotAdd'`, hide the StorageMapping section and the ArrayCredsMapping section, and instead render a "Proxy VM" MUI Select populated with Ready ProxyVMs (label: name + IP, value: name) calling `onChange('proxyVMRef')` on change; (4) when no Ready ProxyVM exists, show an inline warning: "No ready Proxy VM found. Register one in Proxy VMs before proceeding."

**Checkpoint**: Migration form correctly shows/hides sections by copy method; HotAdd path submits correct CR fields.

---

## Phase 5: User Story 4 — View and Manage Proxy VMs (Priority: P2)

**Goal**: Operator can see full Proxy VM list with all status detail and remove stale entries.

**Independent Test**: Register two ProxyVMs, delete one → list reflects change immediately. No migration needed.

### Implementation for User Story 4

- [ ] T011 [US4] In `ui/src/features/proxyVM/pages/ProxyVMPage.tsx`: add a confirmation Dialog before delete ("Remove Proxy VM `{name}`? Any pending Hot-Add migration that depends on it will be blocked.") — only call `deleteProxyVM` after confirmation; show a success/error Snackbar after delete completes; add a "Last Validated" column displaying `status.lastValidationTime` formatted as a relative time (e.g., "2 minutes ago") using the existing date helpers in `ui/src/api/helpers.ts`

**Checkpoint**: Full ProxyVM lifecycle (add → verify → view detail → delete with confirmation) works without any migration.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T012 [P] Export `ProxyVMPage` from `ui/src/features/proxyVM/index.ts` barrel file (create if it doesn't exist) so the App.tsx import is clean
- [ ] T013 [P] Verify no TypeScript errors in modified files: `MigrationForm.tsx`, `NetworkAndStorageMappingStep.tsx`, `App.tsx`, `navigation.tsx` — run `cd ui && yarn tsc --noEmit` and fix any type errors
- [ ] T014 Smoke-test the existing copy methods (normal, StorageAcceleratedCopy) still render their respective mapping sections correctly after the `NetworkAndStorageMappingStep.tsx` changes — manual check in dev server

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Routing)**: Depends on Phase 1 (needs `ProxyVMPage` import) — BLOCKS Phase 3
- **Phase 3 (US1)**: Depends on Phase 1 + Phase 2
- **Phase 4 (US2)**: Depends on Phase 1 (needs `getProxyVMs`) — can run in parallel with Phase 3
- **Phase 5 (US4)**: Depends on Phase 3 (extends the ProxyVM page)
- **Phase 6 (Polish)**: Depends on Phases 3–5

### Within Each Phase

- T006 and T009 are marked [P] — they touch different files and can be worked simultaneously
- T007 depends on T006 (dialog is opened from the page)
- T008 depends on T007 (tooltip is inside the page)
- T010 depends on T009 (needs the `proxyVMRef` FormValues field to call `onChange`)
- T011 depends on T007 (extends the existing page)

---

## Parallel Execution Example

```
# Phase 1 — run all three in sequence (fast, shared infrastructure):
T001 → T002 → T003

# Phase 3 + Phase 4 — run in parallel once Phase 1 is done:
[Thread A] T006 → T007 → T008   (ProxyVM management page)
[Thread B] T009 → T010          (Migration form Hot-Add support)

# Phase 5 — after Thread A completes:
T011
```

---

## Implementation Strategy

### MVP (US1 + US2 only)

1. Complete Phase 1: API client (T001–T003)
2. Complete Phase 2: Routing (T004–T005)
3. Complete Phase 3: ProxyVM page (T006–T008) — operators can register + verify
4. Complete Phase 4: Migration form (T009–T010) — operators can start Hot-Add migrations
5. **STOP and validate**: register a ProxyVM → verify Ready → create Hot-Add migration → confirm CR has correct fields

### Full Delivery (adds US4)

6. Complete Phase 5: Delete confirmation + last-validated column (T011)
7. Complete Phase 6: Polish (T012–T014)
