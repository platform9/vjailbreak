import { useMemo, useEffect, useCallback, useRef } from 'react'
import { ArrayCreds } from 'src/api/array-creds/model'
import type { ResourceMap, StorageCopyMethod } from '../types'

interface MappingsParams {
  networkMappings?: ResourceMap[]
  storageMappings?: ResourceMap[]
  arrayCredsMappings?: ResourceMap[]
}

interface UseFilteredMappingsParams {
  params: MappingsParams
  vmwareNetworks: string[]
  openstackNetworkNames: string[]
  vmWareStorage: string[]
  openstackStorage: string[]
  arrayCredsNames: string[]
  storageCopyMethod: StorageCopyMethod
  validatedArrayCreds: ArrayCreds[]
  onChange: (key: string) => (value: unknown) => void
  // Mappings saved by an applied template, re-applied as their sources appear.
  templatePool?: TemplateMappingPool
}

// sourceList (vmwareNetworks/vmWareStorage) is derived synchronously from the
// currently selected VMs, so an empty sourceList is always the real current
// state — no VMs selected, or none have interfaces/disks left referencing that
// mapping — and stale entries must be dropped.
export function filterMappingsBySourceAndTarget(
  mappings: ResourceMap[] | undefined,
  sourceList: string[],
  targetList: string[]
): ResourceMap[] {
  if (targetList.length === 0) {
    return mappings || []
  }
  return (mappings || []).filter(
    (mapping) => sourceList.includes(mapping.source) && targetList.includes(mapping.target)
  )
}

// Treats "never set" (undefined) the same as "already empty" ([]) so that reconciling
// a fresh/still-prefilling form (current undefined, filtered []) is never mistaken for
// a real prune and doesn't fire a write. See useFilteredMappings' network effect for
// why a phantom write here is dangerous (races an async prefill and can permanently
// stomp it).
export function mappingsNeedReconcile(
  filtered: ResourceMap[],
  current: ResourceMap[] | undefined
): boolean {
  return filtered.length !== (current || []).length
}

// Content-wise comparison, treating undefined as empty. Needed because reconciling now
// both prunes and re-adds in one pass, so a length check alone can miss a real change
// (one entry pruned, one re-added).
export function mappingsEqual(a: ResourceMap[], b: ResourceMap[] | undefined): boolean {
  const other = b || []
  if (a.length !== other.length) return false
  return a.every(
    (mapping, index) =>
      mapping.source === other[index].source && mapping.target === other[index].target
  )
}

// The mappings a template saved, kept as a durable pool so they can be re-applied as
// their sources appear. Applying a template pre-fills every saved mapping, but until VMs
// are selected none of those sources exist yet, so filterMappingsBySourceAndTarget
// legitimately prunes them all. Without a pool the operator's saved mappings would be
// gone for good by the time they pick VMs.
export interface TemplateMappingPool {
  networkMappings?: ResourceMap[]
  storageMappings?: ResourceMap[]
  arrayCredsMappings?: ResourceMap[]
}

// Pool entries that should now be auto-applied: their source has appeared among the
// selected VMs, their target exists on the destination, nothing is mapped for that
// source yet, and they have not been applied before.
//
// `suppressedSources` holds only the sources the operator deleted by hand, so those stay
// deleted. Everything else is free to re-apply, which is what makes the selection track
// the VM list: de-selecting a VM prunes its mapping, re-selecting brings it back.
// Suppression is recorded at the point of a user edit (see the change handlers below)
// rather than inferred from a mapping's disappearance — a prune and a manual delete look
// identical after the fact.
export function selectApplicableTemplateMappings({
  pool,
  current,
  availableSources,
  availableTargets,
  suppressedSources
}: {
  pool: ResourceMap[] | undefined
  current: ResourceMap[] | undefined
  availableSources: string[]
  availableTargets: string[]
  suppressedSources: ReadonlySet<string>
}): ResourceMap[] {
  if (!pool || pool.length === 0) return []
  // Nothing selected yet, or the destination hasn't loaded — applying now would either
  // be meaningless or fight the pruning pass.
  if (availableSources.length === 0 || availableTargets.length === 0) return []

  const sourceSet = new Set(availableSources)
  const targetSet = new Set(availableTargets)
  const mappedSources = new Set((current || []).map((mapping) => mapping.source))
  const additions: ResourceMap[] = []

  pool.forEach((mapping) => {
    if (mappedSources.has(mapping.source)) return
    if (suppressedSources.has(mapping.source)) return
    if (!sourceSet.has(mapping.source)) return
    // A target the destination cluster doesn't have would be pruned straight back out,
    // so skipping it here is what prevents an add/prune loop.
    if (!targetSet.has(mapping.target)) return
    additions.push(mapping)
    // Guards against a malformed pool holding two entries for one source.
    mappedSources.add(mapping.source)
  })

  return additions
}

