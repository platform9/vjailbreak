import { alpha, type Theme } from '@mui/material/styles'
import { CUTOVER_TYPES, OS_TYPES_OPTIONS, STORAGE_COPY_METHOD_OPTIONS } from '../constants'
import type { DataCopyMethod, SavedTemplate } from '../api/migration-blueprints/types'
import type { TemplateGroupNameLookup } from '../hooks/useTemplateGroupLookup'

export const DATA_COPY_METHOD_LABEL: Record<DataCopyMethod, string> = {
  hot: 'Hot',
  cold: 'Cold',
  mock: 'Mock'
}

// Palette key per copy method — mirrors the hot/cold/mock dot colors used on the
// Migrations table's Name column, so the two views read consistently.
export const DATA_COPY_METHOD_CHIP_COLOR: Record<DataCopyMethod, 'warning' | 'info' | 'error'> = {
  hot: 'warning',
  cold: 'info',
  mock: 'error'
}

// Soft pastel chip look (tinted background, saturated text, no border) for the
// migration-mode chip — matches the design's "Hot"/"Cold" pill styling, as
// opposed to MUI's solid-filled default Chip colors used elsewhere.
export function dataCopyMethodChipSx(method: DataCopyMethod) {
  const colorKey = DATA_COPY_METHOD_CHIP_COLOR[method]
  return (theme: Theme) => ({
    bgcolor: alpha(theme.palette[colorKey].main, theme.palette.mode === 'dark' ? 0.24 : 0.14),
    color:
      theme.palette.mode === 'dark' ? theme.palette[colorKey].light : theme.palette[colorKey].dark,
    fontWeight: 600,
    border: 'none'
  })
}

export const CUTOVER_OPTION_LABEL: Record<string, string> = {
  [CUTOVER_TYPES.IMMEDIATE]: 'Immediate cutover',
  [CUTOVER_TYPES.ADMIN_INITIATED]: 'Admin cutover',
  [CUTOVER_TYPES.TIME_WINDOW]: 'Time window cutover'
}

export function cutoverOptionLabel(cutoverOption: string | undefined): string {
  if (!cutoverOption) return CUTOVER_OPTION_LABEL[CUTOVER_TYPES.IMMEDIATE]
  return CUTOVER_OPTION_LABEL[cutoverOption] ?? 'Immediate cutover'
}

export function guestOsLabel(osFamily: string | undefined): string {
  const option = OS_TYPES_OPTIONS.find((opt) => opt.value === osFamily)
  return option?.label ?? 'Auto-detect'
}

export function storageCopyMethodLabel(storageCopyMethod: string | undefined): string | undefined {
  return STORAGE_COPY_METHOD_OPTIONS.find((opt) => opt.value === storageCopyMethod)?.label
}

// Templates saved before MigrationForm started preferring the vCenter cluster's
// display name persisted the k8s object's raw metadata.name instead — for VMs on a
// standalone ESXi host (no real vCenter cluster) that's a generated
// "no-cluster-<datacenter>-<hash>" placeholder, and even a real cluster's raw name
// has "-<datacenter>-<hash>" appended (see pkg/common/utils/k8scompat.go and
// GetClusterK8sID). The datacenter can't be reliably split back out of that string
// alone (it may itself contain hyphens), so `clusterNameLookup` — built by
// useSourceClusterNameLookup from the live VMwareCluster list — is checked first;
// only when the raw name isn't found there (e.g. its source cluster/cred no longer
// exists) do we fall back to best-effort string cleanup.
export function sourceClusterLabel(
  sourceCluster: string | undefined,
  clusterNameLookup?: Record<string, string>
): string | undefined {
  if (!sourceCluster) return sourceCluster
  const resolved = clusterNameLookup?.[sourceCluster]
  if (resolved) return resolved
  if (sourceCluster.toLowerCase().startsWith('no-cluster')) return 'No cluster'
  return sourceCluster.replace(/-[0-9a-f]{5}$/i, '')
}

export interface AdvancedOptionRow {
  label: string
  value: string
}

// One row per advanced option that's actually set on the template, each carrying its
// real value (not just the flag name) — powers the "Advanced options" section on the
// template detail drawer. Options left at their default (false/empty) are omitted so
// the list only shows what the template actually changes. Security/server groups are
// persisted as ids only, so `groupNames` (per-destination id->name lookup) resolves
// them to names for display; falls back to the raw id when a name isn't found.
export function buildAdvancedOptionRows(
  template: SavedTemplate,
  groupNames?: TemplateGroupNameLookup
): AdvancedOptionRow[] {
  const rows: AdvancedOptionRow[] = []
  const postMigrationAction = template.postMigrationAction

  if (template.serverGroup) {
    const name = groupNames?.serverGroups[template.serverGroup] ?? template.serverGroup
    rows.push({ label: 'Server group', value: name })
  }
  if (template.securityGroups.length > 0) {
    const names = template.securityGroups.map(
      (id) => groupNames?.securityGroups[id] ?? id
    )
    rows.push({ label: 'Security groups', value: names.join(', ') })
  }
  if (template.imageProfiles.length > 0) {
    rows.push({ label: 'Image profiles', value: template.imageProfiles.join(', ') })
  }
  if (template.firstBootScript) {
    rows.push({ label: 'Post-migration script', value: template.firstBootScript })
  }
  if (postMigrationAction?.renameVm) {
    rows.push({
      label: 'Rename VM',
      value: postMigrationAction.suffix ? `Add suffix "${postMigrationAction.suffix}"` : 'Yes'
    })
  }
  if (postMigrationAction?.moveToFolder) {
    rows.push({ label: 'Move to folder', value: postMigrationAction.folderName || 'Yes' })
  }
  if (template.networkPersistence) {
    rows.push({ label: 'Network persistence', value: 'Enabled' })
  }
  if (template.removeVMwareTools) {
    rows.push({ label: 'Remove VMware Tools', value: 'Enabled' })
  }
  if (template.disconnectSourceNetwork) {
    rows.push({ label: 'Disconnect source network', value: 'Enabled' })
  }
  if (template.fallbackToDHCP) {
    rows.push({ label: 'Fallback to DHCP', value: 'Enabled' })
  }
  if (template.periodicSyncEnabled) {
    rows.push({
      label: 'Periodic sync',
      value: template.periodicSyncInterval ? `Every ${template.periodicSyncInterval}` : 'Enabled'
    })
  }
  if (template.useGPU) {
    rows.push({ label: 'GPU flavor', value: 'Enabled' })
  }
  if (template.acknowledgeNetworkConflictRisk) {
    rows.push({ label: 'Network conflict risk', value: 'Acknowledged' })
  }
  if (template.spec?.migrationStrategy?.performHealthChecks) {
    rows.push({ label: 'Health checks', value: 'Enabled' })
  }
  if (template.spec?.migrationStrategy?.arrayOffload) {
    rows.push({ label: 'Array offload', value: 'Enabled' })
  }

  return rows
}
