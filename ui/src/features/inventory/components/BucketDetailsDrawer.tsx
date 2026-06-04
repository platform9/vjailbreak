import { useMemo } from 'react'
import {
  Box,
  Chip,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography
} from '@mui/material'
import LanOutlinedIcon from '@mui/icons-material/LanOutlined'
import StorageOutlinedIcon from '@mui/icons-material/StorageOutlined'
import EditOutlinedIcon from '@mui/icons-material/EditOutlined'
import ContentCopyOutlinedIcon from '@mui/icons-material/ContentCopyOutlined'
import ScheduleOutlinedIcon from '@mui/icons-material/ScheduleOutlined'
import dayjs from 'dayjs'
import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  FieldLabel,
  KeyValueGrid,
  Section,
  StatusChip,
  SurfaceCard
} from 'src/components'
import type { FormValues } from 'src/features/migration/types'
import { CUTOVER_TYPES } from 'src/features/migration/constants'
import { DEFAULT_BUCKET_LABEL } from '../constants'
import { bucketStatusLabel, bucketStatusTone, getBucketStatus } from '../utils/bucketStatus'
import type { BucketStatus, MigrationBucket } from '../types'

export interface BucketDetailsDrawerProps {
  open: boolean
  bucket?: MigrationBucket
  statusOverride?: BucketStatus
  onClose: () => void
  onEdit: (bucket: MigrationBucket) => void
  onDuplicate: (bucket: MigrationBucket) => void
  onSchedule: (bucket: MigrationBucket) => void
}

