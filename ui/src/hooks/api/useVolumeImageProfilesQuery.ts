import { useQuery, UseQueryOptions, UseQueryResult } from '@tanstack/react-query'
import { getVolumeImageProfilesList } from 'src/api/volume-image-profiles/volumeImageProfiles'
import { VolumeImageProfile } from 'src/api/volume-image-profiles/model'

export const VOLUME_IMAGE_PROFILES_QUERY_KEY = ['volumeImageProfiles']

type Options = Omit<UseQueryOptions<VolumeImageProfile[]>, 'queryKey' | 'queryFn'>

export const useVolumeImageProfilesQuery = (
  namespace = undefined,
  options: Options = {}
): UseQueryResult<VolumeImageProfile[]> => {
  return useQuery<VolumeImageProfile[]>({
    queryKey: [...VOLUME_IMAGE_PROFILES_QUERY_KEY, namespace],
    queryFn: async () => getVolumeImageProfilesList(namespace),
    staleTime: Infinity,
    refetchOnWindowFocus: true,
    ...options
  })
}