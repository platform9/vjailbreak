# Data Model: Migration Templates and Saved Configurations

> **Status**: Rewritten 2026-07-20 to match what actually shipped. The original version of this file
> described extending `MigrationTemplate` with `Spec.Saved`/`Status.TimesUsed`/`Status.LastUsedAt` —
> that never happened. Backend PR [platform9/vjailbreak#2158](https://github.com/platform9/vjailbreak/pull/2158)
> instead added a brand-new CRD, `MigrationBlueprint`. This file now describes that CRD as it actually
> exists. See `spec.md`'s "Implementation Reality" section for the full list of divergences.

## Entity: `MigrationBlueprint` (new CRD, `vjailbreak.k8s.pf9.io/v1alpha1`, plural `migrationblueprints`)

Completely independent of the pre-existing ephemeral `MigrationTemplate` CRD — a saved template is a
different Kubernetes object kind entirely, not a flagged variant of the same kind. There is no
"saved vs. ephemeral" distinction to gate anywhere, because they're different CRDs.

**No status subresource.** `MigrationBlueprint` has no `Status` field at all — no usage counters, no
observed state of any kind. Every field lives in `Spec`.

### Spec fields (`k8s/migration/api/v1alpha1/migrationblueprint_types.go`; UI-side mirror in
`ui/src/api/migration-blueprints/model.ts`)

