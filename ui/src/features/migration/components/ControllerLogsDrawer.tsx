import { useState, useCallback, useEffect, useRef } from 'react'
import { useDeploymentLogs } from 'src/hooks/useDeploymentLogs'
import BaseLogsDrawer from './BaseLogsDrawer'

interface ControllerLogsDrawerProps {
  open: boolean
  onClose: () => void
}

export default function ControllerLogsDrawer({ open, onClose }: ControllerLogsDrawerProps) {
  const [isPaused, setIsPaused] = useState(false)
  const [sessionKey, setSessionKey] = useState(0)
  // Internal open flag — updated in the same effect as sessionKey so React batches
  // both into one render, giving useDeploymentLogs a single connect() call per open.
  const [streamOpen, setStreamOpen] = useState(false)
  const prevOpenRef = useRef(false)

  useEffect(() => {
    if (open === prevOpenRef.current) return
    prevOpenRef.current = open

    if (open) {
      // Batch all resets with the enable — single render, single connect().
      setSessionKey((prev) => prev + 1)
      setIsPaused(false)
      setStreamOpen(true)
    } else {
      setStreamOpen(false)
    }
  }, [open])

  const {
    logs,
    isLoading,
    error,
    reconnect
  } = useDeploymentLogs({
    deploymentName: 'migration-controller-manager',
    namespace: 'migration-system',
    labelSelector: 'control-plane=controller-manager',
    enabled: streamOpen && !isPaused,
    sessionKey
  })

  const handleReconnect = useCallback(() => {
    setSessionKey((prev) => prev + 1)
    reconnect()
  }, [reconnect])

  const handleClose = useCallback(() => {
    setIsPaused(false)
    onClose()
  }, [onClose])

  return (
    <BaseLogsDrawer
      open={open}
      onClose={handleClose}
      title="Controller Logs"
      logs={logs}
      isLoading={isLoading}
      error={error}
      isPaused={isPaused}
      onPausedChange={setIsPaused}
      onReconnect={handleReconnect}
    />
  )
}
