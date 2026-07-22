# Research: Migration Templates and Saved Configurations

> **⚠ Outcome note (2026-07-20)**: Decisions 1 and 2 below were overridden by backend PR #2158, which
> shipped a new CRD (`MigrationBlueprint`, no status subresource) instead of extending
> `MigrationTemplate`. Kept as historical record of the reasoning at the time — see `spec.md`'s
> "Implementation Reality" for what actually happened and why. Decisions 3–6 (saved-vs-ephemeral
> distinction is moot now, but the tab/drawer/prefill UI decisions) held up as implemented.

## Decision 1: Extend the existing `MigrationTemplate` CRD rather than introduce a new CRD

**Decision**: Add saved-template fields directly to `MigrationTemplateSpec`/`MigrationTemplateStatus` in `k8s/migration/api/v1alpha1/migrationtemplate_types.go`, rather than creating e.g. a `SavedMigrationTemplate` CRD.

**Rationale**: This is the user's explicit direction for this feature (see spec.md Clarifications). It avoids duplicating the mapping/credential-reference schema that already exists on `MigrationTemplate`, and lets `MigrationPlanReconciler`'s existing `getMigrationTemplateAndCreds` continue to consume templates exactly as it does today — a saved template is, after all, still a valid `MigrationTemplate` that a `MigrationPlan` can reference directly if the operator submits a migration from it.

**Alternatives considered**:
- **New CRD** (e.g. `SavedMigrationTemplate`) — cleaner separation between "ephemeral per-session config" and "user-saved template", zero risk of the auto-patch/auto-delete lifecycle ever touching a saved object. Rejected per user direction; the risk is mitigated instead by the `Spec.Saved` guard (Decision 3) rather than by type-level separation. This alternative should be revisited if the `Saved`-guard approach proves fragile in practice (flagged as residual risk in spec.md).

**Outcome**: This is exactly what shipped, just not through this plan — backend PR #2158 introduced `MigrationBlueprint`, a wholly new CRD, for the reason this "alternative" predicted (clean separation, zero risk of touching the ephemeral lifecycle). The `Spec.Saved` guard was never built because it was never needed.

## Decision 2: Usage counters (`TimesUsed`, `LastUsedAt`) live in a new `Status`, not `Spec`

**Decision**: Add a `MigrationTemplateStatus` struct with `TimesUsed int` and `LastUsedAt *metav1.Time`, and a `Status MigrationTemplateStatus` field on `MigrationTemplate`.

**Rationale**: `migrationtemplate_types.go` already carries a `// +kubebuilder:subresource:status` marker (line 76-77 in the pre-feature file) with no backing Go `Status` field — a latent inconsistency, likely left over from when a `MigrationTemplateReconciler` existed (removed in commit `3fd0895b`). Adding the field now both fixes that inconsistency and gives usage stats — observed/runtime data, not desired configuration — the conventional Kubernetes home. Critically, `deploy/installer.yaml`'s `ui-manager-role` **already** grants `get/patch/update` on `migrationtemplates/status` (it was generated for the same subresource marker), so the UI can PATCH usage stats today with zero RBAC changes.

**Alternatives considered**:
- **Counters in `Spec`** — simpler (one PATCH path, no subresource awareness needed in the frontend client) but semantically wrong (mixes desired state with observed state) and would need a new RBAC verb if `ui-manager-role`'s existing `migrationtemplates` (non-status) rule didn't already cover it — it does, but abusing Spec for mutable runtime counters is still poor practice. Rejected.

**Outcome**: Moot — `MigrationBlueprint` shipped with no `Status` field at all, so neither option was available. Usage tracking was dropped from the UI entirely rather than working around the missing subresource client-side.

## Decision 3: Saved vs. ephemeral distinction via `Spec.Saved bool` + a companion label

**Decision**: `Spec.Saved bool` is the authoritative flag consumed by `useCredentialFetching.ts`/`useMigrationFormSubmit.ts` to skip auto-patch/auto-delete. A companion Kubernetes label `vjailbreak.k8s.pf9.io/saved: "true"` is written at the same time purely to support server-side list filtering.

