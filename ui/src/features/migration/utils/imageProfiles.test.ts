import { describe, expect, it } from 'vitest'
import {
  hasOsFamilySelected,
  isProfileApplicable,
  matchesOsFamily,
  reconcileImageProfiles,
  selectApplicableTemplateProfiles
} from './imageProfiles'
import type { VolumeImageProfile } from 'src/api/volume-image-profiles/model'

const profile = (name: string, osFamily?: string): VolumeImageProfile =>
  ({ metadata: { name }, spec: { osFamily } }) as VolumeImageProfile

describe('matchesOsFamily', () => {
  it('accepts the vCenter form and plain form for windows', () => {
    expect(matchesOsFamily('windowsGuest', 'windows')).toBe(true)
    expect(matchesOsFamily('windows', 'windows')).toBe(true)
  })

  it('accepts the vCenter form and plain form for linux', () => {
    expect(matchesOsFamily('linuxGuest', 'linux')).toBe(true)
    expect(matchesOsFamily('linux', 'linux')).toBe(true)
  })

  it('is case and whitespace insensitive', () => {
    expect(matchesOsFamily('  WindowsGuest ', 'windows')).toBe(true)
    expect(matchesOsFamily('LINUX', 'linux')).toBe(true)
  })

  it('does not cross families', () => {
    expect(matchesOsFamily('linuxGuest', 'windows')).toBe(false)
    expect(matchesOsFamily('windowsGuest', 'linux')).toBe(false)
  })

  it('rejects unknown and missing values', () => {
    expect(matchesOsFamily(undefined, 'linux')).toBe(false)
    expect(matchesOsFamily('', 'linux')).toBe(false)
    expect(matchesOsFamily('otherGuestFamily', 'linux')).toBe(false)
  })
})

describe('hasOsFamilySelected', () => {
  it('detects a family across a mixed selection', () => {
    const vms = [{ osFamily: 'linuxGuest' }, { osFamily: 'windowsGuest' }]
    expect(hasOsFamilySelected(vms, 'windows')).toBe(true)
    expect(hasOsFamilySelected(vms, 'linux')).toBe(true)
  })

  it('returns false for an empty or missing selection', () => {
    expect(hasOsFamilySelected([], 'linux')).toBe(false)
    expect(hasOsFamilySelected(undefined, 'linux')).toBe(false)
  })

  it('ignores VMs with no detected OS', () => {
    expect(hasOsFamilySelected([{ osFamily: 'Unknown' }], 'linux')).toBe(false)
  })
})

describe('isProfileApplicable', () => {
  it('always applies a profile with no OS family', () => {
    expect(isProfileApplicable(profile('generic'), false, false)).toBe(true)
    expect(isProfileApplicable(profile('generic', 'any'), false, false)).toBe(true)
  })

  it('applies an OS-specific profile only when that family is selected', () => {
    expect(isProfileApplicable(profile('win', 'windowsGuest'), true, false)).toBe(true)
    expect(isProfileApplicable(profile('win', 'windowsGuest'), false, true)).toBe(false)
    expect(isProfileApplicable(profile('lin', 'linuxGuest'), false, true)).toBe(true)
    expect(isProfileApplicable(profile('lin', 'linuxGuest'), true, false)).toBe(false)
  })

  it('applies a profile written in the plain casing — the mismatch that hid Windows profiles', () => {
    expect(isProfileApplicable(profile('win', 'windows'), true, false)).toBe(true)
  })
})

describe('selectApplicableTemplateProfiles', () => {
  const none: ReadonlySet<string> = new Set()

  it('applies nothing before applicability is known', () => {
    expect(
      selectApplicableTemplateProfiles({
        pool: ['default-linux'],
        current: [],
        applicableNames: new Set(),
        suppressedProfiles: none
      })
    ).toEqual([])
  })

  it('applies a saved profile once it becomes applicable', () => {
    expect(
      selectApplicableTemplateProfiles({
        pool: ['default-linux', 'default-windows'],
        current: [],
        applicableNames: new Set(['default-linux']),
        suppressedProfiles: none
      })
    ).toEqual(['default-linux'])
  })

  it('does not re-apply one already selected', () => {
    expect(
      selectApplicableTemplateProfiles({
        pool: ['default-linux'],
        current: ['default-linux'],
        applicableNames: new Set(['default-linux']),
        suppressedProfiles: none
      })
    ).toEqual([])
  })

  it('does not resurrect one the operator removed', () => {
    expect(
      selectApplicableTemplateProfiles({
        pool: ['default-linux'],
        current: [],
        applicableNames: new Set(['default-linux']),
        suppressedProfiles: new Set(['default-linux'])
      })
    ).toEqual([])
  })

  it('ignores a profile that is no longer in the system', () => {
    expect(
      selectApplicableTemplateProfiles({
        pool: ['deleted-profile'],
        current: [],
        applicableNames: new Set(['default-linux']),
        suppressedProfiles: none
      })
    ).toEqual([])
  })

  it('applies nothing when the template saved no profiles', () => {
    expect(
      selectApplicableTemplateProfiles({
        pool: undefined,
        current: [],
        applicableNames: new Set(['default-linux']),
        suppressedProfiles: none
      })
    ).toEqual([])
  })
})

