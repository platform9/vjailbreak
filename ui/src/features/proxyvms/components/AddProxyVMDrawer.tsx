import { useState } from 'react'
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Typography
} from '@mui/material'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { useForm } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import axios from 'axios'

import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  FormGrid,
  Section,
  SectionHeader,
  SurfaceCard
} from 'src/components'
import { DesignSystemForm, RHFSelect, RHFTextField, TextField } from 'src/shared/components/forms'

import { postProxyVM } from 'src/api/proxyvms/proxyVMs'
import { PROXY_VMS_QUERY_KEY } from 'src/hooks/api/useProxyVMsQuery'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

interface FormData {
  vmName: string
  vmwareCredsRef: string
}

interface AddProxyVMDrawerProps {
  open: boolean
  onClose: () => void
}

function toK8sName(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 63)
}

const PREREQUISITES = [
  'lsblk — block device lister must be installed',
  'nbdkit — network block device kit must be installed',
  'qemu-nbd — QEMU NBD server must be installed',
  'sshd — SSH daemon must be running and accessible',
  'disk.EnableUUID — VMware disk UUID must be enabled on the VM',
  'SSH key authorization — vJailbreak public key must be added to authorized_keys'
]

const FORM_ID = 'add-proxy-vm-form'

export default function AddProxyVMDrawer({ open, onClose }: AddProxyVMDrawerProps) {
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const { data: vmwareCreds = [] } = useVmwareCredentialsQuery()
  const credOptions = vmwareCreds
    .filter((c) => c.status?.vmwareValidationStatus?.toLowerCase() === 'succeeded')
    .map((c) => ({ label: c.metadata.name, value: c.metadata.name }))

  const form = useForm<FormData>({
    defaultValues: { vmName: '', vmwareCredsRef: '' }
  })

  const { watch, reset } = form
  const vmName = watch('vmName')
  const derivedName = vmName ? toK8sName(vmName) : ''

  const handleClose = () => {
    reset()
    setSubmitError(null)
    onClose()
  }

  const onSubmit = async (data: FormData) => {
    setIsSubmitting(true)
    setSubmitError(null)
    try {
      await postProxyVM({
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'ProxyVM',
        metadata: {
          name: derivedName,
          namespace: VJAILBREAK_DEFAULT_NAMESPACE,
          creationTimestamp: '',
          uid: ''
        },
        spec: {
          vmName: data.vmName,
          vmwareCredsRef: { name: data.vmwareCredsRef }
        }
      })
      queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
      handleClose()
    } catch (err) {
      if (axios.isAxiosError(err) && err.response?.status === 409) {
        setSubmitError(`A Proxy VM with the name "${derivedName}" already exists.`)
      } else if (axios.isAxiosError(err)) {
        setSubmitError(err.response?.data?.message || 'Failed to create Proxy VM.')
      } else {
        setSubmitError('Failed to create Proxy VM.')
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      header={
        <DrawerHeader
          title="Add Proxy VM"
          subtitle="Register a vCenter VM to act as a Hot-Add proxy during migration"
          onClose={handleClose}
        />
      }
      footer={
        <DrawerFooter>
          <ActionButton tone="secondary" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            type="submit"
            form={FORM_ID}
            loading={isSubmitting}
            disabled={isSubmitting}
          >
            Add Proxy VM
          </ActionButton>
        </DrawerFooter>
      }
    >
      <DesignSystemForm
        id={FORM_ID}
        form={form}
        onSubmit={onSubmit}
        keyboardSubmitProps={{ open, onClose: handleClose, isSubmitDisabled: isSubmitting }}
      >
        <SurfaceCard>
          <Box sx={{ display: 'grid', gap: 2 }}>
            {submitError && (
              <Alert severity="error" onClose={() => setSubmitError(null)}>
                {submitError}
              </Alert>
            )}

            <Section>
              <SectionHeader
                title="VM Details"
                subtitle="Specify the vCenter VM name and the VMware credentials used to access it."
              />
              <FormGrid gap={2} minWidth={300}>
                <RHFTextField
                  name="vmName"
                  label="VM Name"
                  placeholder="proxy-vm-01"
                  required
                  fullWidth
                  size="small"
                  rules={{ required: 'VM name is required' }}
                />

                {derivedName && (
                  <TextField
                    label="Kubernetes Resource Name"
                    value={derivedName}
                    InputProps={{ readOnly: true }}
                    helperText="Auto-derived from VM name"
                    fullWidth
                  />
                )}

                <RHFSelect
                  name="vmwareCredsRef"
                  label="VMware Credentials"
                  options={credOptions}
                  rules={{ required: 'VMware credentials are required' }}
                  placeholder={
                    credOptions.length === 0
                      ? 'No validated VMware credentials found'
                      : 'Select credentials'
                  }
                />
              </FormGrid>
            </Section>

            <Accordion
              disableGutters
              elevation={0}
              sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}
            >
              <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                <Typography variant="body2" fontWeight={500}>
                  Setup Prerequisites
                </Typography>
              </AccordionSummary>
              <AccordionDetails>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                  Ensure the following are configured on the proxy VM before adding:
                </Typography>
                <Box component="ul" sx={{ m: 0, pl: 2 }}>
                  {PREREQUISITES.map((item) => (
                    <Box component="li" key={item} sx={{ mb: 0.5 }}>
                      <Typography variant="body2">{item}</Typography>
                    </Box>
                  ))}
                </Box>
              </AccordionDetails>
            </Accordion>
          </Box>
        </SurfaceCard>
      </DesignSystemForm>
    </DrawerShell>
  )
}
