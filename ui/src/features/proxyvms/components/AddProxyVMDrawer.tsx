import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Box, LinearProgress, Typography } from '@mui/material'
import CloudDownloadOutlinedIcon from '@mui/icons-material/CloudDownloadOutlined'
import ComputerOutlinedIcon from '@mui/icons-material/ComputerOutlined'
import { useForm } from 'react-hook-form'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import axios from 'axios'

import { ActionButton, DrawerFooter, DrawerHeader, DrawerShell, SurfaceCard } from 'src/components'
import {
  postProxyVM,
  createProxyVMFromOVA,
  getVCenterResources,
  getProxyVMList
} from 'src/api/proxyvms/proxyVMs'
import { PROXY_VMS_QUERY_KEY, useProxyVMsQuery } from 'src/hooks/api/useProxyVMsQuery'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { generateSSHKeyPair, deleteSSHKeyPair } from 'src/api/sshKeyPairs/sshKeyPairs'
import { getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import { createSecret } from 'src/api/secrets/secrets'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

import MethodCard from './MethodCard'
import RegisterVMForm, { SELECT_FORM_ID } from './RegisterVMForm'
import DeployVMForm, { CREATE_FORM_ID } from './DeployVMForm'
import type {
  FormMode,
  SSHKeySource,
  VMOption,
  GeneratedKey,
  SelectFormData,
  CreateFormData
} from './types'

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

export default function AddProxyVMDrawer({ open, onClose }: AddProxyVMDrawerProps) {
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [formMode, setFormMode] = useState<FormMode>('create')
  const [deploymentStarted, setDeploymentStarted] = useState(false)
  const [deployedVMName, setDeployedVMName] = useState<string | null>(null)
  const [sshKeySource, setSshKeySource] = useState<SSHKeySource>('generated')
  const [generatedKey, setGeneratedKey] = useState<GeneratedKey | null>(null)
  const [isGenerating, setIsGenerating] = useState(false)
  const [generateError, setGenerateError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [selectedVM, setSelectedVM] = useState<VMOption | null>(null)
  const generatedKeyRef = useRef<GeneratedKey | null>(null)

  useEffect(() => {
    generatedKeyRef.current = generatedKey
  }, [generatedKey])

  const { data: vmwareCreds = [] } = useVmwareCredentialsQuery()
  const credOptions = vmwareCreds
    .filter((c) => c.status?.vmwareValidationStatus?.toLowerCase() === 'succeeded')
    .map((c) => ({ label: c.metadata.name, value: c.metadata.name }))

  const { data: existingProxyVMs = [] } = useProxyVMsQuery(undefined, { enabled: open })
  const registeredVMNames = useMemo(
    () => new Set(existingProxyVMs.map((vm) => vm.spec.vmName)),
    [existingProxyVMs]
  )

  const selectForm = useForm<SelectFormData>({
    defaultValues: { vmwareCredsRef: '', vmName: '', sshPrivateKey: '', authorizedKeysConfirmed: false },
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
  const datacenterCreate = createWatch('datacenter')

  const activeCredsRef = formMode === 'select' ? vmwareCredsRefSelect : vmwareCredsRefCreate

  const { data: vmOptions = [], isLoading: vmsLoading } = useQuery({
    queryKey: ['vmwaremachines-for-proxy', activeCredsRef],
    queryFn: async () => {
      const result = await getVMwareMachines(VJAILBREAK_DEFAULT_NAMESPACE, activeCredsRef)
      return result.items
        .filter((m) => m.status?.powerState === 'running')
        .map(
          (m): VMOption => ({
            name: m.spec.vms.name,
            ipAddress: m.spec.vms.ipAddress || m.spec.vms.assignedIp,
            cpu: m.spec.vms.cpu,
            powerState: m.status?.powerState ?? 'unknown',
            osFamily: m.spec.vms.osFamily
          })
        )
    },
    enabled: Boolean(activeCredsRef) && open,
    staleTime: 30_000
  })

  const { data: dcResources, isLoading: dcLoading } = useQuery({
    queryKey: ['vcenter-datacenters', vmwareCredsRefCreate],
    queryFn: () => getVCenterResources(vmwareCredsRefCreate),
    enabled: Boolean(vmwareCredsRefCreate) && formMode === 'create' && open,
    staleTime: 60_000
  })

  const { data: scopedResources, isLoading: scopedLoading } = useQuery({
    queryKey: ['vcenter-scoped-resources', vmwareCredsRefCreate, datacenterCreate],
    queryFn: () => getVCenterResources(vmwareCredsRefCreate, datacenterCreate),
    enabled:
      Boolean(vmwareCredsRefCreate) && Boolean(datacenterCreate) && formMode === 'create' && open,
    staleTime: 60_000
  })

  useEffect(() => {
    selectSetValue('vmName', '')
    setSelectedVM(null)
  }, [vmwareCredsRefSelect, selectSetValue])

  useEffect(() => {
    createSetValue('datastore', '')
    createSetValue('network', '')
    createSetValue('cluster', '')
  }, [vmwareCredsRefCreate, datacenterCreate, createSetValue])

  useEffect(() => {
    if (formMode === 'create') {
      if (vmwareCredsRefSelect) createSetValue('vmwareCredsRef', vmwareCredsRefSelect)
    } else {
      if (vmwareCredsRefCreate) selectSetValue('vmwareCredsRef', vmwareCredsRefCreate)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [formMode])

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
    setFormMode('create')
    setDeploymentStarted(false)
    setDeployedVMName(null)
    setSelectedVM(null)
  }, [selectReset, createReset])

  const handleClose = useCallback(() => {
    if (generatedKeyRef.current)
      deleteSSHKeyPair(generatedKeyRef.current.secretName).catch(() => {})
    resetAll()
    onClose()
  }, [resetAll, onClose])

  const { data: polledVMs = [] } = useQuery({
    queryKey: [...PROXY_VMS_QUERY_KEY, 'deploy-poll'],
    queryFn: () => getProxyVMList(),
    enabled: deploymentStarted,
    refetchInterval: deploymentStarted ? 5000 : false
  })

  useEffect(() => {
    if (!deploymentStarted || !deployedVMName) return
    if (polledVMs.some((vm) => vm.metadata?.name === deployedVMName)) handleClose()
  }, [polledVMs, deploymentStarted, deployedVMName, handleClose])

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

  const handleVMChange = useCallback(
    (vm: VMOption | null) => {
      setSelectedVM(vm)
      selectSetValue('vmName', vm?.name ?? '', { shouldValidate: true, shouldDirty: true })
    },
    [selectSetValue]
  )

  const handleRegenerateKey = useCallback(() => {
    if (generatedKey) deleteSSHKeyPair(generatedKey.secretName).catch(() => {})
    setGeneratedKey(null)
    setGenerateError(null)
  }, [generatedKey])

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
        spec: { vmName: data.vmName, vmwareCredsRef: { name: data.vmwareCredsRef }, ...sshSpec }
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
          'This VM is already registered as a vJailbreak Proxy VM. Use the Retry button in the table to re-verify it.'
        )
      } else if (axios.isAxiosError(err)) {
        setSubmitError(err.response?.data?.message || 'Failed to create vJailbreak Proxy VM.')
      } else {
        setSubmitError('Failed to create vJailbreak Proxy VM.')
      }
    } finally {
      setIsSubmitting(false)
    }
  }

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
      setDeployedVMName(data.vmName)
      setDeploymentStarted(true)
    } catch (err: any) {
      setSubmitError(err?.response?.data?.message || err?.message || 'Failed to start VM creation.')
    } finally {
      setIsSubmitting(false)
    }
  }

  const isSelectDisabled =
    isSubmitting || !selectIsValid || (sshKeySource === 'generated' && !generatedKey)
  const isCreateDisabled = isSubmitting || !createIsValid
  const activeFormId = formMode === 'select' ? SELECT_FORM_ID : CREATE_FORM_ID
  const submitLabel =
    formMode === 'create'
      ? isSubmitting
        ? 'Deploying...'
        : 'Deploy & Register VM'
      : isSubmitting
        ? 'Registering...'
        : 'Register vJailbreak Proxy VM'
  const subtitle =
    formMode === 'create'
      ? 'A new vJailbreak Proxy VM is deployed from the OVA template and registered automatically.'
      : 'Register a powered-on VM you have already prepared as a vJailbreak Accelerated Copy proxy.'

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      header={<DrawerHeader title="Add vJailbreak Proxy VM" subtitle={subtitle} onClose={handleClose} />}
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

          <Box sx={{ display: 'grid', gap: 1 }}>
            <Typography variant="overline" color="text.secondary" sx={{ lineHeight: 1 }}>
              Method
            </Typography>
            <MethodCard
              selected={formMode === 'create'}
              onClick={() => !deploymentStarted && setFormMode('create')}
              icon={<CloudDownloadOutlinedIcon fontSize="small" />}
              title="Deploy a new vJailbreak Proxy VM"
              description="Spin one up from the bundled OVA template. SSH access is configured automatically."
              recommended
            />
            <MethodCard
              selected={formMode === 'select'}
              onClick={() => !deploymentStarted && setFormMode('select')}
              icon={<ComputerOutlinedIcon fontSize="small" />}
              title="Register an existing VM"
              description="Point vJailbreak at a powered-on VM you've already prepared as a proxy."
            />
          </Box>

          <Box sx={{ borderBottom: '1px solid', borderColor: 'divider', my: 2 }} />

          {formMode === 'select' && (
            <RegisterVMForm
              form={selectForm}
              onSubmit={onSelectSubmit}
              open={open}
              onClose={handleClose}
              isSubmitDisabled={isSelectDisabled}
              credOptions={credOptions}
              vmwareCredsRefSelect={vmwareCredsRefSelect}
              vmOptions={vmOptions}
              vmsLoading={vmsLoading}
              selectedVM={selectedVM}
              onVMChange={handleVMChange}
              isSubmitting={isSubmitting}
              sshKeySource={sshKeySource}
              onSshKeySourceChange={setSshKeySource}
              generatedKey={generatedKey}
              isGenerating={isGenerating}
              generateError={generateError}
              onClearGenerateError={() => setGenerateError(null)}
              copied={copied}
              onGenerate={handleGenerate}
              onRegenerateKey={handleRegenerateKey}
              onCopy={handleCopy}
              onKeyFileUpload={handleKeyFileUpload}
              onSubmitErrorChange={setSubmitError}
              registeredVMNames={registeredVMNames}
            />
          )}

          {formMode === 'create' && deploymentStarted && (
            <Box sx={{ display: 'grid', gap: 2 }}>
              <Alert severity="success">
                Deployment started for <strong>{createForm.getValues('vmName')}</strong>.
              </Alert>
              <LinearProgress />
              <Typography variant="body2" color="text.secondary">
                vJailbreak is deploying the OVA to vCenter, configuring SSH access, and registering
                the vJailbreak Proxy VM. This typically takes <strong>3–5 minutes</strong>.
              </Typography>
              <Alert severity="info">
                You can close this panel — the VM will appear in the vJailbreak Proxy VMs list once
                provisioning is complete and verification begins automatically.
              </Alert>
            </Box>
          )}

          {formMode === 'create' && !deploymentStarted && (
            <DeployVMForm
              form={createForm}
              onSubmit={onCreateSubmit}
              open={open}
              onClose={handleClose}
              isSubmitDisabled={isCreateDisabled}
              credOptions={credOptions}
              vmwareCredsRefCreate={vmwareCredsRefCreate}
              datacenterCreate={datacenterCreate}
              dcResources={dcResources}
              scopedResources={scopedResources}
              dcLoading={dcLoading}
              scopedLoading={scopedLoading}
              isSubmitting={isSubmitting}
              vmOptions={vmOptions}
              registeredVMNames={registeredVMNames}
            />
          )}
        </Box>
      </SurfaceCard>
    </DrawerShell>
  )
}
