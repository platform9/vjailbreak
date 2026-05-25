import { useState, useMemo } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { patchRdmDisk } from 'src/api/rdm-disks/rdmDisks'
import { RdmDisk } from 'src/api/rdm-disks/model'
import { useRdmDisksQuery, RDM_DISKS_BASE_KEY } from 'src/hooks/api/useRdmDisksQuery'
import type { AmplitudeEventName, EventProperties } from 'src/types/amplitude'
import type { ErrorContext } from 'src/services/errorReporting'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import type { RdmConfiguration } from '../types'

interface UseRdmConfigurationParams {
  selectedVMs: Set<string>
  rdmConfigurations: RdmConfiguration[]
  openstackCredName?: string
  openstackCredentials?: OpenstackCreds
  showToast: (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void
  track: (eventName: AmplitudeEventName, properties?: EventProperties) => void
  reportError: (error: Error, additionalContext?: ErrorContext) => void
}

export function useRdmConfiguration({
  selectedVMs,
  rdmConfigurations,
  openstackCredName,
  openstackCredentials,
  showToast,
  track,
  reportError,
}: UseRdmConfigurationParams) {
  const queryClient = useQueryClient()
  const [rdmConfigDialogOpen, setRdmConfigDialogOpen] = useState(false)
  const [rdmConfirmDialogOpen, setRdmConfirmDialogOpen] = useState(false)
  const [rdmUpdating, setRdmUpdating] = useState(false)

  const { data: rdmDisks = [], isLoading: rdmDisksLoading } = useRdmDisksQuery()

  const hasRdmVolumeTypeWarnings = useMemo(() => {
    if (!rdmConfigurations || rdmConfigurations.length === 0) return false
    const backendVolumeTypeMap = openstackCredentials?.status?.openstack?.backendVolumeTypeMap || {}
    return rdmConfigurations.some((config) => {
      if (!config.cinderBackendPool || !config.volumeType) return false
      const expectedType = backendVolumeTypeMap[config.cinderBackendPool]
      return expectedType && expectedType !== config.volumeType
    })
  }, [rdmConfigurations, openstackCredentials?.status?.openstack?.backendVolumeTypeMap])

  const handleOpenRdmConfigurationDialog = () => {
    if (selectedVMs.size === 0) return
    setRdmConfigDialogOpen(true)
  }

  const handleCloseRdmConfigurationDialog = () => {
    setRdmConfigDialogOpen(false)
  }

  const handleApplyRdmConfigurations = async () => {
    setRdmConfirmDialogOpen(false)
    if (!rdmConfigurations || rdmConfigurations.length === 0) {
      showToast('No RDM configurations to apply', 'warning')
      return
    }

    setRdmUpdating(true)

    try {
      track('rdm_configuration_applied' as AmplitudeEventName, {
        rdmDisksCount: rdmConfigurations.length,
        selectedVMsCount: selectedVMs.size
      } as EventProperties)

      const updatePromises = rdmConfigurations.map(async (config) => {
        const rdmDisk = rdmDisks.find((disk) => disk.spec.uuid === config.uuid)
        if (!rdmDisk) {
          console.warn(`RDM disk not found for diskName: ${config.diskName}`)
          return
        }

        const payload = {
          spec: {
            openstackVolumeRef: {
              cinderBackendPool: config.cinderBackendPool,
              volumeType: config.volumeType,
              openstackCreds: openstackCredName
            }
          }
        } as Partial<RdmDisk>

        return patchRdmDisk(rdmDisk.metadata.name, payload)
      })

      await Promise.all(updatePromises)

      showToast(
        `Successfully configured ${rdmConfigurations.length} RDM disk${
          rdmConfigurations.length > 1 ? 's' : ''
        }`,
        'success'
      )

      handleCloseRdmConfigurationDialog()
      queryClient.invalidateQueries({ queryKey: [RDM_DISKS_BASE_KEY] })
    } catch (error) {
      reportError(error as Error, {
        context: 'rdm-disk-configuration',
        metadata: {
          rdmConfigurationsCount: rdmConfigurations.length,
          action: 'apply-rdm-configurations'
        }
      })
      showToast('Failed to configure RDM disks', 'error')
    } finally {
      setRdmUpdating(false)
    }
  }

  const handleApplyRdmConfigurationsClick = () => {
    if (hasRdmVolumeTypeWarnings) {
      setRdmConfirmDialogOpen(true)
    } else {
      handleApplyRdmConfigurations()
    }
  }

  return {
    rdmDisks,
    rdmDisksLoading,
    rdmConfigDialogOpen,
    setRdmConfigDialogOpen,
    rdmConfirmDialogOpen,
    setRdmConfirmDialogOpen,
    rdmUpdating,
    hasRdmVolumeTypeWarnings,
    handleOpenRdmConfigurationDialog,
    handleCloseRdmConfigurationDialog,
    handleApplyRdmConfigurations,
    handleApplyRdmConfigurationsClick,
  }
}
