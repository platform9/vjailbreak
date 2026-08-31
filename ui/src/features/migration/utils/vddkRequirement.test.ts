import { describe, expect, it } from 'vitest'
import { blocksOnMissingVddk, copyMethodRequiresVddk } from './vddkRequirement'

describe('copyMethodRequiresVddk', () => {
  it('requires VDDK for the normal copy path', () => {
    expect(copyMethodRequiresVddk('normal')).toBe(true)
  })

  it('treats an unset copy method as normal', () => {
    expect(copyMethodRequiresVddk(undefined)).toBe(true)
    expect(copyMethodRequiresVddk('')).toBe(true)
  })

  it('exempts StorageAcceleratedCopy, which clones via vmkfstools on the ESXi host', () => {
    expect(copyMethodRequiresVddk('StorageAcceleratedCopy')).toBe(false)
  })

  it('exempts HotAdd, which serves with qemu-nbd on the proxy VM', () => {
    expect(copyMethodRequiresVddk('HotAdd')).toBe(false)
  })

  it('requires VDDK for an unrecognised method rather than silently exempting it', () => {
    expect(copyMethodRequiresVddk('SomeFutureMethod')).toBe(true)
  })
})

describe('blocksOnMissingVddk', () => {
  it('blocks the normal path when VDDK is confirmed missing', () => {
    expect(blocksOnMissingVddk('normal', false)).toBe(true)
  })

  it('does not block the exempt methods even when VDDK is missing', () => {
    expect(blocksOnMissingVddk('StorageAcceleratedCopy', false)).toBe(false)
    expect(blocksOnMissingVddk('HotAdd', false)).toBe(false)
  })

  it('does not block when VDDK is uploaded', () => {
    expect(blocksOnMissingVddk('normal', true)).toBe(false)
  })

  it('fails open while the status is unknown', () => {
    expect(blocksOnMissingVddk('normal', undefined)).toBe(false)
  })
})
