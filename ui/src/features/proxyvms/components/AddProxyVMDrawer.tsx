import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Alert,
  Box,
  IconButton,
  InputAdornment,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography
} from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import CheckIcon from '@mui/icons-material/Check'
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
import { generateSSHKeyPair, deleteSSHKeyPair } from 'src/api/sshKeyPairs/sshKeyPairs'
import { getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import { createSecret } from 'src/api/secrets/secrets'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { validateSshPrivateKey } from 'src/utils'

type SSHKeySource = 'generated' | 'manual'

interface FormData {
  vmwareCredsRef: string
  vmName: string
  sshPrivateKey: string
}

interface GeneratedKey {
  secretName: string
  publicKey: string
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
  const [sshKeySource, setSshKeySource] = useState<SSHKeySource>('generated')
  const [generatedKey, setGeneratedKey] = useState<GeneratedKey | null>(null)
  const [isGenerating, setIsGenerating] = useState(false)
  const [generateError, setGenerateError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const generatedKeyRef = useRef<GeneratedKey | null>(null)

  // Keep ref in sync so cleanup callback always has the latest value
  useEffect(() => {
    generatedKeyRef.current = generatedKey
  }, [generatedKey])

  const { data: vmwareCreds = [] } = useVmwareCredentialsQuery()
  const credOptions = vmwareCreds
    .filter((c) => c.status?.vmwareValidationStatus?.toLowerCase() === 'succeeded')
    .map((c) => ({ label: c.metadata.name, value: c.metadata.name }))

  const form = useForm<FormData>({
    defaultValues: { vmwareCredsRef: '', vmName: '', sshPrivateKey: '' },
    mode: 'onChange',
    reValidateMode: 'onChange'
  })

  const { watch, reset, setValue, trigger, formState: { isValid } } = form
  const vmwareCredsRef = watch('vmwareCredsRef')
  const vmName = watch('vmName')

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

  // Reset VM selection when credentials change
  useEffect(() => {
    setValue('vmName', '')
  }, [vmwareCredsRef, setValue])

  // When switching key source or vm name changes, clear old generated key
  useEffect(() => {
    if (generatedKey) {
      deleteSSHKeyPair(generatedKey.secretName).catch(() => {})
      setGeneratedKey(null)
      setGenerateError(null)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sshKeySource, vmName])

  useEffect(() => {
    trigger()
  }, [sshKeySource, generatedKey, trigger])

  const handleClose = useCallback(() => {
    // Clean up any generated-but-not-submitted key
    if (generatedKeyRef.current) {
      deleteSSHKeyPair(generatedKeyRef.current.secretName).catch(() => {})
    }
    reset()
    setSubmitError(null)
    setGenerateError(null)
    setGeneratedKey(null)
    setCopied(false)
    setSshKeySource('generated')
    onClose()
  }, [reset, onClose])

  const handleGenerate = async () => {
    const vmNameSafe = toK8sName(vmName)
    if (!vmNameSafe) return
    setIsGenerating(true)
    setGenerateError(null)
    const secretName = `${vmNameSafe}-keypair`
    try {
      let kp
      try {
        kp = await generateSSHKeyPair(secretName)
      } catch (err: any) {
        if (err?.response?.status === 409) {
          // Secret from a prior attempt — delete it and regenerate
          await deleteSSHKeyPair(secretName)
          kp = await generateSSHKeyPair(secretName)
        } else {
          throw err
        }
      }
      setGeneratedKey({ secretName, publicKey: kp.publicKey })
    } catch (err: any) {
      setGenerateError(err?.response?.data?.message || err?.message || 'Key generation failed.')
    } finally {
      setIsGenerating(false)
    }
  }

  const handleCopy = () => {
    if (!generatedKey) return
    navigator.clipboard.writeText(generatedKey.publicKey.trim()).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

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
      let sshSpec: Record<string, unknown>

      if (sshKeySource === 'generated') {
        if (!generatedKey) {
          setSubmitError('Generate a key pair first.')
          return
        }
        sshSpec = { sshKeyPairRef: { name: generatedKey.secretName } }
      } else {
        secretName = vmNameSafe + '-hot-add-ssh-key'
        await createSecret(
          secretName,
          { 'ssh-privatekey': data.sshPrivateKey.trim() },
          VJAILBREAK_DEFAULT_NAMESPACE
        )
        sshSpec = { sshKeySecretRef: { name: secretName } }
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

      // Key was consumed — don't clean it up on close
      setGeneratedKey(null)
      generatedKeyRef.current = null
      queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
      handleClose()
    } catch (err) {
      if (sshKeySource === 'manual' && secretName) {
        import('src/api/secrets/secrets').then(({ deleteSecret }) =>
          deleteSecret(secretName!, VJAILBREAK_DEFAULT_NAMESPACE).catch(() => {})
        )
      }
      if (axios.isAxiosError(err) && err.response?.status === 409) {
        setSubmitError('This VM is already registered as a Proxy VM. Use the Retry button in the table to re-verify it.')
      } else if (axios.isAxiosError(err)) {
        setSubmitError(err.response?.data?.message || 'Failed to create Proxy VM.')
      } else {
        setSubmitError('Failed to create Proxy VM.')
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const vmSelectPlaceholder = !vmwareCredsRef
    ? 'Select VMware credentials first'
    : vmsLoading
      ? 'Loading VMs...'
      : runningVMOptions.length === 0
        ? 'No powered-on VMs found for selected credentials'
        : 'Search and select a VM'

  const isSubmitDisabled =
    isSubmitting ||
    !isValid ||
    (sshKeySource === 'generated' && !generatedKey)

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
                <ToggleButton value="generated">Generate Key Pair</ToggleButton>
                <ToggleButton value="manual">Upload Private Key</ToggleButton>
              </ToggleButtonGroup>

              {sshKeySource === 'generated' ? (
                <Box sx={{ display: 'grid', gap: 2 }}>
                  {!generatedKey ? (
                    <>
                      <Alert severity="info">
                        Generate a key pair — the public key will appear here for you to copy into
                        the Proxy VM&apos;s <strong>/root/.ssh/authorized_keys</strong> before
                        submitting.
                      </Alert>
                      {generateError && (
                        <Alert severity="error" onClose={() => setGenerateError(null)}>
                          {generateError}
                        </Alert>
                      )}
                      <ActionButton
                        tone="primary"
                        onClick={handleGenerate}
                        loading={isGenerating}
                        disabled={!vmName || isGenerating}
                        sx={{ justifySelf: 'start' }}
                      >
                        Generate Key Pair
                      </ActionButton>
                      {!vmName && (
                        <Typography variant="caption" color="text.secondary">
                          Select a VM first to generate a key pair.
                        </Typography>
                      )}
                    </>
                  ) : (
                    <>
                      <Alert severity="success">
                        Key pair generated. Copy the public key below and add it to{' '}
                        <strong>/root/.ssh/authorized_keys</strong> on the Proxy VM before
                        submitting.
                      </Alert>
                      <TextField
                        label="Public Key (copy this to authorized_keys)"
                        value={generatedKey.publicKey.trim()}
                        multiline
                        minRows={4}
                        fullWidth
                        InputProps={{
                          readOnly: true,
                          endAdornment: (
                            <InputAdornment position="end" sx={{ alignSelf: 'flex-start', mt: 1 }}>
                              <Tooltip title={copied ? 'Copied!' : 'Copy public key'}>
                                <IconButton onClick={handleCopy} size="small" edge="end">
                                  {copied ? (
                                    <CheckIcon fontSize="small" color="success" />
                                  ) : (
                                    <ContentCopyIcon fontSize="small" />
                                  )}
                                </IconButton>
                              </Tooltip>
                            </InputAdornment>
                          )
                        }}
                        sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}
                      />
                      <ActionButton
                        tone="secondary"
                        onClick={() => {
                          deleteSSHKeyPair(generatedKey.secretName).catch(() => {})
                          setGeneratedKey(null)
                          setGenerateError(null)
                        }}
                        sx={{ justifySelf: 'start' }}
                        disabled={isSubmitting}
                      >
                        Regenerate
                      </ActionButton>
                    </>
                  )}
                </Box>
              ) : (
                <Box sx={{ display: 'grid', gap: 2 }}>
                  <Alert severity="info">
                    Add the public key corresponding to your private key to the Proxy VM&apos;s{' '}
                    <strong>/root/.ssh/authorized_keys</strong> before submitting.
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
                      Or paste the private key below (OpenSSH, RSA, EC, PKCS#8).
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
