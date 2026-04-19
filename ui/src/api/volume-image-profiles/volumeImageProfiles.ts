import axios from '../axios'
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'
import { VolumeImageProfile, VolumeImageProfileList, VolumeImageProfileSpec } from './model'

const resourcePath = (namespace: string) =>
  `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/volumeimageprofiles`

export const getVolumeImageProfilesList = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<VolumeImageProfile[]> => {
  const response = await axios.get<VolumeImageProfileList>({
    endpoint: resourcePath(namespace)
  })
  return response?.items ?? []
}

export const getVolumeImageProfile = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<VolumeImageProfile> => {
  const response = await axios.get<VolumeImageProfile>({
    endpoint: `${resourcePath(namespace)}/${name}`
  })
  return response
}

export const createVolumeImageProfile = async (
  name: string,
  spec: VolumeImageProfileSpec,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<VolumeImageProfile> => {
  const body: VolumeImageProfile = {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'VolumeImageProfile',
    metadata: { name, namespace },
    spec
  }
  return axios.post<VolumeImageProfile>({ endpoint: resourcePath(namespace), data: body })
}

export const updateVolumeImageProfile = async (
  profile: VolumeImageProfile,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<VolumeImageProfile> => {
  return axios.put<VolumeImageProfile>({
    endpoint: `${resourcePath(namespace)}/${profile.metadata.name}`,
    data: profile
  })
}

export const deleteVolumeImageProfile = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<void> => {
  await axios.del<void>({ endpoint: `${resourcePath(namespace)}/${name}` })
}
