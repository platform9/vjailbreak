import axios from 'axios'
import { createNetworkMappingJson } from 'src/api/network-mapping/helpers'
import { postNetworkMapping } from 'src/api/network-mapping/networkMappings'
import { createStorageMappingJson } from 'src/api/storage-mappings/helpers'
import { postStorageMapping } from 'src/api/storage-mappings/storageMappings'
import { createArrayCredsMappingJson } from 'src/api/arraycreds-mapping/helpers'
import { postArrayCredsMapping } from 'src/api/arraycreds-mapping/arrayCredsMapping'

type SetErrorFn = (value: { title: string; message: string } | null) => void
type GetFieldErrorsUpdater = (key: string | number) => (value: string) => void

type ReportErrorFn = (
  error: Error,
  context: {
    context: string
    metadata?: Record<string, unknown>
  }
) => void

type MappingPair = { source: string; target: string }

type K8sNamedResource = {
  metadata: {
    name: string
  }
}

type Input = {
  networkMappings: MappingPair[] | undefined
  storageMappings: MappingPair[] | undefined
  arrayCredsMappings: MappingPair[] | undefined
  storageCopyMethod: 'normal' | 'StorageAcceleratedCopy'
  setError?: SetErrorFn
  getFieldErrorsUpdater?: GetFieldErrorsUpdater
  reportError?: ReportErrorFn
}

export async function createMigrationMappingsResources(input: Input): Promise<{
  networkMapping: K8sNamedResource
  storageMapping: K8sNamedResource | null
  arrayCredsMapping: K8sNamedResource | null
}> {
  const {
    networkMappings,
    storageMappings,
    arrayCredsMappings,
    storageCopyMethod,
    setError,
    getFieldErrorsUpdater,
    reportError
  } = input

  const networkBody = createNetworkMappingJson({
    networkMappings: networkMappings ?? []
  })

  let networkMapping: K8sNamedResource
  try {
    networkMapping = (await postNetworkMapping(networkBody)) as K8sNamedResource
  } catch (err) {
    setError?.({
      title: 'Error creating network mapping',
      message: axios.isAxiosError(err) ? err?.response?.data?.message : ''
    })
    getFieldErrorsUpdater?.('networksMapping')(
      'Error creating network mapping : ' +
        (axios.isAxiosError(err) ? err?.response?.data?.message : err)
    )
    throw err
  }

  let storageMapping: K8sNamedResource | null = null
  let arrayCredsMapping: K8sNamedResource | null = null

  if (storageCopyMethod === 'StorageAcceleratedCopy') {
    const arrayBody = createArrayCredsMappingJson({
      mappings: arrayCredsMappings ?? []
    })

    try {
      arrayCredsMapping = (await postArrayCredsMapping(arrayBody)) as K8sNamedResource
    } catch (err) {
      reportError?.(err as Error, {
        context: 'arraycreds-mapping-creation',
        metadata: {
          arrayCredsMappingsParams: arrayCredsMappings,
          action: 'create-arraycreds-mapping'
        }
      })
      setError?.({
        title: 'Error creating ArrayCreds mapping',
        message: axios.isAxiosError(err) ? err?.response?.data?.message : ''
      })
      getFieldErrorsUpdater?.('storageMapping')(
        'Error creating ArrayCreds mapping : ' +
          (axios.isAxiosError(err) ? err?.response?.data?.message : err)
      )
      throw err
    }
  } else {
    const storageBody = createStorageMappingJson({
      storageMappings: storageMappings ?? []
    })

    try {
      storageMapping = (await postStorageMapping(storageBody)) as K8sNamedResource
    } catch (err) {
      reportError?.(err as Error, {
        context: 'storage-mapping-creation',
        metadata: {
          storageMappingsParams: storageMappings,
          action: 'create-storage-mapping'
        }
      })
      setError?.({
        title: 'Error creating storage mapping',
        message: axios.isAxiosError(err) ? err?.response?.data?.message : ''
      })
      getFieldErrorsUpdater?.('storageMapping')(
        'Error creating storage mapping : ' +
          (axios.isAxiosError(err) ? err?.response?.data?.message : err)
      )
      throw err
    }
  }

  return { networkMapping, storageMapping, arrayCredsMapping }
}
