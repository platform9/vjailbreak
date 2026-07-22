# Contracts: `MigrationBlueprint` CRD & REST paths

> **Status**: Rewritten 2026-07-20. The original version of this file described an *extended*
> `MigrationTemplate` CRD with a `status` subresource for usage tracking — that was never built.
> Backend PR [platform9/vjailbreak#2158](https://github.com/platform9/vjailbreak/pull/2158) shipped a
> brand-new CRD, `MigrationBlueprint`, with **no status subresource at all**. This file now describes
> that CRD. See `spec.md`'s "Implementation Reality" and `data-model.md` for the full field list.

## CRD: `MigrationBlueprint` (new kind)

`apiVersion: vjailbreak.k8s.pf9.io/v1alpha1`, `kind: MigrationBlueprint`, plural `migrationblueprints`.
Defined in `k8s/migration/api/v1alpha1/migrationblueprint_types.go` (backend PR #2158).

### Example: saved template as it actually looks

```yaml
apiVersion: vjailbreak.k8s.pf9.io/v1alpha1
kind: MigrationBlueprint
metadata:
  name: production-rhel-east
  namespace: migration-system
spec:
  displayName: "Production RHEL · East"
  description: "Standard hot migration for east-region RHEL web & app tiers. Admin-gated cutover, Ceph NVMe storage."
  vmwareRef: vcenter-east-creds
  vmwareClusterName: cluster-east-a          # source vCenter cluster, added after initial ship
  pcdRef: pcd-east-1-creds
  targetPCDClusterName: cluster-prod-a       # a NAME, not the PCD cluster id
  networkMappings:
    - source: vmnet-prod
      target: net-prod-east-a
    - source: vmnet-data
      target: net-data-east
  storageMappings:
    - source: DS_NVME_01
      target: ceph-nvme
    - source: DS_SAS_02
      target: ceph-data-01
  storageCopyMethod: StorageAcceleratedCopy
  migrationStrategy:
    type: hot
    adminInitiatedCutOver: true
  useGPUFlavor: false
# no status block — this CRD has no status subresource
```

There is no "ephemeral vs. saved" variant of this kind — every `MigrationBlueprint` object IS a saved
template. The pre-existing ephemeral, uuid-named, auto-created/auto-deleted per-session config object is
a `MigrationTemplate` (unchanged, untouched, a different kind entirely — see its own
`k8s/migration/api/v1alpha1/migrationtemplate_types.go`).

### Full `MigrationBlueprintSpec` shape (TypeScript mirror, `ui/src/api/migration-blueprints/model.ts` —
authoritative Go source is `k8s/migration/api/v1alpha1/migrationblueprint_types.go` in PR #2158)

```ts
interface MigrationBlueprintSpec {
  displayName: string
  description?: string
  vmwareRef?: string
  pcdRef?: string
  vmwareClusterName?: string
  noVMwareClusterFilter?: boolean
  targetPCDClusterName?: string
  networkMappings?: { source: string; target: string }[]
  storageMappings?: { source: string; target: string }[]
  arrayCredsMappings?: { source: string; target: string }[]
  proxyVMRef?: { name: string }
  migrationStrategy?: {
    type: 'hot' | 'cold' | 'mock'
    dataCopyStart?: string
    vmCutoverStart?: string
    vmCutoverEnd?: string
    adminInitiatedCutOver?: boolean
    performHealthChecks?: boolean
    healthCheckPort?: string
    disconnectSourceNetwork?: boolean
    arrayOffload?: boolean
  }
  advancedOptions?: {
    granularVolumeTypes?: string[]
    granularNetworks?: string[]
    granularPorts?: string[]
    periodicSyncInterval?: string
    periodicSyncEnabled?: boolean
    networkPersistence?: boolean
    removeVMwareTools?: boolean
    acknowledgeNetworkConflictRisk?: boolean
    imageProfiles?: string[]
  }
  postMigrationAction?: {
    renameVm?: boolean
    suffix?: string
    moveToFolder?: boolean
    folderName?: string
  }
  firstBootScript?: string
  securityGroups?: string[]
  serverGroup?: string
  fallbackToDHCP?: boolean
  preserveSourceTags?: boolean
  customMetadata?: Record<string, string>
  useGPUFlavor?: boolean
  storageCopyMethod?: 'normal' | 'StorageAcceleratedCopy' | 'HotAdd'
  osFamily?: 'windowsGuest' | 'linuxGuest'
  virtioWinDriver?: string
}
```

## REST API (existing generic Kubernetes custom-resource path — no new backend service)

Base path (per `ui/src/api/migration-blueprints/migrationBlueprints.ts`):
`/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/{namespace}/migrationblueprints`

| Operation | Method + Path | Notes |
|---|---|---|
| List templates | `GET .../migrationblueprints` | No label selector needed — every object of this kind is a saved template, there's nothing else to filter out. |
| Get one template | `GET .../migrationblueprints/{name}` | |
| Create template | `POST .../migrationblueprints` | `metadata.name` = sanitized `displayName` (`sanitizeTemplateName()`, collision-suffixed by `uniqueTemplateName()`). |
| Update template | `PUT .../migrationblueprints/{name}` | Added 2026-07-22 (Edit Template — see `spec.md` User Story 6). Body includes `metadata.resourceVersion` (optimistic concurrency — standard k8s behavior, a stale `resourceVersion` gets a 409 from the API server; the UI does not retry or merge, it just surfaces the error via `SaveAsTemplateDialog`'s inline `Alert`). Same object name in, same object name out — this updates the existing `MigrationBlueprint` in place rather than creating a new one. `createMigrationBlueprintJson(name, spec, resourceVersion)` in `ui/src/api/migration-blueprints/helpers.ts` builds the body; `resourceVersion` is only included when doing an update (omitted entirely for Create/Clone, both `POST`). |
| Clone template | `POST .../migrationblueprints` | Same as Create, with `spec` copied from the source template's `spec` and a new unique `metadata.name`/`spec.displayName` (default `"<name> (copy)"`). |
| Delete template | `DELETE .../migrationblueprints/{name}` | |

**No `/status` subresource endpoint** — `MigrationBlueprint` doesn't have one, so there is nothing to
PATCH for usage tracking. No RBAC changes were needed for any of the above (same generic
`ui-manager-role` custom-resource grant pattern as every other CRD this UI already talks to).

No changes to `pkg/vpwned/`'s REST surface — all of the above goes through the existing
nginx/vite-proxied path straight to the Kubernetes API server.
