import { useCallback, useEffect, useState } from 'react'
import { Alert, Box, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material'
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
import { useSSHKeyPairsQuery } from 'src/hooks/api/useSSHKeyPairsQuery'
import { getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import { createSecret, deleteSecret } from 'src/api/secrets/secrets'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { validateSshPrivateKey } from 'src/utils'

type SSHKeySource = 'managed' | 'manual'

interface FormData {
  vmwareCredsRef: string
  vmName: string
  sshKeyPairRef: string
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
  const [sshKeySource, setSshKeySource] = useState<SSHKeySource>('managed')

  const { data: vmwareCreds = [] } = useVmwareCredentialsQuery()
  const credOptions = vmwareCreds
    .filter((c) => c.status?.vmwareValidationStatus?.toLowerCase() === 'succeeded')
    .map((c) => ({ label: c.metadata.name, value: c.metadata.name }))

  const { data: sshKeyPairs = [] } = useSSHKeyPairsQuery()
  const keyPairOptions = sshKeyPairs.map((kp) => ({ label: kp.name, value: kp.name }))

  const form = useForm<FormData>({
    defaultValues: {
      vmwareCredsRef: '',
      vmName: '',
      sshKeyPairRef: '',
      sshPrivateKey: ''
    },
    mode: 'onChange',
    reValidateMode: 'onChange'
  })

  const { watch, reset, setValue, trigger, formState: { isValid } } = form
  const vmwareCredsRef = watch('vmwareCredsRef')

  const { data: runningVMOptions = [], isLoading: vmsLoading } = useQuery({
    queryKey: ['vmwaremachines-for-proxy', vmwareCredsRef],
    queryFn: async () => {
      const result = await getVMwareMachines(VJAILBREAK_DEFAULT_NAMESPACE, vmwareCredsRef)
      return result.items
        .filter((m) => m.status?.powerState === 'running')
        .map((m) => ({ label: m.spec.vms.name, value: m.spec.vms.name }))
    },
    enabled: Boolean(vmwareCredsRef) && open,
    staleTime: 30_000
  })

  useEffect(() => {
    setValue('vmName', '')
  }, [vmwareCredsRef, setValue])

  useEffect(() => {
    trigger()
  }, [sshKeySource, trigger])

  const handleClose = useCallback(() => {
    reset()
    setSubmitError(null)
    setSshKeySource('managed')
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
    let secretName: string | null = null

    try {
      const sshSpec =
        sshKeySource === 'managed'
          ? { sshKeyPairRef: { name: data.sshKeyPairRef } }
          : (() => {
              secretName = vmNameSafe + '-hot-add-ssh-key'
              return { sshKeySecretRef: { name: secretName } }
            })()

      if (sshKeySource === 'manual' && secretName) {
        await createSecret(
          secretName,
          { 'ssh-privatekey': data.sshPrivateKey.trim() },
          VJAILBREAK_DEFAULT_NAMESPACE
        )
      }

      await postProxyVM({
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'ProxyVM',
        metadata: { name: vmNameSafe, namespace: VJAILBREAK_DEFAULT_NAMESPACE },
        spec: {
          vmName: data.vmName,
          vmwareCredsRef: { name: data.vmwareCredsRef },
          ...sshSpec
        }
      })

      queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
      handleClose()
    } catch (err) {
      if (sshKeySource === 'manual' && secretName) {
        deleteSecret(secretName, VJAILBREAK_DEFAULT_NAMESPACE).catch(() => {})
      }
      if (axios.isAxiosError(err) && err.response?.status === 409) {
        setSubmitError(`A Proxy VM with this name already exists.`)
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
      : runningVMOptions.length === 0
        ? 'No powered-on VMs found for selected credentials'
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

            {/* Proxy VM identity */}
            <Section>
              <SectionHeader
                title="Proxy VM"
                subtitle="Select the VMware environment and the powered-on VM to register as a proxy."
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
                    Only powered-on VMs are listed. If the VM is missing, revalidate the credentials.
                  </Alert>
                )}
                <RHFSelect
                  name="vmName"
                  label="VM Name"
                  options={runningVMOptions}
                  searchable
                  searchPlaceholder="Search VMs..."
                  rules={{ required: 'VM is required' }}
                  placeholder={vmSelectPlaceholder}
                  disabled={!vmwareCredsRef || vmsLoading}
                />
              </Box>
            </Section>

            {/* SSH Access */}
            <Section>
              <SectionHeader
                title="SSH Access"
                subtitle="Choose how to authenticate SSH access to the Proxy VM."
              />

              <ToggleButtonGroup
                value={sshKeySource}
                exclusive
                onChange={(_, v) => v && setSshKeySource(v as SSHKeySource)}
                size="small"
                sx={{ mb: 2 }}
              >
                <ToggleButton value="managed">Managed Key Pair</ToggleButton>
                <ToggleButton value="manual">Manual Private Key</ToggleButton>
              </ToggleButtonGroup>

              {sshKeySource === 'managed' ? (
                <Box sx={{ display: 'grid', gap: 2 }}>
                  <Alert severity="info">
                    Add the public key of the selected key pair to the Proxy VM&apos;s{' '}
                    <strong>/root/.ssh/authorized_keys</strong> before registering.
                  </Alert>
                  <RHFSelect
                    name="sshKeyPairRef"
                    label="SSH Key Pair"
                    options={keyPairOptions}
                    rules={{
                      required: sshKeySource === 'managed' ? 'SSH key pair is required' : false
                    }}
                    placeholder={
                      keyPairOptions.length === 0
                        ? 'No key pairs found — create one on the SSH Key Pairs page'
                        : 'Select a key pair'
                    }
                    disabled={keyPairOptions.length === 0}
                  />
                </Box>
              ) : (
                <Box sx={{ display: 'grid', gap: 2 }}>
                  <Alert severity="info">
                    Add the public key corresponding to your private key below to the Proxy VM&apos;s{' '}
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
                      Paste only the private key content (OpenSSH, RSA, EC, PKCS#8, or DSA).
                    </Typography>
                  </Box>

                  <RHFTextField
                    name="sshPrivateKey"
                    label="SSH Private Key"
                    required={sshKeySource === 'manual'}
                    multiline
                    minRows={10}
                    disabled={isSubmitting}
                    placeholder={
                      '-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----'
                    }
                    rules={
                      sshKeySource === 'manual'
                        ? {
                            required: 'SSH private key is required',
                            validate: (val: string) => validateSshPrivateKey(val) || true
                          }
                        : {}
                    }
                    onValueChange={() => setSubmitError(null)}
                  />
                </Box>
              )}
            </Section>
          </Box>
        </SurfaceCard>
      </DesignSystemForm>
    </DrawerShell>
  )
}
