import { Box, Alert, Divider, Typography, useMediaQuery } from '@mui/material'
import { ReactNode, useCallback, useEffect, useRef, useState } from 'react'
import { useTheme } from '@mui/material/styles'
import { useForm } from 'react-hook-form'
import { DesignSystemForm } from 'src/shared/components/forms'
import { NavTab, NavTabs, SectionNav, SurfaceCard } from 'src/components'
import { useRdmDisksQuery } from 'src/hooks/api/useRdmDisksQuery'
import useParams from 'src/hooks/useParams'
import type { MigrationTemplate } from 'src/features/migration/api/migration-templates/model'
import type { VMwareCreds } from 'src/api/vmware-creds/model'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import MigrationOptions from '../steps/MigrationOptionsAlt'
import NetworkAndStorageMappingStep from '../steps/NetworkAndStorageMappingStep'
import SecurityGroupAndServerGroupStep from '../steps/SecurityGroupAndServerGroup'
import SourceDestinationClusterSelection from '../steps/SourceDestinationClusterSelection'
import VmsSelectionStep from '../steps/VmsSelectionStep'
import { useClusterData } from '../hooks/useClusterData'
import { useFormValidation } from '../hooks/useFormValidation'
import { useFormSync } from '../hooks/useFormSync'
import { useCredentialFetching } from '../hooks/useCredentialFetching'
import { useSectionTracking } from '../hooks/useSectionTracking'
import type {
  FormValues,
  SelectedMigrationOptionsType,
  FieldErrors,
  MigrationDrawerRHFValues
} from '../types'

