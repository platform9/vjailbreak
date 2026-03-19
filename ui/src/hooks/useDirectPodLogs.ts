import { useCallback, useEffect, useRef, useState } from 'react'
import { streamPodLogs } from '../api/kubernetes/pods'

interface UseDirectPodLogsParams {
  podName: string
  namespace: string
  enabled: boolean
  sessionKey: number
}

interface UseDirectPodLogsReturn {
  logs: string[]
  isLoading: boolean
  error: string | null
  reconnect: () => void
}

const MAX_LOG_LINES = 5000
const INITIAL_RECONNECT_DELAY_MS = 5000
const MAX_RECONNECT_DELAY_MS = 30000
const MAX_RETRIES = 5

export const useDirectPodLogs = ({
  podName,
  namespace,
  enabled,
  sessionKey
}: UseDirectPodLogsParams): UseDirectPodLogsReturn => {
  const [logs, setLogs] = useState<string[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const retryCountRef = useRef(0)
  const hasInitiallyLoadedRef = useRef(false)
  const previousPodRef = useRef<string>('')
  const seenLogsRef = useRef<Set<string>>(new Set())

  const cleanup = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }
    retryCountRef.current = 0
  }, [])

  const connect = useCallback(async () => {
    if (!enabled || !podName || !namespace) {
      return
    }

    const queueReconnect = (transientError = true) => {
      if (!enabled || !podName || !namespace || !transientError) {
        return
      }
      if (retryCountRef.current >= MAX_RETRIES) {
        setError('Max retries exceeded. Please reconnect manually.')
        return
      }

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }

      const delay = Math.min(
        INITIAL_RECONNECT_DELAY_MS * Math.pow(2, retryCountRef.current),
        MAX_RECONNECT_DELAY_MS
      )
      retryCountRef.current += 1

      reconnectTimeoutRef.current = setTimeout(() => {
        reconnectTimeoutRef.current = null
        retryCountRef.current -= 1
        connect()
      }, delay)
    }

    cleanup()
    setIsLoading(true)
    setError(null)

    try {
      const abortController = new AbortController()
      abortControllerRef.current = abortController

      const shouldFetchHistory = !hasInitiallyLoadedRef.current
      const response = await streamPodLogs(namespace, podName, {
        follow: true,
        tailLines: shouldFetchHistory ? '2000' : '200',
        limitBytes: 8 * 1024 * 1024,
        signal: abortController.signal
      })

      if (shouldFetchHistory) {
        hasInitiallyLoadedRef.current = true
      }

      setIsLoading(false)
      retryCountRef.current = 0

      const reader = response.body?.getReader()
      if (!reader) {
        throw new Error('Response body is not readable')
      }

      const decoder = new TextDecoder()
      let buffer = ''

      const readStream = async (): Promise<void> => {
        try {
          const { done, value } = await reader.read()

          if (done) {
            // Process any remaining buffer content
            const remainingLine = buffer.trim()
            if (remainingLine && !seenLogsRef.current.has(remainingLine)) {
              seenLogsRef.current.add(remainingLine)
              setLogs((prevLogs) => {
                const newLogs = [...prevLogs, remainingLine]
                if (newLogs.length > MAX_LOG_LINES) {
                  // Remove oldest from seen set when trimming
                  const removed = newLogs.slice(0, newLogs.length - MAX_LOG_LINES)
                  removed.forEach((log) => seenLogsRef.current.delete(log))
                  return newLogs.slice(-MAX_LOG_LINES)
                }
                return newLogs
              })
            }

            queueReconnect()
            return
          }

          // Decode the chunk and add to buffer
          buffer += decoder.decode(value, { stream: true })

          // Split by newlines and process complete lines
          const lines = buffer.split('\n')
          buffer = lines.pop() || '' // Keep incomplete line in buffer

          if (lines.length > 0) {
            const filteredLines = lines.filter((line) => line.trim())
            const uniqueLines = filteredLines.filter((line) => !seenLogsRef.current.has(line))
            if (uniqueLines.length > 0) {
              uniqueLines.forEach((line) => seenLogsRef.current.add(line))
              setLogs((prevLogs) => {
                const newLogs = [...prevLogs, ...uniqueLines]
                if (newLogs.length > MAX_LOG_LINES) {
                  // Remove oldest from seen set when trimming
                  const removed = newLogs.slice(0, newLogs.length - MAX_LOG_LINES)
                  removed.forEach((log) => seenLogsRef.current.delete(log))
                  return newLogs.slice(-MAX_LOG_LINES)
                }
                return newLogs
              })
            }
          }

          // Continue reading
          return readStream()
        } catch (err) {
          if (err instanceof Error && err.name === 'AbortError') {
            // Stream was aborted, this is expected
            return
          }
          throw err
        }
      }

      await readStream()
    } catch (err) {
      setIsLoading(false)
      if (err instanceof Error && err.name === 'AbortError') {
        // Aborted, don't set error
        return
      }

      const errorMessage =
        err instanceof Error ? err.message : 'Failed to connect to pod logs stream'
      
      const isTransient = !errorMessage.includes('not found') && !errorMessage.includes('unauthorized')
      setError(errorMessage)
      queueReconnect(isTransient)
    }
  }, [enabled, podName, namespace, cleanup])

  const reconnect = useCallback(() => {
    setLogs([])
    hasInitiallyLoadedRef.current = false
    seenLogsRef.current.clear()
    retryCountRef.current = 0
    connect()
  }, [connect])

  useEffect(() => {
    setLogs([])
    hasInitiallyLoadedRef.current = false
    seenLogsRef.current.clear()
    retryCountRef.current = 0
  }, [sessionKey])

  useEffect(() => {
    const currentPodKey = `${namespace}/${podName}`
    if (previousPodRef.current && previousPodRef.current !== currentPodKey) {
      setLogs([])
      hasInitiallyLoadedRef.current = false
      seenLogsRef.current.clear()
      retryCountRef.current = 0
    }
    previousPodRef.current = currentPodKey

    if (enabled && podName && namespace) {
      connect()
    } else {
      cleanup()
      setIsLoading(false)
      setError(null)
    }

    return cleanup
  }, [enabled, podName, namespace, sessionKey, connect, cleanup])

  useEffect(() => {
    return cleanup
  }, [cleanup])

  return {
    logs,
    isLoading,
    error,
    reconnect
  }
}
