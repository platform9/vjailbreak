import { Box, Alert, Divider, Typography, useMediaQuery } from '@mui/material'
import MigrationIcon from '@mui/icons-material/SwapHoriz'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState, useCallback } from 'react'
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
import { useFormSync } from '../hooks/useFormSync'
import { useCredentialFetching } from '../hooks/useCredentialFetching'
import { useMigrationFormSubmit } from '../hooks/useMigrationFormSubmit'

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
  useFlavorless: false,
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
  onSuccess
}: MigrationFormDrawerProps) {
  const navigate = useNavigate()
  const { params, getParamsUpdater } = useParams<FormValues>(defaultValues)
  const { pcdData } = useClusterData()
  const { reportError } = useErrorHandler({ component: 'MigrationForm' })
  const { track } = useAmplitude({ component: 'MigrationForm' })
  // Theses are the errors that will be displayed on the form
  const { params: fieldErrors, getParamsUpdater: getFieldErrorsUpdater } = useParams<FieldErrors>(
    {}
  )
  const queryClient = useQueryClient()

  // Migration Options - Checked or Unchecked state
  const { params: selectedMigrationOptions, getParamsUpdater: updateSelectedMigrationOptions } =
    useParams<SelectedMigrationOptionsType>(defaultMigrationOptions)

  // Generate a unique session ID for this form instance
  const [sessionId] = useState(() => `form-session-${Date.now()}`)

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
  } = useCredentialFetching({ params, pcdData, getFieldErrorsUpdater })

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
    <DrawerShell
      data-testid="migration-form-drawer"
      open={open}
      onClose={handleClose}
      width={drawerWidth}
      ModalProps={{
        keepMounted: false,
        style: { zIndex: 1300 }
      }}
      header={
        <DrawerHeader
          data-testid="migration-form-header"
          closeButtonTestId="migration-form-close"
          title="Start Migration"
          subtitle="Configure source/destination, select VMs, and map resources before starting"
          icon={<MigrationIcon />}
          onClose={handleClose}
        />
      }
      footer={
        <DrawerFooter data-testid="migration-form-footer">
          <ActionButton tone="secondary" onClick={handleClose} data-testid="migration-form-cancel">
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
      }
    >
      <DesignSystemForm
        form={form}
        onSubmit={async () => {
          await handleSubmit()
        }}
        keyboardSubmitProps={{
          open,
          onClose: handleClose,
          isSubmitDisabled: disableSubmit || submitting
        }}
      >
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
                subtitle="Choose where you migrate from and where you migrate to"
                data-testid="migration-form-step1-card"
              >
                <SourceDestinationClusterSelection
                  onChange={getParamsUpdater}
                  errors={fieldErrors}
                  vmwareCluster={params.vmwareCluster}
                  pcdCluster={params.pcdCluster}
                  showHeader={false}
                />
              </SurfaceCard>
            </Box>

            <Divider />

            {/* Step 2 - VM selection now manages its own data fetching with unique session ID */}
            <Box ref={section2Ref} data-testid="migration-form-step-select-vms">
              <SurfaceCard
                variant="section"
                title="Select VMs"
                subtitle="Pick the virtual machines you want to migrate"
                data-testid="migration-form-step2-card"
              >
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
                  selectedVMs={params.vms}
                  openstackCredentials={openstackCredentials}
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
                />
              </SurfaceCard>
            </Box>
            <Divider />

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
          </Box>
        </Box>
      </DesignSystemForm>
    </DrawerShell>
  )
}
