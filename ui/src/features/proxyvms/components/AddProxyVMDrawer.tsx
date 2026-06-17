import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Alert,
  Box,
  IconButton,
  InputAdornment,
  LinearProgress,
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

import { postProxyVM, createProxyVMFromOVA, getVCenterResources } from 'src/api/proxyvms/proxyVMs'
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
  const [deploymentStarted, setDeploymentStarted] = useState(false)

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
  const {
    watch: selectWatch,
    reset: selectReset,
    setValue: selectSetValue,
    trigger: selectTrigger,
    formState: { isValid: selectIsValid }
  } = selectForm
  const vmwareCredsRefSelect = selectWatch('vmwareCredsRef')
  const vmNameSelect = selectWatch('vmName')

  // ── Create form ────────────────────────────────────────────────────────────
  const createForm = useForm<CreateFormData>({
    defaultValues: {
      vmwareCredsRef: '',
      vmName: '',
      datacenter: '',
      datastore: '',
      network: '',
      cluster: ''
    },
    mode: 'onChange',
    reValidateMode: 'onChange'
  })
  const {
    watch: createWatch,
    reset: createReset,
    setValue: createSetValue,
    formState: { isValid: createIsValid }
  } = createForm
  const vmwareCredsRefCreate = createWatch('vmwareCredsRef')

  // Active credentials — whichever form is showing
  const activeCredsRef = formMode === 'select' ? vmwareCredsRefSelect : vmwareCredsRefCreate

  // ── VM list query ─────────────────────────────────────────────────────────
  const { data: runningVMs = [], isLoading: vmsLoading } = useQuery({
    queryKey: ['vmwaremachines-for-proxy', activeCredsRef],
    queryFn: async () => {
      const result = await getVMwareMachines(VJAILBREAK_DEFAULT_NAMESPACE, activeCredsRef)
      return result.items
        .filter((m) => m.status?.powerState === 'running')
        .map((m) => m.spec.vms.name)
    },
    enabled: Boolean(activeCredsRef) && open,
    staleTime: 30_000
  })

  const datacenterCreate = createWatch('datacenter')

  // Step 1: fetch datacenter list (no datacenter param needed)
  const { data: dcResources, isLoading: dcLoading } = useQuery({
    queryKey: ['vcenter-datacenters', vmwareCredsRefCreate],
    queryFn: () => getVCenterResources(vmwareCredsRefCreate),
    enabled: Boolean(vmwareCredsRefCreate) && formMode === 'create' && open,
    staleTime: 60_000
  })

  // Step 2: fetch resources scoped to the selected datacenter
  const { data: scopedResources, isLoading: scopedLoading } = useQuery({
    queryKey: ['vcenter-scoped-resources', vmwareCredsRefCreate, datacenterCreate],
    queryFn: () => getVCenterResources(vmwareCredsRefCreate, datacenterCreate),
    enabled: Boolean(vmwareCredsRefCreate) && Boolean(datacenterCreate) && formMode === 'create' && open,
    staleTime: 60_000
  })

  const toOptions = (items: string[] | undefined) =>
    (items ?? []).map((v) => ({ label: v, value: v }))

  // True when the typed name doesn't match any running VM (and we've finished loading)
  const vmNameEntered = vmNameSelect.trim().length > 0
  const vmExists = runningVMs.some(
    (n) => n.toLowerCase() === vmNameSelect.trim().toLowerCase()
  )
  const showCreateOffer =
    vmNameEntered && !vmsLoading && Boolean(vmwareCredsRefSelect) && !vmExists

  // Reset VM name when credentials change
  useEffect(() => {
    selectSetValue('vmName', '')
  }, [vmwareCredsRefSelect, selectSetValue])

  // Clear scoped fields when credentials or datacenter change
  useEffect(() => {
    createSetValue('datastore', '')
    createSetValue('network', '')
    createSetValue('cluster', '')
  }, [vmwareCredsRefCreate, datacenterCreate, createSetValue])

  // Sync creds across forms when mode switches
  useEffect(() => {
    if (formMode === 'create') {
      if (vmwareCredsRefSelect) createSetValue('vmwareCredsRef', vmwareCredsRefSelect)
      if (vmNameSelect) createSetValue('vmName', toK8sName(vmNameSelect))
    } else {
      if (vmwareCredsRefCreate) selectSetValue('vmwareCredsRef', vmwareCredsRefCreate)
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
    setDeploymentStarted(false)
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

  const switchToCreate = () => {
    setFormMode('create')
  }

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
        setSubmitError(
          'This VM is already registered as a Proxy VM. Use the Retry button in the table to re-verify it.'
        )
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
      setDeploymentStarted(true)
    } catch (err: any) {
      setSubmitError(
        err?.response?.data?.message || err?.message || 'Failed to start VM creation.'
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const isSelectDisabled =
    isSubmitting || !selectIsValid || (sshKeySource === 'generated' && !generatedKey)

  const isCreateDisabled = isSubmitting || !createIsValid

  const activeFormId = formMode === 'select' ? SELECT_FORM_ID : CREATE_FORM_ID
  const submitLabel =
    formMode === 'create' ? (isSubmitting ? 'Creating...' : 'Create & Register VM') : 'Add Proxy VM'

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
            {deploymentStarted ? 'Done' : 'Cancel'}
          </ActionButton>
          {!deploymentStarted && (
            <ActionButton
              tone="primary"
              type="submit"
              form={activeFormId}
              loading={isSubmitting}
              disabled={formMode === 'select' ? isSelectDisabled : isCreateDisabled}
            >
              {submitLabel}
            </ActionButton>
          )}
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

          {/* ── Select (register existing VM) mode ───────────────────────── */}
          {formMode === 'select' && (
            <DesignSystemForm
              id={SELECT_FORM_ID}
              form={selectForm}
              onSubmit={onSelectSubmit}
              keyboardSubmitProps={{
                open,
                onClose: handleClose,
                isSubmitDisabled: isSelectDisabled
              }}
            >
              <Box sx={{ display: 'grid', gap: 2 }}>
                <Section>
                  <SectionHeader
                    title="Proxy VM"
                    subtitle="Select the VMware environment and enter the name of the powered-on VM to register as a proxy."
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
                      placeholder={
                        !vmwareCredsRefSelect
                          ? 'Select credentials first'
                          : vmsLoading
                            ? 'Loading VMs…'
                            : 'Enter the VM name'
                      }
                      disabled={!vmwareCredsRefSelect || vmsLoading || isSubmitting}
                    />

                    {/* VM exists — green confirmation */}
                    {vmNameEntered && !vmsLoading && vmExists && (
                      <Alert severity="success">
                        VM <strong>{vmNameSelect}</strong> found and powered on.
                      </Alert>
                    )}

                    {/* VM not found — offer to create */}
                    {showCreateOffer && (
                      <Alert
                        severity="warning"
                        action={
                          <ActionButton
                            tone="secondary"
                            size="small"
                            onClick={switchToCreate}
                            disabled={isSubmitting}
                          >
                            Create VM
                          </ActionButton>
                        }
                      >
                        No running VM named <strong>{vmNameSelect}</strong> found. Deploy a new
                        Proxy VM from the OVA template instead.
                      </Alert>
                    )}
                  </Box>
                </Section>

                {/* SSH Access — only relevant when registering an existing VM */}
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
                              Enter a VM name first to generate a key pair.
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

          {/* ── Create (deploy from OVA) mode ────────────────────────────── */}
          {formMode === 'create' && deploymentStarted && (
            <Box sx={{ display: 'grid', gap: 2 }}>
              <Alert severity="success">
                Deployment started for <strong>{createForm.getValues('vmName')}</strong>.
              </Alert>
              <LinearProgress />
              <Typography variant="body2" color="text.secondary">
                vJailbreak is deploying the OVA to vCenter, configuring SSH access, and
                registering the Proxy VM. This typically takes <strong>3–5 minutes</strong>.
              </Typography>
              <Alert severity="info">
                You can close this panel — the VM will appear in the Proxy VMs list once
                provisioning is complete and verification begins automatically.
              </Alert>
            </Box>
          )}

          {formMode === 'create' && !deploymentStarted && (
            <DesignSystemForm
              id={CREATE_FORM_ID}
              form={createForm}
              onSubmit={onCreateSubmit}
              keyboardSubmitProps={{
                open,
                onClose: handleClose,
                isSubmitDisabled: isCreateDisabled
              }}
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
                  A new Proxy VM will be deployed from the OVA template. SSH access is configured
                  automatically — no key setup required.
                </Alert>

                <Section>
                  <SectionHeader
                    title="VM Details"
                    subtitle="Provide a name and VMware credentials for the new Proxy VM."
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
                    <RHFSelect
                      name="datacenter"
                      label="Datacenter"
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
                        !datacenterCreate
                          ? 'Select datacenter first'
                          : scopedLoading
                            ? 'Loading…'
                            : 'Leave blank to use first available host'
                      }
                      disabled={!datacenterCreate || scopedLoading || isSubmitting}
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
