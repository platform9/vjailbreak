import { Alert, Box, Typography } from '@mui/material'
import { Section, SectionHeader } from 'src/components'
import { DesignSystemForm, RHFSelect, RHFTextField } from 'src/shared/components/forms'
import type { UseFormReturn } from 'react-hook-form'
import type { Option } from 'src/shared/components/forms/rhf/RHFSelect'
import type { CreateFormData, VMOption } from './types'

export const CREATE_FORM_ID = 'create-proxy-vm-form'

interface VCenterResources {
  datacenters?: string[]
  datastores?: string[]
  networks?: string[]
  clusters?: string[]
}

interface DeployVMFormProps {
  form: UseFormReturn<CreateFormData>
  onSubmit: (data: CreateFormData) => void
  open: boolean
  onClose: () => void
  isSubmitDisabled: boolean
  credOptions: Option[]
  vmwareCredsRefCreate: string
  datacenterCreate: string
  dcResources?: VCenterResources
  scopedResources?: VCenterResources
  dcLoading: boolean
  scopedLoading: boolean
  isSubmitting: boolean
  vmOptions?: VMOption[]
}

function toOptions(items: string[] | undefined) {
  return (items ?? []).map((v) => ({ label: v, value: v }))
}

export default function DeployVMForm({
  form,
  onSubmit,
  open,
  onClose,
  isSubmitDisabled,
  credOptions,
  vmwareCredsRefCreate,
  datacenterCreate,
  dcResources,
  scopedResources,
  dcLoading,
  scopedLoading,
  isSubmitting,
  vmOptions = []
}: DeployVMFormProps) {
  return (
    <DesignSystemForm
      id={CREATE_FORM_ID}
      form={form}
      onSubmit={onSubmit}
      keyboardSubmitProps={{ open, onClose, isSubmitDisabled }}
    >
      <Box sx={{ display: 'grid', gap: 2 }}>
        <Alert severity="info">
          The OVA is deployed, powered on, and registered automatically. SSH keys are injected at
          boot — nothing else to configure.
        </Alert>

        <Section>
          <RHFSelect
            name="vmwareCredsRef"
            label="VMware Credentials"
            required
            options={credOptions}
            rules={{ required: 'VMware credentials are required' }}
            placeholder={
              credOptions.length === 0
                ? 'No validated VMware credentials found'
                : 'Select credentials'
            }
          />
        </Section>

        <Section>
          <Box sx={{ display: 'grid', gap: 1 }}>
            <RHFTextField
              name="vmName"
              label="VM Name"
              required
              rules={{
                required: 'VM name is required',
                validate: (val: string) => {
                  if (!val.trim()) return 'VM name cannot be blank'
                  if (!/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(val))
                    return 'Lowercase letters, numbers, and hyphens only. Cannot start or end with a hyphen.'
                  if (val.length > 63) return 'Must be 63 characters or fewer'
                  if (vmOptions.some((v) => v.name === val))
                    return 'A VM with this name already exists in the selected vCenter.'
                  return true
                }
              }}
              placeholder="vjb-proxy-01"
              disabled={isSubmitting}
            />
            <Typography variant="caption" color="text.secondary">
              Becomes both the vSphere VM name and the vJailbreak Proxy VM record. Must be unique.
            </Typography>
          </Box>
        </Section>

        <Section>
          <SectionHeader
            title="Deployment Target"
            subtitle="Where in the VMware environment to place the new VM."
            sx={{ mb: 1 }}
          />
          <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
            <RHFSelect
              name="datacenter"
              label="Datacenter"
              required
              options={toOptions(dcResources?.datacenters)}
              rules={{ required: 'Datacenter is required' }}
              placeholder={
                !vmwareCredsRefCreate
                  ? 'Select credentials first'
                  : dcLoading
                    ? 'Loading…'
                    : 'Select datacenter'
              }
              disabled={!vmwareCredsRefCreate || dcLoading || isSubmitting}
            />
            <RHFSelect
              name="datastore"
              label="Datastore"
              required
              options={toOptions(scopedResources?.datastores)}
              rules={{ required: 'Datastore is required' }}
              placeholder={
                !datacenterCreate
                  ? 'Select datacenter first'
                  : scopedLoading
                    ? 'Loading…'
                    : 'Select datastore'
              }
              disabled={!datacenterCreate || scopedLoading || isSubmitting}
            />
            <RHFSelect
              name="network"
              label="Network"
              required
              options={toOptions(scopedResources?.networks)}
              rules={{ required: 'Network is required' }}
              placeholder={
                !datacenterCreate
                  ? 'Select datacenter first'
                  : scopedLoading
                    ? 'Loading…'
                    : 'Select network'
              }
              disabled={!datacenterCreate || scopedLoading || isSubmitting}
            />
            <RHFSelect
              name="cluster"
              label="Cluster / Host (optional)"
              options={toOptions(scopedResources?.clusters)}
              placeholder={
                !datacenterCreate ? 'Auto-select' : scopedLoading ? 'Loading…' : 'Auto-select'
              }
              disabled={!datacenterCreate || scopedLoading || isSubmitting}
            />
          </Box>
        </Section>

      </Box>
    </DesignSystemForm>
  )
}
