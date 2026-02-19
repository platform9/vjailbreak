import { useMemo } from 'react'
import { VmData } from 'src/features/migration/api/migration-templates/model'
import { RdmDisk } from 'src/api/rdm-disks/model'

interface RdmConfigValidationProps {
  selectedVMs: VmData[]
  rdmDisks: RdmDisk[]
  backendVolumeTypeMap?: { [key: string]: string }
}

const emptyBackendVolumeTypeMap: { [key: string]: string } = {}

interface RdmConfigValidationResult {
  hasValidationError: boolean
  errorMessage: string
  hasRdmVMs: boolean
  hasPowerStateError: boolean
  powerStateErrorMessage: string
  hasConfigError: boolean
  configErrorMessage: string
  hasVolumeTypeError: boolean
  volumeTypeErrorMessage: string
  hasSelectionError: boolean
  selectionErrorMessage: string
  missingVMs: string[]
  rdmGroups: Record<string, unknown>
  requiredVMs: string[]
  invalidRdmDisks: Array<{
    diskName: string
    ownerVMs: string[]
    missingFields: string[]
    hasPoweredOnVMs: boolean
    poweredOnVMs: string[]
    incompatibleVolumeType?: {
      selectedType: string
      expectedType: string
      backendPool: string
    }
  }>
}

const emptyResult: RdmConfigValidationResult = {
  hasValidationError: false,
  errorMessage: '',
  hasRdmVMs: false,
  hasPowerStateError: false,
  powerStateErrorMessage: '',
  hasConfigError: false,
  configErrorMessage: '',
  hasVolumeTypeError: false,
  volumeTypeErrorMessage: '',
  hasSelectionError: false,
  selectionErrorMessage: '',
  missingVMs: [],
  rdmGroups: {},
  requiredVMs: [],
  invalidRdmDisks: []
}

