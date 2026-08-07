import { useState, useMemo, useEffect } from 'react'
import { validateOpenstackIPs } from 'src/api/openstack-creds/openstackCreds'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import axios from 'axios'
import type { ErrorContext } from 'src/services/errorReporting'
import {
  parseIpList,
  hasMultipleIpEntries,
  isValidIPAddressList,
} from '../utils/ipValidation'
import type { VmDataWithFlavor, BulkIpEdit, BulkIpClear } from '../types'

export interface ClearAllIpsUpdate {
  clearedIPs: Record<string, Record<number, string>>
  clearedStatus: Record<string, Record<number, 'empty' | 'valid' | 'invalid' | 'validating'>>
  clearedPreserveIp: Record<string, Record<number, boolean>>
  clearedCurrentIPs: Record<string, Record<number, string>>
}

/**
 * Pure computation for "Clear All": empties every IP field AND turns Preserve IP off
 * for each one. Preserve IP must flip too — collectIpsToApply/collectIpsToClear in
 * handleApplyBulkIPs both treat an empty field with Preserve IP still on as a no-op,
 * so without this the Apply button silently does nothing to the VM's IP despite the
 * field looking cleared.
 */
export function buildClearAllIpsUpdate(
  bulkEditIPs: Record<string, Record<number, string>>
): ClearAllIpsUpdate {
  const clearedIPs: ClearAllIpsUpdate['clearedIPs'] = {}
  const clearedStatus: ClearAllIpsUpdate['clearedStatus'] = {}
  const clearedPreserveIp: ClearAllIpsUpdate['clearedPreserveIp'] = {}
  const clearedCurrentIPs: ClearAllIpsUpdate['clearedCurrentIPs'] = {}

  Object.keys(bulkEditIPs).forEach((vmName) => {
    clearedIPs[vmName] = {}
    clearedStatus[vmName] = {}
    clearedPreserveIp[vmName] = {}
    clearedCurrentIPs[vmName] = {}

    Object.keys(bulkEditIPs[vmName]).forEach((interfaceIndexStr) => {
      const interfaceIndex = parseInt(interfaceIndexStr)
      clearedIPs[vmName][interfaceIndex] = ''
      clearedStatus[vmName][interfaceIndex] = 'empty'
      clearedPreserveIp[vmName][interfaceIndex] = false
      clearedCurrentIPs[vmName][interfaceIndex] = ''
    })
  })

  return { clearedIPs, clearedStatus, clearedPreserveIp, clearedCurrentIPs }
}

function flattenBulkIps(items: BulkIpEdit[]): Array<{ vmName: string; interfaceIndex: number; ip: string }> {
  const flattened: Array<{ vmName: string; interfaceIndex: number; ip: string }> = []
  items.forEach((item) => {
    const parsed = parseIpList(item.ip)
    if (parsed.length === 0) {
      flattened.push({ ...item, ip: '' })
      return
    }
    parsed.forEach((ip) =>
      flattened.push({ vmName: item.vmName, interfaceIndex: item.interfaceIndex, ip })
    )
  })
  return flattened
}

