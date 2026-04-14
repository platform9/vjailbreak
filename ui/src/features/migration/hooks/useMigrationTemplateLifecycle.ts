import { useCallback, useEffect, useMemo } from 'react'
import axios from 'axios'
import {
  getMigrationTemplate,
  patchMigrationTemplate,
  postMigrationTemplate
} from 'src/features/migration/api/migration-templates/migrationTemplates'
import type { MigrationTemplate } from 'src/features/migration/api/migration-templates/model'
import { createMigrationTemplateJson } from 'src/features/migration/api/migration-templates/helpers'
import { THREE_SECONDS } from 'src/constants'
import { useInterval } from 'src/hooks/useInterval'
import type { FormValues } from 'src/features/migration/types'

export function useMigrationTemplateLifecycle({
  vmwareCredsValidated,
  openstackCredsValidated,
  params,
  vmwareCredentialsName,
  openstackCredentialsName,
  targetPCDClusterName,
  migrationTemplate,
  setMigrationTemplate,
  getFieldErrorsUpdater
}: {
  vmwareCredsValidated: boolean
  openstackCredsValidated: boolean
  params: FormValues
  vmwareCredentialsName?: string
  openstackCredentialsName?: string
  targetPCDClusterName?: string
  migrationTemplate: MigrationTemplate | undefined
  setMigrationTemplate: (value: MigrationTemplate | undefined) => void
  getFieldErrorsUpdater: (key: string | number) => (value: string) => void
}) {
  const shouldPollMigrationTemplate = useMemo(
    () =>
      Boolean(
        migrationTemplate?.metadata?.name &&
          (!migrationTemplate?.status?.openstack?.networks ||
            !migrationTemplate?.status?.openstack?.volumeTypes)
      ),
    [
      migrationTemplate?.metadata?.name,
      migrationTemplate?.status?.openstack?.networks,
      migrationTemplate?.status?.openstack?.volumeTypes
    ]
  )

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
                vmwareRef: vmwareCredentialsName
              },
              destination: {
                openstackRef: openstackCredentialsName
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
          vmwareRef: vmwareCredentialsName,
          openstackRef: openstackCredentialsName,
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
    vmwareCredentialsName,
    openstackCredentialsName,
    targetPCDClusterName,
    params.useFlavorless,
    params.useGPU,
    migrationTemplate?.metadata?.name,
    getFieldErrorsUpdater,
    setMigrationTemplate,
    migrationTemplate
  ])

  const fetchMigrationTemplate = useCallback(async () => {
    if (!migrationTemplate?.metadata?.name) return

    try {
      const updatedMigrationTemplate = await getMigrationTemplate(migrationTemplate.metadata.name)
      setMigrationTemplate(updatedMigrationTemplate)
    } catch (err) {
      console.error('Error retrieving migration templates', err)
      getFieldErrorsUpdater('migrationTemplate')('Error retrieving migration templates')
    }
  }, [migrationTemplate?.metadata?.name, setMigrationTemplate, getFieldErrorsUpdater])

  useInterval(
    async () => {
      if (!shouldPollMigrationTemplate) return
      try {
        await fetchMigrationTemplate()
      } catch (err) {
        console.error('Error retrieving migration templates', err)
        getFieldErrorsUpdater('migrationTemplate')('Error retrieving migration templates')
      }
    },
    THREE_SECONDS,
    shouldPollMigrationTemplate
  )

  useEffect(() => {
    if (vmwareCredsValidated && openstackCredsValidated) return
    setMigrationTemplate(undefined)
  }, [vmwareCredsValidated, openstackCredsValidated, setMigrationTemplate])

  return {
    shouldPollMigrationTemplate,
    fetchMigrationTemplate
  }
}
