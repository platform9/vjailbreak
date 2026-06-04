import { useState, MouseEvent } from 'react'
import { Avatar, Box, Chip, IconButton, Menu, MenuItem, Stack, Typography } from '@mui/material'
import MoreVertIcon from '@mui/icons-material/MoreVert'
import FolderOpenOutlinedIcon from '@mui/icons-material/FolderOpenOutlined'
import StarOutlinedIcon from '@mui/icons-material/StarOutlined'
import ScheduleOutlinedIcon from '@mui/icons-material/ScheduleOutlined'
import dayjs from 'dayjs'
import { SurfaceCard, StatusChip } from 'src/components'
import { DEFAULT_BUCKET_LABEL } from '../constants'
import type { BucketStatus, MigrationBucket } from '../types'
import { bucketStatusLabel, bucketStatusTone, getBucketStatus } from '../utils/bucketStatus'

export interface BucketCardProps {
  bucket: MigrationBucket
  /** Live status derived from real migrations; falls back to the bucket's own status. */
  statusOverride?: BucketStatus
  onEdit?: (bucket: MigrationBucket) => void
  onDuplicate?: (bucket: MigrationBucket) => void
  onDelete?: (bucket: MigrationBucket) => void
}

const VM_CHIP_LIMIT = 6

const avatarColor = (status: BucketStatus): string => {
  const tone = bucketStatusTone(status)
  if (tone === 'success') return 'success.main'
  if (tone === 'warning') return 'warning.main'
  if (tone === 'info') return 'info.main'
  return 'primary.main'
}

/**
 * Bucket card: status-colored avatar, name + default badge, VM count + optional schedule,
 * a preview of member VMs as chips, the status chip, and an actions menu. Default bucket
 * offers Edit + Duplicate only (no Delete). Presentational.
 */
export default function BucketCard({
  bucket,
  statusOverride,
  onEdit,
  onDuplicate,
  onDelete
}: BucketCardProps) {
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null)
  const open = Boolean(anchorEl)

  const isDefault = bucket.spec.isDefault
  const title = isDefault ? DEFAULT_BUCKET_LABEL : bucket.metadata.name
  const vms = bucket.spec.vms
  const status = statusOverride ?? getBucketStatus(bucket)
  const schedule = bucket.spec.schedule

  const closeMenu = () => setAnchorEl(null)
  const handleMenu = (e: MouseEvent<HTMLElement>) => setAnchorEl(e.currentTarget)
  const run = (fn?: (b: MigrationBucket) => void) => () => {
    closeMenu()
    fn?.(bucket)
  }

  const shownVms = vms.slice(0, VM_CHIP_LIMIT)
  const remaining = vms.length - shownVms.length

  return (
    <SurfaceCard
      variant="card"
      data-testid="bucket-card"
      sx={{
        transition: 'border-color 120ms ease, box-shadow 120ms ease',
        '&:hover': { borderColor: 'primary.main', boxShadow: 3 }
      }}
    >
      <Stack direction="row" spacing={2} alignItems="flex-start">
        <Avatar variant="rounded" sx={{ bgcolor: avatarColor(status), width: 44, height: 44 }}>
          {isDefault ? <StarOutlinedIcon /> : <FolderOpenOutlinedIcon />}
        </Avatar>

        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 0.25 }}>
            <Typography variant="subtitle1" sx={{ fontWeight: 700 }} noWrap>
              {title}
            </Typography>
            {isDefault ? <Chip label="Default" size="small" color="primary" variant="outlined" /> : null}
          </Stack>

          <Stack direction="row" spacing={1.5} alignItems="center" sx={{ color: 'text.secondary' }}>
            <Typography variant="caption">
              {vms.length} VM{vms.length === 1 ? '' : 's'}
            </Typography>
            {schedule ? (
              <Stack direction="row" spacing={0.5} alignItems="center">
                <ScheduleOutlinedIcon sx={{ fontSize: 14 }} />
                <Typography variant="caption">{dayjs(schedule).format('MMM D, HH:mm')}</Typography>
              </Stack>
            ) : null}
          </Stack>

          {vms.length > 0 ? (
            <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap sx={{ mt: 1 }}>
              {shownVms.map((name) => (
                <Chip key={name} label={name} size="small" variant="outlined" />
              ))}
              {remaining > 0 ? (
                <Chip label={`+${remaining} more`} size="small" />
              ) : null}
            </Stack>
          ) : (
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
              No VMs in this bucket.
            </Typography>
          )}
        </Box>

        <Stack direction="row" spacing={1} alignItems="center">
          <StatusChip
            label={bucketStatusLabel(status)}
            tone={bucketStatusTone(status)}
            size="small"
            variant="filled"
          />
          <IconButton
            size="small"
            aria-label={`${title} actions`}
            data-testid="bucket-actions"
            onClick={handleMenu}
          >
            <MoreVertIcon fontSize="small" />
          </IconButton>
          <Menu anchorEl={anchorEl} open={open} onClose={closeMenu}>
            <MenuItem onClick={run(onEdit)}>Edit</MenuItem>
            <MenuItem onClick={run(onDuplicate)}>Duplicate</MenuItem>
            {!isDefault ? (
              <MenuItem onClick={run(onDelete)} sx={{ color: 'error.main' }}>
                Delete
              </MenuItem>
            ) : null}
          </Menu>
        </Stack>
      </Stack>
    </SurfaceCard>
  )
}