describe('reconcileImageProfiles', () => {
  const none: ReadonlySet<string> = new Set()
  const POOL = ['default-linux', 'default-windows']

  it('keeps only the Linux profile while just a Linux VM is selected', () => {
    const { next } = reconcileImageProfiles({
      current: POOL,
      pool: POOL,
      applicableNames: new Set(['default-linux']),
      suppressedProfiles: none
    })
    expect(next).toEqual(['default-linux'])
  })

  it('brings the Windows profile back when a Windows VM is added — the reported bug', () => {
    // Step 1: Linux VM only. The Windows profile is pruned out of the form.
    const applied = new Set<string>()
    const first = reconcileImageProfiles({
      current: POOL,
      pool: POOL,
      applicableNames: new Set(['default-linux']),
      suppressedProfiles: applied
    })
    first.additions.forEach((name) => applied.add(name))
    expect(first.next).toEqual(['default-linux'])

    // Step 2: a Windows VM is added, so the saved Windows profile applies again.
    const second = reconcileImageProfiles({
      current: first.next,
      pool: POOL,
      applicableNames: new Set(['default-linux', 'default-windows']),
      suppressedProfiles: applied
    })

    expect(second.next).toEqual(['default-linux', 'default-windows'])
  })

  it('returns the same array instance when nothing changes, so no write is issued', () => {
    const current = ['default-linux']
    const { next } = reconcileImageProfiles({
      current,
      pool: POOL,
      applicableNames: new Set(['default-linux']),
      suppressedProfiles: new Set(['default-linux'])
    })
    expect(next).toBe(current)
  })

  it('settles after applying — a second pass adds nothing', () => {
    const applied = new Set<string>()
    const first = reconcileImageProfiles({
      current: [],
      pool: POOL,
      applicableNames: new Set(['default-linux', 'default-windows']),
      suppressedProfiles: applied
    })
    first.additions.forEach((name) => applied.add(name))

    const second = reconcileImageProfiles({
      current: first.next,
      pool: POOL,
      applicableNames: new Set(['default-linux', 'default-windows']),
      suppressedProfiles: applied
    })

    expect(second.next).toBe(first.next)
    expect(second.additions).toEqual([])
  })

  it('prunes without adding when there is no template pool', () => {
    const { next, additions } = reconcileImageProfiles({
      current: ['default-linux', 'stale-profile'],
      pool: undefined,
      applicableNames: new Set(['default-linux']),
      suppressedProfiles: none
    })
    expect(next).toEqual(['default-linux'])
    expect(additions).toEqual([])
  })

  it('does not re-add a profile the operator deselected while it was applicable', () => {
    const { next } = reconcileImageProfiles({
      current: [],
      pool: POOL,
      applicableNames: new Set(['default-linux', 'default-windows']),
      suppressedProfiles: new Set(POOL)
    })
    expect(next).toEqual([])
  })
})

describe('reconcileImageProfiles — tracking the current VM selection', () => {
  const POOL = ['default-linux', 'default-windows']
  const LINUX_ONLY = new Set(['default-linux'])
  const BOTH = new Set(['default-linux', 'default-windows'])
  const none: ReadonlySet<string> = new Set()

  it('re-selects a profile when its VM is de-selected and selected again', () => {
    // Linux VM selected → Linux profile applied.
    const applied = reconcileImageProfiles({
      current: [],
      pool: POOL,
      applicableNames: LINUX_ONLY,
      suppressedProfiles: none
    })
    expect(applied.next).toEqual(['default-linux'])

    // VM de-selected → nothing applicable, so the profile is pruned.
    const cleared = reconcileImageProfiles({
      current: applied.next,
      pool: POOL,
      applicableNames: new Set(),
      suppressedProfiles: none
    })
    expect(cleared.next).toEqual([])

    // VM selected again → the profile returns.
    const restored = reconcileImageProfiles({
      current: cleared.next,
      pool: POOL,
      applicableNames: LINUX_ONLY,
      suppressedProfiles: none
    })
    expect(restored.next).toEqual(['default-linux'])
  })

  it('keeps only the remaining OS family when one of two VMs is de-selected', () => {
    const both = reconcileImageProfiles({
      current: [],
      pool: POOL,
      applicableNames: BOTH,
      suppressedProfiles: none
    })
    expect(both.next).toEqual(POOL)

    // The Windows VM is de-selected.
    const linuxOnly = reconcileImageProfiles({
      current: both.next,
      pool: POOL,
      applicableNames: LINUX_ONLY,
      suppressedProfiles: none
    })
    expect(linuxOnly.next).toEqual(['default-linux'])
  })

  it('honours a manual deselection across a VM de-select/re-select cycle', () => {
    const suppressed = new Set(['default-windows'])

    const withWindowsVm = reconcileImageProfiles({
      current: ['default-linux'],
      pool: POOL,
      applicableNames: BOTH,
      suppressedProfiles: suppressed
    })
    expect(withWindowsVm.next).toEqual(['default-linux'])

    const afterCycle = reconcileImageProfiles({
      current: [],
      pool: POOL,
      applicableNames: BOTH,
      suppressedProfiles: suppressed
    })
    expect(afterCycle.next).toEqual(['default-linux'])
  })

  it('survives repeated cycles without drift', () => {
    let current: string[] = []
    for (let cycle = 0; cycle < 3; cycle += 1) {
      current = reconcileImageProfiles({
        current,
        pool: POOL,
        applicableNames: LINUX_ONLY,
        suppressedProfiles: none
      }).next
      expect(current).toEqual(['default-linux'])

      current = reconcileImageProfiles({
        current,
        pool: POOL,
        applicableNames: new Set(),
        suppressedProfiles: none
      }).next
      expect(current).toEqual([])
    }
  })
})
