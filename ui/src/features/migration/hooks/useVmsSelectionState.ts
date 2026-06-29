import * as React from 'react'
import { getMigrationPlans } from 'src/features/migration/api/migration-plans/migrationPlans'
import { useVMwareMachinesQuery } from 'src/hooks/api/useVMwareMachinesQuery'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { useRdmConfigValidation } from 'src/hooks/useRdmConfigValidation'
import { VmData, VmNetworkInterface } from 'src/features/migration/api/migration-templates/model'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { getMissingInterfaceIpWarnings } from '../components/missingInterfaceIpWarnings'
import type { VmDataWithFlavor, VmsSelectionStepProps, RdmConfiguration } from '../types'
import { useOsAssignment } from './useOsAssignment'
import { useVmSelection } from './useVmSelection'
import { useFlavorAssignment } from './useFlavorAssignment'
import { useRdmConfiguration } from './useRdmConfiguration'
import { useBulkIPEdit } from './useBulkIPEdit'
import { useBulkIPHandlers } from './useBulkIPHandlers'
import { useFlavorHandlers } from './useFlavorHandlers'
import { useToast } from './useToast'
import { useStandardColumns } from './useStandardColumns'
import { useRollingColumns } from './useRollingColumns'
import { fromVmDataWithFlavor, fromVM, normalizeNetworkInterfaces } from '../utils/vmAdapters'
import { useVmwareRevalidation } from 'src/hooks/api/useVmwareRevalidation'

const { useCallback, useEffect, useMemo, useState } = React

// Merges user-assigned IP addresses from the retry prefill into the VMware-fetched
// NIC list. Only overrides ipAddress for NICs where preserveIp[i] === false
// (user had a custom assignment). NICs with preserveIp[i] !== false keep the
// VMware-fetched IP unchanged.
function mergeNicIpOverrides(
  existingNics: VmNetworkInterface[],
  prefillNics: VmNetworkInterface[],
  preserveIpMap: Record<number, boolean>,
): VmNetworkInterface[] {
  if (existingNics.length === 0) return existingNics
  return existingNics.map((nic, i) => {
    if (preserveIpMap[i] !== false) return nic
    const prefillNic = prefillNics[i]
    if (!prefillNic) return nic
    const prefillIp = prefillNic.ipAddress
    if (!prefillIp || prefillIp.length === 0) return nic
    return { ...nic, ipAddress: prefillIp }
  })
}

