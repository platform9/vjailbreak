import type { VolumeImageProfile } from 'src/api/volume-image-profiles/model'

// vCenter reports a guest family as "windowsGuest"/"linuxGuest", but the value reaching
// the form has been seen in other casings too, so compare loosely. MigrationOptionsAlt
// already did this while the profiles step compared with `=== 'windowsGuest'` — that
// mismatch alone could make Windows profiles look inapplicable for a Windows VM.
export function matchesOsFamily(raw: string | undefined, want: 'windows' | 'linux'): boolean {
  const family = (raw || '').trim().toLowerCase()
  return want === 'windows'
    ? family === 'windows' || family === 'windowsguest'
    : family === 'linux' || family === 'linuxguest'
}

export function hasOsFamilySelected(
  vms: Array<{ osFamily?: string }> | undefined,
  want: 'windows' | 'linux'
): boolean {
  return (vms || []).some((vm) => matchesOsFamily(vm.osFamily, want))
}

// A profile applies when its own osFamily is unset/"any", or when at least one selected VM
// runs that family.
export function isProfileApplicable(
  profile: VolumeImageProfile,
  hasWindows: boolean,
  hasLinux: boolean
): boolean {
  const family = (profile.spec?.osFamily || '').trim()
  if (!family || family === 'any') return true
  if (matchesOsFamily(family, 'windows')) return hasWindows
  if (matchesOsFamily(family, 'linux')) return hasLinux
  return false
}

// Template-saved profiles that should now be auto-selected: the profile has become
// applicable to the VMs chosen so far and isn't selected already.
//
// Mirrors the template mapping pool. Selecting a Linux VM prunes a saved Windows profile
// out of the form, so without a durable pool that profile is gone for good — adding a
// Windows VM afterwards could never bring it back.
//
// `suppressedProfiles` holds only what the operator deselected by hand, so those stay
// deselected while a profile pruned by de-selecting its VM is free to return when that VM
// is selected again.
export function selectApplicableTemplateProfiles({
  pool,
  current,
  applicableNames,
  suppressedProfiles
}: {
  pool: string[] | undefined
  current: string[]
  applicableNames: ReadonlySet<string>
  suppressedProfiles: ReadonlySet<string>
}): string[] {
  if (!pool || pool.length === 0) return []
  if (applicableNames.size === 0) return []

  const selected = new Set(current)
  const additions: string[] = []

  pool.forEach((name) => {
    if (selected.has(name)) return
    if (suppressedProfiles.has(name)) return
    if (!applicableNames.has(name)) return
    additions.push(name)
    selected.add(name)
  })

  return additions
}

// Prune-and-re-apply in one pass, so the two never race inside a single commit.
// Returns the same array instance when nothing changes, so callers can skip the write.
export function reconcileImageProfiles({
  current,
  pool,
  applicableNames,
  suppressedProfiles
}: {
  current: string[]
  pool: string[] | undefined
  applicableNames: ReadonlySet<string>
  suppressedProfiles: ReadonlySet<string>
}): { next: string[]; additions: string[] } {
  const pruned = current.filter((name) => applicableNames.has(name))
  const additions = selectApplicableTemplateProfiles({
    pool,
    current: pruned,
    applicableNames,
    suppressedProfiles
  })

  if (additions.length === 0 && pruned.length === current.length) {
    return { next: current, additions }
  }
  return { next: [...pruned, ...additions], additions }
}
