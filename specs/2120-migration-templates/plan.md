# Implementation Plan: Migration Templates and Saved Configurations

**Branch**: `2120-migration-templates` | **Date**: 2026-07-15 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/2120-migration-templates/spec.md`

## Summary

Let operators save a migration's configuration (source/destination, network/storage mappings, copy method, cutover policy, etc.) as a named, reusable **Migration Template**, browse templates from a new "Templates" tab next to "Migrations", and apply ("Use") a template to pre-fill the New Migration drawer. Adds delete/clone lifecycle management.

Technical approach: extend the existing `MigrationTemplate` CRD (no new CRD, no new controller) to carry saved-template metadata in `Spec` (DisplayName, Description, Saved) and usage stats in a newly-added `Status` (TimesUsed, LastUsedAt) — the CRD already declares `+kubebuilder:subresource:status` and `ui-manager-role` already has `migrationtemplates/status` get/patch/update RBAC, so no new RBAC is required. Reuse the existing generic k8s-API CRUD frontend module, the `DrawerShell` panel primitive, and the existing retry-prefill pattern for "apply template". Gate the existing ephemeral-template auto-patch/auto-delete lifecycle on `Spec.Saved != true` so saved templates are never touched by an unrelated migration session.

---

## Technical Context

**Language/Version**: Go 1.21 (controller), TypeScript/React 18 (UI)
**Primary Dependencies**: controller-runtime, k8s.io/client-go (controller side, unchanged); MUI, React Query, Vite (UI side)
**Storage**: Kubernetes etcd (CRD state) — no new database, no new CRD
**Testing**: `cd k8s/migration && make test` (controller — deepcopy/webhook/type tests only, no reconciler to test since none is added); `yarn test` (UI — component/hook tests)
**Target Platform**: k3s on Linux (vJailbreak appliance), React SPA in browser
**Project Type**: Kubernetes CRD extension + React UI (no new controller, no new backend REST service)
**Performance Goals**: Templates list/search/filter must feel instant for the expected scale (tens of templates per appliance, not thousands) — no server-side pagination required for v1
**Constraints**: Must not regress `MigrationPlanReconciler`'s existing consumption of `MigrationTemplate` by name; must not regress the existing ephemeral per-session template lifecycle used by every New Migration drawer open; no new backend REST surface in `pkg/vpwned`
**Scale/Scope**: Single-appliance, every template visible to every operator — no ownership/visibility concept, no real multi-tenant auth, per Clarifications

---

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Kubernetes-Native Architecture | PASS | Saved-template state lives entirely in the existing `MigrationTemplate` CR (Spec + new Status); no external store |
| II. External Documentation First | PASS | No new external dependency introduced; controller-runtime/kubebuilder subresource conventions reviewed in research.md |
| III. Generated Code Protection | PASS | `make generate` run inside `k8s/migration/` after editing `migrationtemplate_types.go`; `zz_generated.deepcopy.go` and `config/crd/bases/*migrationtemplates*.yaml` / `deploy/00crds.yaml` regenerated, never hand-edited |
| IV. Test-First Development | PASS | New Go code is limited to type definitions (DeepCopy is generated, not hand-written) — no new reconciler logic is added, so no new mocked-dependency unit tests are required on the controller side; UI hook/component logic (save/apply/delete/clone, saved-vs-ephemeral gating) gets unit tests per FR coverage below |
| V. Module Independence | PASS | Only `k8s/migration/api/v1alpha1/` (types) touched on the Go side; `ui/` touched on the frontend side; no cross-module coupling added, no `pkg/vpwned` changes |
| VI. AI-Assisted Development | PASS | Skills invoked for spec/plan generation |
| VII. Code Reuse and Simplicity | PASS | Reuses existing CRD, existing generic CRUD API module, existing `DrawerShell`, existing retry-prefill pattern; no new CRD/controller/backend service introduced |

**Result**: All gates pass. No violations.

---

## Project Structure

### Documentation (this feature)

```text
specs/2120-migration-templates/
├── plan.md              # This file (/speckit-plan output)
├── research.md          # Phase 0 — technical decisions and rationale
├── data-model.md         # Phase 1 — entity definitions and state transitions
├── quickstart.md        # Phase 1 — developer guide
├── contracts/
│   └── crds.md          # Phase 1 — CRD YAML contract (extended MigrationTemplate)
└── tasks.md              # Phase 2 output (/speckit-tasks — NOT created by /speckit-plan)
```

### Source Code Changes

```text
# CONTROLLER (k8s/migration/ module)
k8s/migration/api/v1alpha1/
└── migrationtemplate_types.go     [MODIFY — add Saved/DisplayName/Description to Spec;
                                     add new MigrationTemplateStatus{TimesUsed, LastUsedAt} + Status field]

k8s/migration/api/v1alpha1/
└── zz_generated.deepcopy.go       [REGENERATED via `make generate` — do not hand-edit]

k8s/migration/config/crd/bases/
└── vjailbreak.k8s.pf9.io_migrationtemplates.yaml  [REGENERATED via `make generate`]

deploy/
├── 00crds.yaml                    [REGENERATED via `make generate-manifests`]
└── installer.yaml                 [REGENERATED via `make generate-manifests` — NEVER hand-edit;
                                     no RBAC diff expected, ui-manager-role already grants
                                     migrationtemplates/status get/patch/update]

# No changes to:
#   k8s/migration/internal/controller/migrationplan_controller.go (additive fields, unused by that path)
#   k8s/migration/cmd/main.go (no new controller registered)
#   pkg/vpwned/ (no new REST endpoints)

# UI
ui/src/features/migration/api/migration-templates/
├── model.ts                       [MODIFY — add saved-template fields to MigrationTemplate type;
                                     add MigrationTemplateStatus type]
├── migrationTemplates.ts          [MODIFY — add listSavedMigrationTemplates (label/field-selector
                                     filtered by spec.saved=true), patchMigrationTemplateStatus
                                     (status subresource PATCH for usage tracking)]
└── helpers.ts                     [MODIFY — add buildSavedTemplateJson(name, description,
                                     formValues), cloneTemplateJson(template)]

ui/src/features/migration/pages/
└── MigrationsPage.tsx             [MODIFY — add MUI Tabs (Migrations | Templates); existing
                                     table content becomes the "Migrations" tab panel unchanged]

ui/src/features/migration/components/templates/
├── TemplatesTabPanel.tsx          [NEW — search/filter/sort toolbar + card grid, empty states]
├── TemplateCard.tsx               [NEW — one card per template, per FR-005]
├── TemplateDetailDrawer.tsx       [NEW — built on DrawerShell, modeled on
                                     ui/src/features/proxyvms/components/ProxyVMDetailDrawer.tsx]
├── SaveAsTemplateDialog.tsx       [NEW — name + optional description]
└── DeleteTemplateDialog.tsx       [NEW — confirmation, mirrors DeleteMigrationDialog.tsx pattern]

ui/src/features/migration/hooks/
├── useMigrationTemplatesQuery.ts  [NEW — React Query hook for saved templates list]
├── useSaveAsTemplate.ts           [NEW — wraps buildSavedTemplateJson + postMigrationTemplate]
├── useApplyTemplatePrefill.ts     [NEW — maps a MigrationTemplate to FormValues, mirrors the
                                     mapping already done in useRetryPrefill.ts]
├── useCredentialFetching.ts       [MODIFY — skip auto-patch when the active MigrationTemplate has
                                     spec.saved === true]
└── useMigrationFormSubmit.ts      [MODIFY — skip auto-delete-on-cancel when spec.saved === true;
                                     on successful submit from an applied template, PATCH
                                     status.timesUsed/status.lastUsedAt on that saved template]

ui/src/features/migration/context/
└── MigrationFormContext.tsx       [MODIFY — openMigrationForm accepts an optional
                                     templatePrefill payload alongside the existing retryConfig]
```

**Structure Decision**: Single existing module boundary preserved — only `k8s/migration/api/v1alpha1/` (types) changes on the backend, everything else is additive UI code inside `ui/src/features/migration/`. No new Go module, no new CRD, no new controller, no new `pkg/vpwned` endpoint.

---

## Phase 0: Research

See [research.md](research.md). Key decisions:

| Unknown | Resolution |
|---------|------------|
| Extend existing CRD vs. new CRD | Extend `MigrationTemplateSpec` + add `MigrationTemplateStatus` (per Clarifications; user-confirmed direction) |
| Where do usage counters live — Spec or Status? | `Status` (new) — `+kubebuilder:subresource:status` marker already exists on the type with no backing field; `ui-manager-role` already has `migrationtemplates/status` RBAC; counters are observed/runtime data, not desired-state, so Status is the conventional location |
| Distinguishing saved vs. ephemeral templates | New `Spec.Saved bool` (`omitempty`, default false) — ephemeral per-session templates never set it, saved templates always set it `true` |
| Filtering the list to saved templates only | Kubernetes label selector (`vjailbreak.k8s.pf9.io/saved=true`, set alongside `Spec.Saved` at creation) rather than a spec-field selector, since the k8s API does not support arbitrary spec-field selectors for CRDs without an `x-kubernetes` field-selector extension |
| Page-level tab pattern (no existing precedent) | Plain MUI `Tabs`/`Tab` inside `MigrationsPage.tsx`, not the existing `NavTabs`/`SectionNav` (those are a vertical in-form section nav, a different affordance) |
| Template detail panel | Reuse `DrawerShell`, model on `ProxyVMDetailDrawer.tsx` (closest existing list+detail-drawer precedent) |
| Apply-template prefill mechanism | Extend `MigrationFormContext.openMigrationForm` with an optional prefill payload, mapped via a new hook mirroring `useRetryPrefill.ts`'s existing template→`FormValues` mapping |
| Usage-counter update mechanism | Direct `PATCH` of `.status` from the UI at the moment of successful migration submission (no reconciler — matches the constitution's "no unnecessary controller" precedent set by the removed `MigrationTemplateReconciler`) |

---

## Phase 1: Design

### Data Model

See [data-model.md](data-model.md).

**Modified CRD**: `MigrationTemplate` — `Spec` gains `DisplayName`, `Description`, `Saved`; new `Status` gains `TimesUsed`, `LastUsedAt`.
**No new CRDs, no new controllers, no new v2v-helper types** (this feature is UI/CRD-schema only; it never reaches the migration worker).

### Contracts

See [contracts/crds.md](contracts/crds.md) — extended `MigrationTemplate` CRD YAML (Spec + Status), REST paths for saved-template CRUD and the status-subresource PATCH used for usage tracking.

### Implementation Notes

#### `migrationtemplate_types.go` changes

```go
type MigrationTemplateSpec struct {
    // ... existing fields unchanged ...

    // DisplayName is the user-facing template name shown in the Templates UI.
    // Empty for ephemeral per-session templates.
    // +optional
    DisplayName string `json:"displayName,omitempty"`
    // Description is an optional user-provided description of the template.
    // +optional
    Description string `json:"description,omitempty"`
    // Saved marks this MigrationTemplate as a user-saved, reusable template rather
    // than a disposable per-migration-session config object. The New Migration
    // drawer's auto-patch/auto-delete lifecycle MUST skip any template with Saved=true.
    // +optional
    Saved bool `json:"saved,omitempty"`
}

// MigrationTemplateStatus defines observed usage statistics for a saved template.
type MigrationTemplateStatus struct {
    // TimesUsed counts successful migrations submitted from this template.
    // +optional
    TimesUsed int `json:"timesUsed,omitempty"`
    // LastUsedAt is the timestamp of the most recent successful migration
    // submitted from this template.
    // +optional
    LastUsedAt *metav1.Time `json:"lastUsedAt,omitempty"`
}

type MigrationTemplate struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   MigrationTemplateSpec   `json:"spec,omitempty"`
    Status MigrationTemplateStatus `json:"status,omitempty"`
}
```

Run `make generate` inside `k8s/migration/` — regenerates `zz_generated.deepcopy.go` (now needs a `DeepCopyInto` for `MigrationTemplateStatus` and the `*metav1.Time` pointer field) and the CRD schema. Run `make generate-manifests` (from repo root, per CLAUDE.md — requires `vjail-controller` and `ui` built first) to refresh `deploy/00crds.yaml`/`deploy/installer.yaml`. Do not hand-edit either generated file.

Label `vjailbreak.k8s.pf9.io/saved: "true"` is set on the object's `metadata.labels` at creation time (alongside `Spec.Saved = true`) purely so the list query can use a label selector; the label and the spec field must always be written together — the label is a query optimization, `Spec.Saved` is the source of truth for lifecycle-gating decisions in `useCredentialFetching.ts`/`useMigrationFormSubmit.ts`.

#### Ephemeral-lifecycle gating (`useCredentialFetching.ts`, `useMigrationFormSubmit.ts`)

Both hooks already hold the active `migrationTemplate` object in state. Add a guard at the top of the auto-patch effect and the `handleClose` delete call:

```ts
if (migrationTemplate?.spec?.saved) {
  return // never auto-patch or auto-delete a saved template
}
```

This is the only change needed to protect saved templates from the existing ephemeral-session lifecycle — no change to the shape of that lifecycle itself.

#### "Use template" prefill flow

1. `TemplateCard`/`TemplateDetailDrawer`'s "Use"/"Use template" button calls `openMigrationForm('standard', undefined, templatePrefill)` where `templatePrefill` is the selected `MigrationTemplate`.
2. `MigrationFormContext` stores `templatePrefill` and passes it into `MigrationForm.tsx`.
3. A new `useApplyTemplatePrefill(templatePrefill)` hook (mirroring the existing `useRetryPrefill.ts` mapping from `MigrationTemplate` → `FormValues`) resolves the referenced VMware/OpenStack creds, network/storage mappings, and options into `FormValues`, calling `updateParams(...)` once resolution completes. Any reference that no longer resolves (deleted cred/mapping/cluster) is left unset and reported via a new `staleReferenceWarnings` array rendered as an inline `Alert` in the relevant form section (FR-009).
4. VM selection is deliberately left out of the prefill — the operator picks VMs fresh from the live inventory of the pre-filled source/destination (per spec Assumptions).
5. On successful submit when `templatePrefill` was set, call the new `patchMigrationTemplateStatus(templateName, { timesUsed: template.status.timesUsed + 1, lastUsedAt: <now> })` against the template's `/status` subresource endpoint. Best-effort — a failure to update the counter must not fail the migration submission itself.

#### Templates tab (`MigrationsPage.tsx`)

Wrap existing content in an MUI `Tabs` bar with two panels: "Migrations" (existing table, unchanged) and "Templates" (`<TemplatesTabPanel />`, new). Tab state is local (`useState`), not URL-driven, matching the drawer-based (non-route) pattern already used elsewhere in this feature area — consistent with how `MigrationForm`'s own section nav is local state rather than query-param state.

#### Save-as-template action (`MigrationForm.tsx`)

A "Save as template" button/menu-item alongside the existing Cancel/Submit actions, enabled once source, destination are set (FR-001/FR-003 in spec's Acceptance Scenarios). Opens `SaveAsTemplateDialog` (name + optional description) → calls `useSaveAsTemplate` → `postMigrationTemplate` with `spec.saved=true`, `spec.displayName`, `spec.description`, plus the same source/destination/mapping/options fields the ephemeral auto-patch already writes today. Name-uniqueness (FR-002) is checked client-side against the already-fetched saved-templates list before POST, with a server-side duplicate-name response (409/AlreadyExists on a name collision, since k8s object names are unique) as the authoritative fallback.

---

## Complexity Tracking

No constitution violations. No additional complexity entries needed.
