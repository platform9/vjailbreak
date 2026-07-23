import type { SetStateAction } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { buildClearAllIpsUpdate, useBulkIPEdit } from './useBulkIPEdit'
import type { VmDataWithFlavor } from '../types'

describe('buildClearAllIpsUpdate', () => {
  it('empties the IP field and turns Preserve IP off for every interface', () => {
    const bulkEditIPs = { 'vm-1': { 0: '10.96.9.11' } }
    const result = buildClearAllIpsUpdate(bulkEditIPs)

    expect(result.clearedIPs).toEqual({ 'vm-1': { 0: '' } })
    expect(result.clearedStatus).toEqual({ 'vm-1': { 0: 'empty' } })
    expect(result.clearedPreserveIp).toEqual({ 'vm-1': { 0: false } })
    expect(result.clearedCurrentIPs).toEqual({ 'vm-1': { 0: '' } })
  })

  it('covers every interface on a multi-NIC VM', () => {
    const bulkEditIPs = { 'vm-1': { 0: '10.96.9.11', 1: '10.96.9.12' } }
    const result = buildClearAllIpsUpdate(bulkEditIPs)

    expect(result.clearedPreserveIp['vm-1']).toEqual({ 0: false, 1: false })
    expect(result.clearedIPs['vm-1']).toEqual({ 0: '', 1: '' })
  })

  it('covers every selected VM independently', () => {
    const bulkEditIPs = {
      'vm-1': { 0: '10.96.9.11' },
      'vm-2': { 0: '10.96.9.12' }
    }
    const result = buildClearAllIpsUpdate(bulkEditIPs)

    expect(result.clearedPreserveIp).toEqual({
      'vm-1': { 0: false },
      'vm-2': { 0: false }
    })
  })

  it('turns Preserve IP off even for a field that was already empty', () => {
    const bulkEditIPs = { 'vm-1': { 0: '' } }
    const result = buildClearAllIpsUpdate(bulkEditIPs)

    expect(result.clearedPreserveIp).toEqual({ 'vm-1': { 0: false } })
  })

  it('returns empty maps when nothing is being edited', () => {
    const result = buildClearAllIpsUpdate({})
    expect(result.clearedIPs).toEqual({})
    expect(result.clearedPreserveIp).toEqual({})
  })
})

// applyIpAssignmentsToRows prefers bulkEditOverrides (a snapshot taken at dialog-open
// time) over the live bulkPreserveIp state when persisting nic.preserveIP. Without
// handleClearAllIPs also updating bulkEditOverrides, the persisted preserveIP flag
// would silently snap back to the dialog-open value ("true") even though the user
// just turned Preserve IP off via Clear All — undermining anything downstream that
// reads nic.preserveIP (e.g. the Persist source network interfaces gate).
describe('useBulkIPEdit — Clear All end-to-end', () => {
  it('Clear All + Apply persists preserveIP=false and empties the IP, not the stale dialog-open value', async () => {
    const vm: VmDataWithFlavor = {
      id: 'vm-1',
      name: 'akeyless-vm',
      datastores: [],
      networks: ['VM Network'],
      vmState: 'running',
      networkInterfaces: [
        { mac: '00:50:56:87:3c:63', network: 'VM Network', ipAddress: ['10.96.9.11'] }
      ]
    }

    // Stable references — the hook has an internal effect keyed on vmsWithFlavor's
    // identity, so a fresh array/Set literal per render would loop forever.
    const stableVmsWithFlavor: VmDataWithFlavor[] = [vm]
    const stableSelectedVMs = new Set(['vm-1'])

    let latestVmsWithFlavor: VmDataWithFlavor[] = stableVmsWithFlavor
    const setVmsWithFlavor = vi.fn((updater: SetStateAction<VmDataWithFlavor[]>) => {
      latestVmsWithFlavor =
        typeof updater === 'function' ? updater(latestVmsWithFlavor) : updater
    })
    const setFormVms = vi.fn()

    const { result } = renderHook(() =>
      useBulkIPEdit({
        vmsWithFlavor: stableVmsWithFlavor,
        setVmsWithFlavor,
        selectedVMs: stableSelectedVMs,
        setFormVms,
        openstackCredentials: undefined,
        showToast: vi.fn(),
        reportError: vi.fn()
      })
    )

    act(() => result.current.handleOpenBulkIPAssignment())
    act(() => result.current.handleClearAllIPs())
    await act(async () => {
      await result.current.handleApplyBulkIPs()
    })

    const updatedNic = latestVmsWithFlavor[0].networkInterfaces?.[0]
    expect(updatedNic?.preserveIP).toBe(false)
    expect(updatedNic?.ipAddress).toEqual([])
  })
})