function MappingTable({
  rows,
  sourceLabel,
  targetLabel,
  icon
}: {
  rows: Array<{ source: string; target: string }>
  sourceLabel: string
  targetLabel: string
  icon: React.ReactNode
}) {
  if (!rows.length) {
    return <Typography variant="body2">N/A</Typography>
  }
  return (
    <TableContainer component={Paper} variant="outlined">
      <Table size="small" sx={{ tableLayout: 'fixed' }}>
        <TableHead>
          <TableRow>
            <TableCell sx={{ width: '50%' }}>{sourceLabel}</TableCell>
            <TableCell sx={{ width: '50%' }}>{targetLabel}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((row, idx) => (
            <TableRow key={`${row.source}-${row.target}-${idx}`}>
              <TableCell>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
                  {icon}
                  <Typography variant="body2" sx={{ wordBreak: 'break-word' }}>
                    {row.source}
                  </Typography>
                </Box>
              </TableCell>
              <TableCell>
                <Typography variant="body2" sx={{ wordBreak: 'break-word' }}>
                  {row.target}
                </Typography>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  )
}

/**
 * Read-only bucket details, styled like the Migration Details drawer. The Edit / Duplicate /
 * Schedule actions live in the footer here (the table only handles Delete). All values read from
 * the typed `spec.config` fields so they are reliable regardless of the formValues blob.
 */
export default function BucketDetailsDrawer({
  open,
  bucket,
  statusOverride,
  onClose,
  onEdit,
  onDuplicate,
  onSchedule
}: BucketDetailsDrawerProps) {
  const config = bucket?.spec.config ?? {}
  const fv = useMemo(() => (config.formValues ?? {}) as Partial<FormValues>, [config.formValues])

  const networkMappings = config.networkMappings ?? []
  const storageMappings = config.storageMappings ?? []
  const isDefault = bucket?.spec.isDefault ?? false
  const title = isDefault ? DEFAULT_BUCKET_LABEL : bucket?.metadata.name ?? 'Bucket'
  const status = bucket ? statusOverride ?? getBucketStatus(bucket) : 'NotMigrated'

  const overviewItems = useMemo(
    () => [
      { label: 'Source cluster', value: config.sourceCluster || 'N/A' },
      { label: 'Destination (PCD) cluster', value: config.pcdCluster || 'N/A' },
      { label: 'VMs', value: String(bucket?.spec.vms.length ?? 0) },
      { label: 'Migration type', value: (config.dataCopyMethod || fv.dataCopyMethod || 'N/A') as string },
      { label: 'Storage copy method', value: (fv.storageCopyMethod as string) || 'normal' },
      {
        label: 'Schedule',
        value: bucket?.spec.schedule
          ? dayjs(bucket.spec.schedule).format('MMM D, YYYY HH:mm')
          : 'Not scheduled'
      }
    ],
    [config.sourceCluster, config.pcdCluster, config.dataCopyMethod, fv.dataCopyMethod, fv.storageCopyMethod, bucket?.spec.vms.length, bucket?.spec.schedule]
  )

  const advancedItems = useMemo(() => {
    const items: Array<{ label: string; value: string }> = []
    const push = (label: string, value?: string | false) => {
      if (value) items.push({ label, value })
    }
    const fmt = (t?: string) => (t ? dayjs(t).format('MMM D, YYYY HH:mm') : '')

    push('Scheduled data-copy start', fv.dataCopyStartTime && fmt(fv.dataCopyStartTime))
    if (fv.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED) {
      push('Cutover', 'Admin initiated')
    } else if (
      fv.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
      (fv.cutoverStartTime || fv.cutoverEndTime)
    ) {
      push('Cutover window', `${fmt(fv.cutoverStartTime) || 'N/A'} – ${fmt(fv.cutoverEndTime) || 'N/A'}`)
    }
    push(
      'Security groups',
      Array.isArray(fv.securityGroups) && fv.securityGroups.length
        ? fv.securityGroups.join(', ')
        : false
    )
    push('Server group', fv.serverGroup)
    push('Disconnect source network', fv.disconnectSourceNetwork && 'Enabled')
    push('Fallback to DHCP', fv.fallbackToDHCP && 'Enabled')
    push('Network persistence', fv.networkPersistence && 'Enabled')
    push('Remove VMware tools', fv.removeVMwareTools && 'Enabled')
    push('GPU flavor', fv.useGPU && 'Enabled')
    push(
      'Image profiles',
      Array.isArray(fv.imageProfiles) && fv.imageProfiles.length
        ? fv.imageProfiles.join(', ')
        : false
    )
    push(
      'Post-migration script',
      Boolean(fv.postMigrationScript && fv.postMigrationScript.trim()) && 'Configured'
    )
    return items
  }, [fv])

  const vms = bucket?.spec.vms ?? []

  return (
    <DrawerShell
      open={open}
      onClose={onClose}
      requireCloseConfirmation={false}
      width={860}
      header={
        <DrawerHeader
          title="Bucket Details"
          subtitle={title}
          onClose={onClose}
          actions={
            <StatusChip
              label={bucketStatusLabel(status)}
              tone={bucketStatusTone(status)}
              size="small"
              variant="filled"
            />
          }
        />
      }
      footer={
        <DrawerFooter>
          <ActionButton tone="secondary" onClick={onClose}>
            Close
          </ActionButton>
          {bucket ? (
            <Stack direction="row" spacing={1}>
              <ActionButton
                tone="secondary"
                startIcon={<ScheduleOutlinedIcon />}
                onClick={() => onSchedule(bucket)}
              >
                Schedule
              </ActionButton>
              <ActionButton
                tone="secondary"
                startIcon={<ContentCopyOutlinedIcon />}
                onClick={() => onDuplicate(bucket)}
              >
                Duplicate
              </ActionButton>
              <ActionButton
                tone="primary"
                startIcon={<EditOutlinedIcon />}
                onClick={() => onEdit(bucket)}
              >
                Edit
              </ActionButton>
            </Stack>
          ) : null}
        </DrawerFooter>
      }
    >
      <Box sx={{ display: 'grid', gap: 2 }} data-testid="bucket-detail">
        <Section>
          <Box sx={{ display: 'grid', gap: 2 }}>
            <SurfaceCard variant="card" title="Overview" subtitle="Source, destination and schedule">
              <KeyValueGrid items={overviewItems} />
            </SurfaceCard>

            <SurfaceCard variant="card" title={`VMs (${vms.length})`} subtitle="Members of this bucket">
              {vms.length ? (
                <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap>
                  {vms.map((name) => (
                    <Chip key={name} label={name} size="small" variant="outlined" />
                  ))}
                </Stack>
              ) : (
                <Typography variant="body2">No VMs in this bucket.</Typography>
              )}
            </SurfaceCard>

            <SurfaceCard variant="card" title="Mappings" subtitle="Network and storage mappings">
              <Box sx={{ display: 'grid', gap: 2.5 }}>
                <Box sx={{ display: 'grid', gap: 1 }}>
                  <FieldLabel label="Network Mapping" />
                  <MappingTable
                    rows={networkMappings}
                    sourceLabel="Source Network"
                    targetLabel="Target Network"
                    icon={<LanOutlinedIcon fontSize="small" color="action" />}
                  />
                </Box>
                <Box sx={{ display: 'grid', gap: 1 }}>
                  <FieldLabel label="Storage Mapping" />
                  <MappingTable
                    rows={storageMappings}
                    sourceLabel="Source Datastore"
                    targetLabel="Target Volume Type"
                    icon={<StorageOutlinedIcon fontSize="small" color="action" />}
                  />
                </Box>
              </Box>
            </SurfaceCard>

            <SurfaceCard
              variant="card"
              title="Advanced Options"
              subtitle="Configured migration options"
            >
              {advancedItems.length ? (
                <KeyValueGrid items={advancedItems} />
              ) : (
                <Typography variant="body2">No advanced options configured.</Typography>
              )}
            </SurfaceCard>
          </Box>
        </Section>
      </Box>
    </DrawerShell>
  )
}
