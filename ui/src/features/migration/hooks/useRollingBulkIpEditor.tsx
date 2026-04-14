import { useCallback, useMemo, useState } from 'react'
import { CircularProgress, InputAdornment } from '@mui/material'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import { validateOpenstackIPs } from 'src/api/openstack-creds/openstackCreds'
import { patchVMwareMachine } from 'src/api/vmware-machines/vmwareMachines'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import type { GridRowSelectionModel } from '@mui/x-data-grid'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import type { VM } from 'src/features/migration/hooks/useRollingVmwareInventory'

type ReportErrorFn = (
  error: Error,
  context: {
    context: string
    metadata?: Record<string, unknown>
  }
) => void

type BulkValidationStatus = 'empty' | 'valid' | 'invalid' | 'validating'

type BulkMap<T> = Record<string, Record<number, T>>

const IPV4_FULL_REGEX =
  /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/

const parseIpList = (value: string): string[] => {
  const trimmed = value.trim()
  if (!trimmed) return []
  return trimmed.split(/\s*,\s*/).filter((v) => v !== '')
}

const isValidIPAddressList = (value: string): boolean => {
  const ips = parseIpList(value)
  if (ips.length === 0) return false
  return ips.every((ip) => IPV4_FULL_REGEX.test(ip))
}

