# Data Model: Migration Templates and Saved Configurations

## Entity: `MigrationTemplate` (modified — existing CRD, `vjailbreak.k8s.pf9.io/v1alpha1`)

No new CRD is introduced. The existing `MigrationTemplate` type gains new `Spec` fields and a new `Status` (see contracts/crds.md for the full YAML). The same type continues to serve two distinct lifecycles, distinguished by `Spec.Saved`:

| Lifecycle | `Spec.Saved` | Name | Created by | Deleted by |
|---|---|---|---|---|
| Ephemeral per-session config (existing, unchanged) | `false` / unset | uuid | `useCredentialFetching.ts` auto-create on cred validation | `useMigrationFormSubmit.ts` auto-delete on Cancel |
| Saved, reusable template (new) | `true` | user-provided display name (used as the k8s object name, sanitized) | `SaveAsTemplateDialog` → `useSaveAsTemplate` | explicit operator "Delete" action only |

### Spec fields (new, additive)

| Field | Type | Required | Notes |
|---|---|---|---|
| `displayName` | string | required when `saved=true` | User-facing name shown on cards/detail drawer. Empty for ephemeral templates. |
| `description` | string | optional | Free-text, shown on cards/detail drawer. |
| `saved` | bool | — | `true` for user-saved templates; unset/`false` for ephemeral per-session objects. Authoritative flag for lifecycle-gating logic. |
| `visibility` | enum `shared` \| `private` | optional, default `private` | UI-level label only (see Assumptions in spec.md) — not access-control enforced. |
| `owner` | string | optional | Free-text display label, not a verified identity. |

### Spec fields (existing, unchanged, reused by saved templates)

`osFamily`, `virtioWinDriver`, `networkMapping`, `storageMapping`, `arrayCredsMapping`, `storageCopyMethod`, `proxyVMRef`, `source.vmwareRef`, `destination.openstackRef`, `targetPCDClusterName`, `useGPUFlavor` — a saved template stores the same shape of migration configuration the ephemeral object already stores today; nothing about these fields' meaning changes.

### Status fields (new)

| Field | Type | Notes |
|---|---|---|
| `timesUsed` | int | Incremented by the UI (status-subresource PATCH) on each successful migration submitted from this template. Never written by any controller. |
| `lastUsedAt` | `*metav1.Time` | Set alongside `timesUsed` on each increment. |

### Labels (new, companion to `Spec.Saved`)

| Label | Value | Purpose |
|---|---|---|
| `vjailbreak.k8s.pf9.io/saved` | `"true"` | Set only on saved templates, in lockstep with `Spec.Saved=true`, so the Templates list can use a `labelSelector` query rather than fetching and client-filtering every `MigrationTemplate` in the namespace (which would also return every live ephemeral per-session object). |

### Validation rules

- `displayName` MUST be non-empty and unique among templates where `saved=true` (enforced client-side pre-submit against the already-fetched saved-templates list, and authoritatively by the Kubernetes API's object-name uniqueness — the sanitized `displayName` is used as `metadata.name`, so a duplicate submission fails with an `AlreadyExists` 409, surfaced to the operator per spec FR-002).
- `visibility` MUST be one of `shared` / `private` (CRD enum validation).
- A template with `saved=false`/unset MUST NOT appear in the Templates tab list (label-selector-filtered) and MUST continue to be eligible for the existing ephemeral auto-patch/auto-delete lifecycle.
- A template with `saved=true` MUST be skipped by the ephemeral auto-patch (`useCredentialFetching.ts`) and auto-delete-on-cancel (`useMigrationFormSubmit.ts`) logic.

### State transitions

```
[operator fills New Migration form]
        │
        │ (ephemeral MigrationTemplate auto-created/patched, saved=false — unchanged existing behavior)
        │
        ▼
  "Save as template" ──► POST new MigrationTemplate, saved=true, displayName/description/visibility/owner set,
                          same source/destination/mapping/options values copied from the current form state
        │
        ▼
  [Templates tab lists it, label-selector saved=true]
        │
        ├── "Use" ──► prefill New Migration drawer FormValues from this template's Spec (stale refs → warning, FR-009)
        │                   │
        │                   ▼
        │            [operator edits freely, submits]
        │                   │
        │                   ▼
        │            on submit success: PATCH this template's /status → timesUsed+1, lastUsedAt=now
        │
        ├── "Clone" ──► POST new MigrationTemplate, saved=true, same Spec values, new unique displayName
        │                (default "<name> (copy)"), owner=current session, status reset to zero
        │
        └── "Delete" ──► DELETE this MigrationTemplate object
                          (does not affect any MigrationPlan/Migration already created from it —
                           those hold their own resolved config independently, per FR-010/SC-006)
```

## Entity: `FormValues` (existing frontend type, unchanged shape, new producer/consumer)

No new fields are added to `ui/src/features/migration/types.ts`'s `FormValues`. This feature adds a new way to *populate* an instance of it (via `useApplyTemplatePrefill`, mapping a saved `MigrationTemplate.Spec` → `FormValues`, mirroring the existing `useRetryPrefill.ts` mapping) and a new way to *persist* one (via `useSaveAsTemplate`, mapping the current `FormValues` → a saved `MigrationTemplate.Spec`).
