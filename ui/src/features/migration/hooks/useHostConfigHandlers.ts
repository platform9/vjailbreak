import { useState } from 'react'
import { SelectChangeEvent } from '@mui/material'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import type { ESXHost } from '../types'
import type { ErrorContext } from 'src/services/errorReporting'

interface UseHostConfigHandlersParams {
  orderedESXHosts: ESXHost[]
  setOrderedESXHosts: React.Dispatch<React.SetStateAction<ESXHost[]>>
  openstackCredData: OpenstackCreds | null
  markTouched: (key: 'sourceDestination' | 'baremetal' | 'hosts' | 'vms' | 'mapResources' | 'options') => void
  reportError: (error: Error, additionalContext?: ErrorContext) => void
}

export function useHostConfigHandlers({
  orderedESXHosts,
  setOrderedESXHosts,
  openstackCredData,
  markTouched,
  reportError
}: UseHostConfigHandlersParams) {
  const [pcdHostConfigDialogOpen, setPcdHostConfigDialogOpen] = useState(false)
  const [selectedPcdHostConfig, setSelectedPcdHostConfig] = useState('')
  const [updatingPcdMapping, setUpdatingPcdMapping] = useState(false)

  const handleOpenPcdHostConfigDialog = () => {
    setPcdHostConfigDialogOpen(true)
  }

  const handleClosePcdHostConfigDialog = () => {
    setPcdHostConfigDialogOpen(false)
    setSelectedPcdHostConfig('')
  }

  const handlePcdHostConfigChange = (event: SelectChangeEvent<string>) => {
    setSelectedPcdHostConfig(event.target.value)
  }

  const handleApplyPcdHostConfig = async () => {
    if (!selectedPcdHostConfig) {
      handleClosePcdHostConfigDialog()
      return
    }

    markTouched('hosts')

    setUpdatingPcdMapping(true)

    try {
      const availablePcdHostConfigs = openstackCredData?.spec?.pcdHostConfig || []
      const selectedPcdConfig = availablePcdHostConfigs.find(
        (config) => config.id === selectedPcdHostConfig
      )
      const pcdConfigName = selectedPcdConfig ? selectedPcdConfig.name : selectedPcdHostConfig

      // Update ALL ESX hosts with the selected host config
      const updatedESXHosts = orderedESXHosts.map((host) => ({
        ...host,
        pcdHostConfigName: pcdConfigName
      }))

      setOrderedESXHosts(updatedESXHosts)

      handleClosePcdHostConfigDialog()
    } catch (error) {
      console.error('Error updating PCD host config mapping:', error)
      reportError(error as Error, {
        context: 'pcd-host-config-mapping',
        metadata: {
          selectedPcdHostConfig: selectedPcdHostConfig,
          action: 'update-pcd-host-config-mapping'
        }
      })
    } finally {
      setUpdatingPcdMapping(false)
    }
  }

  const handleIndividualHostConfigChange = async (hostId: string, configName: string) => {
    try {
      markTouched('hosts')
      // Update the ESX host with the selected host config
      const updatedESXHosts = orderedESXHosts.map((host) => {
        if (host.id === hostId) {
          return {
            ...host,
            pcdHostConfigName: configName
          }
        }
        return host
      })

      setOrderedESXHosts(updatedESXHosts)

      console.log(`Successfully assigned host config "${configName}" to ESX host ${hostId}`)
    } catch (error) {
      console.error(`Failed to update host config for ESX host ${hostId}:`, error)
      reportError(error as Error, {
        context: 'individual-host-config-update',
        metadata: {
          hostId: hostId,
          configName: configName,
          action: 'individual-host-config-update'
        }
      })
      alert(
        `Failed to assign host config to ESX host: ${error instanceof Error ? error.message : 'Unknown error'}`
      )
    }
  }

  return {
    pcdHostConfigDialogOpen,
    selectedPcdHostConfig,
    updatingPcdMapping,
    handleOpenPcdHostConfigDialog,
    handleClosePcdHostConfigDialog,
    handlePcdHostConfigChange,
    handleApplyPcdHostConfig,
    handleIndividualHostConfigChange
  }
}
