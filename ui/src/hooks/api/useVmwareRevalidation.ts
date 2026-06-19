import { useState, useRef, useCallback, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { revalidateCredentials } from 'src/api/helpers'
import {
  useVmwareCredentialQuery,
  vmwareCredQueryKey,
} from 'src/hooks/api/useVmwareCredentialQuery'
import { VMWARE_CREDS_QUERY_KEY } from 'src/hooks/api/useVmwareCredentialsQuery'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { VMwareCreds } from 'src/api/vmware-creds/model'

const REVALIDATION_TIMEOUT_MS = 31 * 60 * 1000
const REVALIDATION_TIMEOUT_MINUTES = Math.floor(REVALIDATION_TIMEOUT_MS / (60 * 1000))

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
  // Refs keep refetchInterval closure current without stale captures
  const isRevalidatingRef = useRef(false)
  const timedOutRevalidatingRef = useRef(false)
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
    isRevalidatingRef.current = false
    setIsRevalidating(false)
  }, [clearRevalidationTimeout])

  // Poll a single credential — no O(n) list scan on every 5s tick
  const { data: vmwareCred, dataUpdatedAt } = useVmwareCredentialQuery(
    vmwareCredName ?? '',
    undefined,
    {
      enabled: !!vmwareCredName,
      staleTime: 0,
      refetchOnMount: false,
      refetchInterval: (query) => {
        const data = query.state.data as VMwareCreds | undefined
        const isBackendRevalidating =
          !timedOutRevalidatingRef.current &&
          normalizeStatus(data?.status?.vmwareValidationStatus) === 'Revalidating'
        return isRevalidatingRef.current || isBackendRevalidating ? 5000 : false
      },
    }
  )

  const { mutate: revalidate, isPending: isRevalidationApiPending } = useMutation({
    mutationFn: revalidateCredentials,
    onSuccess: () => {
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: vmwareCredQueryKey(vmwareCredName ?? '') })
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
    const hasFreshStatus = dataUpdatedAt > revalidationStartDataUpdatedAtRef.current

    if (vmwareCred) {
      const status = normalizeStatus(vmwareCred.status?.vmwareValidationStatus)
      const isRevalidationComplete =
        !isRevalidationApiPending && hasFreshStatus && status !== 'Revalidating'
      if (isRevalidationComplete) {
        clearActiveRevalidation()
        timedOutRevalidatingRef.current = false
        setTimedOutRevalidating(false)
        onRevalidationComplete?.()
      }
    } else {
      clearActiveRevalidation()
      timedOutRevalidatingRef.current = false
      setTimedOutRevalidating(false)
    }
  }, [
    vmwareCred,
    dataUpdatedAt,
    isRevalidating,
    isRevalidationApiPending,
    vmwareCredName,
    clearActiveRevalidation,
    onRevalidationComplete,
  ])

  useEffect(() => clearRevalidationTimeout, [clearRevalidationTimeout])

  const handleRefreshAndRevalidate = useCallback(() => {
    if (!vmwareCredName || isRevalidating) return
    const credNamespace = vmwareCred?.metadata?.namespace || VJAILBREAK_DEFAULT_NAMESPACE

    revalidationStartDataUpdatedAtRef.current = dataUpdatedAt
    clearRevalidationTimeout()
    // Update refs before async work so refetchInterval closure sees current values immediately
    timedOutRevalidatingRef.current = false
    setTimedOutRevalidating(false)
    isRevalidatingRef.current = true
    setIsRevalidating(true)

    revalidationTimeoutRef.current = setTimeout(() => {
      revalidationTimeoutRef.current = null
      // Update refs before invalidateQueries — fixes stale closure in refetchInterval
      timedOutRevalidatingRef.current = true
      isRevalidatingRef.current = false
      setTimedOutRevalidating(true)
      setIsRevalidating(false)
      reportError(
        new Error(
          `VMware credential revalidation is still in progress after ${REVALIDATION_TIMEOUT_MINUTES} minutes. You can retry.`
        ),
        {
          context: 'vmware-revalidation-timeout',
          metadata: { credentialName: vmwareCredName },
        }
      )
      queryClient.invalidateQueries({ queryKey: vmwareCredQueryKey(vmwareCredName) })
      queryClient.invalidateQueries({ queryKey: VMWARE_CREDS_QUERY_KEY })
    }, REVALIDATION_TIMEOUT_MS)

    revalidate({
      name: vmwareCredName,
      namespace: credNamespace,
      kind: 'VmwareCreds',
    })
  }, [
    vmwareCredName,
    isRevalidating,
    vmwareCred,
    dataUpdatedAt,
    clearRevalidationTimeout,
    revalidate,
    reportError,
    queryClient,
  ])

  return { isRevalidating, timedOutRevalidating, handleRefreshAndRevalidate }
}