| Field | Type | Notes |
|---|---|---|
| `displayName` | string | Required. User-facing name shown on cards/table/detail drawer. |
| `description` | string | Optional. Free-text. |
| `vmwareRef` | string | Optional. Source VMware credential **name**. |
| `vmwareClusterName` | string | Optional. Added to the CRD after initial ship (not in original PR #2158 diff reviewed 2026-07-20). Source vCenter cluster name. Prefilled into the New Migration form's `vmwareCluster` dropdown by `useApplyTemplatePrefill.ts` (resolved name→id same as `targetPCDClusterName`) and captured on save by `MigrationForm.tsx`'s `buildSaveTemplateInput`. **Not displayed** on the template card, table, or detail drawer — round-trips through save/apply but is invisible in the Templates tab UI. |
| `noVMwareClusterFilter` | bool | Optional. Declared on the CRD and mirrored in `ui/src/api/migration-blueprints/model.ts`, but **not wired anywhere** — no save/apply/display code references it. Dead field as far as this feature's UI is concerned. |
| `pcdRef` | string | Optional. Destination OpenStack/PCD credential **name**. Tenant/project is NOT stored — resolved live from the OpenStack creds list's `spec.projectName` at display time (`useTemplateTenantLookup.ts`), matched by this name. |
| `targetPCDClusterName` | string | Optional. PCD cluster **name**, not an id. `FormValues.pcdCluster` expects an id — prefill resolves name→id via `pcdData.find(p => p.name === ...)`, same pattern as `useRetryPrefill.ts`. |
| `networkMappings` | `Network[]` | Optional. Each `{ source, target }`. |
| `storageMappings` | `Storage[]` | Optional. Each `{ source, target }`. |
| `arrayCredsMappings` | `{ source, target }[]` | Optional. Datastore→array-creds mapping. |
| `proxyVMRef` | `{ name }` | Optional. HotAdd proxy VM reference. |
| `migrationStrategy` | object | Optional. `{ type: 'hot'\|'cold'\|'mock', dataCopyStart?, vmCutoverStart?, vmCutoverEnd?, adminInitiatedCutOver?, performHealthChecks?, healthCheckPort?, disconnectSourceNetwork?, arrayOffload? }`. `type` drives the Hot/Cold/Mock copy chip and icon everywhere in the UI. `adminInitiatedCutOver` drives the cutover-policy label. |
| `advancedOptions` | object | Optional. `{ granularVolumeTypes?, granularNetworks?, granularPorts?, periodicSyncInterval?, periodicSyncEnabled?, networkPersistence?, removeVMwareTools?, acknowledgeNetworkConflictRisk?, imageProfiles? }`. Feeds the detail drawer's "Advanced" summary row. |
| `postMigrationAction` | object | Optional. `{ renameVm?, suffix?, moveToFolder?, folderName? }`. Also feeds the "Advanced" summary row. |
| `firstBootScript` | string | Optional. Presence feeds the "Advanced" summary row ("Post-migration script"). |
| `securityGroups` | string[] | Optional. |
| `serverGroup` | string | Optional. |
| `fallbackToDHCP` | bool | Optional. |
| `preserveSourceTags` | bool | Optional. |
| `customMetadata` | `Record<string,string>` | Optional. |
| `useGPUFlavor` | bool | Optional. |
| `storageCopyMethod` | `'normal' \| 'StorageAcceleratedCopy' \| 'HotAdd'` | Optional. Drives the detail drawer's "Copy method" row (label via `STORAGE_COPY_METHOD_OPTIONS`). |
| `osFamily` | `'windowsGuest' \| 'linuxGuest'` | Optional. Undefined means "Auto-detect" (`guestOsLabel()`). |
| `virtioWinDriver` | string | Optional. |

### Fields that do NOT exist (do not add UI for these without a backend change first)

- No `saved` bool, no `vjailbreak.k8s.pf9.io/saved` label — not needed, different CRD.
- No `status.timesUsed` / `status.lastUsedAt` — no status subresource on this CRD at all.
- No source *datacenter* field — `vmwareClusterName` exists (see above) but there is still no
  datacenter-level field, and the cluster field itself isn't surfaced in the Templates tab UI.
- No `tenantProject` field — always resolved live via OpenStack creds lookup, never stored.

### Validation rules

- `displayName` MUST be non-empty; the k8s object name is a sanitized version of it
  (`sanitizeTemplateName()` in `ui/src/features/migration/api/migration-blueprints/adapters.ts`), with a
  numeric suffix appended on collision (`uniqueTemplateName()`) before POST — enforced client-side, with
  the Kubernetes API's object-name uniqueness (`AlreadyExists` 409) as the authoritative fallback.

### State transitions

```
[operator fills New Migration form]
        │
        │ (ephemeral MigrationTemplate auto-created/patched — a completely separate CRD,
        │  entirely untouched by any of this)
        │
        ▼
  "Save as template" ──► POST new MigrationBlueprint, displayName/description set,
                          same source/destination/mapping/options values copied from
                          the current form state (useSaveAsTemplate / SaveAsTemplateDialog)
        │
        ▼
  [Templates tab lists it — GET .../migrationblueprints, no filtering needed:
   every object of this kind IS a saved template]
        │
        ├── "Use" ──► useApplyTemplatePrefill maps MigrationBlueprint.Spec → FormValues
        │                   │   (stale/missing references silently left unset —
        │                   │    NO warning shown; FR-009 not implemented)
        │                   ▼
        │            [operator edits freely, submits — ordinary MigrationTemplate/
        │             MigrationPlan created exactly as a manually-filled form would]
        │            (no usage-counter write — nothing to write to)
        │
        ├── "Clone" ──► POST new MigrationBlueprint, same Spec values, new unique
        │                displayName (default "<name> (copy)")
        │
        ├── "Edit" ──► same New Migration form, opened with templateMode='edit' —
        │               useApplyTemplatePrefill seeds it same as "Use", footer swaps
        │               to "Save Changes" ──► PUT same MigrationBlueprint object
        │               (metadata.resourceVersion sent for optimistic concurrency;
        │                useUpdateTemplate, added 2026-07-22 — see spec.md User Story 6)
        │
        └── "Delete" ──► DELETE this MigrationBlueprint object
                          (does not affect any MigrationPlan/Migration already created
                           from it — those hold their own resolved config independently)
```

### Create Template (standalone entry point, added 2026-07-22)

Distinct from "Save as template" inside an active New Migration session (US1): the Templates tab's
"Create New Template" button opens the same New Migration form with `templateMode='create'` and no
prefill — footer shows only "Create Template" (no "Start Migration", no separate "Save as template"
secondary action). Saving still goes through the same `POST` as any other create. See spec.md User
Story 7.

## Entity: `FormValues` (existing frontend type, unchanged shape)

No new fields were added to `ui/src/features/migration/types.ts`'s `FormValues`. This feature adds a
new way to *populate* an instance of it (`useApplyTemplatePrefill.ts`, mapping `MigrationBlueprint.Spec`
→ `FormValues`) and a new way to *persist* one (`useSaveAsTemplate` / `savedTemplateInputToBlueprintSpec`
in `adapters.ts`, mapping the current form state → a `MigrationBlueprintSpec`).

## Entity: `SavedTemplate` (frontend-only, UI-facing flattened view)

`ui/src/features/migration/api/migration-blueprints/types.ts` — every card/table/drawer component reads
this shape, not the raw `MigrationBlueprint`, via `blueprintToSavedTemplate()` in `adapters.ts`:

```ts
interface SavedTemplate {
  name: string              // k8s object name
  resourceVersion: string   // metadata.resourceVersion, '' if absent — added 2026-07-22 for Edit
                            // Template's optimistic-concurrency PUT (see contracts/crds.md)
  displayName: string
  description?: string
  createdAt: string         // metadata.creationTimestamp
  sourceVCenter: string     // spec.vmwareRef
  sourceCluster: string     // spec.vmwareClusterName — round-trips but not shown in card/table/drawer
  destination: string       // spec.pcdRef
  targetCluster: string     // spec.targetPCDClusterName
  networkMappings: { source: string; target: string }[]
  storageMappings: { source: string; target: string }[]
  dataCopyMethod: 'hot' | 'cold' | 'mock'   // spec.migrationStrategy.type
  cutoverOption: string                      // derived from migrationStrategy (see adapters.ts)
  osFamily?: string
  useGPU?: boolean
  spec: MigrationBlueprintSpec               // full spec, for clone/re-post round-tripping
}
```