**Rationale**: The Kubernetes API does not support querying custom resources by arbitrary `spec` field values without a CRD-defined field selector (`x-kubernetes-selectable-fields`, a relatively recent/optional CRD feature not currently used anywhere in this codebase's CRDs) — a label selector is the conventional, always-available filtering mechanism for custom resources on the generic REST path this UI already uses (`?labelSelector=...`). Keeping `Spec.Saved` as the field actually inspected by application logic (rather than trusting the label alone) avoids any risk of a stale/incorrectly-set label silently misclassifying a template.

**Alternatives considered**:
- **Label only, no `Spec.Saved` field** — one less field, but couples correctness to label hygiene (labels are more easily hand-edited/forgotten than a typed spec field validated by the CRD schema). Rejected.
- **Naming convention** (e.g. all ephemeral templates prefixed `session-`) — brittle, easy to collide with a user-chosen template name; the existing ephemeral templates are already uuid-named specifically to avoid needing a convention. Rejected.

**Outcome**: Moot — since saved templates live in a different CRD (`MigrationBlueprint`) than ephemeral ones (`MigrationTemplate`), there was never anything to distinguish within a single kind. Neither `Spec.Saved` nor the companion label were built.

## Decision 4: Page-level "Templates" tab uses plain MUI `Tabs`, not `NavTabs`/`SectionNav`

**Decision**: Add a horizontal MUI `Tabs`/`Tab` bar inside `MigrationsPage.tsx` (`Migrations` | `Templates`), local `useState`-driven, not a new route.

**Rationale**: This app's only existing tab-like navigation primitives, `NavTabs`/`SectionNav` (`ui/src/components/design-system/ui/`), are used exclusively inside the New Migration drawer as a vertical section jump-list for a single long form — a different UI affordance (in-page anchor navigation) from a page-level view switcher. The mockup's breadcrumb (`Virtual Machines / Migrations / Templates`) and tab strip (`Migrations | Templates 6`) matches a lightweight top-level tab switch, which plain MUI `Tabs` delivers directly without forcing an ill-fitting reuse of the section-nav component or inventing a new primitive.

**Alternatives considered**:
- **New sidebar nav entry** (like Proxy VMs, Agents) — this app's established pattern for *permanent* sibling sections (`ui/src/config/navigation.tsx`'s `children` array under "Migrations"). Rejected because the mockup explicitly shows Templates as a same-page tab next to Migrations, not a separate sidebar-routed page, and a route change would also require updating the breadcrumb component and existing `/dashboard/migrations` deep links.
- **Reuse `NavTabs`** — visually and semantically mismatched (vertical in-form section list vs. horizontal page switch); would need non-trivial prop changes to repurpose. Rejected in favor of plain `Tabs` (simpler, per Constitution Principle VII).

## Decision 5: Template detail panel reuses `DrawerShell`, modeled on `ProxyVMDetailDrawer`

**Decision**: `TemplateDetailDrawer.tsx` is built on the existing `DrawerShell` primitive (`ui/src/components/design-system/ui/DrawerShell.tsx`), structurally modeled on `ui/src/features/proxyvms/components/ProxyVMDetailDrawer.tsx`.

**Rationale**: `DrawerShell` already implements exactly the mockup's behavior — right-anchored slide-in panel, dimmed backdrop, close (X) control, optional close-confirmation — and is the established pattern for "list page + detail drawer" across this app (Proxy VMs, Storage Management credentials). `ProxyVMDetailDrawer` is the closest existing example of a read-mostly detail panel with a metadata block plus action footer, making it the best structural reference for header/info-block/sections/footer layout.

**Alternatives considered**: A new bespoke modal/panel component — rejected outright; no reason to diverge from an established, working pattern (Constitution Principle VII).

## Decision 6: "Apply template" prefill reuses the `useRetryPrefill.ts` mapping pattern

**Decision**: A new `useApplyTemplatePrefill` hook maps a selected `MigrationTemplate` to `FormValues`, following the same template-by-name → `FormValues` resolution already implemented in `ui/src/features/migration/hooks/useRetryPrefill.ts` for retry-mode prefill.

**Rationale**: Retry-prefill already solves the exact sub-problem this feature needs — reading a `MigrationTemplate`'s `Spec` and resolving it back into the New Migration drawer's `FormValues` shape, including handling of network/storage mapping references, OS family, virtio driver, and GPU flavor settings. Reusing this logic (rather than re-deriving a second mapping function) satisfies Constitution Principle VII and reduces the surface area that must independently handle the "referenced resource no longer exists" edge case (spec FR-009) — that stale-reference handling can be added once, in a shape both retry-prefill and apply-template-prefill can share if useful during implementation.

**Alternatives considered**: A parallel, template-specific mapping function written from scratch — rejected as needless duplication of already-solved logic.
