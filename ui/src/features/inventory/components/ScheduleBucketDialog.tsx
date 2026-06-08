import { useEffect } from 'react'
import {
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle
} from '@mui/material'
import { FormProvider, useForm } from 'react-hook-form'
import { ActionButton } from 'src/components'
import { DEFAULT_BUCKET_LABEL } from '../constants'
import BucketScheduleField from './BucketScheduleField'
import type { MigrationBucket } from '../types'

export interface ScheduleBucketDialogProps {
  open: boolean
  bucket?: MigrationBucket
  submitting?: boolean
  onClose: () => void
  /** Persist the schedule (RFC3339 string, or undefined to clear). */
  onSave: (bucket: MigrationBucket, schedule?: string) => void
}

interface ScheduleForm {
  schedule: string
}

/**
 * Set an optional per-bucket schedule. UI-only for now: the time is stored on `spec.schedule`
 * and surfaced in the table/details, but is not yet auto-enforced by a backend controller — a
 * manual Trigger always takes priority.
 */
export default function ScheduleBucketDialog({
  open,
  bucket,
  submitting = false,
  onClose,
  onSave
}: ScheduleBucketDialogProps) {
  const form = useForm<ScheduleForm>({ defaultValues: { schedule: bucket?.spec.schedule ?? '' } })

  useEffect(() => {
    if (open) form.reset({ schedule: bucket?.spec.schedule ?? '' })
  }, [open, bucket, form])

  const title = bucket?.spec.isDefault ? DEFAULT_BUCKET_LABEL : bucket?.metadata.name ?? 'bucket'

  const handleSave = () => {
    if (!bucket) return
    const value = (form.getValues('schedule') || '').trim()
    onSave(bucket, value || undefined)
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Schedule “{title}”</DialogTitle>
      <DialogContent>
        <DialogContentText variant="body2" sx={{ mb: 2 }}>
          The migrations for the VMs in this bucket will be scheduled at your chosen time.
        </DialogContentText>
        <FormProvider {...form}>
          <BucketScheduleField name="schedule" label="Schedule" disabled={submitting} />
        </FormProvider>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <ActionButton tone="secondary" onClick={onClose} disabled={submitting}>
          Cancel
        </ActionButton>
        <ActionButton tone="primary" onClick={handleSave} loading={submitting}>
          Save
        </ActionButton>
      </DialogActions>
    </Dialog>
  )
}
