import { useState, useCallback, useMemo } from 'react'
import { useDirectPodLogs } from 'src/hooks/useDirectPodLogs'
import { fetchPodDebugLogs } from 'src/api/kubernetes/pods'
import { fetchMigrationResourceBundle } from 'src/api/kubernetes/migrationResourceBundle'
import { Phase } from '../api/migrations'
import BaseLogsDrawer from './BaseLogsDrawer'
import AIAnalysisTab from './AIAnalysisTab'

const STREAM_END_PHASES: Phase[] = [Phase.Succeeded, Phase.Failed]

interface LogsDrawerProps {
  open: boolean
  onClose: () => void
  podName: string
  namespace: string
  migrationName?: string
  migrationPhase?: Phase
}

export default function PodLogsDrawer({
  open,
  onClose,
  podName,
  namespace,
  migrationName,
  migrationPhase
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
  }, [migrationName, podName])

  const handleDownload = useCallback(
    async (filteredLogs: string[]) => {
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
      const fileName = `${vmDisplayName || podName || 'logs'}-pod-${timestamp}.txt`

      let combinedLogs = '='.repeat(80) + '\n'
      combinedLogs += 'STDOUT/STDERR LOGS (pod)\n'
      combinedLogs += '='.repeat(80) + '\n\n'
      combinedLogs += filteredLogs.join('\n')

      if (namespace && migrationName) {
        try {
          const resourceBundle = await fetchMigrationResourceBundle({
            namespace,
            migrationName,
            podName
          })

          if (resourceBundle && resourceBundle.trim()) {
            combinedLogs += '\n\n'
            combinedLogs += '='.repeat(80) + '\n'
            combinedLogs += 'RELATED KUBERNETES RESOURCES\n'
            combinedLogs += '='.repeat(80) + '\n\n'
            combinedLogs += resourceBundle
          }
        } catch {
          combinedLogs += '\n\n'
          combinedLogs += '='.repeat(80) + '\n'
          combinedLogs += 'RELATED KUBERNETES RESOURCES\n'
          combinedLogs += '='.repeat(80) + '\n\n'
          combinedLogs += '[Failed to fetch related Kubernetes resources]\n'
        }
      }

      if (namespace && migrationName) {
        try {
          const debugLogs = await fetchPodDebugLogs(namespace, podName, migrationName)
          if (debugLogs && debugLogs.trim()) {
            combinedLogs += '\n\n'
            combinedLogs += '='.repeat(80) + '\n'
            combinedLogs += 'DEBUG LOGS FROM /var/log/pf9\n'
            combinedLogs += '='.repeat(80) + '\n\n'
            combinedLogs += debugLogs
          }
        } catch {
          combinedLogs += '\n\n'
          combinedLogs += '='.repeat(80) + '\n'
          combinedLogs += 'DEBUG LOGS FROM /var/log/pf9\n'
          combinedLogs += '='.repeat(80) + '\n\n'
          combinedLogs += '[Failed to fetch debug logs from pod filesystem]\n'
        }
      }

      const blob = new Blob([combinedLogs], { type: 'text/plain' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = fileName
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
    },
    [vmDisplayName, podName, namespace, migrationName]
  )

  const aiTabContent =
    migrationPhase === Phase.Failed && migrationName && namespace ? (
      <AIAnalysisTab migrationName={migrationName} namespace={namespace} />
    ) : undefined

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
      aiTabContent={aiTabContent}
    />
  )
}
