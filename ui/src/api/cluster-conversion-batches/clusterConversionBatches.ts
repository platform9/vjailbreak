import axios from '../axios'
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'
import { ClusterConversionBatch, ClusterConversionBatchList } from './model'

export const getClusterConversionBatches = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<ClusterConversionBatch[]> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/clusterconversionbatches`
  const response = await axios.get<ClusterConversionBatchList>({ endpoint })
  return response?.items || []
}

export const getClusterConversionBatch = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<ClusterConversionBatch> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/clusterconversionbatches/${name}`
  return axios.get<ClusterConversionBatch>({ endpoint })
}

export const postClusterConversionBatch = async (
  body: unknown,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<ClusterConversionBatch> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/clusterconversionbatches`
  return axios.post<ClusterConversionBatch>({ endpoint, data: body })
}

export const deleteClusterConversionBatch = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<void> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/clusterconversionbatches/${name}`
  await axios.del<ClusterConversionBatch>({ endpoint })
}

export const patchClusterConversionBatch = async (
  name: string,
  body: unknown,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<ClusterConversionBatch> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/clusterconversionbatches/${name}`
  return axios.patch<ClusterConversionBatch>({
    endpoint,
    data: body,
    config: {
      headers: {
        'Content-Type': 'application/merge-patch+json'
      }
    }
  })
}
