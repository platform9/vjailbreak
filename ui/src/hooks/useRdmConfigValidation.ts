import { useMemo } from "react"
import { VmData } from "src/api/migration-templates/model"
import { RdmDisk } from "src/api/rdm-disks/model"

interface UseRdmConfigValidationProps {
  selectedVMs: VmData[]
  rdmDisks: RdmDisk[]
}

interface RdmConfigValidationResult {
  hasValidationError: boolean
  errorMessage: string
  invalidRdmDisks: Array<{
    diskName: string
    ownerVMs: string[]
    missingFields: string[]
    hasPoweredOnVMs: boolean
    poweredOnVMs: string[]
  }>
}

export const useRdmConfigValidation = ({
  selectedVMs,
  rdmDisks,
}: UseRdmConfigValidationProps): RdmConfigValidationResult => {
  
  const validationResult = useMemo(() => {
    // If no VMs are selected or no RDM disks exist, no validation errors
    if (selectedVMs.length === 0 || rdmDisks.length === 0) {
      return {
        hasValidationError: false,
        errorMessage: "",
        invalidRdmDisks: [],
      }
    }

    // Get selected VM names
    const selectedVmNames = new Set(selectedVMs.map(vm => vm.name))
    
    // Check each RDM disk that has selected VMs as owners
    const invalidRdmDisks: Array<{
      diskName: string
      ownerVMs: string[]
      missingFields: string[]
      hasPoweredOnVMs: boolean
      poweredOnVMs: string[]
    }> = []

    rdmDisks.forEach(rdmDisk => {
      // Check if this RDM disk has any selected VMs as owners
      const relevantOwnerVMs = rdmDisk.spec.ownerVMs.filter(ownerVM => 
        selectedVmNames.has(ownerVM)
      )

      if (relevantOwnerVMs.length === 0) {
        // This RDM disk doesn't affect selected VMs
        return
      }

      // Check for missing required fields
      const missingFields: string[] = []
      
      if (!rdmDisk.spec.openstackVolumeRef?.cinderBackendPool) {
        missingFields.push("cinderBackendPool")
      }
      
      if (!rdmDisk.spec.openstackVolumeRef?.volumeType) {
        missingFields.push("volumeType")
      }
      
      // Check if volumeRef (source) is missing or empty
      if (!rdmDisk.spec.openstackVolumeRef?.source || 
          Object.keys(rdmDisk.spec.openstackVolumeRef.source).length === 0) {
        missingFields.push("volumeRef")
      }

      // Check if any owner VMs are powered on
      const poweredOnVMs = relevantOwnerVMs.filter(vmName => {
        const vm = selectedVMs.find(v => v.name === vmName)
        if (!vm) return false
        
        const powerState = vm.vmState?.toLowerCase()
        return (
          powerState === "running" ||
          powerState === "poweredon" ||
          powerState === "on"
        )
      })

      // If there are missing fields, add to validation results
      if (missingFields.length > 0) {
        invalidRdmDisks.push({
          diskName: rdmDisk.spec.diskName,
          ownerVMs: relevantOwnerVMs,
          missingFields,
          hasPoweredOnVMs: poweredOnVMs.length > 0,
          poweredOnVMs,
        })
      }
    })

    // Generate error message if there are validation errors
    let errorMessage = ""
    
    if (invalidRdmDisks.length > 0) {
      const allMissingFields = Array.from(new Set(
        invalidRdmDisks.flatMap(disk => disk.missingFields)
      ))
      
      const allDiskNames = invalidRdmDisks.map(disk => disk.diskName).join(", ")
      errorMessage = `Cannot submit migration plan: RDM disk${invalidRdmDisks.length > 1 ? 's' : ''} (${allDiskNames}) ${invalidRdmDisks.length > 1 ? 'require' : 'requires'} configuration (${allMissingFields.join(", ")}). Please configure the RDM disk${invalidRdmDisks.length > 1 ? 's' : ''} before proceeding.`
    }

    return {
      hasValidationError: invalidRdmDisks.length > 0,
      errorMessage,
      invalidRdmDisks,
    }
  }, [selectedVMs, rdmDisks])

  return validationResult
}