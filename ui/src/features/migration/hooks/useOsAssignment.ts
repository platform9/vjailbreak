import { useState, useEffect } from 'react'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import type { AmplitudeEventName, EventProperties } from 'src/types/amplitude'
import type { ErrorContext } from 'src/services/errorReporting'
import type { VmDataWithFlavor } from '../types'

interface UseOsAssignmentParams {
  vmsWithFlavor: VmDataWithFlavor[]
  setVmsWithFlavor: React.Dispatch<React.SetStateAction<VmDataWithFlavor[]>>
  showToast: (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void
  track: (eventName: AmplitudeEventName, properties?: EventProperties) => void
  reportError: (error: Error, additionalContext?: ErrorContext) => void
}

export function useOsAssignment({
  vmsWithFlavor,
  setVmsWithFlavor,
  showToast,
  track,
  reportError,
}: UseOsAssignmentParams) {
  const [vmOSAssignments, setVmOSAssignments] = useState<Record<string, string>>({})

  useEffect(() => {
    if (Object.keys(vmOSAssignments).length === 0) return
    setVmsWithFlavor((prev) => {
      const updated = prev.map((vm) => {
        if (vmOSAssignments && Object.prototype.hasOwnProperty.call(vmOSAssignments, vm.id)) {
          return { ...vm, osFamily: vmOSAssignments[vm.id] }
        }
        return vm
      })
      return updated
    })
  }, [vmOSAssignments, setVmsWithFlavor])

  const handleOSAssignment = async (vmId: string, osFamily: string) => {
    try {
      setVmOSAssignments((prev) => ({ ...prev, [vmId]: osFamily }))
      const vm = vmsWithFlavor.find((v) => v.id === vmId)
      if (vm?.vmWareMachineName) {
        await patchVMwareMachine(vm.vmWareMachineName, {
          spec: { vms: { osFamily } }
        })
      }
      track('os_family_assigned' as AmplitudeEventName, {
        vm_name: vmId,
        os_family: osFamily,
        action: 'os-family-assignment'
      } as EventProperties)
      showToast(`OS family successfully assigned for VM "${vmId}"`)
    } catch (error) {
      reportError(error as Error, {
        context: 'os-family-assignment',
        metadata: { vmId, osFamily, action: 'os-family-assignment' }
      })
      setVmOSAssignments((prev) => {
        const newState = { ...prev }
        delete newState[vmId]
        return newState
      })
      showToast(
        `Failed to assign OS family for VM "${vmId}": ${
          error instanceof Error ? error.message : 'Unknown error'
        }`,
        'error'
      )
    }
  }

  return { vmOSAssignments, handleOSAssignment }
}