export const useRdmConfigValidation = ({
  selectedVMs,
  rdmDisks,
  backendVolumeTypeMap = emptyBackendVolumeTypeMap
}: RdmConfigValidationProps): RdmConfigValidationResult => {
  const validationResult = useMemo(() => {
    // If no VMs are selected or no RDM disks exist, no validation errors
    if (selectedVMs.length === 0 || rdmDisks.length === 0) {
      return emptyResult
    }

    // Get selected VM names
    const selectedVmNames = new Set(selectedVMs.map((vm) => vm.name))

    // Check if any selected VM has RDM disks
    const vmsWithRdm = selectedVMs.filter((vm) => vm.hasSharedRdm)
    const hasRdmVMs = vmsWithRdm.length > 0

    if (!hasRdmVMs) {
      return emptyResult
    }

    // Power state validation
    const poweredOnRdmVMs = vmsWithRdm.filter((vm) => {
      if (!vm.vmState) return false
      const powerState = vm.vmState.toLowerCase()
      return powerState === 'running' || powerState === 'poweredon' || powerState === 'on'
    })

    const hasPoweredOnVMs = poweredOnRdmVMs.length > 0
    let powerStateErrorMessage = ''

    if (hasPoweredOnVMs) {
      const poweredOnVmNames = poweredOnRdmVMs.map((vm) => vm.name).join(', ')
      powerStateErrorMessage = `All VMs with shared RDM disks must be powered off for migration. Currently powered on: ${poweredOnVmNames}`
    }

    // Check each RDM disk that has selected VMs as owners for configuration issues
    const invalidRdmDisks: RdmConfigValidationResult['invalidRdmDisks'] = []

    rdmDisks.forEach((rdmDisk) => {
      // Check if this RDM disk has any selected VMs as owners
      const relevantOwnerVMs = rdmDisk.spec.ownerVMs.filter((ownerVM) =>
        selectedVmNames.has(ownerVM)
      )

      if (relevantOwnerVMs.length === 0) {
        // This RDM disk doesn't affect selected VMs
        return
      }

      // Check for missing required fields
      const missingFields: string[] = []

      if (!rdmDisk.spec.openstackVolumeRef?.cinderBackendPool) {
        missingFields.push('cinderBackendPool')
      }

      if (!rdmDisk.spec.openstackVolumeRef?.volumeType) {
        missingFields.push('volumeType')
      }

      // Check if volumeRef (source) is missing or empty
      if (
        !rdmDisk.spec.openstackVolumeRef?.source ||
        Object.keys(rdmDisk.spec.openstackVolumeRef.source).length === 0
      ) {
        missingFields.push('volumeRef')
      }

      // Check if any owner VMs are powered on
      const poweredOnVMs = relevantOwnerVMs.filter((vmName) => {
        const vm = selectedVMs.find((v) => v.name === vmName)
        if (!vm) return false

        const powerState = vm.vmState?.toLowerCase()
        return powerState === 'running' || powerState === 'poweredon' || powerState === 'on'
      })

      // Check volume type compatibility with selected backend
      let incompatibleVolumeType:
        | { selectedType: string; expectedType: string; backendPool: string }
        | undefined
      const backendPool = rdmDisk.spec.openstackVolumeRef?.cinderBackendPool
      const volumeType = rdmDisk.spec.openstackVolumeRef?.volumeType

      if (backendPool && volumeType && backendVolumeTypeMap[backendPool]) {
        const expectedType = backendVolumeTypeMap[backendPool]
        if (expectedType !== volumeType) {
          incompatibleVolumeType = {
            selectedType: volumeType,
            expectedType,
            backendPool
          }
        }
      }

      // If there are missing fields or incompatible volume type, add to validation results
      if (missingFields.length > 0 || incompatibleVolumeType) {
        invalidRdmDisks.push({
          diskName: rdmDisk.spec.diskName,
          ownerVMs: relevantOwnerVMs,
          missingFields,
          hasPoweredOnVMs: poweredOnVMs.length > 0,
          poweredOnVMs,
          incompatibleVolumeType
        })
      }
    })

    // Generate configuration error message if there are validation errors
    let configErrorMessage = ''
    const disksWithMissingFields = invalidRdmDisks.filter((d) => d.missingFields.length > 0)
    const hasConfigError = disksWithMissingFields.length > 0

    if (hasConfigError) {
      const allMissingFields = Array.from(
        new Set(disksWithMissingFields.flatMap((disk) => disk.missingFields))
      )

      const allDiskNames = disksWithMissingFields.map((disk) => disk.diskName).join(', ')
      configErrorMessage = `Cannot submit migration plan: RDM disk${disksWithMissingFields.length > 1 ? 's' : ''} (${allDiskNames}) ${disksWithMissingFields.length > 1 ? 'require' : 'requires'} configuration (${allMissingFields.join(', ')}). Please configure the RDM disk${disksWithMissingFields.length > 1 ? 's' : ''} before proceeding.`
    }

    // Generate volume type error message
    let volumeTypeErrorMessage = ''
    const disksWithVolumeTypeError = invalidRdmDisks.filter((d) => d.incompatibleVolumeType)
    const hasVolumeTypeError = disksWithVolumeTypeError.length > 0

    if (hasVolumeTypeError) {
      const messages = disksWithVolumeTypeError.map((disk) => {
        const vt = disk.incompatibleVolumeType
        if (!vt) return ''
        return `"${disk.diskName}" has volume type "${vt.selectedType}" but backend "${vt.backendPool}" expects "${vt.expectedType}"`
      }).filter(Boolean)
      volumeTypeErrorMessage = `Incompatible volume type mapping: ${messages.join('; ')}.`
    }

    // Combined validation - any error prevents submission
    const hasValidationError = hasPoweredOnVMs || hasConfigError || hasVolumeTypeError

    // Priority: power state > volume type > config
    let errorMessage = ''
    if (hasPoweredOnVMs) {
      errorMessage = powerStateErrorMessage
    } else if (hasVolumeTypeError) {
      errorMessage = volumeTypeErrorMessage
    } else if (hasConfigError) {
      errorMessage = configErrorMessage
    }

    return {
      hasValidationError,
      errorMessage,
      hasRdmVMs,
      hasPowerStateError: hasPoweredOnVMs,
      powerStateErrorMessage,
      hasConfigError,
      configErrorMessage,
      hasVolumeTypeError,
      volumeTypeErrorMessage,
      hasSelectionError: false,
      selectionErrorMessage: '',
      missingVMs: [],
      rdmGroups: {},
      requiredVMs: [],
      invalidRdmDisks
    }
  }, [selectedVMs, rdmDisks, backendVolumeTypeMap])

  return validationResult
}
