import { useState, useEffect, useMemo } from 'react'
import axios from 'axios'
import { getVmwareCredentials } from 'src/api/vmware-creds/vmwareCreds'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import {
  getMigrationTemplate,
  patchMigrationTemplate,
  postMigrationTemplate
} from 'src/features/migration/api/migration-templates/migrationTemplates'
import { createMigrationTemplateJson } from 'src/features/migration/api/migration-templates/helpers'
import { MigrationTemplate } from 'src/features/migration/api/migration-templates/model'
import { VMwareCreds } from 'src/api/vmware-creds/model'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import { useInterval } from 'src/hooks/useInterval'
import { THREE_SECONDS } from 'src/constants'
import { isNilOrEmpty } from 'src/utils'
import type { FormValues } from '../types'

interface UseCredentialFetchingParams {
  params: Partial<FormValues>
  pcdData: Array<{ id: string; name?: string }>
  getFieldErrorsUpdater: (key: string) => (value: string) => void
}

interface UseCredentialFetchingResult {
  vmwareCredentials: VMwareCreds | undefined
  openstackCredentials: OpenstackCreds | undefined
  migrationTemplate: MigrationTemplate | undefined
  setMigrationTemplate: React.Dispatch<React.SetStateAction<MigrationTemplate | undefined>>
  setVmwareCredentials: React.Dispatch<React.SetStateAction<VMwareCreds | undefined>>
  setOpenstackCredentials: React.Dispatch<React.SetStateAction<OpenstackCreds | undefined>>
  vmwareCredsValidated: boolean
  openstackCredsValidated: boolean
  targetPCDClusterName: string | undefined
}

