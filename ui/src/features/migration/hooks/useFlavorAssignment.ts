import { useState } from 'react'
import { VmData } from 'src/features/migration/api/migration-templates/model'
import { OpenStackFlavor } from 'src/api/openstack-creds/model'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import type { ErrorContext } from 'src/services/errorReporting'
import type { VmDataWithFlavor } from '../types'

interface UseFlavorAssignmentParams {
  selectedVMs: Set<string>
  vmsWithFlavor: VmDataWithFlavor[]
  setVmsWithFlavor: React.Dispatch<React.SetStateAction<VmDataWithFlavor[]>>
  openstackFlavors: OpenStackFlavor[]
  vmList: VmData[]
  refreshVMList: () => void
  onChange: (id: string) => (value: unknown) => void
  reportError: (error: Error, additionalContext?: ErrorContext) => void
}

export function useFlavorAssignment({
  selectedVMs,
  vmsWithFlavor,
  setVmsWithFlavor,
  openstackFlavors,
  vmList,
  refreshVMList,
  onChange,
  reportError,
}: UseFlavorAssignmentParams) {
  const [flavorDialogOpen, setFlavorDialogOpen] = useState(false)
  const [selectedFlavor, setSelectedFlavor] = useState<string>('')
  const [snackbarOpen, setSnackbarOpen] = useState(false)
  const [snackbarMessage, setSnackbarMessage] = useState('')
  const [snackbarSeverity, setSnackbarSeverity] = useState<'success' | 'error'>('success')
  const [updating, setUpdating] = useState(false)

  const handleOpenFlavorDialog = () => {
    if (selectedVMs.size === 0) return
    setFlavorDialogOpen(true)
  }

  const handleCloseFlavorDialog = () => {
    setFlavorDialogOpen(false)
    setSelectedFlavor('')
  }

  const handleCloseSnackbar = () => {
    setSnackbarOpen(false)
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

      const updatedVms = vmsWithFlavor.map((vm) => {
        if (selectedVMs.has(vm.id)) {
          return {
            ...vm,
            targetFlavorId: isAutoAssign ? '' : selectedFlavor,
            flavorName,
            flavorNotFound: isAutoAssign ? vm.flavorNotFound : false
          }
        }
        return vm
      })
      onChange('vms')(updatedVms.filter((vm) => selectedVMs.has(vm.id)))
      const selectedVmIds = Array.from(selectedVMs)

      const updatePromises = selectedVmIds.map((vmId) => {
        const vmwareMachineName = vmList.find((vm) => vm.id === vmId)?.vmWareMachineName
        const payload = {
          spec: {
            targetFlavorId: isAutoAssign ? '' : selectedFlavor
          }
        }
        if (!vmwareMachineName) {
          return
        }
        return patchVMwareMachine(vmwareMachineName, payload)
      })

      await Promise.all(updatePromises)

      setVmsWithFlavor(updatedVms)
      onChange('vms')(updatedVms.filter((vm) => selectedVMs.has(vm.id)))

      const actionText = isAutoAssign ? 'cleared flavor assignment for' : 'assigned flavor to'
      setSnackbarMessage(
        `Successfully ${actionText} ${selectedVmIds.length} VM${
          selectedVmIds.length > 1 ? 's' : ''
        }`
      )
      setSnackbarSeverity('success')
      setSnackbarOpen(true)

      refreshVMList()
      handleCloseFlavorDialog()
    } catch (error) {
      reportError(error as Error, {
        context: 'vm-flavors-update',
        metadata: {
          selectedVMs: Array.from(selectedVMs),
          selectedFlavor: selectedFlavor,
          isAutoAssign: selectedFlavor === 'auto-assign',
          action: 'vm-flavors-bulk-update'
        }
      })
      setSnackbarMessage('Failed to assign flavor to VMs')
      setSnackbarSeverity('error')
      setSnackbarOpen(true)
    } finally {
      setUpdating(false)
    }
  }

  return {
    flavorDialogOpen,
    selectedFlavor,
    setSelectedFlavor,
    snackbarOpen,
    snackbarMessage,
    snackbarSeverity,
    updating,
    handleOpenFlavorDialog,
    handleCloseFlavorDialog,
    handleCloseSnackbar,
    handleApplyFlavor,
  }
}
