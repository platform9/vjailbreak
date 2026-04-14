import { useCallback } from 'react'
import type { QueryClient } from '@tanstack/react-query'
import { deleteMigrationTemplate } from 'src/features/migration/api/migration-templates/migrationTemplates'
import { deleteOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { deleteVmwareCredentials } from 'src/api/vmware-creds/vmwareCreds'
import { VMWARE_MACHINES_BASE_KEY } from 'src/hooks/api/useVMwareMachinesQuery'
import type { MigrationTemplate } from 'src/features/migration/api/migration-templates/model'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import type { VMwareCreds } from 'src/api/vmware-creds/model'
import type { FormValues } from 'src/features/migration/types'

type ReportErrorFn = (
  error: Error,
  context: {
    context: string
    metadata?: Record<string, unknown>
  }
) => void

export function useMigrationResourceCleanup({
  migrationTemplate,
  vmwareCredentials,
  openstackCredentials,
  queryClient,
  sessionId,
  onClose,
  params,
  setMigrationTemplate,
  setVmwareCredentials,
  setOpenstackCredentials,
  setError,
  reportError
}: {
  migrationTemplate: MigrationTemplate | undefined
  vmwareCredentials: VMwareCreds | undefined
  openstackCredentials: OpenstackCreds | undefined
  queryClient: QueryClient
  sessionId: string
  onClose: () => void
  params: FormValues
  setMigrationTemplate: (value: MigrationTemplate | undefined) => void
  setVmwareCredentials: (value: VMwareCreds | undefined) => void
  setOpenstackCredentials: (value: OpenstackCreds | undefined) => void
  setError: (value: { title: string; message: string } | null) => void
  reportError: ReportErrorFn
}) {
  const handleClose = useCallback(async () => {
    try {
      setMigrationTemplate(undefined)
      setVmwareCredentials(undefined)
      setOpenstackCredentials(undefined)
      setError(null)

      queryClient.invalidateQueries({ queryKey: [VMWARE_MACHINES_BASE_KEY, sessionId] })
      queryClient.removeQueries({ queryKey: [VMWARE_MACHINES_BASE_KEY, sessionId] })

      onClose()

      if (migrationTemplate?.metadata?.name) {
        await deleteMigrationTemplate(migrationTemplate.metadata.name)
      }

      if (vmwareCredentials?.metadata?.name && !params.vmwareCreds?.existingCredName) {
        await deleteVmwareCredentials(vmwareCredentials.metadata.name)
      }

      if (openstackCredentials?.metadata?.name && !params.openstackCreds?.existingCredName) {
        await deleteOpenstackCredentials(openstackCredentials.metadata.name)
      }
    } catch (err) {
      console.error('Error cleaning up resources', err)
      reportError(err as Error, {
        context: 'resource-cleanup',
        metadata: {
          migrationTemplateName: migrationTemplate?.metadata?.name,
          vmwareCredentialsName: vmwareCredentials?.metadata?.name,
          openstackCredentialsName: openstackCredentials?.metadata?.name,
          action: 'cleanup-resources'
        }
      })
      onClose()
    }
  }, [
    migrationTemplate,
    vmwareCredentials,
    openstackCredentials,
    queryClient,
    sessionId,
    onClose,
    params.vmwareCreds,
    params.openstackCreds,
    setMigrationTemplate,
    setVmwareCredentials,
    setOpenstackCredentials,
    setError,
    reportError
  ])

  return { handleClose }
}
