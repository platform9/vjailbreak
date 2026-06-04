// Trigger-time launch: turn a MigrationBucket into the same objects the Migration Form's submit
// creates — NetworkMapping, StorageMapping, MigrationTemplate, MigrationPlan — reusing the exact
// API helpers/JSON builders so behavior matches the form. Also scales agents.
//
// The bucket's `spec.config.formValues` holds the full Migration Form params; `selectedOptions`
// holds which optional checkboxes were enabled. We read those and replicate the form's flow.

import { getVmwareCredentials } from 'src/api/vmware-creds/vmwareCreds'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { createMigrationTemplateJson } from 'src/features/migration/api/migration-templates/helpers'
import {
  postMigrationTemplate,
  patchMigrationTemplate
} from 'src/features/migration/api/migration-templates/migrationTemplates'
import { createNetworkMappingJson } from 'src/api/network-mapping/helpers'
import { postNetworkMapping } from 'src/api/network-mapping/networkMappings'
import { createStorageMappingJson } from 'src/api/storage-mappings/helpers'
import { postStorageMapping } from 'src/api/storage-mappings/storageMappings'
import { createArrayCredsMappingJson } from 'src/api/arraycreds-mapping/helpers'
import { postArrayCredsMapping } from 'src/api/arraycreds-mapping/arrayCredsMapping'
import { createMigrationPlanJson } from 'src/features/migration/api/migration-plans/helpers'
import { postMigrationPlan } from 'src/features/migration/api/migration-plans/migrationPlans'
import { getNodes, createNodes } from 'src/api/nodes/nodeMappings'
import { CUTOVER_TYPES } from 'src/features/migration/constants'
import type { PcdDataItem } from 'src/features/migration/hooks/useClusterData'
import type { FormValues, SelectedMigrationOptionsType } from 'src/features/migration/types'
import type { MigrationBucket } from '../types'

interface LaunchOptions {
  /** True = trigger now (ignore per-bucket schedule); false = honor the bucket's schedule time. */
  scheduleNow: boolean
  /** PCD cluster list, to map a stored pcdCluster id → its targetPCDClusterName. */
  pcdData: PcdDataItem[]
}

/**
 * Create the migration objects for one bucket, mirroring the Migration Form's submit flow.
 * Throws on failure (caller aggregates).
 */
