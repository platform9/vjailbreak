import { useState, useRef, useCallback, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { revalidateCredentials } from 'src/api/helpers'
import { useVmwareCredentialsQuery, VMWARE_CREDS_QUERY_KEY } from 'src/hooks/api/useVmwareCredentialsQuery'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { VMwareCreds } from 'src/api/vmware-creds/model'

const REVALIDATION_TIMEOUT_MS = 31 * 60 * 1000

const normalizeStatus = (status?: string) => {
  if (!status) return 'Unknown'
  return status === 'Validating' ? 'Revalidating' : status
}

interface UseVmwareRevalidationProps {
  vmwareCredName?: string
  onRevalidationComplete?: () => void
}

export function useVmwareRevalidation({
  vmwareCredName,
  onRevalidationComplete,
}: UseVmwareRevalidationProps) {
  const { reportError } = useErrorHandler({ component: 'VmsSelectionStep' })
  const queryClient = useQueryClient()
  const [isRevalidating, setIsRevalidating] = useState(false)
  const [timedOutRevalidating, setTimedOutRevalidating] = useState(false)
  const revalidationStartDataUpdatedAtRef = useRef(0)
  const revalidationTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const clearRevalidationTimeout = useCallback(() => {
    if (revalidationTimeoutRef.current) {
      clearTimeout(revalidationTimeoutRef.current)
      revalidationTimeoutRef.current = null
    }
  }, [])

  const clearActiveRevalidation = useCallback(() => {
    clearRevalidationTimeout()
    setIsRevalidating(false)
  }, [clearRevalidationTimeout])

  const { data: vmwareCreds, dataUpdatedAt } = useVmwareCredentialsQuery(undefined, {
    enabled: !!vmwareCredName,
    staleTime: 0,
    refetchOnMount: false,
    refetchInterval: (query) => {
      const data = query.state.data as VMwareCreds[] | undefined
      const targetCred = data?.find((cred) => cred.metadata.name === vmwareCredName)
      const isBackendRevalidating =
        !timedOutRevalidating &&
        normalizeStatus(targetCred?.status?.vmwareValidationStatus) === 'Revalidating'
      return isRevalidating || isBackendRevalidating ? 5000 : false
    },
  })

  const { mutate: revalidate, isPending: isRevalidationApiPending } = useMutation({
    mutationFn: revalidateCredentials,
    onSuccess: () => {
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: VMWARE_CREDS_QUERY_KEY })
      }, 500)
    },
    onError: (error: unknown) => {
      reportError(error instanceof Error ? error : new Error(String(error)), {
        context: 'vmware-credentials-revalidation',
        metadata: { credentialName: vmwareCredName },
      })
      clearActiveRevalidation()
    },
  })

  useEffect(() => {
    if (!isRevalidating || !vmwareCredName) return
    const targetCred = vmwareCreds?.find((cred) => cred.metadata.name === vmwareCredName)
    const hasFreshStatus = dataUpdatedAt > revalidationStartDataUpdatedAtRef.current

    if (targetCred) {
      const status = normalizeStatus(targetCred.status?.vmwareValidationStatus)
      if (!isRevalidationApiPending && hasFreshStatus && status !== 'Revalidating') {
        clearActiveRevalidation()
        setTimedOutRevalidating(false)
        onRevalidationComplete?.()
      }
    } else {
      clearActiveRevalidation()
      setTimedOutRevalidating(false)
    }
  }, [
    vmwareCreds,
    dataUpdatedAt,
    isRevalidating,
    isRevalidationApiPending,
    vmwareCredName,
    clearActiveRevalidation,
    onRevalidationComplete,
  ])

  useEffect(() => clearRevalidationTimeout, [clearRevalidationTimeout])

  const handleRefreshAndRevalidate = useCallback(() => {
    if (!vmwareCredName) return
    const cred = vmwareCreds?.find((c) => c.metadata.name === vmwareCredName)
    const credNamespace = cred?.metadata?.namespace || VJAILBREAK_DEFAULT_NAMESPACE

    revalidationStartDataUpdatedAtRef.current = dataUpdatedAt
    clearRevalidationTimeout()
    setTimedOutRevalidating(false)
    setIsRevalidating(true)

    revalidationTimeoutRef.current = setTimeout(() => {
      revalidationTimeoutRef.current = null
      setTimedOutRevalidating(true)
      setIsRevalidating(false)
      reportError(
        new Error(
          'VMware credential revalidation is still in progress after 31 minutes. You can retry.'
        ),
        {
          context: 'vmware-revalidation-timeout',
          metadata: { credentialName: vmwareCredName },
        }
      )
      queryClient.invalidateQueries({ queryKey: VMWARE_CREDS_QUERY_KEY })
    }, REVALIDATION_TIMEOUT_MS)

    revalidate({
      name: vmwareCredName,
      namespace: credNamespace,
      kind: 'VmwareCreds',
    })
  }, [
    vmwareCredName,
    vmwareCreds,
    dataUpdatedAt,
    clearRevalidationTimeout,
    revalidate,
    reportError,
    queryClient,
  ])

  return { isRevalidating, timedOutRevalidating, handleRefreshAndRevalidate }
}
