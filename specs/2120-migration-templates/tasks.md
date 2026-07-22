---
description: "Full task list for Migration Templates and Saved Configurations feature (UI + Backend)"
---

# Tasks: Migration Templates and Saved Configurations

> **⚠ SUPERSEDED / STATUS: feature is implemented, but not via this task list (2026-07-20).**
> Phase 1 (T001–T005, backend CRD extension) never happened — backend PR #2158 shipped a new CRD,
> `MigrationBlueprint`, instead of extending `MigrationTemplate`. Every task below that references
> `ui/src/features/migration/api/migration-templates/` (T006–T008), `patchMigrationTemplateStatus`
> (T007, T022), `Spec.Saved`-guard logic (T018's parenthetical), or usage counters (T022) describes
> work that was not (and now cannot be, per the real CRD) done. US1–US5 all shipped in practice, just
> through `ui/src/features/migration/api/migration-blueprints/` and the component/hook list in
> `plan.md`'s correction note — read `spec.md`'s "Implementation Reality" first. FR-009 (stale-reference
> inline warning, referenced in T018/T021) is the one real gap: not implemented. Leaving the checkboxes
> below unchecked/as-is; they no longer map to real work items.

**Feature**: `2120-migration-templates`
**Branch**: `2120-migration-templates`
**Reference patterns**: Proxy VMs list+detail-drawer (`ProxyVMsPage.tsx` + `ProxyVMDetailDrawer.tsx`) · Retry-prefill mapping (`useRetryPrefill.ts`) · Design system (`DrawerShell`, `SurfaceCard`, `KeyValueGrid`, `StatusChip`, `ActionButton`)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this belongs to
- Exact file paths in every description

---

## Phase 1: Setup — Backend CRD extension

- [ ] T001 Extend `MigrationTemplateSpec` in `k8s/migration/api/v1alpha1/migrationtemplate_types.go` with `DisplayName`, `Description`, `Saved`, `VMwareCluster` (composite `credName:datacenter:clusterName` string, mirrors `FormValues.vmwareCluster` so a saved template round-trips the source cluster selection exactly)
- [ ] T002 Add `MigrationTemplateStatus{TimesUsed int, LastUsedAt *metav1.Time}` struct and `Status MigrationTemplateStatus` field to `MigrationTemplate` in the same file
- [ ] T003 Run `cd k8s/migration && make generate` — regenerates `zz_generated.deepcopy.go` and `config/crd/bases/vjailbreak.k8s.pf9.io_migrationtemplates.yaml`; do not hand-edit either
- [ ] T004 Run `cd k8s/migration && make test` — confirm no regressions
- [ ] T005 From repo root, run `make vjail-controller ui && make generate-manifests` — confirms `deploy/00crds.yaml`/`deploy/installer.yaml` regenerate with no unexpected RBAC diff (research.md Decision 2 predicts none)

---

## Phase 2: Foundational — Frontend API layer

- [ ] T006 [P] Extend `MigrationTemplateSpec` in `ui/src/features/migration/api/migration-templates/model.ts` with `displayName?`, `description?`, `saved?: boolean`, `vmwareCluster?`; extend `MigrationTemplateStatus` with `timesUsed?: number`, `lastUsedAt?: string` (leave existing `openstack`/`vmware` status fields untouched — still read by `useCredentialFetching.ts`'s existing poll); widen `Labels` to allow an index signature for the new `vjailbreak.k8s.pf9.io/saved` label
- [ ] T007 [P] Add `getSavedMigrationTemplatesList(namespace?)` (labelSelector `vjailbreak.k8s.pf9.io/saved=true`) and `patchMigrationTemplateStatus(templateName, statusBody, namespace?)` (PATCH `.../migrationtemplates/{name}/status`, `application/merge-patch+json`) to `ui/src/features/migration/api/migration-templates/migrationTemplates.ts`
- [ ] T008 [P] Add `createSavedMigrationTemplateJson(params)` and `cloneSavedMigrationTemplateJson(template, newDisplayName)` to `ui/src/features/migration/api/migration-templates/helpers.ts`
- [ ] T009 [P] Create `ui/src/features/migration/utils/templateFilters.ts` with pure functions `filterTemplates(templates, query)` and `sortTemplates(templates, sortKey)` — extracted for unit testing per constitution's test-first requirement
- [ ] T010 [P] Unit tests: `ui/src/features/migration/utils/templateFilters.test.ts` (search by name/description, sort by last-used/name) and `ui/src/features/migration/api/migration-templates/helpers.test.ts` (`createSavedMigrationTemplateJson`, `cloneSavedMigrationTemplateJson`)

---

## Phase 3: User Story 1 — Save a Migration Configuration as a Template (P1)

- [ ] T011 [US1] Create `ui/src/features/migration/hooks/useMigrationTemplatesQuery.ts` — React Query hook wrapping `getSavedMigrationTemplatesList`
- [ ] T012 [US1] Create `ui/src/features/migration/hooks/useSaveAsTemplate.ts` — mutation wrapping `createSavedMigrationTemplateJson` + `postMigrationTemplate`; validates name uniqueness against the already-fetched saved-templates list before POST (FR-002), surfaces the API's `AlreadyExists` 409 as a fallback error
- [ ] T013 [US1] [P] Create `ui/src/features/migration/components/templates/SaveAsTemplateDialog.tsx` — name (required) + description (optional), disabled submit until source+destination are set
- [ ] T014 [US1] Wire "Save as template" action into `ui/src/features/migration/pages/MigrationForm.tsx`'s `DrawerFooter` (next to existing Cancel/Submit `ActionButton`s), enabled once `params.vmwareCreds` and `params.pcdCluster` are set; opens `SaveAsTemplateDialog`

---

## Phase 4: User Story 2 — Browse Available Templates (P1)

- [ ] T015 [US2] [P] Create `ui/src/features/migration/components/templates/TemplateCard.tsx` — per FR-005: name, description, source→destination summary, copy-method/cutover/mapping-count tag chips, last-used/usage-count, "Use" `ActionButton`; built on `SurfaceCard`
- [ ] T016 [US2] Create `ui/src/features/migration/components/templates/TemplatesTabPanel.tsx` — search box, sort `Select`, grid/list view toggle, `TemplateCard` grid using `useMigrationTemplatesQuery` + `templateFilters.ts`; empty states for "no templates yet" and "no templates match"
- [ ] T017 [US2] Add MUI `Tabs`/`Tab` ("Migrations" | "Templates", count badge) to `ui/src/features/migration/pages/MigrationsPage.tsx`; existing table becomes the "Migrations" panel unchanged, `<TemplatesTabPanel />` is the new "Templates" panel; local `useState` for active tab (research.md Decision 4)

---

## Phase 5: User Story 3 — Apply a Template to a New Migration (P1)

- [ ] T018 [US3] Create `ui/src/features/migration/hooks/useApplyTemplatePrefill.ts` — given a `MigrationTemplate`, resolves `vmwareRef`/`openstackRef` credentials, `networkMapping`/`storageMapping` (expanded via `getNetworkMapping`/`getStorageMapping`, mirroring `useRetryPrefill.ts`'s mapping resolution), and maps to `updateParams(...)` for `FormValues`; any reference that fails to resolve is reported via a returned `staleReferenceWarnings: string[]` instead of thrown; VM selection and `migrationTemplate`/ephemeral-template state are intentionally left untouched (Use Template only prefills `FormValues` — see plan.md's post-plan implementation note on why the ephemeral-lifecycle guard from the original plan is unnecessary)
- [ ] T019 [US3] Extend `MigrationFormContextValue.openMigrationForm` in `ui/src/features/migration/context/MigrationFormContext.tsx` to accept an optional third `templatePrefill?: MigrationTemplate` argument
- [ ] T020 [US3] Add `templatePrefill` state to `ui/src/App.tsx`, threaded into the `MigrationFormContext.Provider` value and passed as a new `templatePrefill` prop on `<MigrationFormDrawer />`; add `templatePrefill?: MigrationTemplate` to `MigrationFormDrawerProps` in `ui/src/features/migration/types.ts`
- [ ] T021 [US3] Wire `useApplyTemplatePrefill` into `ui/src/features/migration/pages/MigrationForm.tsx` (standard mode only, mirroring how `useRetryPrefill` is only meaningful in retry mode): render `staleReferenceWarnings` as inline `Alert`s in the relevant sections (FR-009); track the applied template's name in local state for usage-count reporting
- [ ] T022 [US3] Add optional `appliedTemplateName?: string` param to `useMigrationFormSubmit` in `ui/src/features/migration/hooks/useMigrationFormSubmit.ts`; on `handleSubmit` success, best-effort `patchMigrationTemplateStatus(appliedTemplateName, { status: { timesUsed: <current+1>, lastUsedAt: <ISO now> } })` wrapped in try/catch that never fails the submission (FR-011)
- [ ] T023 [US3] Add "Use" action wiring from `TemplateCard`/`TemplateDetailDrawer` → `openMigrationForm('standard', undefined, template)`

---

## Phase 6: User Story 4 — View Template Details (P2)

- [ ] T024 [US4] Create `ui/src/features/migration/components/templates/TemplateDetailDrawer.tsx` — built on `DrawerShell`/`DrawerHeader`/`DrawerFooter`, modeled on `ProxyVMDetailDrawer.tsx`: header (icon, name, description, close); `KeyValueGrid` info block (Times Used, Last Used, Created); "Source & Destination" `SurfaceCard` (`KeyValueGrid`); "Network & Storage Mappings" `SurfaceCard` (resolved mapping pairs); footer with Delete / Clone / "Use template" `ActionButton`s
- [ ] T025 [US4] Wire `TemplateCard` click (not the "Use" button) → open `TemplateDetailDrawer` from `TemplatesTabPanel.tsx`

---

## Phase 7: User Story 5 — Delete and Clone Templates (P2)

- [ ] T026 [US5] [P] Create `ui/src/features/migration/components/templates/DeleteTemplateDialog.tsx` — confirmation dialog mirroring `DeleteMigrationDialog.tsx`'s structure, calls `deleteMigrationTemplate`
- [ ] T027 [US5] Add "Clone" handler in `TemplateDetailDrawer.tsx` — calls `cloneSavedMigrationTemplateJson` + `postMigrationTemplate` with a default `"<name> (copy)"` display name, invalidates `useMigrationTemplatesQuery`'s query key
- [ ] T028 [US5] Wire Delete confirm → `deleteMigrationTemplate`, invalidate templates query, close drawer

---

## Phase 8: Polish & Cross-Cutting

- [ ] T029 [P] Barrel export `ui/src/features/migration/components/templates/index.ts`
- [ ] T030 Verify no TypeScript errors: `cd ui && yarn tsc --noEmit`
- [ ] T031 Run `cd ui && yarn test` — all new + existing tests pass
- [ ] T032 Manual smoke test per `quickstart.md`'s checklist (save, browse, apply, stale-reference, detail, delete/clone, ephemeral-lifecycle regression, retry-flow regression)
- [ ] T033 Update `specs/2120-migration-templates/plan.md`'s "Implementation Notes" with any decisions made during implementation that differ from the original plan (pattern established by `specs/003-hot-add-proxy/plan.md`)

---

## Dependencies

- Phase 1 (backend) has no dependency on Phase 2+ (frontend) and can run in parallel with it, but frontend code that reads `status.timesUsed`/`status.lastUsedAt` won't be meaningfully testable end-to-end until Phase 1 lands.
- Phase 2 blocks all of Phases 3–7 (every later phase imports from `model.ts`/`migrationTemplates.ts`/`helpers.ts`).
- US1 (save) should land before US2 (browse) is meaningfully testable (need at least one template to list), but the components themselves (`TemplateCard`, `TemplatesTabPanel`) can be built in parallel against mock data.
- US3 (apply) depends on US1 existing (need a saved template to apply) but its code (T018–T023) touches disjoint files from US1/US2 and can be developed in parallel.
- US4 (detail drawer) and US5 (delete/clone) both extend `TemplateDetailDrawer.tsx` — implement T024 before T026–T028.

## Implementation Strategy

**MVP = US1 + US2 + US3** (T001–T023): save, browse, apply — the core save/reuse loop the issue asks for. US4 (detail drawer) and US5 (delete/clone) are valuable but the feature already delivers value without them (a template can still be identified and used from its card alone).
