import { useEffect, useMemo, useState } from 'react'
import { Box, Checkbox, Typography } from '@mui/material'
import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  InlineHelp,
  StatusChip
} from 'src/components'
import { DEFAULT_BUCKET_LABEL } from '../constants'
import type { MigrationBucket } from '../types'
import { bucketStatusLabel, bucketStatusTone, getBucketStatus } from '../utils/bucketStatus'

export interface TriggerDrawerProps {
  open: boolean
  onClose: () => void
  buckets: MigrationBucket[]
  /** Called with the selected bucket names to proceed to the plan dialog. */
  onContinue: (selectedNames: string[]) => void
}

/** Buckets already running/migrated cannot be (re)triggered. */
const isSelectable = (bucket: MigrationBucket): boolean => {
  const status = getBucketStatus(bucket)
  return status === 'NotMigrated' || status === 'Scheduled'
}

/**
 * Multi-select buckets to trigger (FR-018). Presentational; the container computes the
 * recommendation and launches.
 */
export default function TriggerDrawer({ open, onClose, buckets, onContinue }: TriggerDrawerProps) {
  const [selected, setSelected] = useState<Record<string, boolean>>({})

  useEffect(() => {
    if (open) setSelected({})
  }, [open])

  const toggle = (name: string) => setSelected((prev) => ({ ...prev, [name]: !prev[name] }))

  const selectedNames = useMemo(
    () => buckets.filter((b) => selected[b.metadata.name]).map((b) => b.metadata.name),
    [buckets, selected]
  )

  const header = <DrawerHeader title="Trigger migrations" subtitle="Select buckets to migrate" onClose={onClose} />

  const footer = (
    <DrawerFooter>
      <ActionButton tone="secondary" onClick={onClose}>
        Cancel
      </ActionButton>
      <ActionButton
        tone="primary"
        onClick={() => onContinue(selectedNames)}
        disabled={selectedNames.length === 0}
      >
        Continue ({selectedNames.length})
      </ActionButton>
    </DrawerFooter>
  )

  return (
    <DrawerShell
      open={open}
      onClose={onClose}
      width={560}
      header={header}
      footer={footer}
      requireCloseConfirmation={false}
      data-testid="trigger-drawer"
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {buckets.length === 0 ? (
          <InlineHelp tone="default" icon="info">
            No buckets to trigger yet.
          </InlineHelp>
        ) : (
          buckets.map((bucket) => {
            const name = bucket.metadata.name
            const selectable = isSelectable(bucket)
            const status = getBucketStatus(bucket)
            const label = bucket.spec.isDefault ? DEFAULT_BUCKET_LABEL : name
            return (
              <Box
                key={name}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1,
                  px: 1,
                  py: 0.5,
                  borderRadius: 1,
                  opacity: selectable ? 1 : 0.5
                }}
              >
                <Checkbox
                  size="small"
                  checked={Boolean(selected[name])}
                  disabled={!selectable}
                  onChange={() => toggle(name)}
                  inputProps={{ 'aria-label': `Select ${label}` }}
                />
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography variant="body2" noWrap>
                    {label}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {bucket.spec.vms.length} VM{bucket.spec.vms.length === 1 ? '' : 's'}
                  </Typography>
                </Box>
                <StatusChip
                  label={bucketStatusLabel(status)}
                  tone={bucketStatusTone(status)}
                  size="small"
                  variant="filled"
                />
              </Box>
            )
          })
        )}
      </Box>
    </DrawerShell>
  )
}
