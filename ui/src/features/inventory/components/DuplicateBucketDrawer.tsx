import { useEffect, useMemo } from 'react'
import { FormProvider, useForm } from 'react-hook-form'
import { Box } from '@mui/material'
import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  InlineHelp
} from 'src/components'
import { RHFAutocomplete, RHFTextField } from 'src/shared/components/forms'
import type { BucketIdByVm, InventoryVm, MigrationBucket } from '../types'
import { isVmBlocked, validateBucketVms } from '../utils/bucketMembership'

export interface DuplicateBucketValues {
  name: string
  vmNames: string[]
}

export interface DuplicateBucketDrawerProps {
  open: boolean
  onClose: () => void
  sourceBucket?: MigrationBucket
  vmOptions: InventoryVm[]
  bucketIdByVm: BucketIdByVm
  defaultName?: string
  submitting?: boolean
  onSubmit: (values: DuplicateBucketValues) => void
}

interface FormShape {
  name: string
  vms: string[]
}

/**
 * Duplicate drawer (FR-011): pick which inventory VMs to keep in the derived bucket.
 * VMs already in another bucket are disabled (greyed) and labelled — block, not move.
 * Presentational: the container owns the create mutation.
 */
export default function DuplicateBucketDrawer({
  open,
  onClose,
  sourceBucket,
  vmOptions,
  bucketIdByVm,
  defaultName = '',
  submitting = false,
  onSubmit
}: DuplicateBucketDrawerProps) {
  const form = useForm<FormShape>({ defaultValues: { name: defaultName, vms: [] } })

  useEffect(() => {
    if (open) form.reset({ name: defaultName, vms: [] })
  }, [open, defaultName, form])

  const name = form.watch('name')
  const vms = form.watch('vms')

  const validationError = useMemo(
    () => validateBucketVms(vms ?? [], bucketIdByVm),
    [vms, bucketIdByVm]
  )
  const canSubmit = Boolean(name?.trim()) && !validationError && !submitting

  const handleSave = form.handleSubmit((values) => {
    onSubmit({ name: values.name.trim(), vmNames: values.vms })
  })

  const header = (
    <DrawerHeader
      title="Duplicate bucket"
      subtitle={sourceBucket ? `Derived from "${sourceBucket.metadata.name}"` : undefined}
      onClose={onClose}
    />
  )

  const footer = (
    <DrawerFooter>
      <ActionButton tone="secondary" onClick={onClose} disabled={submitting}>
        Cancel
      </ActionButton>
      <ActionButton tone="primary" onClick={handleSave} loading={submitting} disabled={!canSubmit}>
        Create bucket
      </ActionButton>
    </DrawerFooter>
  )

  return (
    <DrawerShell
      open={open}
      onClose={onClose}
      width={680}
      header={header}
      footer={footer}
      requireCloseConfirmation={false}
      data-testid="duplicate-bucket-drawer"
    >
      <FormProvider {...form}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          <RHFTextField
            name="name"
            label="Bucket name"
            placeholder="e.g. db-servers"
            required
            rules={{ required: 'Bucket name is required' }}
          />

          <RHFAutocomplete<InventoryVm>
            name="vms"
            label="VMs to keep in the duplicate bucket"
            labelHelperText="VMs already assigned to a bucket are disabled."
            placeholder="Select VMs…"
            multiple
            showCheckboxes
            options={vmOptions}
            getOptionValue={(vm) => vm.name}
            getOptionLabel={(vm) => vm.name}
            getOptionDisabled={(vm) => isVmBlocked(vm.name, bucketIdByVm)}
          />

          {validationError ? (
            <InlineHelp tone="warning" icon="auto">
              {validationError}
            </InlineHelp>
          ) : null}
        </Box>
      </FormProvider>
    </DrawerShell>
  )
}
