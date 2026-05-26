import { useState } from 'react'
import { GridRowSelectionModel } from '@mui/x-data-grid'
import { SelectChangeEvent } from '@mui/material'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import { OpenStackFlavor } from 'src/api/openstack-creds/model'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import type { VM } from '../types'
import type { ErrorContext } from 'src/services/errorReporting'

interface UseFlavorHandlersParams {
  vmsWithAssignments: VM[]
  setVmsWithAssignments: React.Dispatch<React.SetStateAction<VM[]>>
  selectedVMs: GridRowSelectionModel
  openstackFlavors: OpenStackFlavor[]
  reportError: (error: Error, additionalContext?: ErrorContext) => void
  fetchClusterVMs: () => Promise<void>
}

export function useFlavorHandlers({
  vmsWithAssignments,
  setVmsWithAssignments,
  selectedVMs,
  openstackFlavors,
  reportError,
  fetchClusterVMs
}: UseFlavorHandlersParams) {
  const [flavorDialogOpen, setFlavorDialogOpen] = useState(false)
  const [selectedFlavor, setSelectedFlavor] = useState('')
  const [updating, setUpdating] = useState(false)

  const handleOpenFlavorDialog = () => {
    if (selectedVMs.length === 0) return
    setFlavorDialogOpen(true)
  }

  const handleCloseFlavorDialog = () => {
    setFlavorDialogOpen(false)
    setSelectedFlavor('')
  }

  const handleFlavorChange = (event: SelectChangeEvent<string>) => {
    setSelectedFlavor(event.target.value)
  }

  const handleIndividualFlavorChange = async (vmId: string, flavorValue: string) => {
    try {
      const isAutoAssign = flavorValue === 'auto-assign'
      const selectedFlavorObj = !isAutoAssign
        ? openstackFlavors.find((f) => f.id === flavorValue)
        : null
      const flavorName = isAutoAssign
        ? 'auto-assign'
        : selectedFlavorObj
          ? selectedFlavorObj.name
          : flavorValue

      // Update VM via API
      const payload = {
        spec: {
          targetFlavorId: isAutoAssign ? '' : flavorValue
        }
      }

      await patchVMwareMachine(vmId, payload, VJAILBREAK_DEFAULT_NAMESPACE)

      // Update local state
      const updatedVMs = vmsWithAssignments.map((vm) => {
        if (vm.id === vmId) {
          return {
            ...vm,
            flavor: flavorName,
            targetFlavorId: isAutoAssign ? '' : flavorValue
          }
        }
        return vm
      })
      setVmsWithAssignments(updatedVMs)

      console.log(`Successfully assigned flavor "${flavorName}" to VM ${vmId}`)
    } catch (error) {
      console.error(`Failed to update flavor for VM ${vmId}:`, error)
      reportError(error as Error, {
        context: 'individual-vm-flavor-update',
        metadata: {
          vmId: vmId,
          flavorValue: flavorValue,
          isAutoAssign: flavorValue === 'auto-assign',
          action: 'individual-vm-flavor-update'
        }
      })
      alert(
        `Failed to assign flavor to VM: ${error instanceof Error ? error.message : 'Unknown error'}`
      )
    }
  }

  const handleApplyFlavor = async () => {
    if (!selectedFlavor) {
      handleCloseFlavorDialog()
      return
    }

    setUpdating(true)

    try {
      const isAutoAssign = selectedFlavor === 'auto-assign'
      const selectedFlavorObj = !isAutoAssign
        ? openstackFlavors.find((f) => f.id === selectedFlavor)
        : null
      const flavorName = isAutoAssign
        ? 'auto-assign'
        : selectedFlavorObj
          ? selectedFlavorObj.name
          : selectedFlavor

      // Update VMs via API
      const updatePromises = selectedVMs.map(async (vmId) => {
        try {
          const payload = {
            spec: {
              targetFlavorId: isAutoAssign ? '' : selectedFlavor
            }
          }

          await patchVMwareMachine(vmId as string, payload, VJAILBREAK_DEFAULT_NAMESPACE)
          return { success: true, vmId }
        } catch (error) {
          console.error(`Failed to update flavor for VM ${vmId}:`, error)
          return { success: false, vmId, error }
        }
      })

      const results = await Promise.all(updatePromises)
      const failedUpdates = results.filter((result) => !result.success)

      if (failedUpdates.length > 0) {
        console.error(`Failed to update flavor for ${failedUpdates.length} VMs`)
        reportError(new Error(`Failed to update flavor for ${failedUpdates.length} VMs`), {
          context: 'vm-flavor-batch-update-failures',
          metadata: {
            failedUpdates: failedUpdates as unknown as Record<string, unknown>,
            totalVMs: selectedVMs.length,
            successCount: results.length - failedUpdates.length,
            failedCount: failedUpdates.length,
            action: 'vm-flavor-batch-update'
          }
        })
        alert(
          `Failed to assign flavor to ${failedUpdates.length} VM${failedUpdates.length > 1 ? 's' : ''}`
        )
      } else {
        // Update local state only if all API calls succeeded
        const updatedVMs = vmsWithAssignments.map((vm) => {
          if (selectedVMs.includes(vm.id)) {
            return {
              ...vm,
              flavor: flavorName,
              targetFlavorId: isAutoAssign ? '' : selectedFlavor
            }
          }
          return vm
        })
        setVmsWithAssignments(updatedVMs)

        const actionText = isAutoAssign ? 'cleared flavor assignment for' : 'assigned flavor to'
        console.log(
          `Successfully ${actionText} ${selectedVMs.length} VM${selectedVMs.length > 1 ? 's' : ''}`
        )

        // Refresh VM list to get updated flavor information from API
        await fetchClusterVMs()
      }

      handleCloseFlavorDialog()
    } catch (error) {
      console.error('Error updating flavors:', error)
      reportError(error as Error, {
        context: 'vm-flavor-assignment',
        metadata: {
          selectedVMs: selectedVMs as unknown as Record<string, unknown>,
          selectedFlavor: selectedFlavor,
          action: 'vm-flavor-assignment'
        }
      })
      alert('Failed to assign flavor to VMs')
    } finally {
      setUpdating(false)
    }
  }

  return {
    flavorDialogOpen,
    selectedFlavor,
    setSelectedFlavor,
    updating,
    handleOpenFlavorDialog,
    handleCloseFlavorDialog,
    handleFlavorChange,
    handleIndividualFlavorChange,
    handleApplyFlavor
  }
}