export function useVmsSelectionState(props: VmsSelectionStepProps) {
  const {
    mode = 'standard',
    onChange,
    open = false,
    vmwareCredsValidated,
    openstackCredsValidated,
    sessionId = Date.now().toString(),
    openstackFlavors = [],
    vmwareCredName,
    openstackCredName,
    openstackCredentials,
    vmwareCluster,
    useGPU = false,
    showHeader = true,
    retryVmName,
    retryPrefillVm,
    error,
    vmsWithAssignments: vmsWithAssignmentsProp = [],
    setVmsWithAssignments: setVmsWithAssignmentsProp,
    vmOSAssignments: vmOSAssignmentsProp = {},
    setVmOSAssignments: setVmOSAssignmentsProp,
    selectedVMs: selectedVMsProp = [],
    onSelectionChange,
    loadingVMs = false,
    vmIpValidationError = '',
    osValidationError = '',
    fetchClusterVMs,
    openstackCredData,
    reportError: reportErrorProp,
  } = props

  const isRolling = mode === 'rolling'

  // --- Error / analytics ---
  const { reportError: internalReportError } = useErrorHandler({ component: 'VmsSelectionStep' })
  const reportError = isRolling && reportErrorProp ? reportErrorProp : internalReportError
  const { track } = useAmplitude({ component: 'VmsSelectionStep' })

  // --- Standard-mode state ---
  const [migratedVms, setMigratedVms] = useState<Set<string>>(new Set())
  const [loadingMigratedVms, setLoadingMigratedVms] = useState(false)
  const [vmsWithFlavor, setVmsWithFlavor] = useState<VmDataWithFlavor[]>([])
  // Stabilize openstackFlavors reference — default [] in destructuring creates new ref each render,
  // which would make the rebuild effect fire on every render (infinite loop).
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const stableOpenstackFlavors = useMemo(() => openstackFlavors ?? [], [JSON.stringify(openstackFlavors)])
  const [rdmConfigurations, setRdmConfigurations] = useState<RdmConfiguration[]>([])

  // --- Toast ---
  const { toastOpen, toastMessage, toastSeverity: toastSeverityIp, showToast, handleCloseToast } = useToast()

  // --- Stable no-ops for inactive-mode hook defaults ---
  const noOpFn = useCallback(() => {}, [])
  const noOpAsync = useCallback(async () => {}, [])
  const noOpSetter = useCallback(() => {}, []) as React.Dispatch<React.SetStateAction<any>>

  // --- Rolling props with fallbacks ---
  const vmsWithAssignments = vmsWithAssignmentsProp ?? []
  const setVmsWithAssignments = setVmsWithAssignmentsProp ?? noOpSetter
  const rollingSelectedVMs = selectedVMsProp ?? []

  // --- Standard form callbacks ---
  const setFormVms = useMemo(() => onChange?.('vms') ?? noOpFn, [onChange, noOpFn])
  const setFormRdmConfigurations = useMemo(
    () => onChange?.('rdmConfigurations') ?? noOpFn,
    [onChange, noOpFn]
  )

  // --- Standard: vm selection ---
  const {
    selectedVMs: selectedVMsStandard,
    setSelectedVMs,
    handleVmSelection,
    isRowSelectable,
    rowSelectionModelArray,
  } = useVmSelection({
    vmsWithFlavor,
    rdmConfigurations,
    setFormVms,
    setFormRdmConfigurations,
  })

  // --- Standard: OS assignment ---
  const { vmOSAssignments: standardVmOSAssignments, handleOSAssignment: standardHandleOSAssignment } =
    useOsAssignment({
      vmsWithFlavor,
      setVmsWithFlavor,
      showToast,
      track,
      reportError,
    })

  const vmOSAssignments = isRolling ? (vmOSAssignmentsProp ?? {}) : standardVmOSAssignments

  // --- Standard: RDM ---
  const {
    rdmDisks,
    rdmDisksLoading,
    rdmConfigDialogOpen,
    rdmConfirmDialogOpen,
    setRdmConfirmDialogOpen,
    rdmUpdating,
    handleOpenRdmConfigurationDialog,
    handleCloseRdmConfigurationDialog,
    handleApplyRdmConfigurations,
    handleApplyRdmConfigurationsClick,
  } = useRdmConfiguration({
    selectedVMs: selectedVMsStandard,
    rdmConfigurations,
    openstackCredName,
    openstackCredentials,
    showToast,
    track,
    reportError,
  })

  const rdmValidation = useRdmConfigValidation({
    selectedVMs: Array.from(selectedVMsStandard)
      .map((vmId) => vmsWithFlavor.find((vm) => vm.id === vmId))
      .filter(Boolean) as VmData[],
    rdmDisks,
    backendVolumeTypeMap: openstackCredentials?.status?.openstack?.backendVolumeTypeMap,
  })

  // --- Standard: bulk IP edit ---
  const standardBulkIP = useBulkIPEdit({
    vmsWithFlavor,
    setVmsWithFlavor,
    selectedVMs: selectedVMsStandard,
    setFormVms,
    openstackCredentials,
    showToast,
    reportError,
  })

  // --- Standard: VMware query ---
  const clusterName = useMemo(() => {
    if (!vmwareCluster) return undefined
    const parts = vmwareCluster.split(':')
    return parts.length === 3 ? parts[2] : undefined
  }, [vmwareCluster])

  const datacenterName = useMemo(() => {
    if (!vmwareCluster) return undefined
    const parts = vmwareCluster.split(':')
    return parts.length === 3 ? parts[1] : undefined
  }, [vmwareCluster])

  const {
    data: vmList = [],
    isLoading: loadingVms,
    refetch: refreshVMList,
  } = useVMwareMachinesQuery({
    vmwareCredsValidated,
    openstackCredsValidated,
    enabled: !isRolling && open,
    sessionId,
    vmwareCredName,
    clusterName,
    datacenterName,
  })

  // --- Standard: VMware revalidation + refresh ---
  const {
    isRevalidating,
    handleRefreshAndRevalidate,
  } = useVmwareRevalidation({
    vmwareCredName,
    onRevalidationComplete: refreshVMList,
  })

  // --- Standard: flavor assignment ---
  const standardFlavor = useFlavorAssignment({
    selectedVMs: selectedVMsStandard,
    vmsWithFlavor,
    setVmsWithFlavor,
    openstackFlavors: openstackFlavors ?? [],
    vmList,
    refreshVMList,
    onChange: onChange ?? (() => () => {}),
    reportError,
  })

  // --- Rolling: bulk IP handlers ---
  const rollingBulkIP = useBulkIPHandlers({
    vmsWithAssignments,
    setVmsWithAssignments,
    selectedVMs: rollingSelectedVMs,
    openstackCredData: openstackCredData ?? null,
    reportError,
  })

  // --- Rolling: flavor handlers ---
  const rollingOpenstackFlavors = useMemo(
    () => openstackCredData?.spec?.flavors ?? [],
    [openstackCredData]
  )

  const rollingFlavor = useFlavorHandlers({
    vmsWithAssignments,
    setVmsWithAssignments,
    selectedVMs: rollingSelectedVMs,
    openstackFlavors: rollingOpenstackFlavors,
    reportError,
    fetchClusterVMs: fetchClusterVMs ?? noOpAsync,
  })

  // --- Rolling: OS assignment handler ---
  const handleRollingOSAssignment = useCallback(
    async (vmId: string, osFamily: string) => {
      try {
        setVmOSAssignmentsProp?.((prev) => ({ ...prev, [vmId]: osFamily }))
        await patchVMwareMachine(
          vmId,
          { spec: { vms: { osFamily } } },
          VJAILBREAK_DEFAULT_NAMESPACE
        )
        setVmsWithAssignments((prev: typeof vmsWithAssignments) =>
          prev.map((v) => (v.id === vmId ? { ...v, osFamily } : v))
        )
      } catch (err) {
        reportError(err as Error, {
          context: 'os-family-assignment',
          metadata: { vmId, osFamily, action: 'os-family-assignment' },
        })
        setVmOSAssignmentsProp?.((prev) => {
          const next = { ...prev }
          delete next[vmId]
          return next
        })
      }
    },
    [setVmOSAssignmentsProp, setVmsWithAssignments, reportError]
  )

  // --- Rolling: sync flavor names from openstackFlavors ---
  useEffect(() => {
    if (!isRolling) return
    if (rollingOpenstackFlavors.length === 0 || vmsWithAssignments.length === 0) return
    const updatedVMs = vmsWithAssignments.map((vm) => {
      if (vm.targetFlavorId) {
        const flavorObj = rollingOpenstackFlavors.find((f) => f.id === vm.targetFlavorId)
        if (flavorObj && vm.flavor !== flavorObj.name) {
          return { ...vm, flavor: flavorObj.name }
        }
      }
      return vm
    })
    const hasChanges = updatedVMs.some((vm, i) => vm.flavor !== vmsWithAssignments[i]?.flavor)
    if (hasChanges) setVmsWithAssignments(updatedVMs)
  }, [isRolling, rollingOpenstackFlavors, vmsWithAssignments, setVmsWithAssignments])

  // --- Standard: duplicate names ---
  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    vmsWithFlavor.forEach((vm) => counts.set(vm.name, (counts.get(vm.name) ?? 0) + 1))
    return new Set(
      Array.from(counts.entries())
        .filter(([, c]) => c > 1)
        .map(([n]) => n)
    )
  }, [vmsWithFlavor])

  // --- Canonical VMs for dialogs ---
  const canonicalVMs = useMemo(
    () =>
      isRolling ? vmsWithAssignments.map(fromVM) : vmsWithFlavor.map(fromVmDataWithFlavor),
    [isRolling, vmsWithAssignments, vmsWithFlavor]
  )

  // --- Standard effects ---
  useEffect(() => {
    if (!open) setSelectedVMs(new Set())
  }, [open])

  useEffect(() => {
    if (!isRolling && open && vmwareCluster !== undefined) {
      refreshVMList()
    }
  }, [vmwareCluster])

  useEffect(() => {
    if (isRolling) return
    const fetchMigratedVms = async () => {
      if (!open) return
      setLoadingMigratedVms(true)
      try {
        const plans = await getMigrationPlans()
        const migratedVmSet = new Set<string>()
        plans.forEach((plan) => {
          plan.spec.virtualMachines.forEach((list) => {
            list.forEach((vm) => migratedVmSet.add(vm))
          })
        })
        setMigratedVms(migratedVmSet)
      } catch (err) {
        console.error('Error fetching migrated VMs:', err)
      } finally {
        setLoadingMigratedVms(false)
      }
    }
    fetchMigratedVms()
  }, [open, vmList, isRolling])

  useEffect(() => {
    if (isRolling) return
    setVmsWithFlavor((prev) => {
      const existingVmsMap = new Map(prev.map((vm) => [vm.id, vm]))
      return vmList.map((vm) => {
        let flavor = ''
        if (vm.targetFlavorId) {
          const foundFlavor = stableOpenstackFlavors.find((f) => f.id === vm.targetFlavorId)
          flavor = foundFlavor ? foundFlavor.name : vm.targetFlavorId
        }

        const flavorNotFound = openstackCredName
          ? vm.labels?.[openstackCredName] === 'NOT_FOUND'
          : false

        const powerState = vm.vmState === 'running' ? 'powered-on' : 'powered-off'
        const existingVm = existingVmsMap.get(vm.id)

        let allIPs = vm.networkInterfaces
          ? vm.networkInterfaces
              .flatMap((nic) => (Array.isArray(nic.ipAddress) ? nic.ipAddress : []))
              .filter((ip) => ip && ip.trim() !== '')
              .join(', ')
          : vm.ipAddress || ''

        if (existingVm && existingVm.ipAddress && existingVm.ipAddress !== '—') {
          allIPs = existingVm.ipAddress ?? allIPs
        }

        let preferredNetworkInterfaces = vm.networkInterfaces
        if (existingVm && existingVm.networkInterfaces && existingVm.networkInterfaces.length > 0) {
          preferredNetworkInterfaces = existingVm.networkInterfaces
        }
        preferredNetworkInterfaces = normalizeNetworkInterfaces(preferredNetworkInterfaces)

        return {
          ...vm,
          ipAddress: allIPs || '—',
          isMigrated:
            migratedVms.has(vm.vmKey || vm.name) ||
            migratedVms.has(vm.name) ||
            Boolean(vm.isMigrated),
          flavor,
          flavorNotFound,
          powerState,
          osFamily: existingVm?.osFamily ?? vm.osFamily,
          ipValidationStatus: 'pending' as const,
          ipValidationMessage: '',
          networkInterfaces: preferredNetworkInterfaces,
        }
      })
    })
  }, [vmList, migratedVms, stableOpenstackFlavors, openstackCredName, isRolling])

  // --- Retry mode: show only the locked VM, un-gray it ---
  const displayVmsWithFlavor = useMemo(() => {
    if (!retryVmName) return vmsWithFlavor
    return vmsWithFlavor
      .filter((vm) => vm.name === retryVmName)
      .map((vm) => ({ ...vm, isMigrated: false }))
  }, [vmsWithFlavor, retryVmName])

  // Auto-select the locked VM when it appears in retry mode.
  // setFormVms is intentionally omitted: useRetryPrefill already sets params.vms
  // with correct preserveIp/preserveMac/networkInterfaces from the original plan.
  // Calling setFormVms here with raw API data would overwrite those values.
  useEffect(() => {
    if (!retryVmName || isRolling) return
    const vm = displayVmsWithFlavor.find((v) => v.name === retryVmName)
    if (!vm) return
    setSelectedVMs(new Set([vm.id]))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [retryVmName, displayVmsWithFlavor])

  // Merge prefilled IP override data into vmsWithFlavor so the "Assign IP" dialog
  // shows the user's previous assignment instead of raw VMware machine IPs.
  // Runs when either retryPrefillVm or displayVmsWithFlavor changes, covering both
  // timing cases: prefill arrives before or after the VM list loads.
  useEffect(() => {
    if (!retryVmName || !retryPrefillVm || isRolling) return
    const vmInList = displayVmsWithFlavor.find((v) => v.name === retryVmName)
    if (!vmInList) return
    setVmsWithFlavor((prev) => {
      const idx = prev.findIndex((v) => v.name === retryVmName)
      if (idx === -1) return prev
      const existing = prev[idx]
      const mergedNetworkInterfaces = mergeNicIpOverrides(
        existing.networkInterfaces ?? [],
        retryPrefillVm.networkInterfaces ?? [],
        retryPrefillVm.preserveIp ?? {},
      )
      const updated = [...prev]
      updated[idx] = {
        ...existing,
        preserveIp: retryPrefillVm.preserveIp ?? existing.preserveIp,
        preserveMac: retryPrefillVm.preserveMac ?? existing.preserveMac,
        networkInterfaces: mergedNetworkInterfaces,
      }
      return updated
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [retryVmName, retryPrefillVm, displayVmsWithFlavor, isRolling])

  // --- Missing IP warnings ---
  const missingInterfaceIpWarnings = useMemo(
    () =>
      isRolling
        ? getMissingInterfaceIpWarnings(
            vmsWithAssignments.filter((vm) => rollingSelectedVMs.includes(vm.id))
          )
        : getMissingInterfaceIpWarnings(vmsWithFlavor.filter((vm) => selectedVMsStandard.has(vm.id))),
    [isRolling, vmsWithAssignments, rollingSelectedVMs, vmsWithFlavor, selectedVMsStandard]
  )

  // --- Standard columns ---
  const standardColumns = useStandardColumns({
    selectedVMs: selectedVMsStandard,
    duplicateNames,
    vmOSAssignments,
    originalIPsPerVM: standardBulkIP.originalIPsPerVM,
    handleOSAssignment: standardHandleOSAssignment,
  })

  // --- Rolling columns ---
  const rollingColumns = useRollingColumns({
    selectedVMs: rollingSelectedVMs,
    vmOSAssignments,
    openstackFlavors: rollingOpenstackFlavors,
    handleOSAssignment: handleRollingOSAssignment,
    handleFlavorChange: rollingFlavor.handleIndividualFlavorChange,
  })

  // --- Rolling: filtered selection (only VMs present in current list) ---
  const rollingFilteredSelection = useMemo(
    () => rollingSelectedVMs.filter((vmId) => vmsWithAssignments.some((vm) => vm.id === vmId)),
    [rollingSelectedVMs, vmsWithAssignments]
  )

  // --- Dialog props ---
  const flavorDialogProps = {
    open: isRolling ? rollingFlavor.flavorDialogOpen : standardFlavor.flavorDialogOpen,
    selectedVMCount: isRolling ? rollingSelectedVMs.length : selectedVMsStandard.size,
    flavors: isRolling ? rollingOpenstackFlavors : (openstackFlavors ?? []),
    selectedFlavor: isRolling ? rollingFlavor.selectedFlavor : standardFlavor.selectedFlavor,
    updating: isRolling ? rollingFlavor.updating : standardFlavor.updating,
    onClose: isRolling ? rollingFlavor.handleCloseFlavorDialog : standardFlavor.handleCloseFlavorDialog,
    onApply: isRolling ? rollingFlavor.handleApplyFlavor : standardFlavor.handleApplyFlavor,
    onFlavorChange: isRolling ? rollingFlavor.setSelectedFlavor : standardFlavor.setSelectedFlavor,
  }

  const bulkIPDialogProps = {
    open: isRolling ? rollingBulkIP.bulkEditDialogOpen : standardBulkIP.bulkEditDialogOpen,
    selectedVMCount: isRolling ? rollingSelectedVMs.length : selectedVMsStandard.size,
    vms: canonicalVMs,
    bulkEditIPs: isRolling ? rollingBulkIP.bulkEditIPs : standardBulkIP.bulkEditIPs,
    bulkPreserveIp: isRolling ? rollingBulkIP.bulkPreserveIp : standardBulkIP.bulkPreserveIp,
    bulkPreserveMac: isRolling ? rollingBulkIP.bulkPreserveMac : standardBulkIP.bulkPreserveMac,
    bulkExistingIPs: isRolling ? rollingBulkIP.bulkExistingIPs : standardBulkIP.bulkExistingIPs,
    bulkCurrentIPs: !isRolling ? standardBulkIP.bulkCurrentIPs : undefined,
    bulkValidationStatus: isRolling ? rollingBulkIP.bulkValidationStatus : standardBulkIP.bulkValidationStatus,
    bulkValidationMessages: isRolling ? rollingBulkIP.bulkValidationMessages : standardBulkIP.bulkValidationMessages,
    assigningIPs: isRolling ? rollingBulkIP.assigningIPs : standardBulkIP.assigningIPs,
    hasBulkIpsToApply: isRolling ? rollingBulkIP.hasBulkIpsToApply : standardBulkIP.hasBulkIpsToApply,
    hasBulkIpValidationErrors: isRolling ? rollingBulkIP.hasBulkIpValidationErrors : standardBulkIP.hasBulkIpValidationErrors,
    duplicateNames: !isRolling ? duplicateNames : undefined,
    onClose: isRolling ? rollingBulkIP.handleCloseBulkEditDialog : standardBulkIP.handleCloseBulkEditDialog,
    onApply: isRolling ? rollingBulkIP.handleApplyBulkIPs : standardBulkIP.handleApplyBulkIPs,
    onClearAll: isRolling ? rollingBulkIP.handleClearAllIPs : standardBulkIP.handleClearAllIPs,
    onPreserveIpChange: isRolling ? rollingBulkIP.handleBulkPreserveIpChange : standardBulkIP.handleBulkPreserveIpChange,
    onPreserveMacChange: isRolling ? rollingBulkIP.handleBulkPreserveMacChange : standardBulkIP.handleBulkPreserveMacChange,
    onIpChange: isRolling ? rollingBulkIP.handleBulkIpChange : standardBulkIP.handleBulkIpChange,
  }

  return {
    // Mode
    isRolling,
    // Props forwarded for JSX convenience
    showHeader,
    error,
    loadingVMs,
    vmIpValidationError,
    osValidationError,
    useGPU,
    onSelectionChange,
    vmwareCredsValidated,
    openstackCredsValidated,
    openstackCredentials,
    // Standard state
    vmsWithFlavor: displayVmsWithFlavor,
    setRdmConfigurations,
    loadingVms,
    loadingMigratedVms,
    // VM selection
    selectedVMsStandard,
    handleVmSelection,
    isRowSelectable,
    rowSelectionModelArray,
    refreshVMList,
    isRevalidating,
    handleRefreshAndRevalidate,
    // RDM
    rdmDisks,
    rdmDisksLoading,
    rdmConfigDialogOpen,
    rdmConfirmDialogOpen,
    setRdmConfirmDialogOpen,
    rdmUpdating,
    handleOpenRdmConfigurationDialog,
    handleCloseRdmConfigurationDialog,
    handleApplyRdmConfigurations,
    handleApplyRdmConfigurationsClick,
    rdmValidation,
    // Standard bulk IP (toolbar needs handleOpenBulkIPAssignment directly)
    standardBulkIP,
    // RDM configuration state
    rdmConfigurations,
    // Standard flavor snackbar
    standardFlavor,
    // Rolling state
    vmsWithAssignments,
    rollingSelectedVMs,
    rollingBulkIP,
    rollingFlavor,
    rollingOpenstackFlavors,
    // Columns
    standardColumns,
    rollingColumns,
    rollingFilteredSelection,
    // Shared
    canonicalVMs,
    missingInterfaceIpWarnings,
    // Toast
    toastOpen,
    toastMessage,
    toastSeverityIp,
    handleCloseToast,
    // Dialog props
    flavorDialogProps,
    bulkIPDialogProps,
  }
}