interface UseBulkIPEditParams {
  vmsWithFlavor: VmDataWithFlavor[]
  setVmsWithFlavor: React.Dispatch<React.SetStateAction<VmDataWithFlavor[]>>
  selectedVMs: Set<string>
  setFormVms: (vms: VmDataWithFlavor[]) => void
  openstackCredentials?: OpenstackCreds
  showToast: (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void
  reportError: (error: Error, additionalContext?: ErrorContext) => void
}

export function useBulkIPEdit({
  vmsWithFlavor,
  setVmsWithFlavor,
  selectedVMs,
  setFormVms,
  openstackCredentials,
  showToast,
  reportError,
}: UseBulkIPEditParams) {
  const [bulkEditDialogOpen, setBulkEditDialogOpen] = useState(false)
  const [bulkEditIPs, setBulkEditIPs] = useState<Record<string, Record<number, string>>>({})
  const [bulkPreserveIp, setBulkPreserveIp] = useState<Record<string, Record<number, boolean>>>({})
  const [bulkPreserveMac, setBulkPreserveMac] = useState<Record<string, Record<number, boolean>>>({})
  const [bulkCurrentIPs, setBulkCurrentIPs] = useState<Record<string, Record<number, string>>>({})
  const [bulkExistingIPs, setBulkExistingIPs] = useState<Record<string, Record<number, string>>>({})
  const [originalIPsPerVM, setOriginalIPsPerVM] = useState<Record<string, Record<number, string>>>({})
  const [bulkValidationStatus, setBulkValidationStatus] = useState<
    Record<string, Record<number, 'empty' | 'valid' | 'invalid' | 'validating'>>
  >({})
  const [bulkValidationMessages, setBulkValidationMessages] = useState<
    Record<string, Record<number, string>>
  >({})
  const [assigningIPs, setAssigningIPs] = useState(false)
  const [bulkEditOverrides, setBulkEditOverrides] = useState<
    Record<string, Record<number, { preserveIP: boolean; preserveMAC: boolean }>>
  >({})

  const hasBulkOverrideChanges = useMemo(() => {
    return Object.values(bulkEditOverrides).some((interfaces) =>
      Object.values(interfaces || {}).some((o) => o.preserveIP === false || o.preserveMAC === false)
    )
  }, [bulkEditOverrides])

  useEffect(() => {
    setOriginalIPsPerVM((prev) => {
      const next = { ...prev }
      vmsWithFlavor.forEach((vm) => {
        if (!next[vm.id]) next[vm.id] = {}
        if (vm.networkInterfaces && vm.networkInterfaces.length > 0) {
          vm.networkInterfaces.forEach((nic, index) => {
            if (next[vm.id][index] !== undefined) return
            const discovered = (Array.isArray((nic as any).ipAddress) ? (nic as any).ipAddress : [])
              .filter((v: string) => v && v.trim() !== '')
              .join(', ')
            if (discovered.trim() !== '') {
              next[vm.id][index] = discovered
            }
          })
        } else {
          if (next[vm.id][0] !== undefined) return
          const discovered = vm.ipAddress && vm.ipAddress !== '—' ? vm.ipAddress : ''
          if (discovered.trim() !== '') {
            next[vm.id][0] = discovered
          }
        }
      })
      return next
    })
  }, [vmsWithFlavor])

  const hasBulkIpValidationErrors = useMemo(() => {
    return Object.values(bulkValidationStatus).some((interfaces) =>
      Object.values(interfaces || {}).some((status) => status === 'invalid')
    )
  }, [bulkValidationStatus])

  const hasBulkIpsToApply = useMemo(() => {
    const anyTypedIp = Object.values(bulkEditIPs).some((interfaces) =>
      Object.values(interfaces || {}).some((ip) => Boolean(ip?.trim()))
    )
    const anyPreserveIpOff = Object.values(bulkPreserveIp).some((interfaces) =>
      Object.values(interfaces || {}).some((flag) => flag === false)
    )
    const anyPreserveMacOff = Object.values(bulkPreserveMac).some((interfaces) =>
      Object.values(interfaces || {}).some((flag) => flag === false)
    )
    return anyTypedIp || anyPreserveIpOff || anyPreserveMacOff || hasBulkOverrideChanges
  }, [bulkEditIPs, bulkPreserveIp, bulkPreserveMac, hasBulkOverrideChanges])

  const getPreserveIpFlag = (vmName: string, interfaceIndex: number) =>
    bulkPreserveIp?.[vmName]?.[interfaceIndex] !== false

  const getExistingIp = (vmName: string, interfaceIndex: number) =>
    bulkExistingIPs?.[vmName]?.[interfaceIndex] || ''

  const updateVmRowsAndForm = (updatedVms: VmDataWithFlavor[]) => {
    setVmsWithFlavor(updatedVms)
    setFormVms(updatedVms.filter((vm) => selectedVMs.has(vm.id)))
  }

  const handleCloseBulkEditDialog = () => {
    setBulkEditDialogOpen(false)
    setBulkEditIPs({})
    setBulkPreserveIp({})
    setBulkPreserveMac({})
    setBulkCurrentIPs({})
    setBulkEditOverrides({})
    setBulkValidationStatus({})
    setBulkValidationMessages({})
  }

  const handleBulkPreserveIpChange = (vmName: string, interfaceIndex: number, preserveIp: boolean) => {
    setBulkPreserveIp((prev) => ({
      ...prev,
      [vmName]: { ...prev[vmName], [interfaceIndex]: preserveIp }
    }))

    if (preserveIp) {
      const originalIp = bulkExistingIPs?.[vmName]?.[interfaceIndex] || ''
      setBulkEditIPs((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: originalIp }
      }))
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: originalIp.trim() ? 'valid' : 'empty' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: '' }
      }))
    } else {
      const currentIp = bulkCurrentIPs?.[vmName]?.[interfaceIndex] || ''
      setBulkEditIPs((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: currentIp }
      }))
      setBulkCurrentIPs((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: currentIp }
      }))
      const trimmed = currentIp.trim()
      if (!trimmed) {
        setBulkValidationStatus((prev) => ({
          ...prev,
          [vmName]: { ...prev[vmName], [interfaceIndex]: 'empty' }
        }))
        setBulkValidationMessages((prev) => ({
          ...prev,
          [vmName]: { ...prev[vmName], [interfaceIndex]: '' }
        }))
      } else if (hasMultipleIpEntries(trimmed)) {
        setBulkValidationStatus((prev) => ({
          ...prev,
          [vmName]: { ...prev[vmName], [interfaceIndex]: 'invalid' }
        }))
        setBulkValidationMessages((prev) => ({
          ...prev,
          [vmName]: {
            ...prev[vmName],
            [interfaceIndex]: 'Multiple IPs are not supported when Preserve IP is disabled'
          }
        }))
      } else if (!isValidIPAddressList(trimmed)) {
        setBulkValidationStatus((prev) => ({
          ...prev,
          [vmName]: { ...prev[vmName], [interfaceIndex]: 'invalid' }
        }))
        setBulkValidationMessages((prev) => ({
          ...prev,
          [vmName]: { ...prev[vmName], [interfaceIndex]: 'Invalid IP format' }
        }))
      } else {
        setBulkValidationStatus((prev) => ({
          ...prev,
          [vmName]: { ...prev[vmName], [interfaceIndex]: 'valid' }
        }))
        setBulkValidationMessages((prev) => ({
          ...prev,
          [vmName]: { ...prev[vmName], [interfaceIndex]: '' }
        }))
      }
    }
    if (!preserveIp) {
      const current = bulkCurrentIPs?.[vmName]?.[interfaceIndex] ?? ''
      const trimmed = current.trim()
      const { status, message } = !trimmed
        ? { status: 'empty' as const, message: '' }
        : hasMultipleIpEntries(trimmed)
          ? ({
              status: 'invalid' as const,
              message: 'Multiple IPs are not supported when Preserve IP is disabled'
            } as const)
          : !isValidIPAddressList(trimmed)
            ? ({ status: 'invalid' as const, message: 'Invalid IP format' } as const)
            : ({ status: 'valid' as const, message: '' } as const)

      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: status }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: message }
      }))
    }
  }

  const handleBulkPreserveMacChange = (vmName: string, interfaceIndex: number, value: boolean) => {
    setBulkPreserveMac((prev) => ({
      ...prev,
      [vmName]: { ...prev[vmName], [interfaceIndex]: value }
    }))
  }

  const handleBulkIpChange = (vmName: string, interfaceIndex: number, value: string) => {
    setBulkEditIPs((prev) => ({
      ...prev,
      [vmName]: { ...prev[vmName], [interfaceIndex]: value }
    }))

    if (bulkPreserveIp?.[vmName]?.[interfaceIndex] === false) {
      setBulkCurrentIPs((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: value }
      }))
    }

    if (!value.trim()) {
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: 'empty' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: '' }
      }))
    } else if (
      bulkPreserveIp?.[vmName]?.[interfaceIndex] === false &&
      hasMultipleIpEntries(value)
    ) {
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: 'invalid' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmName]: {
          ...prev[vmName],
          [interfaceIndex]: 'Multiple IPs are not supported when Preserve IP is disabled'
        }
      }))
    } else if (!isValidIPAddressList(value.trim())) {
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: 'invalid' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmName]: {
          ...prev[vmName],
          [interfaceIndex]: 'Invalid IP format'
        }
      }))
    } else {
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmName]: { ...prev[vmName], [interfaceIndex]: 'valid' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmName]: {
          ...prev[vmName],
          [interfaceIndex]: ''
        }
      }))
    }
  }

  const handleClearAllIPs = () => {
    const { clearedIPs, clearedStatus, clearedPreserveIp, clearedCurrentIPs } =
      buildClearAllIpsUpdate(bulkEditIPs)

    setBulkEditIPs(clearedIPs)
    setBulkValidationStatus(clearedStatus)
    setBulkValidationMessages({})
    setBulkPreserveIp((prev) => {
      const next = { ...prev }
      Object.entries(clearedPreserveIp).forEach(([vmName, interfaces]) => {
        next[vmName] = { ...next[vmName], ...interfaces }
      })
      return next
    })
    setBulkCurrentIPs((prev) => {
      const next = { ...prev }
      Object.entries(clearedCurrentIPs).forEach(([vmName, interfaces]) => {
        next[vmName] = { ...next[vmName], ...interfaces }
      })
      return next
    })
    // bulkEditOverrides is a second, independent snapshot of preserveIP/preserveMAC
    // taken at dialog-open time — applyIpAssignmentsToRows prefers it over the live
    // bulkPreserveIp state when persisting nic.preserveIP. Left stale here, the
    // persisted flag would silently snap back to "true" despite Preserve IP now
    // being off, undermining any check that relies on nic.preserveIP (e.g. the
    // Persist source network interfaces gate).
    setBulkEditOverrides((prev) => {
      const next = { ...prev }
      Object.entries(clearedPreserveIp).forEach(([vmName, interfaces]) => {
        next[vmName] = { ...next[vmName] }
        Object.keys(interfaces).forEach((interfaceIndexStr) => {
          const interfaceIndex = parseInt(interfaceIndexStr)
          const existing = next[vmName][interfaceIndex]
          next[vmName][interfaceIndex] = {
            preserveIP: false,
            preserveMAC: existing?.preserveMAC ?? true
          }
        })
      })
      return next
    })
  }

  const validateRequiredIpsForPreserveEnabled = () => {
    let missingRequiredIp = false
    Object.entries(bulkEditIPs).forEach(([vmName, interfaces]) => {
      Object.entries(interfaces).forEach(([interfaceIndexStr, ip]) => {
        const interfaceIndex = parseInt(interfaceIndexStr)
        const preserveIp = getPreserveIpFlag(vmName, interfaceIndex)
        const existingIp = getExistingIp(vmName, interfaceIndex)

        if (preserveIp && existingIp.trim() === '' && ip.trim() === '') {
          setBulkValidationStatus((prev) => ({
            ...prev,
            [vmName]: { ...prev[vmName], [interfaceIndex]: 'empty' }
          }))
          setBulkValidationMessages((prev) => ({
            ...prev,
            [vmName]: { ...prev[vmName], [interfaceIndex]: '' }
          }))
        }
      })
    })
    return !missingRequiredIp
  }

  const collectIpsToApply = (): BulkIpEdit[] => {
    const ipsToApply: BulkIpEdit[] = []
    Object.entries(bulkEditIPs).forEach(([vmName, interfaces]) => {
      Object.entries(interfaces).forEach(([interfaceIndexStr, ip]) => {
        const interfaceIndex = parseInt(interfaceIndexStr)
        const preserveIp = getPreserveIpFlag(vmName, interfaceIndex)
        const existingIp = getExistingIp(vmName, interfaceIndex)
        const typedIp = ip.trim()

        if (typedIp === '') return
        if (preserveIp && existingIp.trim() !== '' && typedIp === existingIp.trim()) return

        ipsToApply.push({ vmName, interfaceIndex, ip: typedIp })
      })
    })
    return ipsToApply
  }

  const collectIpsToClear = (): BulkIpClear[] => {
    const clears: BulkIpClear[] = []
    Object.entries(bulkEditIPs).forEach(([vmName, interfaces]) => {
      Object.entries(interfaces).forEach(([interfaceIndexStr, ip]) => {
        const interfaceIndex = parseInt(interfaceIndexStr)
        const preserveIp = getPreserveIpFlag(vmName, interfaceIndex)
        const existingIp = getExistingIp(vmName, interfaceIndex)
        const typedIp = ip.trim()

        if (!preserveIp && typedIp === '' && existingIp.trim() !== '') {
          clears.push({ vmName, interfaceIndex })
        }
      })
    })
    return clears
  }

  const applyClearsToAssignedIpsMap = (
    assignedIPsPerVM: Record<string, string[]>,
    clears: BulkIpClear[]
  ) => {
    clears.forEach(({ vmName, interfaceIndex }) => {
      if (!assignedIPsPerVM[vmName]) {
        assignedIPsPerVM[vmName] = []
      }
      while (assignedIPsPerVM[vmName].length <= interfaceIndex) {
        assignedIPsPerVM[vmName].push('')
      }
      assignedIPsPerVM[vmName][interfaceIndex] = ''
    })
  }

  const markBulkValidationFailure = (ips: BulkIpEdit[], message: string) => {
    setBulkValidationStatus((prev) => {
      const newStatus = { ...prev }
      ips.forEach(({ vmName, interfaceIndex }) => {
        if (!newStatus[vmName]) newStatus[vmName] = {}
        newStatus[vmName][interfaceIndex] = 'invalid'
      })
      return newStatus
    })
    setBulkValidationMessages((prev) => {
      const newMessages = { ...prev }
      ips.forEach(({ vmName, interfaceIndex }) => {
        if (!newMessages[vmName]) newMessages[vmName] = {}
        newMessages[vmName][interfaceIndex] = message
      })
      return newMessages
    })
  }

  const setBulkStatusForIps = (
    ips: BulkIpEdit[],
    status: 'empty' | 'valid' | 'invalid' | 'validating'
  ) => {
    setBulkValidationStatus((prev) => {
      const next = { ...prev }
      ips.forEach(({ vmName, interfaceIndex }) => {
        if (!next[vmName]) next[vmName] = {}
        next[vmName][interfaceIndex] = status
      })
      return next
    })
  }

  const buildAssignedIpsPerVm = (ips: BulkIpEdit[]) => {
    const assignedIPsPerVM: Record<string, string[]> = {}
    ips.forEach(({ vmName, interfaceIndex, ip }) => {
      if (!assignedIPsPerVM[vmName]) {
        assignedIPsPerVM[vmName] = []
      }
      while (assignedIPsPerVM[vmName].length <= interfaceIndex) {
        assignedIPsPerVM[vmName].push('')
      }
      if (assignedIPsPerVM[vmName].length > interfaceIndex) {
        assignedIPsPerVM[vmName][interfaceIndex] = ip
      }
    })
    return assignedIPsPerVM
  }

  const applyIpAssignmentsToRows = (assignedIPsPerVM: Record<string, string[]>) => {
    const updatedVms = vmsWithFlavor.map((vm) => {
      const assignedIPs = assignedIPsPerVM[vm.id]
      const preserveIp = bulkPreserveIp[vm.id]
      const preserveMac = bulkPreserveMac[vm.id]
      if (!assignedIPs && !preserveIp && !preserveMac) return vm

      const vmOverrides = bulkEditOverrides[vm.id]

      let updatedNetworkInterfaces = vm.networkInterfaces
      if (updatedNetworkInterfaces && updatedNetworkInterfaces.length > 0) {
        updatedNetworkInterfaces = updatedNetworkInterfaces.map((nic, index) => {
          const assignedIP = assignedIPs?.[index]
          const overrides = vmOverrides?.[index]
          const preserveIP = bulkPreserveIp?.[vm.id]?.[index] !== false
          const preserveMAC = bulkPreserveMac?.[vm.id]?.[index] !== false
          const parsed = assignedIP !== undefined ? parseIpList(assignedIP) : undefined
          return {
            ...nic,
            ...(assignedIP !== undefined
              ? parsed && parsed.length > 0
                ? { ipAddress: parsed }
                : !preserveIP
                  ? { ipAddress: [] }
                  : {}
              : {}),
            ...(overrides
              ? {
                  preserveIP: overrides.preserveIP,
                  preserveMAC: overrides.preserveMAC
                }
              : { preserveIP, preserveMAC })
          }
        })
      }

      if (!assignedIPs && !vmOverrides) return vm

      const displayIPs = updatedNetworkInterfaces
        ? updatedNetworkInterfaces
            .flatMap((nic) => (Array.isArray(nic.ipAddress) ? nic.ipAddress : []))
            .filter((ip) => ip && ip.trim() !== '')
        : []
      const ipDisplay = displayIPs.join(', ')

      return {
        ...vm,
        ...(updatedNetworkInterfaces && {
          ipAddress: ipDisplay || '—',
          networkInterfaces: updatedNetworkInterfaces
        }),
        ...(preserveIp && { preserveIp }),
        ...(preserveMac && { preserveMac })
      }
    })

    updateVmRowsAndForm(updatedVms)
  }

  const applyPreserveFlagsOnly = () => {
    const updatedVms = vmsWithFlavor.map((vm) => {
      const preserveIp = bulkPreserveIp[vm.id]
      const preserveMac = bulkPreserveMac[vm.id]
      const hasAnyPreserveFlags = Boolean(preserveIp) || Boolean(preserveMac)
      if (!hasAnyPreserveFlags) return vm

      const updatedNetworkInterfaces = vm.networkInterfaces?.map((nic, index) => {
        const preserveIP = preserveIp?.[index] !== false
        const preserveMAC = preserveMac?.[index] !== false
        return { ...nic, preserveIP, preserveMAC }
      })

      return {
        ...vm,
        networkInterfaces: updatedNetworkInterfaces,
        ...(preserveIp && { preserveIp }),
        ...(preserveMac && { preserveMac })
      }
    })

    updateVmRowsAndForm(updatedVms)
    showToast('Preserve settings saved.', 'success')
    handleCloseBulkEditDialog()
  }

  const applyOverrideChangesOnly = () => {
    const updatedVms = vmsWithFlavor.map((vm) => {
      const vmOverrides = bulkEditOverrides[vm.id]
      if (!vmOverrides) return vm
      const updatedNetworkInterfaces = vm.networkInterfaces?.map((nic, index) => {
        const overrides = vmOverrides[index]
        return overrides
          ? { ...nic, preserveIP: overrides.preserveIP, preserveMAC: overrides.preserveMAC }
          : nic
      })
      const preserveIp = bulkPreserveIp[vm.id]
      const preserveMac = bulkPreserveMac[vm.id]
      return {
        ...vm,
        networkInterfaces: updatedNetworkInterfaces,
        ...(preserveIp && { preserveIp }),
        ...(preserveMac && { preserveMac })
      }
    })

    updateVmRowsAndForm(updatedVms)
    showToast('Network override settings applied', 'success')
    handleCloseBulkEditDialog()
  }

  const handleApplyBulkIPs = async () => {
    const hasRequiredIps = validateRequiredIpsForPreserveEnabled()
    if (!hasRequiredIps) {
      showToast('Provide an IP address for all interfaces where Preserve IP is enabled.', 'error')
      return
    }

    const ipsToApply = collectIpsToApply()
    const ipsToClear = collectIpsToClear()

    if (ipsToApply.length === 0 && ipsToClear.length === 0) {
      if (hasBulkOverrideChanges) {
        applyOverrideChangesOnly()
      } else {
        applyPreserveFlagsOnly()
      }
      return
    }
    if (hasBulkIpValidationErrors) {
      showToast('Resolve invalid IP addresses before applying changes.', 'error')
      return
    }

    setAssigningIPs(true)

    try {
      if (openstackCredentials) {
        const flattenedIps = flattenBulkIps(ipsToApply)
        const ipList = flattenedIps.map((item) => item.ip)

        setBulkStatusForIps(ipsToApply, 'validating')

        let validationResult
        try {
          validationResult = await validateOpenstackIPs({
            ip: ipList,
            accessInfo: {
              secret_name: `${openstackCredentials.metadata.name}-openstack-secret`,
              secret_namespace: openstackCredentials.metadata.namespace
            }
          })
        } catch (error) {
          if (axios.isAxiosError(error) && error.response?.status === 500) {
            const responseData = error.response?.data as { message?: string } | string | undefined
            const apiMessage =
              typeof responseData === 'string' ? responseData : responseData?.message
            const validationErrorMessage =
              apiMessage ||
              'PCD IP validation service is unavailable (500). Please verify credentials or try again later.'

            markBulkValidationFailure(ipsToApply, validationErrorMessage)
            showToast(validationErrorMessage, 'error')
            reportError(error as Error, {
              context: 'bulk-ip-validation-request',
              metadata: {
                bulkEditIPs: bulkEditIPs,
                action: 'bulk-ip-validation-assignment',
                status: error.response?.status
              }
            })
            setAssigningIPs(false)
            return
          }

          throw error
        }

        const validIPs: Array<{ vmName: string; interfaceIndex: number; ip: string }> = []
        let hasInvalidIPs = false

        const byInterfaceKey = new Map<string, { ok: boolean; reason?: string }>()
        flattenedIps.forEach((flatItem, index) => {
          const key = `${flatItem.vmName}__${flatItem.interfaceIndex}`
          const isValid = validationResult.isValid[index]
          const reason = validationResult.reason[index]
          const current = byInterfaceKey.get(key)
          if (!current) {
            byInterfaceKey.set(key, { ok: Boolean(isValid), reason: isValid ? undefined : reason })
          } else if (current.ok && !isValid) {
            byInterfaceKey.set(key, { ok: false, reason })
          }
        })

        ipsToApply.forEach((item) => {
          const key = `${item.vmName}__${item.interfaceIndex}`
          const result = byInterfaceKey.get(key)
          const ok = result?.ok !== false
          if (ok) {
            validIPs.push(item)
            setBulkValidationStatus((prev) => ({
              ...prev,
              [item.vmName]: { ...prev[item.vmName], [item.interfaceIndex]: 'valid' }
            }))
            setBulkValidationMessages((prev) => ({
              ...prev,
              [item.vmName]: { ...prev[item.vmName], [item.interfaceIndex]: 'Valid' }
            }))
          } else {
            hasInvalidIPs = true
            setBulkValidationStatus((prev) => ({
              ...prev,
              [item.vmName]: { ...prev[item.vmName], [item.interfaceIndex]: 'invalid' }
            }))
            setBulkValidationMessages((prev) => ({
              ...prev,
              [item.vmName]: {
                ...prev[item.vmName],
                [item.interfaceIndex]: result?.reason || 'Invalid IP format'
              }
            }))
          }
        })

        if (hasInvalidIPs) {
          setAssigningIPs(false)
          return
        }

        const assignedIPsPerVM = buildAssignedIpsPerVm(validIPs)
        applyClearsToAssignedIpsMap(assignedIPsPerVM, ipsToClear)
        applyIpAssignmentsToRows(assignedIPsPerVM)

        validIPs.forEach(({ vmName, interfaceIndex }) => {
          setBulkValidationStatus((prev) => ({
            ...prev,
            [vmName]: { ...prev[vmName], [interfaceIndex]: 'valid' }
          }))
          setBulkValidationMessages((prev) => ({
            ...prev,
            [vmName]: { ...prev[vmName], [interfaceIndex]: 'IP assigned locally' }
          }))
        })

        showToast(
          `Successfully applied network changes to ${validIPs.length + ipsToClear.length} interface(s)`,
          'success'
        )

        handleCloseBulkEditDialog()
      } else {
        const assignedIPsPerVM = buildAssignedIpsPerVm(ipsToApply)
        applyClearsToAssignedIpsMap(assignedIPsPerVM, ipsToClear)
        applyIpAssignmentsToRows(assignedIPsPerVM)
        showToast(
          `Successfully applied network changes to ${ipsToApply.length + ipsToClear.length} interface(s)`,
          'success'
        )
        handleCloseBulkEditDialog()
      }
    } catch (error) {
      reportError(error as Error, {
        context: 'bulk-ip-validation-assignment',
        metadata: {
          bulkEditIPs: bulkEditIPs,
          action: 'bulk-ip-validation-assignment'
        }
      })
    } finally {
      setAssigningIPs(false)
    }
  }

  const handleOpenBulkIPAssignment = () => {
    if (selectedVMs.size === 0) return

    const initialBulkEditIPs: Record<string, Record<number, string>> = {}
    const initialBulkPreserveIp: Record<string, Record<number, boolean>> = {}
    const initialBulkPreserveMac: Record<string, Record<number, boolean>> = {}
    const initialBulkCurrentIPs: Record<string, Record<number, string>> = {}
    const initialBulkExistingIPs: Record<string, Record<number, string>> = {}
    const initialBulkEditOverrides: Record<
      string,
      Record<number, { preserveIP: boolean; preserveMAC: boolean }>
    > = {}
    const initialValidationStatus: Record<
      string,
      Record<number, 'empty' | 'valid' | 'invalid' | 'validating'>
    > = {}

    Array.from(selectedVMs).forEach((vmId) => {
      const vm = vmsWithFlavor.find((v) => v.id === vmId)
      if (!vm) return

      initialBulkEditIPs[vmId] = {}
      initialBulkPreserveIp[vmId] = {}
      initialBulkPreserveMac[vmId] = {}
      initialBulkCurrentIPs[vmId] = {}
      initialBulkExistingIPs[vmId] = {}
      initialBulkEditOverrides[vmId] = {}
      initialValidationStatus[vmId] = {}

      if (vm.networkInterfaces && vm.networkInterfaces.length > 0) {
        vm.networkInterfaces.forEach((nic, index) => {
          const originalIp =
            originalIPsPerVM?.[vmId]?.[index] !== undefined
              ? originalIPsPerVM[vmId][index]
              : (Array.isArray((nic as any).ipAddress) ? (nic as any).ipAddress : [])
                  .filter((ip: string) => ip && ip.trim() !== '')
                  .join(', ')
          const currentIp = (Array.isArray((nic as any).ipAddress) ? (nic as any).ipAddress : [])
            .filter((ip: string) => ip && ip.trim() !== '')
            .join(', ')

          initialBulkExistingIPs[vmId][index] = originalIp
          initialBulkCurrentIPs[vmId][index] = currentIp

          const initialPreserveIp = vm.preserveIp?.[index] !== false
          const initialPreserveMac = vm.preserveMac?.[index] !== false

          const isPoweredOff = vm.vmState !== 'running'
          const effectivePreserveIp = isPoweredOff ? false : initialPreserveIp
          initialBulkPreserveIp[vmId][index] = effectivePreserveIp
          initialBulkPreserveMac[vmId][index] = initialPreserveMac

          initialBulkEditIPs[vmId][index] = effectivePreserveIp ? originalIp : currentIp
          initialBulkEditOverrides[vmId][index] = {
            preserveIP: effectivePreserveIp,
            preserveMAC: initialPreserveMac
          }

          const initialValue = initialBulkEditIPs[vmId][index]
          initialValidationStatus[vmId][index] = initialValue.trim() ? 'valid' : 'empty'
        })
      } else {
        const tableIp = vm.ipAddress && vm.ipAddress !== '—' ? vm.ipAddress : ''
        const originalIp =
          originalIPsPerVM?.[vmId]?.[0] !== undefined ? originalIPsPerVM[vmId][0] : tableIp
        const currentIp = tableIp

        initialBulkExistingIPs[vmId][0] = originalIp
        initialBulkCurrentIPs[vmId][0] = currentIp

        const isPoweredOff = vm.vmState !== 'running'
        const effectivePreserveIp = isPoweredOff ? false : vm.preserveIp?.[0] !== false
        const initialPreserveMac = vm.preserveMac?.[0] !== false

        initialBulkPreserveIp[vmId][0] = effectivePreserveIp
        initialBulkPreserveMac[vmId][0] = initialPreserveMac
        initialBulkEditIPs[vmId][0] = effectivePreserveIp ? originalIp : currentIp
        initialBulkEditOverrides[vmId][0] = {
          preserveIP: effectivePreserveIp,
          preserveMAC: initialPreserveMac
        }
        initialValidationStatus[vmId][0] = initialBulkEditIPs[vmId][0].trim() ? 'valid' : 'empty'
      }
    })

    setBulkEditIPs(initialBulkEditIPs)
    setBulkPreserveIp(initialBulkPreserveIp)
    setBulkPreserveMac(initialBulkPreserveMac)
    setBulkExistingIPs(initialBulkExistingIPs)
    setBulkCurrentIPs(initialBulkCurrentIPs)
    setBulkEditOverrides(initialBulkEditOverrides)
    setBulkValidationStatus(initialValidationStatus)
    setBulkValidationMessages({})
    setBulkEditDialogOpen(true)
  }

  return {
    originalIPsPerVM,
    bulkEditDialogOpen,
    bulkEditIPs,
    bulkValidationStatus,
    bulkValidationMessages,
    bulkPreserveIp,
    bulkPreserveMac,
    bulkExistingIPs,
    bulkCurrentIPs,
    assigningIPs,
    hasBulkIpsToApply,
    hasBulkIpValidationErrors,
    handleOpenBulkIPAssignment,
    handleCloseBulkEditDialog,
    handleApplyBulkIPs,
    handleBulkPreserveIpChange,
    handleBulkPreserveMacChange,
    handleBulkIpChange,
    handleClearAllIPs,
  }
}
