import { Box, Alert, Divider, Typography, useMediaQuery } from '@mui/material'
import MigrationIcon from '@mui/icons-material/SwapHoriz'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import useParams from 'src/hooks/useParams'
import MigrationOptions from '../steps/MigrationOptionsAlt'
import NetworkAndStorageMappingStep from '../steps/NetworkAndStorageMappingStep'
import SecurityGroupAndServerGroupStep from '../steps/SecurityGroupAndServerGroup'
import SourceDestinationClusterSelection from '../steps/SourceDestinationClusterSelection'
import VmsSelectionStep from '../steps/VmsSelectionStep'
import { useClusterData } from '../hooks/useClusterData'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useRdmDisksQuery } from 'src/hooks/api/useRdmDisksQuery'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { useFormValidation } from '../hooks/useFormValidation'
import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  NavTab,
  NavTabs,
  SectionNav,
  SurfaceCard
} from 'src/components'
import { useTheme } from '@mui/material/styles'
import { useForm } from 'react-hook-form'
import { DesignSystemForm } from 'src/shared/components/forms'
import type {
  FormValues,
  SelectedMigrationOptionsType,
  FieldErrors,
  MigrationDrawerRHFValues,
  MigrationFormDrawerProps
} from '../types'
import { useSectionTracking } from '../hooks/useSectionTracking'
import { useNetworkIPsMap } from '../hooks/useNetworkIPsMap'
import { useNetworkSubnetCompatibility } from '../hooks/useNetworkSubnetCompatibility'
import { hasAnySubnetMismatch } from '../utils/subnetMismatch'
import { useFormSync } from '../hooks/useFormSync'
import { useCredentialFetching } from '../hooks/useCredentialFetching'
import { useMigrationFormSubmit } from '../hooks/useMigrationFormSubmit'
import { useSettingsConfigMapQuery } from 'src/hooks/api/useSettingsConfigMapQuery'
import { useRetryPrefill } from '../hooks/useRetryPrefill'
import { useRetrySubmit } from '../hooks/useRetrySubmit'
import { Banner } from 'src/components'
import { RetrySourceDestinationSummary } from '../components/RetryMigration'
import { useApplyTemplatePrefill } from '../hooks/useApplyTemplatePrefill'
import SaveAsTemplateDialog from '../components/templates/SaveAsTemplateDialog'
import type { SaveAsTemplateInput } from '../api/migration-blueprints/types'
import { CUTOVER_TYPES } from '../constants'

const drawerWidth = 1400

// Default state for checkboxes
const defaultMigrationOptions = {
  dataCopyMethod: false,
  dataCopyStartTime: false,
  cutoverOption: false,
  cutoverStartTime: false,
  cutoverEndTime: false,
  postMigrationScript: false,
  useGPU: false,
  postMigrationAction: {
    suffix: false,
    folderName: false,
    renameVm: false,
    moveToFolder: false
  }
}

const defaultValues: Partial<FormValues> = {}