// Folds a user edit into the suppression set: sources the edit removed are suppressed,
// sources it (re-)added are un-suppressed. Exported for unit testing.
export function recordUserMappingEdits(
  suppressed: Set<string>,
  previous: ResourceMap[] | undefined,
  next: ResourceMap[]
): void {
  const previousSources = new Set((previous || []).map((mapping) => mapping.source))
  const nextSources = new Set(next.map((mapping) => mapping.source))

  previousSources.forEach((source) => {
    if (!nextSources.has(source)) suppressed.add(source)
  })
  nextSources.forEach((source) => suppressed.delete(source))
}

export function useFilteredMappings({
  params,
  vmwareNetworks,
  openstackNetworkNames,
  vmWareStorage,
  openstackStorage,
  arrayCredsNames,
  storageCopyMethod,
  validatedArrayCreds,
  onChange,
  templatePool
}: UseFilteredMappingsParams) {
  const removedAutoArrayCredsSourcesRef = useRef<Set<string>>(new Set())
  const userRemovedNetworkSourcesRef = useRef<Set<string>>(new Set())
  const userRemovedStorageSourcesRef = useRef<Set<string>>(new Set())
  const userRemovedArrayCredsSourcesRef = useRef<Set<string>>(new Set())

  const filteredNetworkMappings = useMemo(
    () => filterMappingsBySourceAndTarget(params.networkMappings, vmwareNetworks, openstackNetworkNames),
    [params.networkMappings, vmwareNetworks, openstackNetworkNames]
  )

  const filteredStorageMappings = useMemo(
    () => filterMappingsBySourceAndTarget(params.storageMappings, vmWareStorage, openstackStorage),
    [params.storageMappings, vmWareStorage, openstackStorage]
  )

  const filteredArrayCredsMappings = useMemo(
    () => filterMappingsBySourceAndTarget(params.arrayCredsMappings, vmWareStorage, arrayCredsNames),
    [params.arrayCredsMappings, vmWareStorage, arrayCredsNames]
  )

  // Prune-and-re-apply is deliberately a single write per mapping kind. Splitting it
  // across two effects would have them race within one commit — each computing from the
  // same pre-write params — so the second would clobber the first.
  useEffect(() => {
    const additions = selectApplicableTemplateMappings({
      pool: templatePool?.networkMappings,
      current: filteredNetworkMappings,
      availableSources: vmwareNetworks,
      availableTargets: openstackNetworkNames,
      suppressedSources: userRemovedNetworkSourcesRef.current
    })
    const desired = additions.length ? [...filteredNetworkMappings, ...additions] : filteredNetworkMappings

    if (mappingsEqual(desired, params.networkMappings)) return
    onChange('networkMappings')(desired)
  }, [
    filteredNetworkMappings,
    onChange,
    params.networkMappings,
    templatePool?.networkMappings,
    vmwareNetworks,
    openstackNetworkNames
  ])

  useEffect(() => {
    if (storageCopyMethod === 'StorageAcceleratedCopy') return

    // Pruning stays scoped to "normal" exactly as before — HotAdd also uses
    // storageMappings but has never had them pruned here, and widening that now would be
    // an unrelated behaviour change. Template re-apply still runs for both.
    const base = storageCopyMethod === 'normal' ? filteredStorageMappings : params.storageMappings || []

    const additions = selectApplicableTemplateMappings({
      pool: templatePool?.storageMappings,
      current: base,
      availableSources: vmWareStorage,
      availableTargets: openstackStorage,
      suppressedSources: userRemovedStorageSourcesRef.current
    })
    const desired = additions.length ? [...base, ...additions] : base

    if (mappingsEqual(desired, params.storageMappings)) return
    onChange('storageMappings')(desired)
  }, [
    filteredStorageMappings,
    onChange,
    params.storageMappings,
    storageCopyMethod,
    templatePool?.storageMappings,
    vmWareStorage,
    openstackStorage
  ])

  useEffect(() => {
    if (storageCopyMethod !== 'StorageAcceleratedCopy') return

    const additions = selectApplicableTemplateMappings({
      pool: templatePool?.arrayCredsMappings,
      current: filteredArrayCredsMappings,
      availableSources: vmWareStorage,
      availableTargets: arrayCredsNames,
      suppressedSources: userRemovedArrayCredsSourcesRef.current
    })
    const desired = additions.length
      ? [...filteredArrayCredsMappings, ...additions]
      : filteredArrayCredsMappings

    if (mappingsEqual(desired, params.arrayCredsMappings)) return
    onChange('arrayCredsMappings')(desired)
  }, [
    filteredArrayCredsMappings,
    onChange,
    params.arrayCredsMappings,
    storageCopyMethod,
    templatePool?.arrayCredsMappings,
    vmWareStorage,
    arrayCredsNames
  ])

  // Auto-map datastores to ArrayCreds based on dataStore information in ArrayCreds status
  useEffect(() => {
    if (
      storageCopyMethod !== 'StorageAcceleratedCopy' ||
      !validatedArrayCreds.length ||
      !vmWareStorage.length
    ) {
      return
    }

    const datastoreToArrayCredsMap = new Map<string, string>()
    validatedArrayCreds.forEach((cred) => {
      const datastores = cred.status?.dataStore || []
      datastores.forEach((ds) => {
        datastoreToArrayCredsMap.set(ds.name, cred.metadata.name)
      })
    })

    const currentMappings = params.arrayCredsMappings || []
    const currentMappedSources = new Set(currentMappings.map((m) => m.source))

    const autoMappings: ResourceMap[] = []
    vmWareStorage.forEach((datastore) => {
      if (removedAutoArrayCredsSourcesRef.current.has(datastore)) {
        return
      }
      if (!currentMappedSources.has(datastore) && datastoreToArrayCredsMap.has(datastore)) {
        autoMappings.push({
          source: datastore,
          target: datastoreToArrayCredsMap.get(datastore)!
        })
      }
    })

    if (autoMappings.length > 0) {
      onChange('arrayCredsMappings')([...currentMappings, ...autoMappings])
    }
  }, [storageCopyMethod, validatedArrayCreds, vmWareStorage, params.arrayCredsMappings, onChange])

  const handleArrayCredsMappingsChange = useCallback(
    (nextMappings: ResourceMap[]) => {
      const prevMappings = params.arrayCredsMappings || []

      const prevSources = new Set(prevMappings.map((m) => m.source))
      const nextSources = new Set(nextMappings.map((m) => m.source))

      for (const source of prevSources) {
        if (!nextSources.has(source)) {
          removedAutoArrayCredsSourcesRef.current.add(source)
        }
      }

      for (const source of nextSources) {
        if (removedAutoArrayCredsSourcesRef.current.has(source)) {
          removedAutoArrayCredsSourcesRef.current.delete(source)
        }
      }

      // Also suppress template re-application for a source the operator just cleared.
      recordUserMappingEdits(
        userRemovedArrayCredsSourcesRef.current,
        params.arrayCredsMappings,
        nextMappings
      )

      onChange('arrayCredsMappings')(nextMappings)
    },
    [onChange, params.arrayCredsMappings]
  )

  // A source dropped by a user edit must stay dropped; re-adding it by hand clears the
  // suppression again. This is the only place the two can be told apart — by the time an
  // effect sees the new state, a manual delete and a de-selected VM look the same.
  const handleNetworkMappingsChange = useCallback(
    (nextMappings: ResourceMap[]) => {
      recordUserMappingEdits(
        userRemovedNetworkSourcesRef.current,
        params.networkMappings,
        nextMappings
      )
      onChange('networkMappings')(nextMappings)
    },
    [onChange, params.networkMappings]
  )

  const handleStorageMappingsChange = useCallback(
    (nextMappings: ResourceMap[]) => {
      recordUserMappingEdits(
        userRemovedStorageSourcesRef.current,
        params.storageMappings,
        nextMappings
      )
      onChange('storageMappings')(nextMappings)
    },
    [onChange, params.storageMappings]
  )

  return {
    filteredNetworkMappings,
    filteredStorageMappings,
    filteredArrayCredsMappings,
    handleArrayCredsMappingsChange,
    handleNetworkMappingsChange,
    handleStorageMappingsChange
  }
}
