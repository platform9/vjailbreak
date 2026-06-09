import { useCallback, useEffect, useState } from 'react'
import { Alert, Box, Typography } from '@mui/material'
import { useForm } from 'react-hook-form'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import axios from 'axios'

import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  Section,
  SectionHeader,
  SurfaceCard
} from 'src/components'
import { DesignSystemForm, RHFSelect, RHFTextField } from 'src/shared/components/forms'

import { postProxyVM } from 'src/api/proxyvms/proxyVMs'
import { PROXY_VMS_QUERY_KEY } from 'src/hooks/api/useProxyVMsQuery'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import { createSecret, deleteSecret } from 'src/api/secrets/secrets'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { validateSshPrivateKey } from 'src/utils'

interface FormData {
  vmwareCredsRef: string
  vmName: string
  sshPrivateKey: string
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
    defaultValues: { vmwareCredsRef: '', vmName: '', sshPrivateKey: '' },
    mode: 'onChange',
    reValidateMode: 'onChange'
  })

  const {
    watch,
    reset,
    setValue,
    formState: { isValid }
  } = form

  const vmwareCredsRef = watch('vmwareCredsRef')

  const { data: vmOptions = [], isLoading: vmsLoading } = useQuery({
    queryKey: ['vmwaremachines-for-proxy', vmwareCredsRef],
    queryFn: async () => {
      const result = await getVMwareMachines(VJAILBREAK_DEFAULT_NAMESPACE, vmwareCredsRef)
      return result.items
        .filter((m) => m.status?.powerState === 'running')
        .map((m) => ({ label: m.spec.vms.name, value: m.spec.vms.name }))
    },
    enabled: Boolean(vmwareCredsRef),
    staleTime: 30_000
  })

  useEffect(() => {
    setValue('vmName', '')
  }, [vmwareCredsRef, setValue])

  const handleClose = useCallback(() => {
    reset()
    setSubmitError(null)
    onClose()
  }, [reset, onClose])

  const handleKeyFileUpload = useCallback(
    async (file: File | null) => {
      if (!file) return
      if (file.size > 1024 * 1024) {
        setSubmitError('File too large. SSH private key must be under 1 MB.')
        return
      }
      try {
        const text = await file.text()
        setValue('sshPrivateKey', text, { shouldDirty: true, shouldValidate: true })
        setSubmitError(null)
      } catch {
        setSubmitError('Failed to read key file.')
      }
    },
    [setValue]
  )

  const onSubmit = async (data: FormData) => {
    setIsSubmitting(true)
    setSubmitError(null)
    const vmNameSafe = toK8sName(data.vmName)
    const proxyVmName = vmNameSafe + '-hot-add-ssh-key'
    let secretCreated = false
    try {
      await createSecret(
        proxyVmName,
        { 'ssh-privatekey': data.sshPrivateKey.trim() },
        VJAILBREAK_DEFAULT_NAMESPACE
      )
      secretCreated = true

      await postProxyVM({
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'ProxyVM',
        metadata: {
          name: vmNameSafe,
          namespace: VJAILBREAK_DEFAULT_NAMESPACE
        },
        spec: {
          vmName: data.vmName,
          vmwareCredsRef: { name: data.vmwareCredsRef },
          sshKeySecretRef: { name: proxyVmName }
        }
      })

      queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
      handleClose()
    } catch (err) {
      if (secretCreated) {
        deleteSecret(proxyVmName, VJAILBREAK_DEFAULT_NAMESPACE).catch(() => {})
      }
      if (axios.isAxiosError(err) && err.response?.status === 409) {
        setSubmitError(`A Proxy VM with the name "${proxyVmName}" already exists.`)
      } else if (axios.isAxiosError(err)) {
        setSubmitError(err.response?.data?.message || 'Failed to create Proxy VM.')
      } else {
        setSubmitError('Failed to create Proxy VM.')
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const isSubmitDisabled = isSubmitting || !isValid

  const vmSelectPlaceholder = !vmwareCredsRef
    ? 'Select VMware credentials first'
    : vmsLoading
      ? 'Loading VMs...'
      : vmOptions.length === 0
        ? 'No VMs found for selected credentials'
        : 'Search and select a VM'

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      header={
        <DrawerHeader
          title="Add Proxy VM"
          subtitle="Register a Proxy VM for Hot-Add data copy migrations"
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
            disabled={isSubmitDisabled}
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
        keyboardSubmitProps={{ open, onClose: handleClose, isSubmitDisabled }}
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
                title="Proxy VM"
                subtitle="Select the VMware environment and the vCenter VM to register as a proxy."
              />
              <Box sx={{ display: 'grid', gap: 2 }}>
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

                {vmwareCredsRef && !vmsLoading && (
                  <Alert severity="info">
                    Only powered on VMs can be added as a Proxy VM. If the VM is powered on but not
                    listed, please revalidate the credentials.
                  </Alert>
                )}
                <RHFSelect
                  name="vmName"
                  label="VM Name"
                  options={vmOptions}
                  searchable
                  searchPlaceholder="Search VMs..."
                  rules={{ required: 'VM is required' }}
                  placeholder={vmSelectPlaceholder}
                  disabled={!vmwareCredsRef || vmsLoading}
                />
              </Box>
            </Section>

            <Section>
              <SectionHeader
                title="SSH Access"
                subtitle="Provide the private key used to SSH into the proxy VM for disk access."
              />

              <Alert severity="info">
                Add the public key corresponding to your private key below to the Proxy VM's{' '}
                <strong>/root/.ssh/authorized_keys</strong> before registering.
              </Alert>

              <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
                <ActionButton
                  tone="secondary"
                  component="label"
                  disabled={isSubmitting}
                  sx={{ whiteSpace: 'nowrap' }}
                >
                  Upload key file
                  <input
                    type="file"
                    hidden
                    onChange={(e) => handleKeyFileUpload(e.target.files?.[0] ?? null)}
                  />
                </ActionButton>
                <Typography variant="body2" color="text.secondary">
                  Paste only the private key content (OpenSSH, RSA, EC, PKCS#8, or DSA — no field
                  name prefix).
                </Typography>
              </Box>

              <RHFTextField
                name="sshPrivateKey"
                label="SSH Private Key"
                required
                multiline
                minRows={10}
                disabled={isSubmitting}
                placeholder={
                  '-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----\n\nAlso accepted: RSA, EC, PKCS#8 (PRIVATE KEY), DSA'
                }
                rules={{
                  required: 'SSH private key is required',
                  validate: (val: string) => validateSshPrivateKey(val) || true
                }}
                onValueChange={() => setSubmitError(null)}
              />
            </Section>
          </Box>
        </SurfaceCard>
      </DesignSystemForm>
    </DrawerShell>
  )
}
