import {
  Box,
  Paper,
  Alert,
  Select,
  MenuItem,
  GlobalStyles,
  useMediaQuery,
  Divider,
  Typography
} from '@mui/material'
import { ActionButton } from 'src/components'
import ClusterIcon from '@mui/icons-material/Hub'
import React, { useState, useMemo, useEffect, useCallback } from 'react'
import { DataGrid, GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { useNavigate } from 'react-router-dom'
import { useKeyboardSubmit } from 'src/hooks/ui/useKeyboardSubmit'
import { CustomSearchToolbar } from 'src/components/grid'
import MaasConfigDetailDialog from '../components/MaasConfigDetailDialog'
import MaasConfigDetailsModal from '../components/MaasConfigDetailsModal'
import HostConfigAssignmentDialog from '../components/HostConfigAssignmentDialog'
import NetworkAndStorageMappingStep from '../steps/NetworkAndStorageMappingStep'
import RollingVmsSelectionStep from '../steps/RollingVmsSelectionStep'
import SecurityGroupAndServerGroupStep from '../steps/SecurityGroupAndServerGroup'
import SourceDestinationClusterSelection from '../steps/SourceDestinationClusterSelection'
import useParams from 'src/hooks/useParams'
import MigrationOptions from '../steps/MigrationOptionsAlt'
import WarningIcon from '@mui/icons-material/Warning'
import { useClusterData } from '../hooks/useClusterData'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useHostConfigHandlers } from '../hooks/useHostConfigHandlers'
import { useRollingFormData } from '../hooks/useRollingFormData'
import { useRollingFormValidation } from '../hooks/useRollingFormValidation'
import { useRollingFormSubmit } from '../hooks/useRollingFormSubmit'
import { useSectionTracking } from '../hooks/useSectionTracking'
import { useRollingFormSync } from '../hooks/useRollingFormSync'

// Import CDS icons
import '@cds/core/icon/register.js'
import { ClarityIcons, buildingIcon, clusterIcon, hostIcon, vmIcon } from '@cds/core/icon'
import { useAmplitude } from 'src/hooks/useAmplitude'

import { DrawerShell, DrawerHeader, DrawerFooter, SectionNav, SurfaceCard } from 'src/components'
import { styled, useTheme } from '@mui/material/styles'
import { FormProvider, useForm } from 'react-hook-form'
import type {
  RollingMigrationRHFValues,
  RollingMigrationFormDrawerProps,
  FieldErrors,
  RollingFormParams
} from '../types'

// RollingMigration includes osFamily which the shared SelectedMigrationOptionsType does not
export interface SelectedMigrationOptionsType extends Record<string, unknown> {
  dataCopyMethod: boolean
  dataCopyStartTime: boolean
  cutoverOption: boolean
  cutoverStartTime: boolean
  cutoverEndTime: boolean
  postMigrationScript: boolean
  osFamily: boolean
  useGPU?: boolean
  useFlavorless?: boolean
  postMigrationAction?: {
    suffix?: boolean
    folderName?: boolean
    renameVm?: boolean
    moveToFolder?: boolean
  }
}

// Default state for checkboxes
const defaultMigrationOptions: SelectedMigrationOptionsType = {
  dataCopyMethod: false,
  dataCopyStartTime: false,
  cutoverOption: false,
  cutoverStartTime: false,
  cutoverEndTime: false,
  postMigrationScript: false,
  osFamily: false,
  useGPU: false,
  useFlavorless: false,
  postMigrationAction: {
    suffix: false,
    folderName: false,
    renameVm: false,
    moveToFolder: false
  }
}

// Register clarity icons
ClarityIcons.addIcons(buildingIcon, clusterIcon, hostIcon, vmIcon)

// Style for Clarity icons
const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center'
})

const drawerWidth = 1400

const CustomESXToolbarWithActions = (props) => {
  const { onAssignHostConfig, ...toolbarProps } = props

  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: 'flex-end',
        width: '100%',
        padding: '4px 8px',
        gap: 1
      }}
    >
      <ActionButton variant="text" color="primary" onClick={onAssignHostConfig} size="small">
        Assign Host Config
      </ActionButton>
      <CustomSearchToolbar {...toolbarProps} />
    </Box>
  )
}

