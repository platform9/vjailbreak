import { useEffect, useMemo, useState, useCallback, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { MigrationTemplate } from 'src/features/migration/api/migration-templates/model'
import { MIGRATIONS_QUERY_KEY } from 'src/hooks/api/useMigrationsQuery'
import useParams from 'src/hooks/useParams'
import { CUTOVER_TYPES } from './constants'
import { uniq } from 'ramda'
import { flatten } from 'ramda'
import { useClusterData } from './useClusterData'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useRdmConfigValidation } from 'src/hooks/useRdmConfigValidation'
import { useRdmDisksQuery } from 'src/hooks/api/useRdmDisksQuery'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { useForm, useWatch } from 'react-hook-form'
import {
  getUnmappedCount,
  getDisableSubmit,
  getStepCompletion,
  getAreSelectedMigrationOptionsConfigured,
  getHasAnyMigrationOptionSelected,
  getStep5Complete,
  validateSelectedVmsOsAssigned
} from 'src/features/migration/utils'
import type {
  FieldErrors,
  FormValues,
  SelectedMigrationOptionsType
} from 'src/features/migration/types'
import MigrationFormLayout from 'src/features/migration/components/MigrationFormLayout'
import { useMigrationFormSections } from 'src/features/migration/hooks/useMigrationFormSections'
import type { MigrationFormSectionId } from 'src/features/migration/hooks/useMigrationFormSections'
import type { SectionNavItem } from 'src/components'
import { useMigrationFormRHFParamsSync } from 'src/features/migration/hooks/useMigrationFormRHFParamsSync'
import {
  useExistingOpenstackCredentials,
  useExistingVmwareCredentials
} from 'src/features/migration/hooks/useExistingCredentials'
import { useMigrationTemplateLifecycle } from 'src/features/migration/hooks/useMigrationTemplateLifecycle'
import { useMigrationSubmit } from 'src/features/migration/hooks/useMigrationSubmit'
import { useMigrationResourceCleanup } from 'src/features/migration/hooks/useMigrationResourceCleanup'

const stringsCompareFn = (a: string, b: string) => a.toLowerCase().localeCompare(b.toLowerCase())

const drawerWidth = 1400

type MigrationDrawerRHFValues = {
  securityGroups: string[]
  serverGroup: string
  dataCopyStartTime: string
  cutoverStartTime: string
  cutoverEndTime: string
  postMigrationActionSuffix: string
  postMigrationActionFolderName: string
}

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