export function useCredentialFetching({
  params,
  pcdData,
  getFieldErrorsUpdater
}: UseCredentialFetchingParams): UseCredentialFetchingResult {
  const [vmwareCredentials, setVmwareCredentials] = useState<VMwareCreds | undefined>(undefined)
  const [openstackCredentials, setOpenstackCredentials] = useState<OpenstackCreds | undefined>(
    undefined
  )
  const [migrationTemplate, setMigrationTemplate] = useState<MigrationTemplate | undefined>(
    undefined
  )

  const vmwareCredsValidated = vmwareCredentials?.status?.vmwareValidationStatus === 'Succeeded'
  const openstackCredsValidated =
    openstackCredentials?.status?.openstackValidationStatus === 'Succeeded'

  const shouldPollMigrationTemplate =
    migrationTemplate?.metadata?.name &&
    (!migrationTemplate?.status?.openstack?.networks ||
      !migrationTemplate?.status?.openstack?.volumeTypes)

  const targetPCDClusterName = useMemo(() => {
    if (!params.pcdCluster) return undefined
    const selectedPCD = pcdData.find((p) => p.id === params.pcdCluster)
    return selectedPCD?.name
  }, [params.pcdCluster, pcdData])

  useEffect(() => {
    const fetchCredentials = async () => {
      if (!params.vmwareCreds || !params.vmwareCreds.existingCredName) return

      try {
        const existingCredName = params.vmwareCreds.existingCredName
        const response = await getVmwareCredentials(existingCredName)
        setVmwareCredentials(response)
      } catch (error) {
        console.error('Error fetching existing VMware credentials:', error)
        getFieldErrorsUpdater('vmwareCreds')(
          'Error fetching VMware credentials: ' +
            (axios.isAxiosError(error) ? error?.response?.data?.message : error)
        )
      }
    }

    if (isNilOrEmpty(params.vmwareCreds)) return
    setVmwareCredentials(undefined)
    getFieldErrorsUpdater('vmwareCreds')('')
    fetchCredentials()
  }, [params.vmwareCreds, getFieldErrorsUpdater])

  useEffect(() => {
    const fetchCredentials = async () => {
      if (!params.openstackCreds || !params.openstackCreds.existingCredName) return

      try {
        const existingCredName = params.openstackCreds.existingCredName
        const response = await getOpenstackCredentials(existingCredName)
        setOpenstackCredentials(response)
      } catch (error) {
        console.error('Error fetching existing OpenStack credentials:', error)
        getFieldErrorsUpdater('openstackCreds')(
          'Error fetching PCD credentials: ' +
            (axios.isAxiosError(error) ? error?.response?.data?.message : error)
        )
      }
    }

    if (isNilOrEmpty(params.openstackCreds)) return
    setOpenstackCredentials(undefined)
    getFieldErrorsUpdater('openstackCreds')('')
    fetchCredentials()
  }, [params.openstackCreds, getFieldErrorsUpdater])

  useEffect(() => {
    if (!vmwareCredsValidated || !openstackCredsValidated) return

    const syncMigrationTemplate = async () => {
      try {
        if (migrationTemplate?.metadata?.name) {
          const patchBody = {
            spec: {
              source: {
                ...(params.vmwareCreds?.datacenter && {
                  datacenter: params.vmwareCreds.datacenter
                }),
                vmwareRef: vmwareCredentials?.metadata.name
              },
              destination: {
                openstackRef: openstackCredentials?.metadata.name
              },
              ...(targetPCDClusterName && {
                targetPCDClusterName
              }),
              useFlavorless: params.useFlavorless || false,
              useGPUFlavor: params.useGPU || false
            }
          }

          const updated = await patchMigrationTemplate(migrationTemplate.metadata.name, patchBody)
          setMigrationTemplate(updated)
          return
        }

        const body = createMigrationTemplateJson({
          ...(params.vmwareCreds?.datacenter && { datacenter: params.vmwareCreds.datacenter }),
          vmwareRef: vmwareCredentials?.metadata.name,
          openstackRef: openstackCredentials?.metadata.name,
          targetPCDClusterName,
          useFlavorless: params.useFlavorless || false,
          useGPUFlavor: params.useGPU || false
        })
        const created = await postMigrationTemplate(body)
        setMigrationTemplate(created)
      } catch (err) {
        console.error('Error syncing migration template', err)
        getFieldErrorsUpdater('migrationTemplate')(
          'Error syncing migration template: ' +
            (axios.isAxiosError(err)
              ? err?.response?.data?.message
              : err instanceof Error
                ? err.message
                : String(err))
        )
      }
    }

    syncMigrationTemplate()
  }, [
    vmwareCredsValidated,
    openstackCredsValidated,
    params.vmwareCreds?.datacenter,
    vmwareCredentials?.metadata.name,
    openstackCredentials?.metadata.name,
    targetPCDClusterName,
    params.useFlavorless,
    params.useGPU,
    migrationTemplate?.metadata?.name,
    getFieldErrorsUpdater
  ])

  const fetchMigrationTemplate = async () => {
    try {
      const updatedMigrationTemplate = await getMigrationTemplate(migrationTemplate!.metadata!.name)
      setMigrationTemplate(updatedMigrationTemplate)
    } catch (err) {
      console.error('Error retrieving migration templates', err)
      getFieldErrorsUpdater('migrationTemplate')('Error retrieving migration templates')
    }
  }

  useInterval(
    async () => {
      if (shouldPollMigrationTemplate) {
        try {
          fetchMigrationTemplate()
        } catch (err) {
          console.error('Error retrieving migration templates', err)
          getFieldErrorsUpdater('migrationTemplate')('Error retrieving migration templates')
        }
      }
    },
    THREE_SECONDS,
    shouldPollMigrationTemplate
  )

  useEffect(() => {
    if (vmwareCredsValidated && openstackCredsValidated) return
    setMigrationTemplate(undefined)
  }, [vmwareCredsValidated, openstackCredsValidated])

  return {
    vmwareCredentials,
    openstackCredentials,
    migrationTemplate,
    setMigrationTemplate,
    setVmwareCredentials,
    setOpenstackCredentials,
    vmwareCredsValidated,
    openstackCredsValidated,
    targetPCDClusterName
  }
}