export default function RollingMigrationFormDrawer({
  open,
  onClose
}: RollingMigrationFormDrawerProps) {
  const navigate = useNavigate()
  const { reportError } = useErrorHandler({ component: 'RollingMigrationForm' })
  const { track } = useAmplitude({ component: 'RollingMigrationForm' })
  const [submitting, setSubmitting] = useState(false)
  const [selectedVMs, setSelectedVMs] = useState<GridRowSelectionModel>([])
  const [maasConfigDialogOpen, setMaasConfigDialogOpen] = useState(false)
  const [maasDetailsModalOpen, setMaasDetailsModalOpen] = useState(false)

  // IP editing and validation state - updated for multiple interfaces
  // IP editing and validation state removed - using bulk assignment instead

  // OS assignment state
  const [vmOSAssignments, setVmOSAssignments] = useState<Record<string, string>>({})

  // Migration Options state
  const { params, getParamsUpdater } = useParams<RollingFormParams>({})
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const getFieldErrorsUpdater = useCallback(
    (key: string | number) => (value: string) => {
      setFieldErrors((prev) => ({ ...prev, [key]: value }))
    },
    []
  )

  const rhfForm = useForm<RollingMigrationRHFValues, any, RollingMigrationRHFValues>({
    defaultValues: {
      securityGroups: [],
      serverGroup: '',
      dataCopyStartTime: params.dataCopyStartTime ?? '',
      cutoverStartTime: params.cutoverStartTime ?? '',
      cutoverEndTime: params.cutoverEndTime ?? '',
      postMigrationActionSuffix: (params as any)?.postMigrationAction?.suffix ?? '',
      postMigrationActionFolderName: (params as any)?.postMigrationAction?.folderName ?? ''
    }
  })
  const { params: selectedMigrationOptions, getParamsUpdater: updateSelectedMigrationOptions } =
    useParams<SelectedMigrationOptionsType>(defaultMigrationOptions)

  const { sourceData, pcdData } = useClusterData()

  const {
    loadingHosts,
    loadingVMs,
    orderedESXHosts,
    setOrderedESXHosts,
    vmsWithAssignments,
    setVmsWithAssignments,
    maasConfigs,
    selectedMaasConfig,
    loadingMaasConfig,
    openstackCredData,
    loadingOpenstackDetails,
    selectedVMwareCredName,
    selectedPcdCredName,
    fetchClusterVMs
  } = useRollingFormData({
    open,
    vmwareCluster: params.vmwareCluster ?? '',
    pcdCluster: params.pcdCluster ?? '',
    sourceData,
    pcdData,
    selectedVMs,
    setSelectedVMs
  })

  const paginationModel = { page: 0, pageSize: 5 }

  // Clear selection when component is closed
  useEffect(() => {
    if (!open) {
      setSelectedVMs([])
    }
  }, [open])

  useRollingFormSync({
    form: rhfForm,
    params,
    getParamsUpdater,
    selectedMigrationOptions
  })

  const handleCloseMaasConfig = () => {
    setMaasConfigDialogOpen(false)
  }

  useEffect(() => {
    if (params.vmwareCluster || params.pcdCluster) {
      markTouched('sourceDestination')
    }
  }, [params.vmwareCluster, params.pcdCluster])

  useEffect(() => {
    if ((params.securityGroups ?? []).length > 0 || params.serverGroup) {
      markTouched('security')
    }
  }, [params.securityGroups, params.serverGroup])

  const availableVmwareNetworks = useMemo(() => {
    if (!vmsWithAssignments.length || !selectedVMs.length) return []

    const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))

    const extractedNetworks = selectedVMsData
      .filter((vm) => vm.networks)
      .flatMap((vm) => vm.networks || [])

    if (extractedNetworks.length > 0) {
      return extractedNetworks.sort() // Remove Array.from(new Set()) to keep duplicates
    }
    return []
  }, [vmsWithAssignments, selectedVMs])

  const availableVmwareDatastores = useMemo(() => {
    if (!vmsWithAssignments.length || !selectedVMs.length) return []

    const selectedVMsData = vmsWithAssignments.filter((vm) => selectedVMs.includes(vm.id))

    const extractedDatastores = selectedVMsData
      .filter((vm) => vm.datastores)
      .flatMap((vm) => vm.datastores || [])

    if (extractedDatastores.length > 0) {
      return Array.from(new Set(extractedDatastores)).sort()
    }
    return []
  }, [vmsWithAssignments, selectedVMs])

  // Define ESX columns inside component to access state and functions
  const esxColumns: GridColDef[] = [
    {
      field: 'name',
      headerName: 'ESX Name',
      flex: 2,
      renderCell: (params) => (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <CdsIconWrapper>
            {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
            {/* @ts-ignore */}
            <cds-icon shape="host" size="md" badge="info"></cds-icon>
          </CdsIconWrapper>
          {params.value}
        </Box>
      )
    },
    {
      field: 'vms',
      headerName: 'VM Count',
      flex: 0.5,
      renderCell: (params) => (
        <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
          <Typography variant="body2">{params.value}</Typography>
        </Box>
      )
    },
    {
      field: 'state',
      headerName: 'State',
      flex: 0.8,
      renderCell: (params) => {
        const state = params.value || 'Unknown'
        let color = 'text.secondary'
        if (state === 'connected') color = 'success.main'
        if (state === 'disconnected' || state === 'notResponding') color = 'error.main'
        if (state === 'maintenance') color = 'warning.main'

        return (
          <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
            <Typography variant="body2" sx={{ color, textTransform: 'capitalize' }}>
              {state}
            </Typography>
          </Box>
        )
      }
    },
    {
      field: 'pcdHostConfigName',
      headerName: 'Host Config',
      flex: 1,
      renderCell: (params) => {
        const hostId = params.row.id
        const currentConfig = params.value || ''

        return (
          <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
            <Select
              size="small"
              value={currentConfig}
              onChange={(e) => handleIndividualHostConfigChange(hostId, e.target.value)}
              displayEmpty
              sx={{
                width: 250,
                '& .MuiSelect-select': {
                  padding: '4px 8px',
                  fontSize: '0.875rem'
                }
              }}
              renderValue={(selected) => {
                if (!selected) {
                  return (
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 1,
                        color: 'text.secondary'
                      }}
                    >
                      <WarningIcon sx={{ fontSize: 16 }} />
                      <em>Select Host Config</em>
                    </Box>
                  )
                }
                return <Typography variant="body2">{selected}</Typography>
              }}
            >
              <MenuItem value="">
                <Box
                  sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}
                >
                  <WarningIcon sx={{ fontSize: 16 }} />
                  <em>Select Host Config</em>
                </Box>
              </MenuItem>
              {(openstackCredData?.spec?.pcdHostConfig || []).map((config) => (
                <MenuItem key={config.id} value={config.name}>
                  <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                    <Typography variant="body2">{config.name}</Typography>
                    <Typography variant="caption" color="text.secondary">
                      {config.mgmtInterface}
                    </Typography>
                  </Box>
                </MenuItem>
              ))}
            </Select>
          </Box>
        )
      }
    }
  ]

  const openstackNetworks = useMemo(() => {
    if (!openstackCredData) return []

    const networks = openstackCredData?.status?.openstack?.networks || []
    return networks.sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()))
  }, [openstackCredData])

  const openstackVolumeTypes = useMemo(() => {
    if (!openstackCredData) return []

    const volumeTypes = openstackCredData?.status?.openstack?.volumeTypes || []
    return volumeTypes.sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()))
  }, [openstackCredData])

  // Update ESXi host config names when OpenStack host configs become available
  useEffect(() => {
    const pcdHostConfigs = openstackCredData?.spec?.pcdHostConfig || []
    if (pcdHostConfigs.length === 0) return

    if (orderedESXHosts.length === 0) return

    const needsUpdate = orderedESXHosts.some((host) => {
      if (!host.pcdHostConfigId) return false
      const configObj = pcdHostConfigs.find((c) => c.id === host.pcdHostConfigId)
      if (!configObj) return false
      return host.pcdHostConfigName !== configObj.name
    })

    if (!needsUpdate) return

    setOrderedESXHosts((prevHosts) => {
      if (prevHosts.length === 0) return prevHosts

      const updatedHosts = prevHosts.map((host) => {
        if (!host.pcdHostConfigId) return host

        const configObj = pcdHostConfigs.find((c) => c.id === host.pcdHostConfigId)
        if (!configObj) return host

        if (host.pcdHostConfigName !== configObj.name) {
          return { ...host, pcdHostConfigName: configObj.name }
        }

        return host
      })

      const hasChanges = updatedHosts.some(
        (host, index) => host.pcdHostConfigName !== prevHosts[index]?.pcdHostConfigName
      )

      return hasChanges ? updatedHosts : prevHosts
    })
  }, [openstackCredData, orderedESXHosts])

  const theme = useTheme()
  const isSmallNav = useMediaQuery(theme.breakpoints.down('md'))
  const [activeSectionId, setActiveSectionId] = useState<string>('source-destination')

  const [touchedSections, setTouchedSections] = useState({
    sourceDestination: false,
    baremetal: false,
    hosts: false,
    vms: false,
    mapResources: false,
    security: false,
    options: false
  })

  const markTouched = useCallback(
    (key: keyof typeof touchedSections) => {
      setTouchedSections((prev) => (prev[key] ? prev : { ...prev, [key]: true }))
    },
    [setTouchedSections]
  )

  const {
    vmIpValidationError,
    esxHostConfigValidationError,
    osValidationError,
    networkMappingError,
    storageMappingError,
    setNetworkMappingError,
    setStorageMappingError,
    esxHostMappingStatus,
    isSubmitDisabled,
    sectionNavItems
  } = useRollingFormValidation({
    selectedVMs,
    vmsWithAssignments,
    orderedESXHosts,
    vmOSAssignments,
    selectedMaasConfig,
    submitting,
    availableVmwareNetworks,
    availableVmwareDatastores,
    selectedMigrationOptions,
    params,
    fieldErrors,
    touchedSections
  })

  const { handleSubmit, handleClose } = useRollingFormSubmit({
    selectedVMs,
    vmsWithAssignments,
    selectedMaasConfig,
    orderedESXHosts,
    openstackCredData,
    sourceData,
    pcdData,
    availableVmwareNetworks,
    availableVmwareDatastores,
    params,
    selectedMigrationOptions,
    selectedVMwareCredName,
    selectedPcdCredName,
    submitting,
    setSubmitting,
    onClose,
    navigate,
    track,
    reportError,
    setNetworkMappingError,
    setStorageMappingError
  })

  useKeyboardSubmit({
    open,
    isSubmitDisabled: isSubmitDisabled,
    onSubmit: handleSubmit,
    onClose: handleClose
  })

  const {
    pcdHostConfigDialogOpen,
    selectedPcdHostConfig,
    updatingPcdMapping,
    handleOpenPcdHostConfigDialog,
    handleClosePcdHostConfigDialog,
    handlePcdHostConfigChange,
    handleApplyPcdHostConfig,
    handleIndividualHostConfigChange
  } = useHostConfigHandlers({
    orderedESXHosts,
    setOrderedESXHosts,
    openstackCredData,
    markTouched,
    reportError
  })

  const handleMappingsChange = useCallback(
    (key: string) => (value: unknown) => {
      markTouched('mapResources')
      ;(getParamsUpdater as any)(key)(value)
    },
    [getParamsUpdater, markTouched]
  )

  useEffect(() => {
    if (!open) return
    setTouchedSections({
      sourceDestination: false,
      baremetal: false,
      hosts: false,
      vms: false,
      mapResources: false,
      security: false,
      options: false
    })
  }, [open])

  const contentRootRef = React.useRef<HTMLDivElement | null>(null)
  const sourceDestRef = React.useRef<HTMLDivElement | null>(null)
  const baremetalRef = React.useRef<HTMLDivElement | null>(null)
  const hostsRef = React.useRef<HTMLDivElement | null>(null)
  const vmsRef = React.useRef<HTMLDivElement | null>(null)
  const mapResourcesRef = React.useRef<HTMLDivElement | null>(null)
  const securityRef = React.useRef<HTMLDivElement | null>(null)
  const optionsRef = React.useRef<HTMLDivElement | null>(null)
  const previewRef = React.useRef<HTMLDivElement | null>(null)

  useSectionTracking({
    open,
    contentRootRef,
    sections: [
      { ref: sourceDestRef, id: 'source-destination' },
      { ref: baremetalRef, id: 'baremetal' },
      { ref: hostsRef, id: 'hosts' },
      { ref: vmsRef, id: 'vms' },
      { ref: mapResourcesRef, id: 'map-resources' },
      { ref: securityRef, id: 'security' },
      { ref: optionsRef, id: 'options' }
    ],
    setActiveSectionId
  })

  const scrollToSection = useCallback((id: string) => {
    const map: Record<string, React.RefObject<HTMLDivElement | null>> = {
      'source-destination': sourceDestRef,
      baremetal: baremetalRef,
      hosts: hostsRef,
      vms: vmsRef,
      'map-resources': mapResourcesRef,
      security: securityRef,
      options: optionsRef
    }

    const el = map[id]?.current
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    setActiveSectionId(id)
  }, [])

  const uniqueVmwareNetworks = useMemo(() => {
    return Array.from(new Set(availableVmwareNetworks))
  }, [availableVmwareNetworks])

  const unmappedNetworksCount = useMemo(() => {
    return uniqueVmwareNetworks.filter(
      (n) => !(params.networkMappings ?? []).some((m) => m.source === n)
    ).length
  }, [uniqueVmwareNetworks, params.networkMappings])

  const unmappedStorageCount = useMemo(() => {
    return availableVmwareDatastores.filter(
      (d) => !(params.storageMappings ?? []).some((m) => m.source === d)
    ).length
  }, [availableVmwareDatastores, params.storageMappings])

  const handleViewMaasConfig = () => {
    markTouched('baremetal')
    setMaasDetailsModalOpen(true)
  }

  const handleCloseMaasDetailsModal = () => {
    setMaasDetailsModalOpen(false)
  }

  return (
    <>
      <GlobalStyles
        styles={{
          '.MuiDataGrid-columnsManagement, .MuiDataGrid-columnsManagementPopover': {
            '& .MuiFormControlLabel-label': {
              fontSize: '0.875rem !important'
            },
            '& .MuiCheckbox-root': {
              padding: '4px !important'
            },
            '& .MuiListItem-root': {
              fontSize: '0.875rem !important',
              minHeight: '32px !important',
              padding: '2px 8px !important'
            },
            '& .MuiTypography-root': {
              fontSize: '0.875rem !important'
            },
            '& .MuiInputBase-input': {
              fontSize: '0.875rem !important'
            },
            '& .MuiTextField-root .MuiInputBase-input': {
              fontSize: '0.875rem !important'
            }
          }
        }}
      />
      <FormProvider {...rhfForm}>
        <DrawerShell
          data-testid="rolling-migration-form-drawer"
          open={open}
          onClose={handleClose}
          width={drawerWidth}
          ModalProps={{
            keepMounted: false,
            style: { zIndex: 1300 }
          }}
          header={
            <DrawerHeader
              data-testid="rolling-migration-form-header"
              icon={<ClusterIcon />}
              title="Rolling Cluster Conversion"
              subtitle="Configure source/destination, Bare Metal, ESXi Hosts, and map resources before starting"
            />
          }
          footer={
            <DrawerFooter data-testid="rolling-migration-form-footer">
              <ActionButton
                tone="secondary"
                onClick={handleClose}
                disabled={submitting}
                data-testid="rolling-migration-form-cancel"
              >
                Cancel
              </ActionButton>
              <ActionButton
                tone="primary"
                onClick={handleSubmit}
                disabled={isSubmitDisabled}
                loading={submitting}
                data-testid="rolling-migration-form-submit"
              >
                Start Conversion
              </ActionButton>
            </DrawerFooter>
          }
        >
          <Box
            ref={contentRootRef}
            data-testid="rolling-migration-form-content"
            sx={{
              display: 'grid',
              gridTemplateColumns: isSmallNav ? '1fr' : '56px 1fr',
              gap: 3
            }}
          >
            {!isSmallNav ? (
              <SectionNav
                data-testid="rolling-migration-form-section-nav"
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
                  data-testid="rolling-migration-form-steps-card"
                >
                  <Select
                    size="small"
                    value={activeSectionId}
                    onChange={(e) => scrollToSection(e.target.value as string)}
                    fullWidth
                    data-testid="rolling-migration-form-steps-select"
                  >
                    {sectionNavItems.map((item) => (
                      <MenuItem key={item.id} value={item.id}>
                        {item.title}
                      </MenuItem>
                    ))}
                  </Select>
                </SurfaceCard>
              ) : null}

              <Box ref={sourceDestRef} data-testid="rolling-migration-form-step-source-destination">
                <SurfaceCard
                  variant="section"
                  title="Source And Destination"
                  subtitle="Choose where you convert from and where you convert to"
                  data-testid="rolling-migration-form-step1-card"
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

              <Box ref={baremetalRef} data-testid="rolling-migration-form-step-baremetal">
                <SurfaceCard
                  variant="section"
                  title="Bare Metal Config"
                  subtitle="Verify the selected configuration"
                  data-testid="rolling-migration-form-step2-card"
                >
                  {loadingMaasConfig ? (
                    <Typography variant="body2">Loading Bare Metal Config...</Typography>
                  ) : maasConfigs.length === 0 ? (
                    <Typography variant="body2">No Bare Metal Config available</Typography>
                  ) : (
                    <Typography
                      variant="subtitle2"
                      component="a"
                      data-testid="rolling-migration-form-baremetal-view-details"
                      sx={{
                        color: 'primary.main',
                        textDecoration: 'underline',
                        cursor: 'pointer',
                        fontWeight: '500'
                      }}
                      onClick={handleViewMaasConfig}
                    >
                      View Bare Metal Config Details
                    </Typography>
                  )}
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={hostsRef} data-testid="rolling-migration-form-step-hosts">
                <SurfaceCard
                  variant="section"
                  title="ESXi Hosts"
                  subtitle="Assign PCD host configurations to all ESXi hosts"
                  data-testid="rolling-migration-form-step3-card"
                >
                  <Box
                    sx={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      mb: 1
                    }}
                  >
                    <Typography variant="body2" color="text.secondary">
                      Select ESXi hosts and assign PCD host configurations
                    </Typography>
                    {esxHostMappingStatus.fullyMapped && esxHostMappingStatus.total > 0 ? (
                      <Typography variant="body2" color="success.main">
                        All hosts mapped ✓
                      </Typography>
                    ) : (
                      <Typography variant="body2" color="warning.main">
                        {esxHostMappingStatus.mapped} of {esxHostMappingStatus.total} hosts unmapped
                      </Typography>
                    )}
                  </Box>
                  <Paper
                    sx={{ width: '100%', height: 389 }}
                    data-testid="rolling-migration-form-hosts-grid"
                  >
                    <DataGrid
                      rows={orderedESXHosts}
                      columns={esxColumns}
                      initialState={{
                        pagination: { paginationModel },
                        columns: {
                          columnVisibilityModel: {}
                        }
                      }}
                      pageSizeOptions={[5, 10, 25]}
                      rowHeight={45}
                      slots={{
                        toolbar: (props) => (
                          <CustomESXToolbarWithActions
                            {...props}
                            onAssignHostConfig={handleOpenPcdHostConfigDialog}
                          />
                        )
                      }}
                      disableColumnMenu
                      disableColumnFilter
                      loading={loadingHosts}
                    />
                  </Paper>
                  {esxHostConfigValidationError && (
                    <Alert severity="warning" sx={{ mt: 2 }}>
                      {esxHostConfigValidationError}
                    </Alert>
                  )}
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={vmsRef} data-testid="rolling-migration-form-step-vms">
                <SurfaceCard
                  variant="section"
                  title="Select VMs"
                  subtitle="Choose the virtual machines to convert and assign required fields"
                  data-testid="rolling-migration-form-step4-card"
                >
                  <RollingVmsSelectionStep
                    vmsWithAssignments={vmsWithAssignments}
                    setVmsWithAssignments={setVmsWithAssignments}
                    selectedVMs={selectedVMs}
                    onSelectionChange={(ids) => {
                      markTouched('vms')
                      setSelectedVMs(ids)
                    }}
                    vmOSAssignments={vmOSAssignments}
                    setVmOSAssignments={setVmOSAssignments}
                    openstackCredData={openstackCredData}
                    loadingVMs={loadingVMs}
                    reportError={reportError}
                    fetchClusterVMs={fetchClusterVMs}
                    vmIpValidationError={vmIpValidationError}
                    osValidationError={osValidationError}
                  />
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={mapResourcesRef} data-testid="rolling-migration-form-step-map-resources">
                <SurfaceCard
                  variant="section"
                  title="Map Networks And Storage"
                  subtitle="Ensure VMware networks/datastores have PCD targets"
                  data-testid="rolling-migration-form-step5-card"
                >
                  {params.vmwareCluster && params.pcdCluster ? (
                    <NetworkAndStorageMappingStep
                      vmwareNetworks={availableVmwareNetworks}
                      vmWareStorage={availableVmwareDatastores}
                      openstackNetworks={openstackNetworks}
                      openstackStorage={openstackVolumeTypes}
                      params={params}
                      onChange={handleMappingsChange}
                      networkMappingError={networkMappingError}
                      storageMappingError={storageMappingError}
                      loading={loadingOpenstackDetails}
                      showHeader={false}
                      selectedVMs={vmsWithAssignments as any}
                      openstackCredentials={openstackCredData || undefined}
                    />
                  ) : (
                    <Typography variant="body2" color="text.secondary">
                      Please select both source cluster and destination PCD to configure mappings.
                    </Typography>
                  )}
                </SurfaceCard>
              </Box>

              <Divider />
              <Box
                ref={securityRef}
                data-testid="rolling-migration-form-step-security"
                onChangeCapture={() => markTouched('security')}
                onInputCapture={() => markTouched('security')}
              >
                <SurfaceCard
                  variant="section"
                  title="Security groups, server group & image profiles"
                  subtitle="Optional placement, security settings, and boot volume metadata"
                  data-testid="rolling-migration-form-step-security-card"
                >
                  <SecurityGroupAndServerGroupStep
                    params={params}
                    onChange={getParamsUpdater}
                    openstackCredentials={openstackCredData || undefined}
                    openstackNetworks={openstackNetworks}
                    stepNumber="7"
                    showHeader={false}
                  />
                </SurfaceCard>
              </Box>

              <Divider />

              <Box
                ref={optionsRef}
                data-testid="rolling-migration-form-step-options"
                onChangeCapture={() => markTouched('options')}
                onInputCapture={() => markTouched('options')}
              >
                <SurfaceCard
                  variant="section"
                  title="Migration Options"
                  subtitle="Optional scheduling, cutover behavior, and advanced settings"
                  data-testid="rolling-migration-form-step6-card"
                >
                  <MigrationOptions
                    stepNumber="6"
                    params={params}
                    onChange={getParamsUpdater}
                    openstackCredentials={openstackCredData || undefined}
                    selectedMigrationOptions={selectedMigrationOptions}
                    updateSelectedMigrationOptions={updateSelectedMigrationOptions}
                    errors={fieldErrors}
                    getErrorsUpdater={getFieldErrorsUpdater}
                    showHeader={false}
                  />
                </SurfaceCard>
              </Box>

              <Divider />

              <Box ref={previewRef} data-testid="rolling-migration-form-step-preview">
                <SurfaceCard
                  variant="section"
                  title="Preview"
                  subtitle="Verify your selections before starting the conversion"
                  data-testid="rolling-migration-form-step7-card"
                >
                  <Box sx={{ display: 'grid', gap: 1.5 }}>
                    <Typography variant="subtitle2">Summary</Typography>
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
                        <Typography variant="body2">{params.pcdCluster || '—'}</Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          Bare metal config
                        </Typography>
                        <Typography variant="body2">
                          {selectedMaasConfig
                            ? (selectedMaasConfig.metadata?.name ?? 'Selected')
                            : '—'}
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          ESXi hosts
                        </Typography>
                        <Typography variant="body2">
                          {esxHostMappingStatus.total === 0
                            ? '—'
                            : esxHostMappingStatus.fullyMapped
                              ? `All ${esxHostMappingStatus.total} mapped`
                              : `${esxHostMappingStatus.mapped} of ${esxHostMappingStatus.total} mapped`}
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          VMs selected
                        </Typography>
                        <Typography variant="body2">{selectedVMs.length}</Typography>
                      </Box>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
                        <Typography variant="body2" color="text.secondary">
                          Network mappings
                        </Typography>
                        <Typography variant="body2">
                          {uniqueVmwareNetworks.length === 0
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
        </DrawerShell>
      </FormProvider>

      <MaasConfigDetailDialog
        open={maasConfigDialogOpen}
        onClose={handleCloseMaasConfig}
        selectedMaasConfig={selectedMaasConfig}
        loadingMaasConfig={loadingMaasConfig}
      />

      {maasConfigs && maasConfigs.length > 0 && (
        <MaasConfigDetailsModal
          open={maasDetailsModalOpen}
          onClose={handleCloseMaasDetailsModal}
          config={maasConfigs[0]}
        />
      )}

      <HostConfigAssignmentDialog
        open={pcdHostConfigDialogOpen}
        onClose={handleClosePcdHostConfigDialog}
        openstackCredData={openstackCredData}
        selectedPcdHostConfig={selectedPcdHostConfig}
        onChange={handlePcdHostConfigChange}
        onApply={handleApplyPcdHostConfig}
        loading={updatingPcdMapping}
      />
    </>
  )
}