interface MigrationFormDrawerProps {
  open: boolean
  onClose: () => void
  reloadMigrations?: () => void
  onSuccess?: (message: string) => void
}

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
  const [, setError] = useState<{ title: string; message: string } | null>(null)
  // Theses are the errors that will be displayed on the form
  const { params: fieldErrors, getParamsUpdater: getFieldErrorsUpdater } = useParams<FieldErrors>(
    {}
  )
  const queryClient = useQueryClient()

  const reviewRef = useRef<HTMLDivElement | null>(null)

  // Migration Options - Checked or Unchecked state
  const { params: selectedMigrationOptions, getParamsUpdater: updateSelectedMigrationOptions } =
    useParams<SelectedMigrationOptionsType>(defaultMigrationOptions)

  // Form Statuses
  const [submitting, setSubmitting] = useState(false)

  // Migration Resources
  const { vmwareCredentials, setVmwareCredentials } = useExistingVmwareCredentials({
    params,
    getFieldErrorsUpdater
  })
  const { openstackCredentials, setOpenstackCredentials } = useExistingOpenstackCredentials({
    params,
    getFieldErrorsUpdater
  })
  const [migrationTemplate, setMigrationTemplate] = useState<MigrationTemplate | undefined>(
    undefined
  )

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

  const rhfSecurityGroups = useWatch({ control: form.control, name: 'securityGroups' })
  const rhfServerGroup = useWatch({ control: form.control, name: 'serverGroup' })
  const rhfDataCopyStartTime = useWatch({ control: form.control, name: 'dataCopyStartTime' })
  const rhfCutoverStartTime = useWatch({ control: form.control, name: 'cutoverStartTime' })
  const rhfCutoverEndTime = useWatch({ control: form.control, name: 'cutoverEndTime' })
  const rhfPostMigrationActionSuffix = useWatch({
    control: form.control,
    name: 'postMigrationActionSuffix'
  })
  const rhfPostMigrationActionFolderName = useWatch({
    control: form.control,
    name: 'postMigrationActionFolderName'
  })

  useMigrationFormRHFParamsSync({
    form,
    params,
    getParamsUpdater,
    selectedMigrationOptions,
    rhfValues: {
      securityGroups: rhfSecurityGroups,
      serverGroup: rhfServerGroup,
      dataCopyStartTime: rhfDataCopyStartTime,
      cutoverStartTime: rhfCutoverStartTime,
      cutoverEndTime: rhfCutoverEndTime,
      postMigrationActionSuffix: rhfPostMigrationActionSuffix,
      postMigrationActionFolderName: rhfPostMigrationActionFolderName
    }
  })

  const vmwareCredsValidated = vmwareCredentials?.status?.vmwareValidationStatus === 'Succeeded'

  const openstackCredsValidated =
    openstackCredentials?.status?.openstackValidationStatus === 'Succeeded'

  // Query RDM disks
  const { data: rdmDisks = [] } = useRdmDisksQuery({
    enabled: vmwareCredsValidated && openstackCredsValidated
  })

  const targetPCDClusterName = useMemo(() => {
    if (!params.pcdCluster) return undefined
    const selectedPCD = pcdData.find((p) => p.id === params.pcdCluster)
    return selectedPCD?.name
  }, [params.pcdCluster, pcdData])

  useMigrationTemplateLifecycle({
    vmwareCredsValidated,
    openstackCredsValidated,
    params,
    vmwareCredentialsName: vmwareCredentials?.metadata?.name,
    openstackCredentialsName: openstackCredentials?.metadata?.name,
    targetPCDClusterName,
    migrationTemplate,
    setMigrationTemplate,
    getFieldErrorsUpdater
  })

  const availableVmwareNetworks = useMemo(() => {
    if (params.vms === undefined) return []
    return uniq(flatten(params.vms.map((vm) => vm.networks || []))).sort(stringsCompareFn) // Back to unique networks only
  }, [params.vms])

  const availableVmwareDatastores = useMemo(() => {
    if (params.vms === undefined) return []
    return uniq(flatten(params.vms.map((vm) => vm.datastores || []))).sort(stringsCompareFn)
  }, [params.vms])

  const { submit: handleSubmit } = useMigrationSubmit({
    params,
    selectedMigrationOptions,
    migrationTemplate,
    queryClient,
    migrationsQueryKey: MIGRATIONS_QUERY_KEY,
    onSuccess,
    onClose,
    navigate,
    setSubmitting,
    setError,
    getFieldErrorsUpdater,
    track,
    reportError
  })

  const migrationOptionValidated = useMemo(() => {
    return Object.keys(selectedMigrationOptions).every((key) => {
      if (key === 'postMigrationAction') {
        // Post-migration actions are optional, so we don't validate them here
        return true
      }
      // TODO - Need to figure out a better way to add validation for periodic sync interval
      if (key === 'periodicSyncEnabled' && selectedMigrationOptions.periodicSyncEnabled) {
        return params?.periodicSyncInterval !== '' && fieldErrors['periodicSyncInterval'] === ''
      }
      if (key === 'dataCopyStartTime' && selectedMigrationOptions.dataCopyStartTime) {
        const value = String(params?.dataCopyStartTime ?? '').trim()
        return value !== '' && !fieldErrors['dataCopyStartTime']
      }
      if (key === 'postMigrationScript' && selectedMigrationOptions.postMigrationScript) {
        const value = String(params?.postMigrationScript ?? '').trim()
        return value !== '' && !fieldErrors['postMigrationScript']
      }
      if (selectedMigrationOptions[key as keyof typeof selectedMigrationOptions]) {
        return params?.[key as keyof typeof params] !== undefined && !fieldErrors[key]
      }
      return true
    })
  }, [selectedMigrationOptions, params, fieldErrors])

  // VM validation - ensure OS is assigned/detected for selected VMs
  const vmValidation = useMemo(() => {
    return validateSelectedVmsOsAssigned(params.vms)
  }, [params.vms])

  // RDM validation - check if RDM disks have missing required configuration
  const rdmValidation = useRdmConfigValidation({
    selectedVMs: params.vms || [],
    rdmDisks: rdmDisks,
    backendVolumeTypeMap: openstackCredentials?.status?.openstack?.backendVolumeTypeMap
  })

  const disableSubmit = getDisableSubmit({
    vmwareCredsValidated,
    openstackCredsValidated,
    params,
    availableVmwareNetworks,
    availableVmwareDatastores,
    fieldErrors,
    migrationOptionValidated,
    vmValidation,
    rdmValidation
  })

  const sortedOpenstackNetworks = useMemo(() => {
    const networks = openstackCredentials?.status?.openstack?.networks || []
    if (!Array.isArray(networks) || networks.length === 0) return []

    return networks
      .filter((n) => n && typeof n.name === 'string')
      .slice()
      .sort((a, b) => stringsCompareFn(a?.name, b?.name))
  }, [openstackCredentials?.status?.openstack?.networks])
  const sortedOpenstackVolumeTypes = useMemo(
    () =>
      (openstackCredentials?.status?.openstack?.volumeTypes || []).slice().sort(stringsCompareFn),
    [openstackCredentials?.status?.openstack?.volumeTypes]
  )

  const { handleClose } = useMigrationResourceCleanup({
    migrationTemplate,
    vmwareCredentials,
    openstackCredentials,
    queryClient,
    sessionId,
    onClose,
    params,
    setMigrationTemplate,
    setVmwareCredentials,
    setOpenstackCredentials,
    setError,
    reportError
  })

  const {
    contentRootRef,
    section1Ref,
    section2Ref,
    section3Ref,
    section4Ref,
    section5Ref,
    activeSectionId,
    scrollToSection
  } = useMigrationFormSections({ open })

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

  const { isStep1Complete, isStep2Complete, isStep3Complete, step4Complete } = useMemo(
    () =>
      getStepCompletion({
        params,
        fieldErrors,
        availableVmwareNetworks,
        availableVmwareDatastores,
        vmValidation,
        rdmValidation
      }),
    [
      params,
      fieldErrors,
      availableVmwareNetworks,
      availableVmwareDatastores,
      vmValidation,
      rdmValidation
    ]
  )

  const unmappedNetworksCount = useMemo(() => {
    return getUnmappedCount(availableVmwareNetworks, params.networkMappings)
  }, [availableVmwareNetworks, params.networkMappings])

  const unmappedStorageCount = useMemo(() => {
    const currentStorageCopyMethod = params.storageCopyMethod || 'normal'
    if (currentStorageCopyMethod === 'StorageAcceleratedCopy') {
      return getUnmappedCount(availableVmwareDatastores, params.arrayCredsMappings)
    }
    return getUnmappedCount(availableVmwareDatastores, params.storageMappings)
  }, [
    availableVmwareDatastores,
    params.storageMappings,
    params.arrayCredsMappings,
    params.storageCopyMethod
  ])

  const step1HasErrors = Boolean(
    fieldErrors['vmwareCluster'] ||
      fieldErrors['pcdCluster'] ||
      fieldErrors['vmwareCreds'] ||
      fieldErrors['openstackCreds']
  )

  const step2HasErrors = Boolean(
    fieldErrors['vms'] || vmValidation.hasError || rdmValidation.hasConfigError
  )

  const step3HasErrors = Boolean(fieldErrors['networksMapping'] || fieldErrors['storageMapping'])

  const step5HasErrors = Boolean(
    (selectedMigrationOptions.dataCopyStartTime && fieldErrors['dataCopyStartTime']) ||
      (selectedMigrationOptions.cutoverOption &&
        (fieldErrors['cutoverOption'] ||
          (params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
            (fieldErrors['cutoverStartTime'] || fieldErrors['cutoverEndTime'])) ||
          (params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED &&
            selectedMigrationOptions.periodicSyncEnabled &&
            fieldErrors['periodicSyncInterval']))) ||
      (selectedMigrationOptions.postMigrationScript && fieldErrors['postMigrationScript'])
  )

  const hasAnyMigrationOptionSelected = useMemo(
    () =>
      getHasAnyMigrationOptionSelected({
        selectedMigrationOptions,
        removeVMwareTools: params.removeVMwareTools
      }),
    [selectedMigrationOptions, params.removeVMwareTools]
  )

  const areSelectedMigrationOptionsConfigured = useMemo(
    () =>
      getAreSelectedMigrationOptionsConfigured({
        hasAnyMigrationOptionSelected,
        selectedMigrationOptions,
        params,
        fieldErrors
      }),
    [hasAnyMigrationOptionSelected, selectedMigrationOptions, params, fieldErrors]
  )

  const step5Complete = useMemo(
    () =>
      getStep5Complete({
        isTouched: touchedSections.options,
        areSelectedMigrationOptionsConfigured,
        params,
        step5HasErrors
      }),
    [touchedSections.options, areSelectedMigrationOptionsConfigured, params, step5HasErrors]
  )

  const sectionNavItems = useMemo<SectionNavItem[]>(
    () => [
      {
        id: 'source-destination',
        title: 'Source And Destination',
        description: 'Pick clusters and credentials',
        status: isStep1Complete ? 'complete' : step1HasErrors ? 'attention' : 'incomplete'
      },
      {
        id: 'select-vms',
        title: 'Select VMs',
        description: 'Choose VMs and assign required fields',
        status: isStep2Complete ? 'complete' : step2HasErrors ? 'attention' : 'incomplete'
      },
      {
        id: 'map-resources',
        title: 'Map Networks And Storage',
        description: 'Map VMware networks/datastores to PCD',
        status: isStep3Complete ? 'complete' : step3HasErrors ? 'attention' : 'incomplete'
      },
      {
        id: 'security',
        title: 'Security And Placement',
        description: 'Security groups and server group',
        status: step4Complete ? 'complete' : 'incomplete'
      },
      {
        id: 'options',
        title: 'Migration Options',
        description: 'Scheduling and advanced behavior',
        status: step5HasErrors ? 'attention' : step5Complete ? 'complete' : 'incomplete'
      }
    ],
    [
      isStep1Complete,
      isStep2Complete,
      isStep3Complete,
      step4Complete,
      step1HasErrors,
      step2HasErrors,
      step3HasErrors,
      step5HasErrors,
      step5Complete
    ]
  )

  const submitDisabled = disableSubmit || submitting

  return (
    <MigrationFormLayout
      open={open}
      drawerWidth={drawerWidth}
      form={form}
      onSubmit={handleSubmit}
      onClose={handleClose}
      submitting={submitting}
      submitDisabled={submitDisabled}
      params={params}
      fieldErrors={fieldErrors}
      getParamsUpdater={getParamsUpdater}
      getFieldErrorsUpdater={getFieldErrorsUpdater}
      selectedMigrationOptions={selectedMigrationOptions}
      updateSelectedMigrationOptions={updateSelectedMigrationOptions}
      vmwareCredsValidated={vmwareCredsValidated}
      openstackCredsValidated={openstackCredsValidated}
      sessionId={sessionId}
      openstackCredentials={openstackCredentials}
      sortedOpenstackNetworks={sortedOpenstackNetworks}
      sortedOpenstackVolumeTypes={sortedOpenstackVolumeTypes}
      availableVmwareNetworks={availableVmwareNetworks}
      availableVmwareDatastores={availableVmwareDatastores}
      sectionNavItems={sectionNavItems}
      activeSectionId={activeSectionId}
      onSelectSection={(id) => scrollToSection(id as MigrationFormSectionId)}
      contentRootRef={contentRootRef}
      section1Ref={section1Ref}
      section2Ref={section2Ref}
      section3Ref={section3Ref}
      section4Ref={section4Ref}
      section5Ref={section5Ref}
      reviewRef={reviewRef}
      markOptionsTouched={() => markTouched('options')}
      vmValidation={vmValidation}
      rdmValidation={rdmValidation}
      targetPCDClusterName={targetPCDClusterName}
      unmappedNetworksCount={unmappedNetworksCount}
      unmappedStorageCount={unmappedStorageCount}
    />
  )
}
