import {
  Box,
  Chip,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography
} from '@mui/material'
import { FieldLabel, SurfaceCard } from 'src/components'
import type { MigrationPlan } from 'src/api/migration-plans/model'
import type { VMwareMachine } from 'src/api/vmware-machines/model'

interface MigrationTagsMetadataCardProps {
  migrationPlan?: MigrationPlan | null
  vmwareMachine?: VMwareMachine | null
}

/**
 * Shows what tags/metadata the migrated VM carries. Hidden entirely when the
 * plan neither preserves source tags nor defines custom metadata — same
 * pattern as unconfigured advanced options.
 */
export default function MigrationTagsMetadataCard({
  migrationPlan,
  vmwareMachine
}: MigrationTagsMetadataCardProps) {
  const preserveSourceTags = Boolean(migrationPlan?.spec?.preserveSourceTags)
  const customMetadata = migrationPlan?.spec?.customMetadata || {}
  const hasCustomMetadata = Object.keys(customMetadata).length > 0

  if (!preserveSourceTags && !hasCustomMetadata) return null

  const sourceTags = vmwareMachine?.spec?.vms?.tags || {}
  const sourceAttributes = vmwareMachine?.spec?.vms?.customAttributes || {}
  const hasSourceEntries =
    Object.keys(sourceTags).length > 0 || Object.keys(sourceAttributes).length > 0

  return (
    <SurfaceCard
      variant="card"
      title="Tags & Metadata"
      subtitle="Organizational context carried to the migrated VM"
      data-testid="migration-tags-metadata-card"
    >
      <Box sx={{ display: 'grid', gap: 2.5 }}>
        {preserveSourceTags && (
          <Box sx={{ display: 'grid', gap: 1 }}>
            <FieldLabel label="Preserved source tags & attributes" />
            {hasSourceEntries ? (
              <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.75 }}>
                {Object.entries(sourceTags).map(([category, tagNames]) => (
                  <Chip
                    key={`tag-${category}`}
                    size="small"
                    color="primary"
                    variant="outlined"
                    label={`${category}: ${tagNames}`}
                    sx={{ fontFamily: 'monospace', fontSize: 11 }}
                  />
                ))}
                {Object.entries(sourceAttributes).map(([name, value]) => (
                  <Chip
                    key={`attr-${name}`}
                    size="small"
                    variant="outlined"
                    label={`${name}: ${value}`}
                    sx={{ fontFamily: 'monospace', fontSize: 11 }}
                  />
                ))}
              </Box>
            ) : (
              <Typography variant="body2" color="text.secondary">
                Preserving is enabled, but no tags or custom attributes were found on the source
                VM.
              </Typography>
            )}
          </Box>
        )}

        {hasCustomMetadata && (
          <Box sx={{ display: 'grid', gap: 1 }}>
            <FieldLabel label="Custom metadata" />
            <TableContainer component={Paper} variant="outlined">
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>Key</TableCell>
                    <TableCell>Value</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {Object.entries(customMetadata).map(([key, value]) => (
                    <TableRow key={key}>
                      <TableCell sx={{ fontFamily: 'monospace' }}>{key}</TableCell>
                      <TableCell sx={{ fontFamily: 'monospace', wordBreak: 'break-word' }}>
                        {value}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        )}
      </Box>
    </SurfaceCard>
  )
}
