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

import { postProxyVM, createProxyVMFromOVA } from 'src/api/proxyvms/proxyVMs'
import { PROXY_VMS_QUERY_KEY } from 'src/hooks/api/useProxyVMsQuery'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { generateSSHKeyPair, deleteSSHKeyPair } from 'src/api/sshKeyPairs/sshKeyPairs'
import { getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import { createSecret } from 'src/api/secrets/secrets'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { validateSshPrivateKey } from 'src/utils'

type SSHKeySource = 'generated' | 'manual'
type FormMode = 'select' | 'create'

interface SelectFormData {
  vmwareCredsRef: string
  vmName: string
  sshPrivateKey: string
}

interface CreateFormData {
  vmwareCredsRef: string
  vmName: string
  datacenter: string
  datastore: string
  network: string
  cluster: string
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

const SELECT_FORM_ID = 'add-proxy-vm-form'
const CREATE_FORM_ID = 'create-proxy-vm-form'

export default function AddProxyVMDrawer({ open, onClose }: AddProxyVMDrawerProps) {
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [formMode, setFormMode] = useState<FormMode>('select')

  // ── Select mode state ────────────────────────────────────────────────────
  const [sshKeySource, setSshKeySource] = useState<SSHKeySource>('generated')
  const [generatedKey, setGeneratedKey] = useState<GeneratedKey | null>(null)
  const [isGenerating, setIsGenerating] = useState(false)
  const [generateError, setGenerateError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const generatedKeyRef = useRef<GeneratedKey | null>(null)

  useEffect(() => {
    generatedKeyRef.current = generatedKey
  }, [generatedKey])

  // ── VMware credentials ────────────────────────────────────────────────────
  const { data: vmwareCreds = [] } = useVmwareCredentialsQuery()
  const credOptions = vmwareCreds
    .filter((c) => c.status?.vmwareValidationStatus?.toLowerCase() === 'succeeded')
    .map((c) => ({ label: c.metadata.name, value: c.metadata.name }))

  // ── Select form ────────────────────────────────────────────────────────────
  const selectForm = useForm<SelectFormData>({
    defaultValues: { vmwareCredsRef: '', vmName: '', sshPrivateKey: '' },
    mode: 'onChange',
    reValidateMode: 'onChange'
  })
  const { watch: selectWatch, reset: selectReset, setValue: selectSetValue, trigger: selectTrigger, formState: { isValid: selectIsValid } } = selectForm
  const vmwareCredsRefSelect = selectWatch('vmwareCredsRef')
  const vmNameSelect = selectWatch('vmName')

  // ── Create form ────────────────────────────────────────────────────────────
  const createForm = useForm<CreateFormData>({
    defaultValues: { vmwareCredsRef: '', vmName: '', datacenter: '', datastore: '', network: '', cluster: '' },
    mode: 'onChange',
    reValidateMode: 'onChange'
  })
  const { watch: createWatch, reset: createReset, setValue: createSetValue, formState: { isValid: createIsValid } } = createForm
  const vmwareCredsRefCreate = createWatch('vmwareCredsRef')

  // Active credentials — whichever form is showing
  const activeCredsRef = formMode === 'select' ? vmwareCredsRefSelect : vmwareCredsRefCreate

  // ── VM list query ─────────────────────────────────────────────────────────
  const { data: runningVMOptions = [], isLoading: vmsLoading } = useQuery({
    queryKey: ['vmwaremachines-for-proxy', activeCredsRef],
    queryFn: async () => {
      const result = await getVMwareMachines(VJAILBREAK_DEFAULT_NAMESPACE, activeCredsRef)
      return result.items
        .filter((m) => m.status?.powerState === 'running')
        .map((m) => ({ label: m.spec.vms.name, value: m.spec.vms.name }))
    },
    enabled: Boolean(activeCredsRef) && open,
    staleTime: 30_000
  })

  const showCreateOption =
    Boolean(activeCredsRef) && !vmsLoading && runningVMOptions.length === 0

  // Reset VM selection when credentials change (select mode)
  useEffect(() => {
    selectSetValue('vmName', '')
  }, [vmwareCredsRefSelect, selectSetValue])

  // Sync creds across forms when mode switches
  useEffect(() => {
    if (formMode === 'create' && vmwareCredsRefSelect) {
      createSetValue('vmwareCredsRef', vmwareCredsRefSelect)
    } else if (formMode === 'select' && vmwareCredsRefCreate) {
      selectSetValue('vmwareCredsRef', vmwareCredsRefCreate)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [formMode])

  // When switching key source or vm name changes, clear old generated key
  useEffect(() => {
    if (generatedKey) {
      deleteSSHKeyPair(generatedKey.secretName).catch(() => {})
      setGeneratedKey(null)
      setGenerateError(null)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sshKeySource, vmNameSelect])

  useEffect(() => {
    selectTrigger()
  }, [sshKeySource, generatedKey, selectTrigger])

  const resetAll = useCallback(() => {
    selectReset()
    createReset()
    setSubmitError(null)
    setGenerateError(null)
    setGeneratedKey(null)
    setCopied(false)
    setSshKeySource('generated')
    setFormMode('select')
  }, [selectReset, createReset])

  const handleClose = useCallback(() => {
    if (generatedKeyRef.current) {
      deleteSSHKeyPair(generatedKeyRef.current.secretName).catch(() => {})
    }
    resetAll()
    onClose()
  }, [resetAll, onClose])

  // ── Generate SSH keypair ──────────────────────────────────────────────────
  const handleGenerate = async () => {
    const vmNameSafe = toK8sName(vmNameSelect)
    if (!vmNameSafe) return
    setIsGenerating(true)
    setGenerateError(null)
    const secretName = `${vmNameSafe}-keypair`
    try {
      const kp = await generateSSHKeyPair(secretName)
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
        selectSetValue('sshPrivateKey', text, { shouldDirty: true, shouldValidate: true })
        setSubmitError(null)
      } catch {
        setSubmitError('Failed to read key file.')
      }
    },
    [selectSetValue]
  )

  // ── Submit: select mode ────────────────────────────────────────────────────
  const onSelectSubmit = async (data: SelectFormData) => {
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

  // ── Submit: create mode ────────────────────────────────────────────────────
  const onCreateSubmit = async (data: CreateFormData) => {
    setIsSubmitting(true)
    setSubmitError(null)
    try {
      await createProxyVMFromOVA({
        vmName: data.vmName,
        vmwareCredsRef: data.vmwareCredsRef,
        datacenter: data.datacenter,
        datastore: data.datastore,
        network: data.network,
        cluster: data.cluster || undefined
      })
      queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
      handleClose()
    } catch (err: any) {
      setSubmitError(
        err?.response?.data?.message || err?.message || 'Failed to start VM creation.'
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const vmSelectPlaceholder = !activeCredsRef
    ? 'Select VMware credentials first'
    : vmsLoading
      ? 'Loading VMs...'
      : runningVMOptions.length === 0
        ? 'No powered-on VMs found'
        : 'Search and select a VM'

  const isSelectDisabled =
    isSubmitting ||
    !selectIsValid ||
    (sshKeySource === 'generated' && !generatedKey)

  const isCreateDisabled = isSubmitting || !createIsValid

  const activeFormId = formMode === 'select' ? SELECT_FORM_ID : CREATE_FORM_ID
  const submitLabel = formMode === 'create'
    ? (isSubmitting ? 'Creating...' : 'Create & Register VM')
    : 'Add Proxy VM'

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      header={
        <DrawerHeader
          title="Add Proxy VM"
          subtitle={
            formMode === 'create'
              ? 'Deploy a new Proxy VM from the OVA template and register it automatically'
              : 'Register a Proxy VM for Hot-Add data copy migrations'
          }
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
            form={activeFormId}
            loading={isSubmitting}
            disabled={formMode === 'select' ? isSelectDisabled : isCreateDisabled}
          >
            {submitLabel}
          </ActionButton>
        </DrawerFooter>
      }
    >
      <SurfaceCard>
        <Box sx={{ display: 'grid', gap: 2 }}>
          {submitError && (
            <Alert severity="error" onClose={() => setSubmitError(null)}>
              {submitError}
            </Alert>
          )}

          {/* ── Select mode ─────────────────────────────────────────────── */}
          {formMode === 'select' && (
            <DesignSystemForm
              id={SELECT_FORM_ID}
              form={selectForm}
              onSubmit={onSelectSubmit}
              keyboardSubmitProps={{ open, onClose: handleClose, isSubmitDisabled: isSelectDisabled }}
            >
              <Box sx={{ display: 'grid', gap: 2 }}>
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
                    {vmwareCredsRefSelect && !vmsLoading && (
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
                      disabled={!vmwareCredsRefSelect || vmsLoading}
                    />

                    {/* Offer to create a VM when no running VMs are found */}
                    {showCreateOption && (
                      <Alert
                        severity="warning"
                        action={
                          <ActionButton
                            tone="secondary"
                            size="small"
                            onClick={() => setFormMode('create')}
                          >
                            Create VM
                          </ActionButton>
                        }
                      >
                        No powered-on VMs found. Deploy a new Proxy VM from the OVA template instead.
                      </Alert>
                    )}
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
                            Generate a key pair — the public key will appear here for you to copy
                            into the Proxy VM&apos;s{' '}
                            <strong>/root/.ssh/authorized_keys</strong> before submitting.
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
                            disabled={!vmNameSelect || isGenerating}
                            sx={{ justifySelf: 'start' }}
                          >
                            Generate Key Pair
                          </ActionButton>
                          {!vmNameSelect && (
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
                                <InputAdornment
                                  position="end"
                                  sx={{ alignSelf: 'flex-start', mt: 1 }}
                                >
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
            </DesignSystemForm>
          )}

          {/* ── Create mode ──────────────────────────────────────────────── */}
          {formMode === 'create' && (
            <DesignSystemForm
              id={CREATE_FORM_ID}
              form={createForm}
              onSubmit={onCreateSubmit}
              keyboardSubmitProps={{ open, onClose: handleClose, isSubmitDisabled: isCreateDisabled }}
            >
              <Box sx={{ display: 'grid', gap: 2 }}>
                <Alert
                  severity="info"
                  action={
                    <ActionButton
                      tone="secondary"
                      size="small"
                      onClick={() => setFormMode('select')}
                      disabled={isSubmitting}
                    >
                      Back
                    </ActionButton>
                  }
                >
                  A new Proxy VM will be deployed from the OVA template. SSH access is
                  configured automatically — no key setup required.
                </Alert>

                <Section>
                  <SectionHeader
                    title="VM Details"
                    subtitle="Provide a name and target location for the new Proxy VM."
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
                    <RHFTextField
                      name="vmName"
                      label="VM Name"
                      rules={{ required: 'VM name is required' }}
                      placeholder="vjailbreak-ha-proxy"
                      disabled={isSubmitting}
                    />
                  </Box>
                </Section>

                <Section>
                  <SectionHeader
                    title="Deployment Target"
                    subtitle="Specify where in the VMware environment to deploy the VM."
                  />
                  <Box sx={{ display: 'grid', gap: 2 }}>
                    <RHFTextField
                      name="datacenter"
                      label="Datacenter"
                      rules={{ required: 'Datacenter is required' }}
                      placeholder="e.g. prison"
                      disabled={isSubmitting}
                    />
                    <RHFTextField
                      name="datastore"
                      label="Datastore"
                      rules={{ required: 'Datastore is required' }}
                      placeholder="e.g. datastore-nfs"
                      disabled={isSubmitting}
                    />
                    <RHFTextField
                      name="network"
                      label="Network"
                      rules={{ required: 'Network is required' }}
                      placeholder="e.g. network-19"
                      disabled={isSubmitting}
                    />
                    <RHFTextField
                      name="cluster"
                      label="Cluster (optional)"
                      placeholder="Leave blank to use the first available host"
                      disabled={isSubmitting}
                    />
                  </Box>
                </Section>
              </Box>
            </DesignSystemForm>
          )}
        </Box>
      </SurfaceCard>
    </DrawerShell>
  )
}