export function useRollingBulkIpEditor({
  selectedVMs,
  vmsWithAssignments,
  setVmsWithAssignments,
  openstackCredData,
  reportError,
  extractFirstIPv4
}: {
  selectedVMs: GridRowSelectionModel
  vmsWithAssignments: VM[]
  setVmsWithAssignments: (value: VM[] | ((prev: VM[]) => VM[])) => void
  openstackCredData: OpenstackCreds | null
  reportError: ReportErrorFn
  extractFirstIPv4: (value: string) => string
}) {
  const [assigningIPs, setAssigningIPs] = useState(false)
  const [bulkEditDialogOpen, setBulkEditDialogOpen] = useState(false)
  const [bulkEditIPs, setBulkEditIPs] = useState<BulkMap<string>>({})
  const [bulkPreserveIp, setBulkPreserveIp] = useState<BulkMap<boolean>>({})
  const [bulkPreserveMac, setBulkPreserveMac] = useState<BulkMap<boolean>>({})
  const [bulkExistingIPs, setBulkExistingIPs] = useState<BulkMap<string>>({})
  const [bulkValidationStatus, setBulkValidationStatus] = useState<BulkMap<BulkValidationStatus>>({})
  const [bulkValidationMessages, setBulkValidationMessages] = useState<BulkMap<string>>({})

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

  const handleCloseBulkEditDialog = useCallback(() => {
    setBulkEditDialogOpen(false)
    setBulkEditIPs({})
    setBulkPreserveIp({})
    setBulkPreserveMac({})
    setBulkExistingIPs({})
    setBulkValidationStatus({})
    setBulkValidationMessages({})
  }, [])

  const handleBulkPreserveIpChange = useCallback(
    (vmId: string, interfaceIndex: number, value: boolean) => {
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
        const current = bulkEditIPs?.[vmId]?.[interfaceIndex] ?? ''
        const trimmed = current.trim()
        const next = !trimmed
          ? { status: 'empty' as const, message: '' }
          : !isValidIPAddressList(trimmed)
            ? ({ status: 'invalid' as const, message: 'Invalid IP format' } as const)
            : ({ status: 'valid' as const, message: '' } as const)

        setBulkValidationStatus((prev) => ({
          ...prev,
          [vmId]: { ...prev[vmId], [interfaceIndex]: next.status }
        }))
        setBulkValidationMessages((prev) => ({
          ...prev,
          [vmId]: { ...prev[vmId], [interfaceIndex]: next.message }
        }))
      }
    },
    [bulkEditIPs, bulkExistingIPs]
  )

  const handleBulkPreserveMacChange = useCallback((vmId: string, interfaceIndex: number, value: boolean) => {
    setBulkPreserveMac((prev) => ({
      ...prev,
      [vmId]: { ...prev[vmId], [interfaceIndex]: value }
    }))
  }, [])

  const handleBulkIpChange = useCallback((vmId: string, interfaceIndex: number, value: string) => {
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
  }, [])

  const handleClearAllIPs = useCallback(() => {
    const clearedIPs: BulkMap<string> = {}
    const clearedStatus: BulkMap<BulkValidationStatus> = {}

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
  }, [bulkEditIPs])

  const renderValidationAdornment = useCallback(
    (status?: BulkValidationStatus) => {
      if (!status || status === 'empty') return null

      if (status === 'validating') {
        return (
          <InputAdornment position="end" sx={{ alignItems: 'center' }}>
            <CircularProgress size={16} />
          </InputAdornment>
        )
      }

      if (status === 'valid') {
        return (
          <InputAdornment position="end" sx={{ alignItems: 'center' }}>
            <CheckCircleIcon color="success" fontSize="small" />
          </InputAdornment>
        )
      }

      if (status === 'invalid') {
        return (
          <InputAdornment position="end" sx={{ alignItems: 'center' }}>
            <ErrorIcon color="error" fontSize="small" />
          </InputAdornment>
        )
      }

      return null
    },
    []
  )

  const handleOpenBulkIPAssignment = useCallback(() => {
    if (selectedVMs.length === 0) return

    const initialBulkEditIPs: BulkMap<string> = {}
    const initialBulkPreserveIp: BulkMap<boolean> = {}
    const initialBulkPreserveMac: BulkMap<boolean> = {}
    const initialBulkExistingIPs: BulkMap<string> = {}
    const initialValidationStatus: BulkMap<BulkValidationStatus> = {}

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
  }, [extractFirstIPv4, selectedVMs, vmsWithAssignments])

  const handleApplyBulkIPs = useCallback(async () => {
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

    if (missingRequiredIp) return

    Object.entries(bulkEditIPs).forEach(([vmId, interfaces]) => {
      Object.entries(interfaces).forEach(([interfaceIndexStr, ip]) => {
        const interfaceIndex = parseInt(interfaceIndexStr)
        const preserveIp = bulkPreserveIp?.[vmId]?.[interfaceIndex] !== false
        const existingIp = bulkExistingIPs?.[vmId]?.[interfaceIndex] || ''
        const typedIp = ip.trim()

        if (!preserveIp && typedIp === '') {
          if (existingIp.trim() !== '') {
            clearIpsToApply.push({ vmId, interfaceIndex })
          }
          return
        }

        if (typedIp === '') return

        if (preserveIp && existingIp.trim() !== '' && typedIp === existingIp.trim()) {
          return
        }

        ipsToApply.push({ vmId, interfaceIndex, ip: typedIp })
      })
    })

    if (ipsToApply.length === 0 && clearIpsToApply.length === 0) {
      setVmsWithAssignments((prev) =>
        prev.map((vm) => {
          const preserveIp = bulkPreserveIp[vm.id]
          const preserveMac = bulkPreserveMac[vm.id]
          if (!preserveIp && !preserveMac) return vm

          return {
            ...vm,
            ...(preserveIp && { preserveIp }),
            ...(preserveMac && { preserveMac })
          }
        })
      )
      handleCloseBulkEditDialog()
      return
    }

    setAssigningIPs(true)

    try {
      if (openstackCredData) {
        const flattenedIps: Array<{ vmId: string; interfaceIndex: number; ip: string }> = []
        ipsToApply.forEach((item) => {
          const parsed = parseIpList(item.ip)
          if (parsed.length === 0) {
            flattenedIps.push({ ...item, ip: '' })
            return
          }
          parsed.forEach((ip) => flattenedIps.push({ vmId: item.vmId, interfaceIndex: item.interfaceIndex, ip }))
        })

        const ipList = flattenedIps.map((item) => item.ip)

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

        let hasInvalidIPs = false
        ipsToApply.forEach((item) => {
          const key = `${item.vmId}__${item.interfaceIndex}`
          const result = byInterfaceKey.get(key)
          const ok = result?.ok !== false

          if (ok) {
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

        if (hasInvalidIPs) return

        const updatePromises = ipsToApply.map(async ({ vmId, interfaceIndex, ip }) => {
          const vm = vmsWithAssignments.find((v) => v.id === vmId)
          if (!vm) throw new Error('VM not found')

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
        })

        const clearPromises = clearIpsToApply.map(async ({ vmId, interfaceIndex }) => {
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
        })

        const results = await Promise.all([...updatePromises, ...clearPromises])
        const failedUpdates = results.filter((result) => !result.success)
        if (failedUpdates.length > 0) return

        setVmsWithAssignments((prev) =>
          prev.map((vm) => {
            const preserveIp = bulkPreserveIp[vm.id]
            const preserveMac = bulkPreserveMac[vm.id]
            const vmUpdates = ipsToApply.filter((item) => item.vmId === vm.id)
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
                .flatMap((nic) => (Array.isArray(nic.ipAddress) ? nic.ipAddress : []))
                .filter((ip) => ip && ip.trim() !== '')
                .join(', ')
              updatedVM.ip = allIPs || '—'
            } else {
              const firstUpdate = ipsToApply.find((u) => u.vmId === vm.id)
              if (firstUpdate) updatedVM.ip = firstUpdate.ip
              const hasClear = vmClears.some((c) => c.interfaceIndex === 0)
              if (hasClear) updatedVM.ip = '—'
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
          bulkEditIPs: bulkEditIPs,
          action: 'bulk-ip-validation-assignment'
        }
      })
    } finally {
      setAssigningIPs(false)
    }
  }, [
    bulkEditIPs,
    bulkExistingIPs,
    bulkPreserveIp,
    bulkPreserveMac,
    openstackCredData,
    reportError,
    setVmsWithAssignments,
    vmsWithAssignments,
    handleCloseBulkEditDialog
  ])

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

    setBulkValidationMessages,
    handleOpenBulkIPAssignment,
    handleCloseBulkEditDialog,
    handleClearAllIPs,
    handleApplyBulkIPs,
    handleBulkIpChange,
    handleBulkPreserveIpChange,
    handleBulkPreserveMacChange,
    renderValidationAdornment
  }
}
