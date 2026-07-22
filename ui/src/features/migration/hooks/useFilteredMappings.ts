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
}

// An empty source or target list means either side's data has not loaded yet
// (e.g. VMs not selected yet so vmwareNetworks/vmWareStorage is empty, or
// OpenStack creds still loading) — filtering now would wipe valid mappings
// (e.g. ones prefilled from a template or a retry) before both sides are ready.
export function filterMappingsBySourceAndTarget(
  mappings: ResourceMap[] | undefined,
  sourceList: string[],
  targetList: string[]
): ResourceMap[] {
  if (sourceList.length === 0 || targetList.length === 0) {
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

export function useFilteredMappings({
  params,
  vmwareNetworks,
  openstackNetworkNames,
  vmWareStorage,
  openstackStorage,
  arrayCredsNames,
  storageCopyMethod,
  validatedArrayCreds,
  onChange
}: UseFilteredMappingsParams) {
  const removedAutoArrayCredsSourcesRef = useRef<Set<string>>(new Set())

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

  useEffect(() => {
    if (mappingsNeedReconcile(filteredNetworkMappings, params.networkMappings)) {
      onChange('networkMappings')(filteredNetworkMappings)
    }
  }, [filteredNetworkMappings, onChange, params.networkMappings])

  useEffect(() => {
    if (
      storageCopyMethod === 'normal' &&
      mappingsNeedReconcile(filteredStorageMappings, params.storageMappings)
    ) {
      onChange('storageMappings')(filteredStorageMappings)
    }
  }, [filteredStorageMappings, onChange, params.storageMappings, storageCopyMethod])

  useEffect(() => {
    if (
      storageCopyMethod === 'StorageAcceleratedCopy' &&
      mappingsNeedReconcile(filteredArrayCredsMappings, params.arrayCredsMappings)
    ) {
      onChange('arrayCredsMappings')(filteredArrayCredsMappings)
    }
  }, [filteredArrayCredsMappings, onChange, params.arrayCredsMappings, storageCopyMethod])

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

      onChange('arrayCredsMappings')(nextMappings)
    },
    [onChange, params.arrayCredsMappings]
  )

  return {
    filteredNetworkMappings,
    filteredStorageMappings,
    filteredArrayCredsMappings,
    handleArrayCredsMappingsChange
  }
}
