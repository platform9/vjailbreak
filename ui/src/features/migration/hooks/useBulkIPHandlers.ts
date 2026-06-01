import { useState, useMemo } from 'react'
import { GridRowSelectionModel } from '@mui/x-data-grid'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import { validateOpenstackIPs } from 'src/api/openstack-creds/openstackCreds'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import {
  parseIpList,
  extractFirstIPv4,
  hasMultipleIpEntries,
  isValidIPAddressList
} from '../utils/ipValidation'
import type { VM } from '../types'
import type { ErrorContext } from 'src/services/errorReporting'

interface UseBulkIPHandlersParams {
  vmsWithAssignments: VM[]
  setVmsWithAssignments: React.Dispatch<React.SetStateAction<VM[]>>
  selectedVMs: GridRowSelectionModel
  openstackCredData: OpenstackCreds | null
  reportError: (error: Error, additionalContext?: ErrorContext) => void
}

export function useBulkIPHandlers({
  vmsWithAssignments,
  setVmsWithAssignments,
  selectedVMs,
  openstackCredData,
  reportError
}: UseBulkIPHandlersParams) {
  const [assigningIPs, setAssigningIPs] = useState(false)
  const [bulkEditDialogOpen, setBulkEditDialogOpen] = useState(false)
  const [bulkEditIPs, setBulkEditIPs] = useState<Record<string, Record<number, string>>>({})
  const [bulkPreserveIp, setBulkPreserveIp] = useState<Record<string, Record<number, boolean>>>({})
  const [bulkPreserveMac, setBulkPreserveMac] = useState<Record<string, Record<number, boolean>>>(
    {}
  )
  const [bulkExistingIPs, setBulkExistingIPs] = useState<Record<string, Record<number, string>>>({})
  const [bulkValidationStatus, setBulkValidationStatus] = useState<
    Record<string, Record<number, 'empty' | 'valid' | 'invalid' | 'validating'>>
  >({})
  const [bulkValidationMessages, setBulkValidationMessages] = useState<
    Record<string, Record<number, string>>
  >({})

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
    return anyTypedIp || anyPreserveIpOff || anyPreserveMacOff
  }, [bulkEditIPs, bulkPreserveIp, bulkPreserveMac])

  const handleCloseBulkEditDialog = () => {
    setBulkEditDialogOpen(false)
    setBulkEditIPs({})
    setBulkPreserveIp({})
    setBulkPreserveMac({})
    setBulkExistingIPs({})
    setBulkValidationStatus({})
    setBulkValidationMessages({})
  }

  const handleBulkPreserveIpChange = (vmId: string, interfaceIndex: number, value: boolean) => {
    setBulkPreserveIp((prev) => ({
      ...prev,
      [vmId]: { ...prev[vmId], [interfaceIndex]: value }
    }))

    if (value) {
      const existingIp = bulkExistingIPs?.[vmId]?.[interfaceIndex] || ''
      if (existingIp.trim() !== '') {
        setBulkEditIPs((prev) => ({
          ...prev,
          [vmId]: { ...prev[vmId], [interfaceIndex]: existingIp }
        }))
        setBulkValidationStatus((prev) => ({
          ...prev,
          [vmId]: { ...prev[vmId], [interfaceIndex]: 'valid' }
        }))
        setBulkValidationMessages((prev) => ({
          ...prev,
          [vmId]: { ...prev[vmId], [interfaceIndex]: '' }
        }))
      }
    }

    if (!value) {
      // Preserve IP disabled: keep the current value so the user can edit/override it.
      const current = bulkEditIPs?.[vmId]?.[interfaceIndex] ?? ''
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
        [vmId]: { ...prev[vmId], [interfaceIndex]: status }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmId]: { ...prev[vmId], [interfaceIndex]: message }
      }))
    }
  }

  const handleBulkPreserveMacChange = (vmId: string, interfaceIndex: number, value: boolean) => {
    setBulkPreserveMac((prev) => ({
      ...prev,
      [vmId]: { ...prev[vmId], [interfaceIndex]: value }
    }))
  }

  const handleBulkIpChange = (vmId: string, interfaceIndex: number, value: string) => {
    setBulkEditIPs((prev) => ({
      ...prev,
      [vmId]: { ...prev[vmId], [interfaceIndex]: value }
    }))

    if (!value.trim()) {
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmId]: { ...prev[vmId], [interfaceIndex]: 'empty' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmId]: { ...prev[vmId], [interfaceIndex]: '' }
      }))
    } else if (bulkPreserveIp?.[vmId]?.[interfaceIndex] === false && hasMultipleIpEntries(value)) {
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmId]: { ...prev[vmId], [interfaceIndex]: 'invalid' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmId]: {
          ...prev[vmId],
          [interfaceIndex]: 'Multiple IPs are not supported when Preserve IP is disabled'
        }
      }))
    } else if (!isValidIPAddressList(value.trim())) {
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmId]: { ...prev[vmId], [interfaceIndex]: 'invalid' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmId]: { ...prev[vmId], [interfaceIndex]: 'Invalid IP format' }
      }))
    } else {
      setBulkValidationStatus((prev) => ({
        ...prev,
        [vmId]: { ...prev[vmId], [interfaceIndex]: 'empty' }
      }))
      setBulkValidationMessages((prev) => ({
        ...prev,
        [vmId]: { ...prev[vmId], [interfaceIndex]: '' }
      }))
    }
  }

  const handleClearAllIPs = () => {
    const clearedIPs: Record<string, Record<number, string>> = {}
    const clearedStatus: Record<
      string,
      Record<number, 'empty' | 'valid' | 'invalid' | 'validating'>
    > = {}

    Object.keys(bulkEditIPs).forEach((vmId) => {
      clearedIPs[vmId] = {}
      clearedStatus[vmId] = {}

      Object.keys(bulkEditIPs[vmId]).forEach((interfaceIndexStr) => {
        const interfaceIndex = parseInt(interfaceIndexStr)
        clearedIPs[vmId][interfaceIndex] = ''
        clearedStatus[vmId][interfaceIndex] = 'empty'
      })
    })

    setBulkEditIPs(clearedIPs)
    setBulkValidationStatus(clearedStatus)
    setBulkValidationMessages({})
  }

  const handleApplyBulkIPs = async () => {
    const ipsToApply: Array<{ vmId: string; interfaceIndex: number; ip: string }> = []
    const clearIpsToApply: Array<{ vmId: string; interfaceIndex: number }> = []

    let missingRequiredIp = false
    Object.entries(bulkEditIPs).forEach(([vmId, interfaces]) => {
      Object.entries(interfaces).forEach(([interfaceIndexStr, ip]) => {
        const interfaceIndex = parseInt(interfaceIndexStr)
        const preserveIp = bulkPreserveIp?.[vmId]?.[interfaceIndex] !== false
        const existingIp = bulkExistingIPs?.[vmId]?.[interfaceIndex] || ''

        if (preserveIp && existingIp.trim() === '' && ip.trim() === '') {
          missingRequiredIp = true
          setBulkValidationStatus((prev) => ({
            ...prev,
            [vmId]: { ...prev[vmId], [interfaceIndex]: 'invalid' }
          }))
          setBulkValidationMessages((prev) => ({
            ...prev,
            [vmId]: {
              ...prev[vmId],
              [interfaceIndex]: 'IP is required when Preserve IP is enabled'
            }
          }))
        }
      })
    })

    if (missingRequiredIp) {
      return
    }

    Object.entries(bulkEditIPs).forEach(([vmId, interfaces]) => {
      Object.entries(interfaces).forEach(([interfaceIndexStr, ip]) => {
        const interfaceIndex = parseInt(interfaceIndexStr)
        const preserveIp = bulkPreserveIp?.[vmId]?.[interfaceIndex] !== false
        const existingIp = bulkExistingIPs?.[vmId]?.[interfaceIndex] || ''
        const typedIp = ip.trim()

        // When Preserve IP is disabled, allow clearing the IP (empty value)
        if (!preserveIp && typedIp === '') {
          if (existingIp.trim() !== '') {
            clearIpsToApply.push({ vmId, interfaceIndex })
          }
          return
        }

        if (typedIp === '') return

        // If Preserve IP is enabled and an existing IP is present, we keep it as-is.
        if (preserveIp && existingIp.trim() !== '' && typedIp === existingIp.trim()) {
          return
        }

        ipsToApply.push({
          vmId,
          interfaceIndex,
          ip: typedIp
        })
      })
    })

    if (ipsToApply.length === 0 && clearIpsToApply.length === 0) {
      const updatedVMs = vmsWithAssignments.map((vm) => {
        const preserveIp = bulkPreserveIp[vm.id]
        const preserveMac = bulkPreserveMac[vm.id]
        if (!preserveIp && !preserveMac) return vm
        return {
          ...vm,
          ...(preserveIp && { preserveIp }),
          ...(preserveMac && { preserveMac })
        }
      })
      setVmsWithAssignments(updatedVMs)
      handleCloseBulkEditDialog()
      return
    }

    setAssigningIPs(true)

    try {
      // Batch validation before applying any changes
      if (openstackCredData) {
        const flattenedIps: Array<{ vmId: string; interfaceIndex: number; ip: string }> = []
        ipsToApply.forEach((item) => {
          const parsed = parseIpList(item.ip)
          if (parsed.length === 0) {
            flattenedIps.push({ ...item, ip: '' })
            return
          }
          parsed.forEach((ip) =>
            flattenedIps.push({ vmId: item.vmId, interfaceIndex: item.interfaceIndex, ip })
          )
        })

        const ipList = flattenedIps.map((item) => item.ip)

        // Set validating status for all IPs
        setBulkValidationStatus((prev) => {
          const newStatus = { ...prev }
          ipsToApply.forEach(({ vmId, interfaceIndex }) => {
            if (!newStatus[vmId]) newStatus[vmId] = {}
            newStatus[vmId][interfaceIndex] = 'validating'
          })
          return newStatus
        })

        const validationResult =
          ipList.length > 0
            ? await validateOpenstackIPs({
                ip: ipList,
                accessInfo: {
                  secret_name: `${openstackCredData.metadata.name}-openstack-secret`,
                  secret_namespace: openstackCredData.metadata.namespace
                }
              })
            : { isValid: [], reason: [] }

        // Process validation results
        const validIPs: Array<{ vmId: string; interfaceIndex: number; ip: string }> = []
        let hasInvalidIPs = false

        const byInterfaceKey = new Map<string, { ok: boolean; reason?: string }>()
        flattenedIps.forEach((flatItem, index) => {
          const key = `${flatItem.vmId}__${flatItem.interfaceIndex}`
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
          const key = `${item.vmId}__${item.interfaceIndex}`
          const result = byInterfaceKey.get(key)
          const ok = result?.ok !== false
          if (ok) {
            validIPs.push(item)
            setBulkValidationStatus((prev) => ({
              ...prev,
              [item.vmId]: { ...prev[item.vmId], [item.interfaceIndex]: 'valid' }
            }))
            setBulkValidationMessages((prev) => ({
              ...prev,
              [item.vmId]: { ...prev[item.vmId], [item.interfaceIndex]: 'Valid' }
            }))
          } else {
            hasInvalidIPs = true
            setBulkValidationStatus((prev) => ({
              ...prev,
              [item.vmId]: { ...prev[item.vmId], [item.interfaceIndex]: 'invalid' }
            }))
            setBulkValidationMessages((prev) => ({
              ...prev,
              [item.vmId]: {
                ...prev[item.vmId],
                [item.interfaceIndex]: result?.reason || 'Invalid IP format'
              }
            }))
          }
        })

        // Only proceed if ALL IPs are valid
        if (hasInvalidIPs) {
          setAssigningIPs(false)
          return
        }

        const updatePromises = validIPs.map(async ({ vmId, interfaceIndex, ip }) => {
          try {
            const vm = vmsWithAssignments.find((v) => v.id === vmId)
            if (!vm) throw new Error('VM not found')

            // Update network interfaces
            if (vm.networkInterfaces && vm.networkInterfaces[interfaceIndex]) {
              const updatedInterfaces = [...vm.networkInterfaces]
              updatedInterfaces[interfaceIndex] = {
                ...updatedInterfaces[interfaceIndex],
                ipAddress: ip.trim() !== '' ? parseIpList(ip) : []
              }

              await patchVMwareMachine(
                vm.id,
                {
                  spec: {
                    vms: {
                      networkInterfaces: updatedInterfaces
                    }
                  }
                },
                VJAILBREAK_DEFAULT_NAMESPACE
              )
            } else {
              // Fallback for single IP assignment
              await patchVMwareMachine(
                vmId,
                {
                  spec: {
                    vms: {
                      assignedIp: ip
                    }
                  }
                },
                VJAILBREAK_DEFAULT_NAMESPACE
              )
            }

            return { success: true, vmId, interfaceIndex, ip }
          } catch (error) {
            setBulkValidationStatus((prev) => ({
              ...prev,
              [vmId]: { ...prev[vmId], [interfaceIndex]: 'invalid' }
            }))
            setBulkValidationMessages((prev) => ({
              ...prev,
              [vmId]: {
                ...prev[vmId],
                [interfaceIndex]: error instanceof Error ? error.message : 'Failed to apply IP'
              }
            }))
            return { success: false, vmId, interfaceIndex, error }
          }
        })

        const clearPromises = clearIpsToApply.map(async ({ vmId, interfaceIndex }) => {
          try {
            const vm = vmsWithAssignments.find((v) => v.id === vmId)
            if (!vm) throw new Error('VM not found')

            if (vm.networkInterfaces && vm.networkInterfaces[interfaceIndex]) {
              const existingIp = bulkExistingIPs?.[vmId]?.[interfaceIndex] || ''
              const parsedExisting = existingIp.trim() !== '' ? parseIpList(existingIp) : []
              const updatedInterfaces = [...vm.networkInterfaces]
              updatedInterfaces[interfaceIndex] = {
                ...updatedInterfaces[interfaceIndex],
                ipAddress: parsedExisting
              }

              await patchVMwareMachine(
                vm.id,
                {
                  spec: {
                    vms: {
                      networkInterfaces: updatedInterfaces
                    }
                  }
                },
                VJAILBREAK_DEFAULT_NAMESPACE
              )
            } else {
              await patchVMwareMachine(
                vmId,
                {
                  spec: {
                    vms: {
                      assignedIp: ''
                    }
                  }
                },
                VJAILBREAK_DEFAULT_NAMESPACE
              )
            }

            return { success: true, vmId, interfaceIndex }
          } catch (error) {
            setBulkValidationStatus((prev) => ({
              ...prev,
              [vmId]: { ...prev[vmId], [interfaceIndex]: 'invalid' }
            }))
            setBulkValidationMessages((prev) => ({
              ...prev,
              [vmId]: {
                ...prev[vmId],
                [interfaceIndex]: error instanceof Error ? error.message : 'Failed to clear IP'
              }
            }))
            return { success: false, vmId, interfaceIndex, error }
          }
        })

        const results = await Promise.all([...updatePromises, ...clearPromises])

        // Check if any updates failed
        const failedUpdates = results.filter((result) => !result.success)
        if (failedUpdates.length > 0) {
          setAssigningIPs(false)
          return // Don't close modal if any updates failed
        }

        // Update bulk validation status
        const newBulkValidationStatus = { ...bulkValidationStatus }
        const newBulkValidationMessages = { ...bulkValidationMessages }

        validIPs.forEach(({ vmId, interfaceIndex }) => {
          if (!newBulkValidationStatus[vmId]) newBulkValidationStatus[vmId] = {}
          if (!newBulkValidationMessages[vmId]) newBulkValidationMessages[vmId] = {}

          newBulkValidationStatus[vmId][interfaceIndex] = 'valid'
          newBulkValidationMessages[vmId][interfaceIndex] = 'IP validated and applied successfully'
        })

        setBulkValidationStatus(newBulkValidationStatus)
        setBulkValidationMessages(newBulkValidationMessages)

        // Update local VM state so the table immediately reflects Auto IP/Auto MAC chips
        setVmsWithAssignments((prev) =>
          prev.map((vm) => {
            const preserveIp = bulkPreserveIp[vm.id]
            const preserveMac = bulkPreserveMac[vm.id]
            const vmUpdates = validIPs.filter((item) => item.vmId === vm.id)
            const vmClears = clearIpsToApply.filter((item) => item.vmId === vm.id)

            if (vmUpdates.length === 0 && vmClears.length === 0 && !preserveIp && !preserveMac) {
              return vm
            }

            const updatedVM: any = {
              ...vm,
              ...(preserveIp && { preserveIp }),
              ...(preserveMac && { preserveMac })
            }

            if (vm.networkInterfaces) {
              const updatedInterfaces = [...vm.networkInterfaces]
              vmUpdates.forEach(({ interfaceIndex, ip }) => {
                if (updatedInterfaces[interfaceIndex]) {
                  updatedInterfaces[interfaceIndex] = {
                    ...updatedInterfaces[interfaceIndex],
                    ipAddress: ip.trim() !== '' ? parseIpList(ip) : []
                  }
                }
              })
              vmClears.forEach(({ interfaceIndex }) => {
                if (updatedInterfaces[interfaceIndex]) {
                  updatedInterfaces[interfaceIndex] = {
                    ...updatedInterfaces[interfaceIndex],
                    ipAddress: []
                  }
                }
              })
              updatedVM.networkInterfaces = updatedInterfaces

              const allIPs = updatedInterfaces
                .flatMap((nic: any) => (Array.isArray(nic.ipAddress) ? nic.ipAddress : []))
                .filter((ip: string) => ip && ip.trim() !== '')
                .join(', ')
              updatedVM.ip = allIPs || '—'
            } else {
              const firstUpdate = vmUpdates[0]
              if (firstUpdate) {
                updatedVM.ip = firstUpdate.ip
              }
              const hasClear = vmClears.some((c) => c.interfaceIndex === 0)
              if (hasClear) {
                updatedVM.ip = '—'
              }
            }

            return updatedVM
          })
        )

        handleCloseBulkEditDialog()
      }
    } catch (error) {
      console.error('Error in bulk IP validation/assignment:', error)
      reportError(error as Error, {
        context: 'bulk-ip-validation-assignment',
        metadata: {
          bulkEditIPs: bulkEditIPs as unknown as Record<string, unknown>,
          action: 'bulk-ip-validation-assignment'
        }
      })
    } finally {
      setAssigningIPs(false)
    }
  }

  const handleOpenBulkIPAssignment = () => {
    if (selectedVMs.length === 0) return

    // Initialize bulk edit IPs for selected VMs
    const initialBulkEditIPs: Record<string, Record<number, string>> = {}
    const initialBulkPreserveIp: Record<string, Record<number, boolean>> = {}
    const initialBulkPreserveMac: Record<string, Record<number, boolean>> = {}
    const initialBulkExistingIPs: Record<string, Record<number, string>> = {}
    const initialValidationStatus: Record<
      string,
      Record<number, 'empty' | 'valid' | 'invalid' | 'validating'>
    > = {}

    selectedVMs.forEach((vmId) => {
      const vm = vmsWithAssignments.find((v) => v.id === vmId)
      if (!vm) return

      initialBulkEditIPs[vm.id] = {}
      initialBulkPreserveIp[vm.id] = {}
      initialBulkPreserveMac[vm.id] = {}
      initialBulkExistingIPs[vm.id] = {}
      initialValidationStatus[vm.id] = {}

      const isPoweredOff = vm.powerState !== 'powered-on'

      if (vm.networkInterfaces && vm.networkInterfaces.length > 0) {
        vm.networkInterfaces.forEach((nic, index) => {
          const existingIp = (Array.isArray(nic.ipAddress) ? nic.ipAddress : [])
            .filter((ip) => ip && ip.trim() !== '')
            .join(', ')
          initialBulkExistingIPs[vm.id][index] = existingIp
          initialBulkEditIPs[vm.id][index] = existingIp

          const effectivePreserveIp = isPoweredOff ? false : vm.preserveIp?.[index] !== false
          initialBulkPreserveIp[vm.id][index] = effectivePreserveIp
          initialBulkPreserveMac[vm.id][index] = vm.preserveMac?.[index] !== false
          initialValidationStatus[vm.id][index] = existingIp ? 'valid' : 'empty'
        })
      } else {
        const existingIp = extractFirstIPv4(vm.ip && vm.ip !== '—' ? vm.ip : '')
        initialBulkExistingIPs[vm.id][0] = existingIp
        initialBulkEditIPs[vm.id][0] = existingIp
        initialBulkPreserveIp[vm.id][0] = isPoweredOff ? false : vm.preserveIp?.[0] !== false
        initialBulkPreserveMac[vm.id][0] = vm.preserveMac?.[0] !== false
        initialValidationStatus[vm.id][0] = existingIp ? 'valid' : 'empty'
      }
    })

    setBulkEditIPs(initialBulkEditIPs)
    setBulkPreserveIp(initialBulkPreserveIp)
    setBulkPreserveMac(initialBulkPreserveMac)
    setBulkExistingIPs(initialBulkExistingIPs)
    setBulkValidationStatus(initialValidationStatus)
    setBulkValidationMessages({})
    setBulkEditDialogOpen(true)
  }

  return {
    assigningIPs,
    bulkEditDialogOpen,
    bulkEditIPs,
    bulkPreserveIp,
    bulkPreserveMac,
    bulkExistingIPs,
    bulkValidationStatus,
    bulkValidationMessages,
    hasBulkIpValidationErrors,
    hasBulkIpsToApply,
    handleCloseBulkEditDialog,
    handleBulkPreserveIpChange,
    handleBulkPreserveMacChange,
    handleBulkIpChange,
    handleClearAllIPs,
    handleApplyBulkIPs,
    handleOpenBulkIPAssignment
  }
}
