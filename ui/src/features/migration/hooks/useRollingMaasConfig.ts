import { useCallback, useEffect, useState } from 'react'
import { getBMConfig, getBMConfigList } from 'src/api/bmconfig/bmconfig'
import type { BMConfig } from 'src/api/bmconfig/model'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

type ReportErrorFn = (
  error: Error,
  context: {
    context: string
    metadata?: Record<string, unknown>
  }
) => void

export function useRollingMaasConfig({
  open,
  reportError
}: {
  open: boolean
  reportError: ReportErrorFn
}) {
  const [maasConfigDialogOpen, setMaasConfigDialogOpen] = useState(false)
  const [maasConfigs, setMaasConfigs] = useState<BMConfig[]>([])
  const [selectedMaasConfig, setSelectedMaasConfig] = useState<BMConfig | null>(null)
  const [loadingMaasConfig, setLoadingMaasConfig] = useState(false)
  const [maasDetailsModalOpen, setMaasDetailsModalOpen] = useState(false)

  const fetchMaasConfigs = useCallback(async () => {
    try {
      setLoadingMaasConfig(true)
      const configs = await getBMConfigList(VJAILBREAK_DEFAULT_NAMESPACE)
      if (configs && configs.length > 0) {
        setMaasConfigs(configs)
        try {
          const config = await getBMConfig(configs[0].metadata.name, VJAILBREAK_DEFAULT_NAMESPACE)
          setSelectedMaasConfig(config)
        } catch (error) {
          console.error(`Failed to fetch Bare Metal config:`, error)
          reportError(error as Error, {
            context: 'rolling-maas-config-fetch',
            metadata: { action: 'getBMConfig' }
          })
        }
      } else {
        setMaasConfigs([])
        setSelectedMaasConfig(null)
      }
    } catch (error) {
      console.error('Failed to fetch Bare Metal configs:', error)
      reportError(error as Error, {
        context: 'rolling-maas-config-list',
        metadata: { action: 'getBMConfigList' }
      })
    } finally {
      setLoadingMaasConfig(false)
    }
  }, [reportError])

  useEffect(() => {
    if (open) {
      fetchMaasConfigs()
    }
  }, [open, fetchMaasConfigs])

  const handleViewMaasConfig = useCallback(() => {
    setMaasDetailsModalOpen(true)
  }, [])

  const handleCloseMaasDetailsModal = useCallback(() => {
    setMaasDetailsModalOpen(false)
  }, [])

  const handleCloseMaasConfigDialog = useCallback(() => {
    setMaasConfigDialogOpen(false)
  }, [])

  const handleOpenMaasConfigDialog = useCallback(() => {
    setMaasConfigDialogOpen(true)
  }, [])

  return {
    maasConfigDialogOpen,
    setMaasConfigDialogOpen,
    handleOpenMaasConfigDialog,
    handleCloseMaasConfigDialog,

    maasConfigs,
    selectedMaasConfig,
    loadingMaasConfig,
    fetchMaasConfigs,

    maasDetailsModalOpen,
    handleViewMaasConfig,
    handleCloseMaasDetailsModal
  }
}
