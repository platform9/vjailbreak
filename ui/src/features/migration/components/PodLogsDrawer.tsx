import { useState, useCallback, useMemo } from 'react'
import { useDirectPodLogs } from 'src/hooks/useDirectPodLogs'
import { downloadDebugBundle } from 'src/api/migrations/debugBundle'
import { Phase } from '../api/migrations'
import BaseLogsDrawer from './BaseLogsDrawer'

const STREAM_END_PHASES: Phase[] = [Phase.Succeeded, Phase.Failed]

interface LogsDrawerProps {
  open: boolean
  onClose: () => void
  podName: string
  namespace: string
  migrationName?: string
  migrationPhase?: Phase
  vmName?: string
}

export default function PodLogsDrawer({
  open,
  onClose,
  podName,
  namespace,
  migrationName,
  migrationPhase,
  vmName
}: LogsDrawerProps) {
  const [isPaused, setIsPaused] = useState(false)
  const [sessionKey, setSessionKey] = useState(0)

  const useLiveFollow =
    migrationPhase === undefined ? true : !STREAM_END_PHASES.includes(migrationPhase)

  const { logs, isLoading, error } = useDirectPodLogs({
    podName,
    namespace,
    enabled: open && !isPaused,
    follow: useLiveFollow,
    sessionKey
  })

  const handleReconnect = useCallback(() => {
    setSessionKey((prev) => prev + 1)
  }, [])

  const handleClose = useCallback(() => {
    setIsPaused(false)
    setSessionKey(0)
    onClose()
  }, [onClose])

  const vmDisplayName = useMemo(() => {
    // Prefer the migration's spec.vmName — the same name shown in the Migration
    // Details drawer header — so both views stay consistent. Fall back to deriving
    // a name from the migration/pod object names only when vmName is unavailable.
    const trimmedVmName = vmName?.trim()
    if (trimmedVmName) return trimmedVmName

    const fromMigration = (() => {
      if (!migrationName) return null
      const withoutPrefix = migrationName.replace(/^migration-/, '')
      const withoutSuffix = withoutPrefix.replace(/-[0-9a-f]{5}$/i, '')
      return withoutSuffix || null
    })()

    if (fromMigration) return fromMigration
    if (!podName) return null
    const withoutPrefix = podName.replace(/^v2v-helper-/, '')
    const parts = withoutPrefix.split('-')
    if (parts.length >= 4) return parts.slice(0, -3).join('-') || withoutPrefix
    if (parts.length >= 3) return parts.slice(0, -2).join('-') || withoutPrefix
    return withoutPrefix
  }, [vmName, migrationName, podName])

  // The backend assembles the entire bundle (pod logs + related resource
  // YAMLs + /var/log/pf9 debug logs); the drawer's filtered view does not
  // affect the downloaded file.
  const handleDownload = useCallback(async () => {
    await downloadDebugBundle(migrationName, namespace, podName)
  }, [migrationName, namespace, podName])

  return (
    <BaseLogsDrawer
      data-testid="pod-logs-drawer"
      open={open}
      onClose={handleClose}
      title="Migration Pod Logs"
      subtitle={vmDisplayName || ''}
      logs={logs}
      isLoading={isLoading}
      error={error}
      isPaused={isPaused}
      onPausedChange={setIsPaused}
      onReconnect={handleReconnect}
      onDownload={handleDownload}
    />
  )
}