export async function launchBucketMigration(
  bucket: MigrationBucket,
  { scheduleNow, pcdData }: LaunchOptions
): Promise<void> {
  const config = bucket.spec.config ?? {}
  // formValues is the saved Migration Form params (opaque RawExtension on the CRD).
  const fv = ((config as Record<string, unknown>).formValues ?? {}) as Partial<FormValues>
  const options = ((config as Record<string, unknown>).selectedOptions ??
    {}) as Partial<SelectedMigrationOptionsType>

  const vmwareCredName = fv.vmwareCreds?.existingCredName ?? bucket.spec.vmwareCredsRef.name
  const openstackCredName = fv.openstackCreds?.existingCredName
  if (!vmwareCredName || !openstackCredName) {
    throw new Error(
      `Bucket "${bucket.metadata.name}" is missing VMware/OpenStack credentials in its config. Open Edit and Save it first.`
    )
  }

  const [vmwareCreds, openstackCreds] = await Promise.all([
    getVmwareCredentials(vmwareCredName),
    getOpenstackCredentials(openstackCredName)
  ])

  const sourceClusterValue: string = fv.vmwareCluster ?? config.sourceCluster ?? ''
  // Cluster id format is `credName:datacenter:clusterName`; prefer the explicit form value
  // (mirrors useCredentialFetching), then the cluster id segment, then the credential's own DC.
  const datacenter =
    fv.vmwareCreds?.datacenter || sourceClusterValue.split(':')[1] || vmwareCreds.spec?.datacenter
  const pcdId: string | undefined = fv.pcdCluster ?? config.pcdCluster
  const targetPCDClusterName = pcdData.find((p) => p.id === pcdId)?.name

  // 1) Migration template (created first, mappings patched in after they exist — like the form).
  const template = await postMigrationTemplate(
    createMigrationTemplateJson({
      datacenter,
      vmwareRef: vmwareCreds.metadata.name,
      openstackRef: openstackCreds.metadata.name,
      targetPCDClusterName,
      useFlavorless: fv.useFlavorless || false,
      useGPUFlavor: fv.useGPU || false
    })
  )

  // 2) Network mapping (always) + storage mapping depending on copy method.
  const networkMappings = fv.networkMappings ?? config.networkMappings ?? []
  const networkMapping = await postNetworkMapping(createNetworkMappingJson({ networkMappings }))

  const storageCopyMethod = fv.storageCopyMethod || 'normal'

  // 3) Patch template — mirrors the form's updateMigrationTemplate branching:
  //    HotAdd → storage mapping + proxyVMRef; StorageAcceleratedCopy → arrayCredsMapping;
  //    normal → storage mapping.
  const templateSpec: Record<string, unknown> = {
    networkMapping: networkMapping.metadata.name,
    storageCopyMethod
  }

  if (storageCopyMethod === 'StorageAcceleratedCopy') {
    const arrayCredsMapping = await postArrayCredsMapping(
      createArrayCredsMappingJson({ mappings: fv.arrayCredsMappings ?? [] })
    )
    templateSpec.arrayCredsMapping = arrayCredsMapping.metadata.name
  } else {
    const storageMappings = fv.storageMappings ?? config.storageMappings ?? []
    const storageMapping = await postStorageMapping(createStorageMappingJson({ storageMappings }))
    templateSpec.storageMapping = storageMapping.metadata.name
    if (storageCopyMethod === 'HotAdd' && fv.proxyVMRef) {
      templateSpec.proxyVMRef = { name: fv.proxyVMRef }
    }
  }

  await patchMigrationTemplate(template.metadata.name, { spec: templateSpec })

  // 4) Migration plan.
  const vmsToMigrate: string[] = (fv.vms ?? [])
    .map((vm) => vm.vmKey || vm.name)
    .filter((v): v is string => typeof v === 'string' && v.length > 0)

  const useScheduledStart =
    !scheduleNow && Boolean(options.dataCopyStartTime) && Boolean(fv.dataCopyStartTime)

  const migrationFields: Record<string, unknown> = {
    migrationTemplateName: template.metadata.name,
    virtualMachines: vmsToMigrate.length > 0 ? vmsToMigrate : bucket.spec.vms,
    type: fv.dataCopyMethod,
    ...(useScheduledStart && { dataCopyStart: fv.dataCopyStartTime }),
    ...(options.cutoverOption &&
      fv.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED && { adminInitiatedCutOver: true }),
    ...(options.cutoverOption &&
      fv.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
      fv.cutoverStartTime && { vmCutoverStart: fv.cutoverStartTime }),
    ...(options.cutoverOption &&
      fv.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
      fv.cutoverEndTime && { vmCutoverEnd: fv.cutoverEndTime }),
    ...(Array.isArray(fv.securityGroups) &&
      fv.securityGroups.length > 0 && { securityGroups: fv.securityGroups }),
    ...(fv.serverGroup && { serverGroup: fv.serverGroup }),
    disconnectSourceNetwork: fv.disconnectSourceNetwork || false,
    fallbackToDHCP: fv.fallbackToDHCP || false,
    ...(options.postMigrationScript &&
      fv.postMigrationScript && { postMigrationScript: fv.postMigrationScript }),
    ...(typeof fv.networkPersistence === 'boolean' && { networkPersistence: fv.networkPersistence }),
    ...(typeof fv.removeVMwareTools === 'boolean' && { removeVMwareTools: fv.removeVMwareTools }),
    ...(Array.isArray(fv.imageProfiles) &&
      fv.imageProfiles.length > 0 && { imageProfiles: fv.imageProfiles })
  }

  await postMigrationPlan(createMigrationPlanJson(migrationFields))
}

/**
 * Scale the migration-agent worker pool UP TO `targetAgents` total agents, reusing the master
 * node's image, flavor, and OpenStack credential (the same inputs the Scale-Up drawer uses).
 *
 * The master node itself runs one agent, so the worker target is `targetAgents - 1`. We create
 * only the delta over the workers that already exist, and never scale down. No-op when no new
 * workers are needed.
 */
export async function scaleAgentsForTrigger(targetAgents: number): Promise<void> {
  const desiredWorkers = Math.max(0, targetAgents - 1)
  const nodes = (await getNodes()) ?? []
  const master = nodes.find((n) => n.spec?.nodeRole === 'master')
  const existingWorkers = nodes.filter((n) => n.spec?.nodeRole !== 'master').length
  const toCreate = desiredWorkers - existingWorkers
  if (toCreate <= 0) return
  if (!master?.spec?.openstackImageID || !master?.spec?.openstackFlavorID) {
    throw new Error('Cannot scale agents: master node image/flavor not found.')
  }
  await createNodes({
    imageId: master.spec.openstackImageID,
    flavorId: master.spec.openstackFlavorID,
    openstackCreds: master.spec.openstackCreds,
    count: toCreate
  })
}
