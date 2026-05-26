import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { GridRowSelectionModel, GridRowParams } from '@mui/x-data-grid'
import type { VmDataWithFlavor, RdmConfiguration } from '../types'

function areSetsEqual(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false
  for (const value of a) {
    if (!b.has(value)) return false
  }
  return true
}

interface UseVmSelectionParams {
  vmsWithFlavor: VmDataWithFlavor[]
  rdmConfigurations: RdmConfiguration[]
  setFormVms: (vms: VmDataWithFlavor[]) => void
  setFormRdmConfigurations: (configs: RdmConfiguration[]) => void
}

export function useVmSelection({
  vmsWithFlavor,
  rdmConfigurations,
  setFormVms,
  setFormRdmConfigurations,
}: UseVmSelectionParams) {
  const [selectedVMs, setSelectedVMs] = useState<Set<string>>(new Set())
  const lastSelectedVmsPayloadRef = useRef<string>('__initial__')
  const lastRdmConfigPayloadRef = useRef<string>('__initial__')

  const syncSelectedVmSelection = useCallback(
    (selectedVmData: VmDataWithFlavor[]) => {
      const payload = JSON.stringify(selectedVmData)
      if (payload === lastSelectedVmsPayloadRef.current) return
      lastSelectedVmsPayloadRef.current = payload
      setFormVms(selectedVmData)
    },
    [setFormVms]
  )

  const syncRdmConfigurations = useCallback(
    (configs: RdmConfiguration[]) => {
      const payload = JSON.stringify(configs)
      if (payload === lastRdmConfigPayloadRef.current) return
      lastRdmConfigPayloadRef.current = payload
      setFormRdmConfigurations(configs)
    },
    [setFormRdmConfigurations]
  )

  // Clean up selection when VM list changes
  useEffect(() => {
    if (vmsWithFlavor.length === 0) return

    const availableVmIds = new Set(vmsWithFlavor.map((vm) => vm.id))
    const cleanedSelection = new Set(
      Array.from(selectedVMs).filter((vmId) => availableVmIds.has(vmId))
    )

    if (!areSetsEqual(cleanedSelection, selectedVMs)) {
      setSelectedVMs(cleanedSelection)

      const selectedVmData = vmsWithFlavor.filter((vm) => cleanedSelection.has(vm.id))
      syncSelectedVmSelection(selectedVmData)

      if (rdmConfigurations.length > 0) {
        syncRdmConfigurations(rdmConfigurations)
      }
    }
  }, [vmsWithFlavor, selectedVMs, rdmConfigurations, syncSelectedVmSelection, syncRdmConfigurations])

  // Sync selection to form on any change
  useEffect(() => {
    const selectedVmData = vmsWithFlavor.filter((vm) => selectedVMs.has(vm.id))
    syncSelectedVmSelection(selectedVmData)

    if (selectedVmData.length > 0 && rdmConfigurations.length > 0) {
      syncRdmConfigurations(rdmConfigurations)
    }
  }, [vmsWithFlavor, selectedVMs, rdmConfigurations, syncSelectedVmSelection, syncRdmConfigurations])

  const handleVmSelection = (selectedRowIds: GridRowSelectionModel) => {
    const newSelection = new Set<string>(selectedRowIds as string[])

    if (areSetsEqual(newSelection, selectedVMs)) {
      return
    }

    setSelectedVMs(newSelection)

    const selectedVmData = vmsWithFlavor.filter((vm) => newSelection.has(vm.id))
    syncSelectedVmSelection(selectedVmData)

    if (rdmConfigurations.length > 0) {
      syncRdmConfigurations(rdmConfigurations)
    }
  }

  const isRowSelectable = (params: GridRowParams) => {
    return !params.row.isMigrated
  }

  const rowSelectionModelArray = useMemo(
    () => Array.from(selectedVMs).filter((vmId) => vmsWithFlavor.some((vm) => vm.id === vmId)),
    [selectedVMs, vmsWithFlavor]
  )

  return { selectedVMs, setSelectedVMs, handleVmSelection, isRowSelectable, rowSelectionModelArray }
}
