import { Box, Chip, Divider, Typography } from '@mui/material'
import { KeyValueGrid, KeyValueItem } from 'src/components/design-system/ui'
import { Migration } from '../../api/migrations'
import { MigrationDetailResources } from 'src/hooks/api/useMigrationDetailResourcesQuery'

interface MigrationSpecCardProps {
  migration: Migration
  resources?: MigrationDetailResources | null
}

export default function MigrationSpecCard({ migration, resources }: MigrationSpecCardProps) {
  const spec = migration.spec
  const status = migration.status
  const metadata = migration.metadata

  const planName =
    (spec?.migrationPlan as string | undefined) ||
    (metadata?.labels as unknown as Record<string, string> | undefined)?.migrationplan ||
    '—'

  const migrationType =
    (spec?.migrationType as string | undefined) ||
    (resources?.migrationTemplate?.spec as any)?.migrationType ||
    '—'

  const cutoverMode = spec?.initiateCutover ? 'Admin initiated' : 'Automatic'

  const networkMappingName =
    (resources?.networkMapping?.metadata as any)?.name ||
    (resources?.migrationTemplate?.spec as any)?.networkMapping ||
    '—'

  const storageMappingName =
    (resources?.storageMapping?.metadata as any)?.name ||
    (resources?.migrationTemplate?.spec as any)?.storageMapping ||
    '—'

  const stagedVolumeIDs: string[] = Array.isArray(
    (status as any)?.stagedVolumeIDs
  )
    ? ((status as any).stagedVolumeIDs as unknown[])
        .map((v) => String(v).trim())
        .filter(Boolean)
    : []

  const items: KeyValueItem[] = [
    { label: 'Migration name', value: metadata?.name },
    { label: 'VM name',        value: spec?.vmName as string | undefined },
    { label: 'Plan',           value: planName },
    { label: 'Migration type', value: migrationType },
    { label: 'Cutover',        value: cutoverMode },
    {
      label: 'Disconnect source network',
      value: spec?.disconnectSourceNetwork ? 'Yes' : 'No',
    },
    { label: 'Total disks', value: status?.totalDisks ? String(status.totalDisks) : undefined },
    { label: 'Pod ref',     value: spec?.podRef as string | undefined },
    { label: 'Network mapping', value: networkMappingName },
    { label: 'Storage mapping', value: storageMappingName },
  ]

  return (
    <Box
      sx={{
        p: 2.5,
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        minWidth: 0,
      }}
    >
      <Typography variant="subtitle2" fontWeight={600} sx={{ mb: 2 }}>
        Migration spec
      </Typography>
      <KeyValueGrid items={items} labelWidth={180} mdGrids={1} />

      {stagedVolumeIDs.length > 0 && (
        <Box sx={{ mt: 2 }}>
          <Divider sx={{ mb: 1.5 }} />
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1, fontWeight: 600, textTransform: 'uppercase', letterSpacing: 0.6 }}>
            Staged Volume IDs
          </Typography>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75 }}>
            {stagedVolumeIDs.map((id) => (
              <Chip
                key={id}
                label={id}
                size="small"
                variant="outlined"
                color="info"
                sx={{ fontFamily: '"Fira Code", monospace', fontSize: '0.75rem' }}
              />
            ))}
          </Box>
        </Box>
      )}
    </Box>
  )
}
