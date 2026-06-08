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

  // An empty target list means the destination data has not loaded yet — filtering
  // against it would wipe valid mappings (e.g. ones prefilled for a retry) before the
  // OpenStack credentials finish loading.
  const filteredNetworkMappings = useMemo(() => {
    if (openstackNetworkNames.length === 0) return params.networkMappings || []
    return (params.networkMappings || []).filter(
      (mapping) =>
        vmwareNetworks.includes(mapping.source) && openstackNetworkNames.includes(mapping.target)
    )
  }, [params.networkMappings, vmwareNetworks, openstackNetworkNames])

  const filteredStorageMappings = useMemo(() => {
    if (openstackStorage.length === 0) return params.storageMappings || []
    return (params.storageMappings || []).filter(
      (mapping) =>
        vmWareStorage.includes(mapping.source) && openstackStorage.includes(mapping.target)
    )
  }, [params.storageMappings, vmWareStorage, openstackStorage])

  const filteredArrayCredsMappings = useMemo(() => {
    if (arrayCredsNames.length === 0) return params.arrayCredsMappings || []
    return (params.arrayCredsMappings || []).filter(
      (mapping) =>
        vmWareStorage.includes(mapping.source) && arrayCredsNames.includes(mapping.target)
    )
  }, [params.arrayCredsMappings, vmWareStorage, arrayCredsNames])

  useEffect(() => {
    // Don't prune while the reference lists are still loading. An empty source or target list
    // can't prove any mapping invalid, and writing the empty filtered result back would wipe
    // mappings that were seeded before the lists arrived (the bucket editor preloads saved
    // mappings, then `availableVmwareNetworks`/PCD networks resolve async). Prune only once both
    // lists are present.
    if (vmwareNetworks.length === 0 || openstackNetworkNames.length === 0) return
    if (filteredNetworkMappings.length !== params.networkMappings?.length) {
      onChange('networkMappings')(filteredNetworkMappings)
    }
  }, [
    filteredNetworkMappings,
    onChange,
    params.networkMappings,
    vmwareNetworks.length,
    openstackNetworkNames.length
  ])

  useEffect(() => {
    // See the networks effect above: skip pruning until both reference lists have loaded so
    // seeded storage mappings aren't cleared during load.
    if (vmWareStorage.length === 0 || openstackStorage.length === 0) return
    if (
      storageCopyMethod === 'normal' &&
      filteredStorageMappings.length !== params.storageMappings?.length
    ) {
      onChange('storageMappings')(filteredStorageMappings)
    }
  }, [
    filteredStorageMappings,
    onChange,
    params.storageMappings,
    storageCopyMethod,
    vmWareStorage.length,
    openstackStorage.length
  ])

  useEffect(() => {
    // See the networks effect above: skip pruning until both reference lists have loaded.
    if (vmWareStorage.length === 0 || arrayCredsNames.length === 0) return
    if (
      storageCopyMethod === 'StorageAcceleratedCopy' &&
      filteredArrayCredsMappings.length !== params.arrayCredsMappings?.length
    ) {
      onChange('arrayCredsMappings')(filteredArrayCredsMappings)
    }
  }, [
    filteredArrayCredsMappings,
    onChange,
    params.arrayCredsMappings,
    storageCopyMethod,
    vmWareStorage.length,
    arrayCredsNames.length
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
