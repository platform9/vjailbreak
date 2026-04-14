import { Alert, Box, Divider, Typography, useMediaQuery } from '@mui/material'
import MigrationIcon from '@mui/icons-material/SwapHoriz'
import { useTheme } from '@mui/material/styles'
import type { UseFormReturn } from 'react-hook-form'
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
import type { SectionNavItem } from 'src/components'
import { DesignSystemForm } from 'src/shared/components/forms'
import type { OpenstackCreds, PCDNetworkInfo } from 'src/api/openstack-creds/model'
import type { VmData } from 'src/features/migration/api/migration-templates/model'
import SourceDestinationClusterSelection from 'src/features/migration/steps/SourceDestinationClusterSelection'
import VmsSelectionStep from 'src/features/migration/steps/VmsSelectionStep'
import NetworkAndStorageMappingStep from 'src/features/migration/steps/NetworkAndStorageMappingStep'
import SecurityGroupAndServerGroupStep from 'src/features/migration/steps/SecurityGroupAndServerGroup'
import MigrationOptions from 'src/features/migration/MigrationOptionsAlt'
import type {
  FieldErrors,
  FormValues,
  SelectedMigrationOptionsType
} from 'src/features/migration/types'

type VmValidationResult = {
  hasError: boolean
  errorMessage: string
}

type RdmValidationResult = {
  hasConfigError: boolean
  configErrorMessage: string
}

type Props = {
  open: boolean
  drawerWidth: number
  form: UseFormReturn<any, any, any>
  onSubmit: () => Promise<void>
  onClose: () => void

  submitting: boolean
  submitDisabled: boolean

  params: FormValues
  fieldErrors: FieldErrors
  getParamsUpdater: (key: string) => (value: unknown) => void
  getFieldErrorsUpdater: (key: string | number) => (value: string) => void

  selectedMigrationOptions: SelectedMigrationOptionsType
  updateSelectedMigrationOptions: (
    key:
      | keyof SelectedMigrationOptionsType
      | 'postMigrationAction.suffix'
      | 'postMigrationAction.folderName'
  ) => (value: unknown) => void

  vmwareCredsValidated: boolean
  openstackCredsValidated: boolean
  sessionId: string

  openstackCredentials?: OpenstackCreds

  sortedOpenstackNetworks: PCDNetworkInfo[]
  sortedOpenstackVolumeTypes: string[]

  availableVmwareNetworks: string[]
  availableVmwareDatastores: string[]

  sectionNavItems: SectionNavItem[]
  activeSectionId: string
  onSelectSection: (id: string) => void

  contentRootRef: React.RefObject<HTMLDivElement | null>
  section1Ref: React.RefObject<HTMLDivElement | null>
  section2Ref: React.RefObject<HTMLDivElement | null>
  section3Ref: React.RefObject<HTMLDivElement | null>
  section4Ref: React.RefObject<HTMLDivElement | null>
  section5Ref: React.RefObject<HTMLDivElement | null>
  reviewRef: React.RefObject<HTMLDivElement | null>

  markOptionsTouched: () => void

  vmValidation: VmValidationResult
  rdmValidation: RdmValidationResult

  targetPCDClusterName?: string
  unmappedNetworksCount: number
  unmappedStorageCount: number
}

export default function MigrationFormLayout({
  open,
  drawerWidth,
  form,
  onSubmit,
  onClose,
  submitting,
  submitDisabled,
  params,
  fieldErrors,
  getParamsUpdater,
  getFieldErrorsUpdater,
  selectedMigrationOptions,
  updateSelectedMigrationOptions,
  vmwareCredsValidated,
  openstackCredsValidated,
  sessionId,
  openstackCredentials,
  sortedOpenstackNetworks,
  sortedOpenstackVolumeTypes,
  availableVmwareNetworks,
  availableVmwareDatastores,
  sectionNavItems,
  activeSectionId,
  onSelectSection,
  contentRootRef,
  section1Ref,
  section2Ref,
  section3Ref,
  section4Ref,
  section5Ref,
  reviewRef,
  markOptionsTouched,
  vmValidation,
  rdmValidation,
  targetPCDClusterName,
  unmappedNetworksCount,
  unmappedStorageCount
}: Props) {
  const theme = useTheme()
  const isSmallNav = useMediaQuery(theme.breakpoints.down('md'))

  return (
    <DrawerShell
      data-testid="migration-form-drawer"
      open={open}
      onClose={onClose}
      width={drawerWidth}
      ModalProps={{
        keepMounted: false,
        style: { zIndex: 1300 }
      }}
      header={
        <DrawerHeader
          data-testid="migration-form-header"
          title="Start Migration"
          subtitle="Configure source/destination, select VMs, and map resources before starting"
          icon={<MigrationIcon />}
          onClose={onClose}
        />
      }
      footer={
        <DrawerFooter data-testid="migration-form-footer">
          <ActionButton tone="secondary" onClick={onClose} data-testid="migration-form-cancel">
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            onClick={onSubmit}
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
          await onSubmit()
        }}
        keyboardSubmitProps={{
          open,
          onClose,
          isSubmitDisabled: submitDisabled
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
              onSelect={onSelectSection}
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
                  onChange={(_e, value) => onSelectSection(value as string)}
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

            <Box ref={section2Ref} data-testid="migration-form-step-select-vms">
              <SurfaceCard
                variant="section"
                title="Select VMs"
                subtitle="Pick the virtual machines you want to migrate"
                data-testid="migration-form-step2-card"
              >
                <VmsSelectionStep
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
                  selectedVMs={params.vms as VmData[] | undefined}
                  openstackCredentials={openstackCredentials}
                />
              </SurfaceCard>
            </Box>

            <Divider />

            <Box ref={section4Ref} data-testid="migration-form-step-security">
              <SurfaceCard
                variant="section"
                title="Security groups and server group"
                subtitle="Optional placement and security settings"
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

            <Box
              ref={section5Ref}
              data-testid="migration-form-step-options"
              onChangeCapture={markOptionsTouched}
              onInputCapture={markOptionsTouched}
              onClickCapture={markOptionsTouched}
              onKeyDownCapture={markOptionsTouched}
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
