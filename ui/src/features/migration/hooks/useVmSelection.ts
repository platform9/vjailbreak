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
      if (payload === lastSelectedVmsPayloadRef.current) {
        return
      }
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
    // Grid not loaded yet — there is nothing to sync, and pushing an empty selection would
    // wipe a form that was seeded with VMs (e.g. the bucket editor), momentarily emptying the
    // derived VMware networks/datastores and making the mapping step render blank.
    if (vmsWithFlavor.length === 0) return
    // Bucket editor: a pre-selection is still pending (names not yet matched to grid rows).
    // Don't clobber the seeded VMs — and their networks/datastores — with an empty selection,
    // or the saved network/storage mappings appear to "vanish" on reopen until the grid catches up.
    if (initialSelectedVmNames && initialSelectedVmNames.length > 0 && !seededRef.current) return

    const selectedVmData = vmsWithFlavor.filter((vm) => selectedVMs.has(vm.id))
    syncSelectedVmSelection(selectedVmData)

    if (selectedVmData.length > 0 && rdmConfigurations.length > 0) {
      syncRdmConfigurations(rdmConfigurations)
    }
  }, [
    vmsWithFlavor,
    selectedVMs,
    rdmConfigurations,
    syncSelectedVmSelection,
    syncRdmConfigurations,
    initialSelectedVmNames
  ])

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
