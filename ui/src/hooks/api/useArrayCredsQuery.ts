import { useQuery, useMutation, useQueryClient, UseQueryOptions } from '@tanstack/react-query'
import {
  getArrayCreds,
  getArrayCredsById,
  createArrayCreds,
  updateArrayCreds,
  deleteArrayCreds,
  ArrayCreds,
  ArrayCredsFormData,
} from '../../api/array-creds'

export const ARRAY_CREDS_QUERY_KEY = 'arraycreds'

export const useArrayCredsQuery = (options?: UseQueryOptions<ArrayCreds[], Error>) => {
  return useQuery<ArrayCreds[], Error>({
    queryKey: [ARRAY_CREDS_QUERY_KEY],
    queryFn: getArrayCreds,
    ...options,
  })
}

export const useArrayCredsByIdQuery = (
  name: string,
  options?: UseQueryOptions<ArrayCreds, Error>
) => {
  return useQuery<ArrayCreds, Error>({
    queryKey: [ARRAY_CREDS_QUERY_KEY, name],
    queryFn: () => getArrayCredsById(name),
    enabled: !!name,
    ...options,
  })
}

export const useCreateArrayCredsMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: ArrayCredsFormData) => createArrayCreds(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ARRAY_CREDS_QUERY_KEY] })
    },
  })
}

export const useUpdateArrayCredsMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ name, data }: { name: string; data: Partial<ArrayCredsFormData> }) =>
      updateArrayCreds(name, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ARRAY_CREDS_QUERY_KEY] })
    },
  })
}

export const useDeleteArrayCredsMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (name: string) => deleteArrayCreds(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ARRAY_CREDS_QUERY_KEY] })
    },
  })
}