export default function MigrationFormDrawer({
  open,
  onClose,
  onSuccess,
  retryConfig,
  templatePrefill,
  templateMode
}: MigrationFormDrawerProps) {
  const isRetryMode = Boolean(retryConfig)
  const isCreateTemplateMode = templateMode === 'create'
  const isEditTemplateMode = templateMode === 'edit'
  const isTemplateMode = isCreateTemplateMode || isEditTemplateMode
  const navigate = useNavigate()
  const { params, getParamsUpdater, updateParams } = useParams<FormValues>(defaultValues)
  const { pcdData, sourceData } = useClusterData()
  const { reportError } = useErrorHandler({ component: 'MigrationForm' })
  const { track } = useAmplitude({ component: 'MigrationForm' })
  // Theses are the errors that will be displayed on the form
  const { params: fieldErrors, getParamsUpdater: getFieldErrorsUpdater } = useParams<FieldErrors>(
    {}
  )
  const queryClient = useQueryClient()

  // Migration Options - Checked or Unchecked state
  const {
    params: selectedMigrationOptions,
    getParamsUpdater: updateSelectedMigrationOptions,
    updateParams: updateSelectedMigrationOptionsBulk
  } = useParams<SelectedMigrationOptionsType>(defaultMigrationOptions)

  // Generate a unique session ID for this form instance
  const [sessionId] = useState(() => `form-session-${Date.now()}`)

  const { data: settingsConfigMap } = useSettingsConfigMapQuery()
  const networkPersistenceSeedRef = useRef(false)

  const skipDefaultSeeding = isRetryMode || Boolean(templatePrefill)

  // Seed networkPersistence from the global default once per open session.
  // Reset the flag when the drawer closes so the next open starts fresh.
  useEffect(() => {
    if (!open) {
      networkPersistenceSeedRef.current = false
      return
    }
    if (skipDefaultSeeding) return
    if (networkPersistenceSeedRef.current) return
    if (!settingsConfigMap) return
    networkPersistenceSeedRef.current = true
    const raw = settingsConfigMap.data?.DEFAULT_NETWORK_PERSISTENCE
    updateParams({ networkPersistence: raw === 'true' })
  }, [open, settingsConfigMap, updateParams, skipDefaultSeeding])

  const form = useForm<MigrationDrawerRHFValues, any, MigrationDrawerRHFValues>({
    defaultValues: {
      securityGroups: params.securityGroups ?? [],
      serverGroup: params.serverGroup ?? '',
      dataCopyStartTime: params.dataCopyStartTime ?? '',
      cutoverStartTime: params.cutoverStartTime ?? '',
      cutoverEndTime: params.cutoverEndTime ?? '',
      postMigrationActionSuffix: params.postMigrationAction?.suffix ?? '',
      postMigrationActionFolderName: params.postMigrationAction?.folderName ?? ''
    }
  })

  useFormSync({ form, params, getParamsUpdater, selectedMigrationOptions })

  const {
    vmwareCredentials,
    openstackCredentials,
    migrationTemplate,
    setMigrationTemplate,
    setVmwareCredentials,
    setOpenstackCredentials,
    vmwareCredsValidated,
    openstackCredsValidated,
    targetPCDClusterName
  } = useCredentialFetching({
    params,
    pcdData,
    getFieldErrorsUpdater,
    // Retry mode reuses the plan's existing template; never auto-create or auto-patch it.
    disableTemplateSync: isRetryMode
  })

  const {
    prefillLoading,
    blockingError,
    retryPlan,
    retryTemplate,
    retryVm,
    vmK8sName,
    sourceCluster
  } = useRetryPrefill({
    open,
    retryConfig,
    pcdData,
    updateParams,
    updateSelectedOptions: updateSelectedMigrationOptionsBulk,
    form,
    setMigrationTemplate
  })

  // "Use template" prefill — mirrors useRetryPrefill's template → FormValues mapping,
  // but never touches migrationTemplate/ephemeral-template state: Use Template only
  // seeds FormValues, so the normal New Migration flow (a fresh ephemeral
  // MigrationTemplate auto-created on cred validation) proceeds untouched.
  useApplyTemplatePrefill({
    open: open && !isRetryMode,
    templatePrefill,
    pcdData,
    sourceData,
    currentPcdCluster: params.pcdCluster,
    currentVmwareCluster: params.vmwareCluster,
    updateParams,
    updateSelectedOptions: updateSelectedMigrationOptionsBulk
  })

  const [saveTemplateOpen, setSaveTemplateOpen] = useState(false)
  const canSaveAsTemplate = !isRetryMode && vmwareCredsValidated && openstackCredsValidated

  const selectedVmwareClusterName = useMemo(() => {
    if (!params.vmwareCluster) return ''
    const credName = params.vmwareCluster.split(':')[0]
    const sourceItem = sourceData.find((item) => item.credName === credName)
    const clusterObj = sourceItem?.clusters.find((cluster) => cluster.id === params.vmwareCluster)
    return clusterObj?.name || ''
  }, [sourceData, params.vmwareCluster])

  const buildSaveTemplateInput = useCallback(
    (fields: { displayName: string; description?: string }): SaveAsTemplateInput => ({
      ...fields,
      sourceVCenter:
        vmwareCredentials?.metadata?.name || params.vmwareCreds?.existingCredName || '',
      sourceCluster: selectedVmwareClusterName,
      destination:
        openstackCredentials?.metadata?.name || params.openstackCreds?.existingCredName || '',
      targetCluster: targetPCDClusterName || '',
      networkMappings: params.networkMappings || [],
      storageMappings: params.storageMappings || [],
      arrayCredsMappings: params.arrayCredsMappings || [],
      dataCopyMethod: (params.dataCopyMethod || 'cold') as SaveAsTemplateInput['dataCopyMethod'],
      dataCopyStartTime: selectedMigrationOptions.dataCopyStartTime
        ? params.dataCopyStartTime
        : undefined,
      storageCopyMethod: params.storageCopyMethod,
      proxyVMRef: params.proxyVMRef,
      cutoverOption: params.cutoverOption || CUTOVER_TYPES.IMMEDIATE,
      disconnectSourceNetwork: params.disconnectSourceNetwork || false,
      fallbackToDHCP: params.fallbackToDHCP || false,
      securityGroups: params.securityGroups || [],
      serverGroup: params.serverGroup || '',
      firstBootScript: selectedMigrationOptions.postMigrationScript
        ? params.postMigrationScript
        : undefined,
      networkPersistence: params.networkPersistence,
      removeVMwareTools: params.removeVMwareTools,
      imageProfiles: params.imageProfiles || [],
      periodicSyncInterval: params.periodicSyncInterval,
      periodicSyncEnabled: selectedMigrationOptions.periodicSyncEnabled,
      acknowledgeNetworkConflictRisk: params.acknowledgeNetworkConflictRisk,
      postMigrationAction: selectedMigrationOptions.postMigrationAction
        ? params.postMigrationAction
        : undefined,
      osFamily: params.osFamily,
      useGPU: params.useGPU || false
    }),
    [
      vmwareCredentials,
      openstackCredentials,
      params.vmwareCreds,
      params.openstackCreds,
      params.networkMappings,
      params.storageMappings,
      params.arrayCredsMappings,
      params.dataCopyMethod,
      params.dataCopyStartTime,
      params.storageCopyMethod,
      params.proxyVMRef,
      params.cutoverOption,
      params.disconnectSourceNetwork,
      params.fallbackToDHCP,
      params.securityGroups,
      params.serverGroup,
      params.postMigrationScript,
      params.networkPersistence,
      params.removeVMwareTools,
      params.imageProfiles,
      params.periodicSyncInterval,
      params.acknowledgeNetworkConflictRisk,
      params.postMigrationAction,
      params.osFamily,
      params.useGPU,
      selectedMigrationOptions.postMigrationScript,
      selectedMigrationOptions.periodicSyncEnabled,
      selectedMigrationOptions.postMigrationAction,
      selectedMigrationOptions.dataCopyStartTime,
      targetPCDClusterName,
      selectedVmwareClusterName
    ]
  )

  const [selectedFlavorId, setSelectedFlavorId] = useState('')
  useEffect(() => {
    setSelectedFlavorId(retryVm?.targetFlavorId || '')
  }, [retryVm])

  // Resolve selected PCD cluster name from id for template patching.
  const selectedPcdClusterName = useMemo(
    () => pcdData.find((p) => p.id === params.pcdCluster)?.name || params.pcdCluster || '',
    [pcdData, params.pcdCluster]
  )

  // Re-resolve pcdCluster from name → id once pcdData finishes loading.
  // useRetryPrefill runs at drawer open; if pcdData isn't ready yet it stores
  // the cluster name string. Once pcdData loads, swap it for the real id so
  // the target-cluster dropdown selects the right item.
  useEffect(() => {
    if (!isRetryMode || !pcdData.length || !params.pcdCluster) return
    const alreadyId = pcdData.some((p) => p.id === params.pcdCluster)
    if (alreadyId) return
    const match = pcdData.find((p) => p.name === params.pcdCluster)
    if (match) updateParams({ pcdCluster: match.id })
  }, [pcdData, isRetryMode, params.pcdCluster, updateParams])

  // When target cluster changes in retry mode, reset network/storage mappings because
  // the previously mapped networks/volume-types may not exist on the new cluster.
  const handleRetryClusterChange = useCallback(
    (newClusterId: string) => {
      updateParams({ pcdCluster: newClusterId, networkMappings: [], storageMappings: [] })
    },
    [updateParams]
  )

  // Query RDM disks
  const { data: rdmDisks = [] } = useRdmDisksQuery({
    enabled: vmwareCredsValidated && openstackCredsValidated
  })

  const contentRootRef = useRef<HTMLDivElement | null>(null)
  const section1Ref = useRef<HTMLDivElement | null>(null)
  const section2Ref = useRef<HTMLDivElement | null>(null)
  const section3Ref = useRef<HTMLDivElement | null>(null)
  const section4Ref = useRef<HTMLDivElement | null>(null)
  const section5Ref = useRef<HTMLDivElement | null>(null)
  const reviewRef = useRef<HTMLDivElement | null>(null)
  const [activeSectionId, setActiveSectionId] = useState<string>('source-destination')

  const [touchedSections, setTouchedSections] = useState({
    options: false
  })

  const markTouched = useCallback(
    (key: keyof typeof touchedSections) => {
      setTouchedSections((prev) => (prev[key] ? prev : { ...prev, [key]: true }))
    },
    [setTouchedSections]
  )

  useEffect(() => {
    if (!open) return
    setTouchedSections({
      options: false
    })
  }, [open])

  const {
    availableVmwareNetworks,
    availableVmwareDatastores,
    sortedOpenstackNetworks,
    sortedOpenstackVolumeTypes,
    vmValidation,
    rdmValidation,
    networkMappingRequired,
    disableSubmit,
    unmappedNetworksCount,
    unmappedStorageCount,
    sectionNavItems
  } = useFormValidation({
    params,
    fieldErrors,
    selectedMigrationOptions,
    vmwareCredsValidated,
    openstackCredsValidated,
    rdmDisks,
    openstackCredentials,
    touchedSections
  })

  // Subnet compatibility between selected VM IPs and mapped target networks.
  // Shared by the mapping step (per-network warnings) and the options step
  // (persist IP is blocked while any mismatch exists).
  const networkIPsMap = useNetworkIPsMap(params.vms || [])
  const subnetWarnings = useNetworkSubnetCompatibility({
    networkMappings: params.networkMappings,
    openstackCredentials,
    selectedVMs: params.vms || [],
    networkIPsMap,
    openstackNetworks: sortedOpenstackNetworks
  })
  const hasSubnetMismatch = hasAnySubnetMismatch(subnetWarnings)

  const { submitting, handleSubmit, handleClose } = useMigrationFormSubmit({
    params,
    selectedMigrationOptions,
    migrationTemplate,
    vmwareCredentials,
    openstackCredentials,
    setMigrationTemplate,
    setVmwareCredentials,
    setOpenstackCredentials,
    getFieldErrorsUpdater,
    reportError,
    track,
    queryClient,
    navigate,
    onClose,
    onSuccess,
    sessionId,
    networkMappingRequired
  })

  const { retrySubmitting, retryError, handleEditAndRetry } = useRetrySubmit({
    retryConfig,
    params,
    selectedMigrationOptions,
    retryPlan,
    retryTemplate,
    retryVm,
    vmK8sName,
    selectedFlavorId,
    selectedPcdClusterName,
    networkMappingRequired,
    queryClient,
    navigate,
    onClose,
    onSuccess,
    reportError
  })

  // In retry mode the template belongs to the live MigrationPlan — the standard close
  // handler would delete it. Cancelling a retry must not modify anything.
  const handleDrawerClose = isRetryMode ? onClose : () => handleClose()

  const scrollToSection = useCallback((id: string) => {
    const map: Record<string, React.RefObject<HTMLDivElement | null>> = {
      'source-destination': section1Ref,
      'select-vms': section2Ref,
      'map-resources': section3Ref,
      security: section4Ref,
      options: section5Ref
    }

    const el = map[id]?.current
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    setActiveSectionId(id)
  }, [])

  useSectionTracking({
    open,
    contentRootRef,
    sections: [
      { ref: section1Ref, id: 'source-destination' },
      { ref: section2Ref, id: 'select-vms' },
      { ref: section3Ref, id: 'map-resources' },
      { ref: section4Ref, id: 'security' },
      { ref: section5Ref, id: 'options' }
    ],
    setActiveSectionId
  })

  const submitDisabled = disableSubmit || submitting

  const theme = useTheme()
  const isSmallNav = useMediaQuery(theme.breakpoints.down('md'))

  return (
    <>
      <DrawerShell
        data-testid="migration-form-drawer"
        open={open}
        onClose={handleDrawerClose}
        width={drawerWidth}
        ModalProps={{
          keepMounted: false,
          style: { zIndex: 1300 }
        }}
        header={
          <DrawerHeader
            data-testid="migration-form-header"
            closeButtonTestId="migration-form-close"
            title={
              isRetryMode
                ? 'Retry Migration'
                : isEditTemplateMode
                  ? 'Edit Template'
                  : isCreateTemplateMode
                    ? 'Create Template'
                    : 'Start Migration'
            }
            subtitle={
              isRetryMode
                ? `Review and adjust the configuration of "${retryConfig?.vmName}" before retrying`
                : isEditTemplateMode
                  ? `Update the configuration saved in "${templatePrefill?.displayName}"`
                  : isCreateTemplateMode
                    ? 'Configure source/destination and mappings, then save it as a reusable template'
                    : 'Configure source/destination, select VMs, and map resources before starting'
            }
            icon={<MigrationIcon />}
            onClose={handleDrawerClose}
          />
        }
        footer={
          isRetryMode ? (
            <DrawerFooter data-testid="migration-form-footer">
              <ActionButton
                tone="secondary"
                onClick={handleDrawerClose}
                data-testid="migration-form-cancel"
              >
                Cancel
              </ActionButton>
              <ActionButton
                tone="primary"
                onClick={handleEditAndRetry}
                disabled={Boolean(blockingError) || prefillLoading || retrySubmitting}
                loading={retrySubmitting}
                data-testid="migration-form-retry"
              >
                Retry
              </ActionButton>
            </DrawerFooter>
          ) : isTemplateMode ? (
            <DrawerFooter data-testid="migration-form-footer">
              <ActionButton
                tone="secondary"
                onClick={handleDrawerClose}
                data-testid="migration-form-cancel"
              >
                Cancel
              </ActionButton>
              <ActionButton
                tone="primary"
                onClick={() => setSaveTemplateOpen(true)}
                disabled={!canSaveAsTemplate}
                data-testid="migration-form-save-template-mode"
              >
                {isEditTemplateMode ? 'Save Changes' : 'Create Template'}
              </ActionButton>
            </DrawerFooter>
          ) : (
            <DrawerFooter data-testid="migration-form-footer">
              <ActionButton
                tone="secondary"
                onClick={() => setSaveTemplateOpen(true)}
                disabled={!canSaveAsTemplate}
                sx={{ mr: 'auto' }}
                data-testid="migration-form-save-template"
              >
                Save as template
              </ActionButton>
              <ActionButton
                tone="secondary"
                onClick={() => handleClose()}
                data-testid="migration-form-cancel"
              >
                Cancel
              </ActionButton>
              <ActionButton
                tone="primary"
                onClick={handleSubmit}
                disabled={submitDisabled}
                loading={submitting}
                data-testid="migration-form-submit"
              >
                Start Migration
              </ActionButton>
            </DrawerFooter>
          )
        }
      >
        <DesignSystemForm
          form={form}
          onSubmit={async () => {
            if (isRetryMode) {
              await handleEditAndRetry()
              return
            }
            if (isTemplateMode) {
              setSaveTemplateOpen(true)
              return
            }
            await handleSubmit()
          }}
          keyboardSubmitProps={{
            open,
            onClose: handleDrawerClose,
            isSubmitDisabled: isRetryMode
              ? Boolean(blockingError) || prefillLoading || retrySubmitting
              : isTemplateMode
                ? !canSaveAsTemplate
                : disableSubmit || submitting
          }}
        >
          {isRetryMode && blockingError ? (
            <Box data-testid="retry-blocking-banner" sx={{ mb: 2 }}>
              <Banner
                variant="error"
                title="This migration cannot be retried"
                message={blockingError}
              />
            </Box>
          ) : null}
          {isRetryMode && retryError ? (
            <Box data-testid="retry-error-banner" sx={{ mb: 2 }}>
              <Banner variant="error" title="Retry failed" message={retryError} />
            </Box>
          ) : null}
          {isRetryMode && prefillLoading ? (
            <Box data-testid="retry-prefill-loading-banner" sx={{ mb: 2 }}>
              <Banner variant="info" message="Loading the failed migration's configuration…" />
            </Box>
          ) : null}
          <Box
            ref={contentRootRef}
            data-testid="migration-form-content"
            sx={{
              display: 'grid',
              gridTemplateColumns: isSmallNav ? '1fr' : '56px 1fr',
              gap: 3
            }}
          >
            {!isSmallNav ? (
              <SectionNav
                data-testid="migration-form-section-nav"
                items={sectionNavItems}
                activeId={activeSectionId}
                onSelect={scrollToSection}
                dense
                showDescriptions={false}
              />
            ) : null}

            <Box sx={{ display: 'grid', gap: 3 }}>
              {isSmallNav ? (
                <SurfaceCard
                  title="Steps"
                  subtitle="Jump to any section"
                  data-testid="migration-form-steps-card"
                >
                  <NavTabs
                    value={activeSectionId}
                    onChange={(_e, value) => scrollToSection(value as string)}
                    data-testid="migration-form-steps-tabs"
                  >
                    {sectionNavItems.map((item) => (
                      <NavTab
                        key={item.id}
                        value={item.id}
                        label={item.title}
                        description={item.description}
                        data-testid={`migration-form-steps-tab-${item.id}`}
                      />
                    ))}
                  </NavTabs>
                </SurfaceCard>
              ) : null}

              {/* Step 1 */}
              <Box ref={section1Ref} data-testid="migration-form-step-source-destination">
                <SurfaceCard
                  variant="section"
                  title="Source And Destination"
                  subtitle={
                    isRetryMode
                      ? 'Locked to the failed migration’s environments'
                      : 'Choose where you migrate from and where you migrate to'
                  }
                  data-testid="migration-form-step1-card"
                >
                  {isRetryMode ? (
                    <RetrySourceDestinationSummary
                      vmwareCredName={params.vmwareCreds?.existingCredName}
                      sourceCluster={sourceCluster}
                      openstackCredName={params.openstackCreds?.existingCredName}
                      pcdClusters={pcdData}
                      selectedPcdClusterId={params.pcdCluster || ''}
                      onPcdClusterChange={handleRetryClusterChange}
                      disabled={prefillLoading || retrySubmitting}
                    />
                  ) : (
                    <SourceDestinationClusterSelection
                      onChange={getParamsUpdater}
                      errors={fieldErrors}
                      vmwareCluster={params.vmwareCluster}
                      pcdCluster={params.pcdCluster}
                      showHeader={false}
                    />
                  )}
                </SurfaceCard>
              </Box>

              <Divider />

              {/* Step 2 - VM selection now manages its own data fetching with unique session ID */}
              <Box ref={section2Ref} data-testid="migration-form-step-select-vms">
                <SurfaceCard
                  variant="section"
                  title={isRetryMode ? 'Virtual Machine' : 'Select VMs'}
                  subtitle={
                    isRetryMode
                      ? 'The failed VM is locked for this retry'
                      : 'Pick the virtual machines you want to migrate'
                  }
                  data-testid="migration-form-step2-card"
                >
                  {isRetryMode ? (
                    <VmsSelectionStep
                      mode="standard"
                      onChange={getParamsUpdater}
                      error={fieldErrors['vms']}
                      open={open}
                      vmwareCredsValidated={vmwareCredsValidated}
                      openstackCredsValidated={openstackCredsValidated}
                      sessionId={sessionId}
                      openstackFlavors={openstackCredentials?.spec?.flavors}
                      vmwareCredName={params.vmwareCreds?.existingCredName}
                      openstackCredName={params.openstackCreds?.existingCredName}
                      openstackCredentials={openstackCredentials}
                      vmwareCluster={params.vmwareCluster}
                      useGPU={params.useGPU}
                      showHeader={false}
                      retryVmName={retryConfig?.vmName}
                      retryPrefillVm={params.vms?.[0]}
                    />
                  ) : (
                    <>
                      <VmsSelectionStep
                        mode="standard"
                        onChange={getParamsUpdater}
                        error={fieldErrors['vms']}
                        open={open}
                        vmwareCredsValidated={vmwareCredsValidated}
                        openstackCredsValidated={openstackCredsValidated}
                        sessionId={sessionId}
                        openstackFlavors={openstackCredentials?.spec?.flavors}
                        vmwareCredName={params.vmwareCreds?.existingCredName}
                        openstackCredName={params.openstackCreds?.existingCredName}
                        openstackCredentials={openstackCredentials}
                        vmwareCluster={params.vmwareCluster}
                        useGPU={params.useGPU}
                        showHeader={false}
                      />
                      {vmValidation.hasError && (
                        <Alert severity="warning">{vmValidation.errorMessage}</Alert>
                      )}
                      {rdmValidation.hasConfigError && (
                        <Alert severity="error">{rdmValidation.configErrorMessage}</Alert>
                      )}
                    </>
                  )}
                </SurfaceCard>
              </Box>
              <Divider />

              {/* Step 3 */}
              <Box ref={section3Ref} data-testid="migration-form-step-map-resources">
                <SurfaceCard
                  variant="section"
                  title="Map Networks And Storage"
                  subtitle="Ensure all VMware networks and datastores have PCD targets"
                  data-testid="migration-form-step3-card"
                >
                  <NetworkAndStorageMappingStep
                    vmwareNetworks={availableVmwareNetworks}
                    vmWareStorage={availableVmwareDatastores}
                    openstackNetworks={sortedOpenstackNetworks}
                    openstackStorage={sortedOpenstackVolumeTypes}
                    params={params}
                    onChange={getParamsUpdater}
                    networkMappingError={fieldErrors['networksMapping']}
                    storageMappingError={fieldErrors['storageMapping']}
                    showHeader={false}
                    subnetWarnings={subnetWarnings}
                  />
                </SurfaceCard>
              </Box>
              <Divider />

              {/* Step 4 */}
              <Box ref={section4Ref} data-testid="migration-form-step-security">
                <SurfaceCard
                  variant="section"
                  title="Security groups, server group & image profiles"
                  subtitle="Optional placement, security settings, and boot volume metadata"
                  data-testid="migration-form-step4-card"
                >
                  <SecurityGroupAndServerGroupStep
                    params={params}
                    onChange={getParamsUpdater}
                    openstackCredentials={openstackCredentials}
                    openstackNetworks={sortedOpenstackNetworks}
                    stepNumber="4"
                    showHeader={false}
                  />
                </SurfaceCard>
              </Box>
              <Divider />

              {/* Step 5 */}
              <Box
                ref={section5Ref}
                data-testid="migration-form-step-options"
                onChangeCapture={() => markTouched('options')}
                onInputCapture={() => markTouched('options')}
                onClickCapture={() => markTouched('options')}
                onKeyDownCapture={() => markTouched('options')}
              >
                <SurfaceCard
                  variant="section"
                  title="Migration Options"
                  subtitle="Optional scheduling, cutover behavior, and advanced settings"
                  data-testid="migration-form-step5-card"
                >
                  <MigrationOptions
                    params={params}
                    onChange={getParamsUpdater}
                    openstackCredentials={openstackCredentials}
                    selectedMigrationOptions={selectedMigrationOptions}
                    updateSelectedMigrationOptions={updateSelectedMigrationOptions}
                    errors={fieldErrors}
                    getErrorsUpdater={getFieldErrorsUpdater}
                    stepNumber="5"
                    showHeader={false}
                    hasSubnetMismatch={hasSubnetMismatch}
                    skipDefaultSeeding={skipDefaultSeeding}
                  />
                </SurfaceCard>
              </Box>
              {!isRetryMode && <Divider />}

              {!isRetryMode && (
                <Box ref={reviewRef} data-testid="migration-form-step-review">
                  <SurfaceCard
                    variant="section"
                    title="Preview"
                    subtitle="Verify your selections before starting the migration"
                    data-testid="migration-form-step6-card"
                  >
                    <Box sx={{ display: 'grid', gap: 1.5 }}>
                      <Typography variant="subtitle2">Summary</Typography>
                      <Divider />

                      <Box sx={{ display: 'grid', gap: 1 }}>
                        <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                          <Typography variant="body2" color="text.secondary">
                            Source
                          </Typography>
                          <Typography variant="body2">{params.vmwareCluster || '—'}</Typography>
                        </Box>

                        <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                          <Typography variant="body2" color="text.secondary">
                            Destination
                          </Typography>
                          <Typography variant="body2">
                            {targetPCDClusterName || params.pcdCluster || '—'}
                          </Typography>
                        </Box>

                        <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                          <Typography variant="body2" color="text.secondary">
                            VMs selected
                          </Typography>
                          <Typography variant="body2">{params.vms?.length || 0}</Typography>
                        </Box>

                        <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                          <Typography variant="body2" color="text.secondary">
                            Network mappings
                          </Typography>
                          <Typography variant="body2">
                            {availableVmwareNetworks.length === 0
                              ? '—'
                              : unmappedNetworksCount === 0
                                ? 'All mapped'
                                : `${unmappedNetworksCount} unmapped`}
                          </Typography>
                        </Box>

                        <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                          <Typography variant="body2" color="text.secondary">
                            Storage mappings
                          </Typography>
                          <Typography variant="body2">
                            {availableVmwareDatastores.length === 0
                              ? '—'
                              : unmappedStorageCount === 0
                                ? 'All mapped'
                                : `${unmappedStorageCount} unmapped`}
                          </Typography>
                        </Box>

                        <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                          <Typography variant="body2" color="text.secondary">
                            Security groups
                          </Typography>
                          <Typography variant="body2">
                            {(params.securityGroups ?? []).length === 0
                              ? '—'
                              : `${(params.securityGroups ?? []).length} selected`}
                          </Typography>
                        </Box>

                        <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                          <Typography variant="body2" color="text.secondary">
                            Server group
                          </Typography>
                          <Typography variant="body2">{params.serverGroup || '—'}</Typography>
                        </Box>
                      </Box>
                    </Box>
                  </SurfaceCard>
                </Box>
              )}
            </Box>
          </Box>
        </DesignSystemForm>
      </DrawerShell>

      <SaveAsTemplateDialog
        open={saveTemplateOpen}
        onClose={() => setSaveTemplateOpen(false)}
        onSaved={isTemplateMode ? () => handleClose({ preserveCredentials: true }) : undefined}
        buildTemplateInput={buildSaveTemplateInput}
        editingTemplate={isEditTemplateMode ? templatePrefill : undefined}
      />
    </>
  )
}