/** Default checkbox state for the optional migration options. */
export const defaultMigrationOptions: SelectedMigrationOptionsType = {
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

/** Snapshot of the form's internal state, lifted to the parent for submit/footer wiring. */
export interface MigrationConfigState {
  params: Partial<FormValues>
  selectedMigrationOptions: SelectedMigrationOptionsType
  fieldErrors: FieldErrors
  getFieldErrorsUpdater: (key: string) => (value: string) => void
  migrationTemplate: MigrationTemplate | undefined
  vmwareCredentials: VMwareCreds | undefined
  openstackCredentials: OpenstackCreds | undefined
  setMigrationTemplate: React.Dispatch<React.SetStateAction<MigrationTemplate | undefined>>
  setVmwareCredentials: React.Dispatch<React.SetStateAction<VMwareCreds | undefined>>
  setOpenstackCredentials: React.Dispatch<React.SetStateAction<OpenstackCreds | undefined>>
  targetPCDClusterName: string | undefined
  disableSubmit: boolean
  networkMappingRequired: boolean
}

export interface MigrationConfigFormProps {
  open: boolean
  /** Unique session id for VM discovery (shared with the parent's submit/cleanup). */
  sessionId: string
  /** Initial params to seed the form (e.g. a bucket's saved config). */
  seed?: Partial<FormValues>
  /** Initial option checkboxes. */
  seedOptions?: SelectedMigrationOptionsType
  /** VM names to pre-select once the VM list loads (bucket editor). */
  initialSelectedVmNames?: string[]
  /**
   * Bucket-editor only: auto-fill source cluster (from the first selected VM), destination
   * cluster (first PCD), and network/storage mappings (first source → first target) from live
   * data, when those values are missing/unresolved. The "Start Migration" flow leaves this off.
   */
  autoDefaults?: boolean
  /** Enter-key / form submit handler. */
  onSubmit?: () => void | Promise<void>
  /** Close handler (used by keyboard submit). */
  onClose?: () => void
  /** Whether submit is disabled (drives the keyboard submit guard). */
  submitDisabled?: boolean
  /** Emits the latest internal state so the parent can wire its footer + submit. */
  onStateChange?: (state: MigrationConfigState) => void
  /** Render-prop: receives the form content; parent wraps it with drawer chrome. */
  children: (content: ReactNode) => ReactNode
}

/**
 * The shared migration configuration form — the step stack + all wiring extracted from
 * MigrationForm so it can be reused verbatim by both the "Start Migration" drawer and the
 * bucket editor. It renders the steps and emits its state; parents own the drawer, footer,
 * and the submit action (create migration vs. save bucket).
 */
export default function MigrationConfigForm({
  open,
  sessionId,
  seed,
  seedOptions,
  initialSelectedVmNames,
  autoDefaults = false,
  onSubmit,
  onClose,
  submitDisabled = false,
  onStateChange,
  children
}: MigrationConfigFormProps) {
  const { params, getParamsUpdater, setParams } = useParams<FormValues>((seed ?? {}) as FormValues)
  const { pcdData, sourceData } = useClusterData()
  const { params: fieldErrors, getParamsUpdater: getFieldErrorsUpdater } = useParams<FieldErrors>({})
  const {
    params: selectedMigrationOptions,
    getParamsUpdater: updateSelectedMigrationOptions,
    setParams: setSelectedOptions
  } = useParams<SelectedMigrationOptionsType>(seedOptions ?? defaultMigrationOptions)

  const form = useForm<MigrationDrawerRHFValues>({
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
    openstackCredentials,
    migrationTemplate,
    setMigrationTemplate,
    setVmwareCredentials,
    setOpenstackCredentials,
    vmwareCredentials,
    vmwareCredsValidated,
    openstackCredsValidated,
    targetPCDClusterName
  } = useCredentialFetching({ params, pcdData, getFieldErrorsUpdater })

  const { data: rdmDisks = [] } = useRdmDisksQuery({
    enabled: vmwareCredsValidated && openstackCredsValidated
  })

  // Re-seed when the form (re)opens, so editing a bucket shows its saved config.
  const wasOpen = useRef(false)
  useEffect(() => {
    if (open && !wasOpen.current) {
      setParams((seed ?? {}) as FormValues)
      setSelectedOptions(seedOptions ?? defaultMigrationOptions)
      form.reset({
        securityGroups: seed?.securityGroups ?? [],
        serverGroup: seed?.serverGroup ?? '',
        dataCopyStartTime: seed?.dataCopyStartTime ?? '',
        cutoverStartTime: seed?.cutoverStartTime ?? '',
        cutoverEndTime: seed?.cutoverEndTime ?? '',
        postMigrationActionSuffix: seed?.postMigrationAction?.suffix ?? '',
        postMigrationActionFolderName: seed?.postMigrationAction?.folderName ?? ''
      })
    }
    wasOpen.current = open
  }, [open, seed, seedOptions, setParams, setSelectedOptions, form])

  const contentRootRef = useRef<HTMLDivElement | null>(null)
  const section1Ref = useRef<HTMLDivElement | null>(null)
  const section2Ref = useRef<HTMLDivElement | null>(null)
  const section3Ref = useRef<HTMLDivElement | null>(null)
  const section4Ref = useRef<HTMLDivElement | null>(null)
  const section5Ref = useRef<HTMLDivElement | null>(null)
  const reviewRef = useRef<HTMLDivElement | null>(null)
  const [activeSectionId, setActiveSectionId] = useState<string>('source-destination')

  const [touchedSections, setTouchedSections] = useState({ options: false })
  const markTouched = useCallback((key: keyof typeof touchedSections) => {
    setTouchedSections((prev) => (prev[key] ? prev : { ...prev, [key]: true }))
  }, [])

  useEffect(() => {
    if (!open) return
    setTouchedSections({ options: false })
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

  const theme = useTheme()
  const isSmallNav = useMediaQuery(theme.breakpoints.down('md'))

  // Bucket-editor auto-defaults: resolve source cluster (from the first selected VM),
  // destination cluster (first PCD), and network/storage mappings (each source → first target)
  // from live data, only filling values that are missing or unresolved. Each runs once.
  const clusterDefaultedRef = useRef(false)
  const pcdDefaultedRef = useRef(false)
  const netDefaultedRef = useRef(false)
  const storageDefaultedRef = useRef(false)

  useEffect(() => {
    if (!autoDefaults) return

    // Source cluster ← the datacenter's "NO CLUSTER" pseudo-cluster, which surfaces every VM so
    // a cross-cluster bucket can select all its VMs. Only set when missing/unresolved.
    if (!clusterDefaultedRef.current && sourceData.length > 0) {
      const validIds = new Set(sourceData.flatMap((s) => s.clusters.map((c) => c.id)))
      if (!params.vmwareCluster || !validIds.has(params.vmwareCluster)) {
        let noClusterId: string | undefined
        for (const s of sourceData) {
          const noC = s.clusters.find(
            (c) =>
              c.name.toLowerCase().startsWith('no-cluster-') ||
              c.displayName?.toUpperCase() === 'NO CLUSTER'
          )
          if (noC) {
            noClusterId = noC.id
            break
          }
        }
        const resolvedId = noClusterId ?? sourceData[0]?.clusters[0]?.id
        if (resolvedId) {
          clusterDefaultedRef.current = true
          getParamsUpdater('vmwareCluster')(resolvedId)
        }
      } else {
        clusterDefaultedRef.current = true
      }
    }

    // Destination ← first PCD cluster.
    if (!pcdDefaultedRef.current && pcdData.length > 0) {
      const validPcd = new Set(pcdData.map((p) => p.id))
      if (!params.pcdCluster || !validPcd.has(params.pcdCluster)) {
        pcdDefaultedRef.current = true
        getParamsUpdater('pcdCluster')(pcdData[0].id)
      } else {
        pcdDefaultedRef.current = true
      }
    }

    // Network mappings ← each source network → first PCD network.
    if (
      !netDefaultedRef.current &&
      availableVmwareNetworks.length > 0 &&
      sortedOpenstackNetworks.length > 0
    ) {
      if (!params.networkMappings || params.networkMappings.length === 0) {
        const target = sortedOpenstackNetworks[0].name
        netDefaultedRef.current = true
        getParamsUpdater('networkMappings')(
          availableVmwareNetworks.map((source) => ({ source, target }))
        )
      } else {
        netDefaultedRef.current = true
      }
    }

    // Storage mappings ← each source datastore → first PCD volume type.
    if (
      !storageDefaultedRef.current &&
      availableVmwareDatastores.length > 0 &&
      sortedOpenstackVolumeTypes.length > 0
    ) {
      if (!params.storageMappings || params.storageMappings.length === 0) {
        const target = sortedOpenstackVolumeTypes[0]
        storageDefaultedRef.current = true
        getParamsUpdater('storageMappings')(
          availableVmwareDatastores.map((source) => ({ source, target }))
        )
      } else {
        storageDefaultedRef.current = true
      }
    }
  }, [
    autoDefaults,
    params.vms,
    params.vmwareCluster,
    params.pcdCluster,
    params.networkMappings,
    params.storageMappings,
    sourceData,
    pcdData,
    availableVmwareNetworks,
    availableVmwareDatastores,
    sortedOpenstackNetworks,
    sortedOpenstackVolumeTypes,
    getParamsUpdater
  ])

  // Emit state upward for the parent's footer + submit.
  useEffect(() => {
    onStateChange?.({
      params,
      selectedMigrationOptions,
      fieldErrors,
      getFieldErrorsUpdater,
      migrationTemplate,
      vmwareCredentials,
      openstackCredentials,
      setMigrationTemplate,
      setVmwareCredentials,
      setOpenstackCredentials,
      targetPCDClusterName,
      disableSubmit,
      networkMappingRequired
    })
  }, [
    onStateChange,
    params,
    selectedMigrationOptions,
    fieldErrors,
    getFieldErrorsUpdater,
    migrationTemplate,
    vmwareCredentials,
    openstackCredentials,
    setMigrationTemplate,
    setVmwareCredentials,
    setOpenstackCredentials,
    targetPCDClusterName,
    disableSubmit,
    networkMappingRequired
  ])

  const content = (
    <DesignSystemForm
      form={form}
      onSubmit={async () => {
        await onSubmit?.()
      }}
      keyboardSubmitProps={{ open, onClose: onClose ?? (() => {}), isSubmitDisabled: submitDisabled }}
    >
      <Box
        ref={contentRootRef}
        data-testid="migration-config-content"
        sx={{ display: 'grid', gridTemplateColumns: isSmallNav ? '1fr' : '56px 1fr', gap: 3 }}
      >
        {!isSmallNav ? (
          <SectionNav
            items={sectionNavItems}
            activeId={activeSectionId}
            onSelect={scrollToSection}
            dense
            showDescriptions={false}
          />
        ) : null}

        <Box sx={{ display: 'grid', gap: 3 }}>
          {isSmallNav ? (
            <SurfaceCard title="Steps" subtitle="Jump to any section">
              <NavTabs
                value={activeSectionId}
                onChange={(_e, value) => scrollToSection(value as string)}
              >
                {sectionNavItems.map((item) => (
                  <NavTab
                    key={item.id}
                    value={item.id}
                    label={item.title}
                    description={item.description}
                  />
                ))}
              </NavTabs>
            </SurfaceCard>
          ) : null}

          <Box ref={section1Ref}>
            <SurfaceCard
              variant="section"
              title="Source And Destination"
              subtitle="Choose where you migrate from and where you migrate to"
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

          <Box ref={section2Ref}>
            <SurfaceCard
              variant="section"
              title="Select VMs"
              subtitle="Pick the virtual machines you want to migrate"
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
                initialSelectedVmNames={initialSelectedVmNames}
              />
              {vmValidation.hasError && <Alert severity="warning">{vmValidation.errorMessage}</Alert>}
              {rdmValidation.hasConfigError && (
                <Alert severity="error">{rdmValidation.configErrorMessage}</Alert>
              )}
            </SurfaceCard>
          </Box>

          <Divider />

          <Box ref={section3Ref}>
            <SurfaceCard
              variant="section"
              title="Map Networks And Storage"
              subtitle="Ensure all VMware networks and datastores have PCD targets"
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

          <Box ref={section4Ref}>
            <SurfaceCard
              variant="section"
              title="Security groups, server group & image profiles"
              subtitle="Optional placement, security settings, and boot volume metadata"
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
            onChangeCapture={() => markTouched('options')}
            onInputCapture={() => markTouched('options')}
            onClickCapture={() => markTouched('options')}
            onKeyDownCapture={() => markTouched('options')}
          >
            <SurfaceCard
              variant="section"
              title="Migration Options"
              subtitle="Optional scheduling, cutover behavior, and advanced settings"
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

          <Box ref={reviewRef}>
            <SurfaceCard
              variant="section"
              title="Preview"
              subtitle="Verify your selections before saving"
            >
              <Box sx={{ display: 'grid', gap: 1 }}>
                <PreviewRow label="Source" value={params.vmwareCluster || '—'} />
                <PreviewRow label="Destination" value={targetPCDClusterName || params.pcdCluster || '—'} />
                <PreviewRow label="VMs selected" value={String(params.vms?.length || 0)} />
                <PreviewRow
                  label="Network mappings"
                  value={
                    availableVmwareNetworks.length === 0
                      ? '—'
                      : unmappedNetworksCount === 0
                        ? 'All mapped'
                        : `${unmappedNetworksCount} unmapped`
                  }
                />
                <PreviewRow
                  label="Storage mappings"
                  value={
                    availableVmwareDatastores.length === 0
                      ? '—'
                      : unmappedStorageCount === 0
                        ? 'All mapped'
                        : `${unmappedStorageCount} unmapped`
                  }
                />
                <PreviewRow
                  label="Security groups"
                  value={
                    (params.securityGroups ?? []).length === 0
                      ? '—'
                      : `${(params.securityGroups ?? []).length} selected`
                  }
                />
                <PreviewRow label="Server group" value={params.serverGroup || '—'} />
              </Box>
            </SurfaceCard>
          </Box>
        </Box>
      </Box>
    </DesignSystemForm>
  )

  return <>{children(content)}</>
}

function PreviewRow({ label, value }: { label: string; value: string }) {
  return (
    <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2">{value}</Typography>
    </Box>
  )
}
