import type { BundleEntry, KubernetesObject } from './types'
import { RELATED_CRD_PLURALS } from './constants'
import {
  addValue,
  addValues,
  field,
  hasValue,
  isOwnedBy,
  objectName,
  record,
  value
} from './objectUtils'
import { getCoreObject, listCoreObjects, listVjailbreakCrs } from './kubernetesClient'
import { formatYamlBundle } from './formatBundleYaml'

export const fetchMigrationResourceBundle = async ({
  namespace,
  migrationName,
  podName
}: {
  namespace: string
  migrationName?: string
  podName?: string
}): Promise<string> => {
  const warnings: string[] = []
  const resourceLists = new Map<string, KubernetesObject[]>()

  await Promise.all(
    RELATED_CRD_PLURALS.map(async (plural) => {
      resourceLists.set(plural, await listVjailbreakCrs(plural, namespace, warnings))
    })
  )

  const migrations = resourceLists.get('migrations') || []
  const migration =
    migrations.find((item) => objectName(item) === migrationName) ||
    migrations.find((item) => value(field(item.spec, 'podRef')) === podName)

  if (!migration) {
    warnings.push(`Migration resource not found for migrationName=${migrationName || '<empty>'}`)
    return formatYamlBundle([], warnings)
  }

  const entries = new Map<string, BundleEntry>()
  const addEntry = (plural: string, object: KubernetesObject) => {
    const name = objectName(object)
    if (!name) return
    entries.set(`${plural}/${name}`, {
      path: `kubernetes/${plural}/${name}.yaml`,
      object
    })
  }

  addEntry('migrations', migration)

  const migrationUid = value(migration.metadata?.uid)
  const migrationOwnerKeys = new Set<string>()
  addValue(migrationOwnerKeys, migration.metadata?.name)
  addValue(migrationOwnerKeys, migration.metadata?.uid)

  const vmK8sName = objectName(migration).replace(/^migration-/, '')
  const vmNames = new Set<string>()
  addValue(vmNames, vmK8sName)
  addValue(vmNames, field(migration.spec, 'vmName'))
  addValue(vmNames, migration.metadata?.annotations?.['vjailbreak.k8s.pf9.io/original-vm-name'])
  addValue(vmNames, migration.metadata?.labels?.['vjailbreak.k8s.pf9.io/vm-key'])

  const planNames = new Set<string>()
  addValue(planNames, field(migration.spec, 'migrationPlan'))
  addValue(planNames, migration.metadata?.labels?.migrationplan)

  const nodeNames = new Set<string>()
  addValue(nodeNames, field(migration.status, 'agentName'))

  const configMapNames = new Set<string>()
  addValue(configMapNames, vmK8sName ? `migration-config-${vmK8sName}` : '')
  addValue(configMapNames, vmK8sName ? `firstboot-config-${vmK8sName}` : '')

  const configMaps = await listCoreObjects('configmaps', namespace, warnings)
  for (const configMap of configMaps) {
    if (
      hasValue(configMapNames, configMap.metadata?.name) ||
      isOwnedBy(configMap, migrationOwnerKeys)
    ) {
      addEntry('configmaps', configMap)
      addValue(vmNames, configMap.data?.VMWARE_MACHINE_OBJECT_NAME)
      addValue(vmNames, configMap.data?.SOURCE_VM_NAME)
      addValue(vmNames, configMap.data?.SOURCE_VM_KEY)
      addValue(nodeNames, configMap.data?.VJAILBREAK_NODE)
    }
  }

  if (podName) {
    const pod = await getCoreObject('pods', podName, namespace, warnings)
    if (pod) addEntry('pods', pod)
  }

  const templateNames = new Set<string>()
  const networkMappingNames = new Set<string>()
  const storageMappingNames = new Set<string>()
  const arrayCredsMappingNames = new Set<string>()
  const arrayCredsNames = new Set<string>()
  const openstackCredNames = new Set<string>()
  const vmwareCredNames = new Set<string>()
  const pcdClusterNames = new Set<string>()
  const pcdHostNames = new Set<string>()
  const rdmDiskNames = new Set<string>()
  const vmwareClusterNames = new Set<string>()
  const vmwareHostNames = new Set<string>()
  const volumeImageProfileNames = new Set<string>()
  const rollingMigrationPlanNames = new Set<string>()
  const bmConfigNames = new Set<string>()

  for (const plan of resourceLists.get('migrationplans') || []) {
    if (!hasValue(planNames, plan.metadata?.name)) continue
    addEntry('migrationplans', plan)
    addValue(templateNames, field(plan.spec, 'migrationTemplate'))
    addValues(volumeImageProfileNames, field(plan.spec, 'advancedOptions', 'imageProfiles'))
  }

  for (const template of resourceLists.get('migrationtemplates') || []) {
    if (!hasValue(templateNames, template.metadata?.name)) continue
    addEntry('migrationtemplates', template)
    addValue(networkMappingNames, field(template.spec, 'networkMapping'))
    addValue(storageMappingNames, field(template.spec, 'storageMapping'))
    addValue(arrayCredsMappingNames, field(template.spec, 'arrayCredsMapping'))
    addValue(openstackCredNames, field(template.spec, 'destination', 'openstackRef'))
    addValue(vmwareCredNames, field(template.spec, 'source', 'vmwareRef'))
    addValue(pcdClusterNames, field(template.spec, 'targetPCDClusterName'))
  }

  for (const machine of resourceLists.get('vmwaremachines') || []) {
    const machineVM = record(field(machine.spec, 'vms'))
    if (
      hasValue(vmNames, machine.metadata?.name) ||
      hasValue(vmNames, machineVM.name) ||
      hasValue(vmNames, machine.metadata?.labels?.['vjailbreak.k8s.pf9.io/vm-key'])
    ) {
      addEntry('vmwaremachines', machine)
      addValue(vmNames, machineVM.name)
      addValues(rdmDiskNames, machineVM.rdmDisks)
      addValue(vmwareClusterNames, machineVM.clusterName)
      addValue(vmwareHostNames, machineVM.esxiName)
    }
  }

  const exactResourceNames: Record<string, Set<string>> = {
    networkmappings: networkMappingNames,
    storagemappings: storageMappingNames,
    arraycredsmappings: arrayCredsMappingNames,
    openstackcreds: openstackCredNames,
    vmwarecreds: vmwareCredNames,
    volumeimageprofiles: volumeImageProfileNames,
    vjailbreaknodes: nodeNames,
    bmconfigs: bmConfigNames
  }

  for (const [plural, names] of Object.entries(exactResourceNames)) {
    for (const object of resourceLists.get(plural) || []) {
      if (
        hasValue(names, object.metadata?.name) ||
        (migrationUid && isOwnedBy(object, migrationOwnerKeys))
      ) {
        addEntry(plural, object)
      }
    }
  }

  const addMatchingOpenstackCreds = () => {
    for (const cred of resourceLists.get('openstackcreds') || []) {
      if (
        hasValue(openstackCredNames, cred.metadata?.name) ||
        (migrationUid && isOwnedBy(cred, migrationOwnerKeys))
      ) {
        addEntry('openstackcreds', cred)
      }
    }
  }

  for (const mapping of resourceLists.get('arraycredsmappings') || []) {
    if (!hasValue(arrayCredsMappingNames, mapping.metadata?.name)) continue
    for (const item of Array.isArray(field(mapping.spec, 'mappings'))
      ? (field(mapping.spec, 'mappings') as unknown[])
      : []) {
      addValue(arrayCredsNames, field(item, 'target'))
    }
  }

  for (const creds of resourceLists.get('arraycreds') || []) {
    if (hasValue(arrayCredsNames, creds.metadata?.name) || isOwnedBy(creds, migrationOwnerKeys)) {
      addEntry('arraycreds', creds)
    }
  }

  for (const disk of resourceLists.get('rdmdisks') || []) {
    const ownerVMs = Array.isArray(field(disk.spec, 'ownerVMs'))
      ? (field(disk.spec, 'ownerVMs') as unknown[])
      : []
    const ownerMatches = ownerVMs.some((ownerVM) => hasValue(vmNames, ownerVM))
    if (
      hasValue(rdmDiskNames, disk.metadata?.name) ||
      hasValue(rdmDiskNames, field(disk.spec, 'diskName')) ||
      ownerMatches ||
      isOwnedBy(disk, migrationOwnerKeys)
    ) {
      addEntry('rdmdisks', disk)
      addValue(openstackCredNames, field(disk.spec, 'openstackVolumeRef', 'openstackCreds'))
    }
  }

  /** OpenStack credential names can be discovered from RDM disks after first exact loop. */
  addMatchingOpenstackCreds()

  for (const cluster of resourceLists.get('pcdclusters') || []) {
    const clusterCreds = value(cluster.metadata?.labels?.['vjailbreak.k8s.pf9.io/openstackcreds'])
    const exactClusterMatch = hasValue(pcdClusterNames, cluster.metadata?.name)
    const displayClusterMatch = hasValue(pcdClusterNames, field(cluster.spec, 'clusterName'))
    const credsMatch = !clusterCreds || openstackCredNames.has(clusterCreds)

    if (exactClusterMatch || (displayClusterMatch && credsMatch)) {
      addEntry('pcdclusters', cluster)
      addValues(pcdHostNames, field(cluster.spec, 'hosts'))
    }
  }

  for (const host of resourceLists.get('pcdhosts') || []) {
    if (
      hasValue(pcdHostNames, host.metadata?.name) ||
      hasValue(pcdHostNames, field(host.spec, 'hostName'))
    ) {
      addEntry('pcdhosts', host)
    }
  }

  for (const cluster of resourceLists.get('vmwareclusters') || []) {
    if (
      hasValue(vmwareClusterNames, cluster.metadata?.name) ||
      hasValue(vmwareClusterNames, field(cluster.spec, 'name'))
    ) {
      addEntry('vmwareclusters', cluster)
      addValues(vmwareHostNames, field(cluster.spec, 'hosts'))
    }
  }

  for (const host of resourceLists.get('vmwarehosts') || []) {
    if (
      hasValue(vmwareHostNames, host.metadata?.name) ||
      hasValue(vmwareHostNames, field(host.spec, 'name'))
    ) {
      addEntry('vmwarehosts', host)
    }
  }

  for (const rollingPlan of resourceLists.get('rollingmigrationplans') || []) {
    const vmMigrationPlans = Array.isArray(field(rollingPlan.spec, 'vmMigrationPlans'))
      ? (field(rollingPlan.spec, 'vmMigrationPlans') as unknown[])
      : []
    const linkedPlan = vmMigrationPlans.some((planName) => hasValue(planNames, planName))
    const linkedVM =
      hasValue(vmNames, field(rollingPlan.status, 'currentVM')) ||
      (Array.isArray(field(rollingPlan.status, 'migratedVMs')) &&
        (field(rollingPlan.status, 'migratedVMs') as unknown[]).some((vm) =>
          hasValue(vmNames, vm)
        )) ||
      (Array.isArray(field(rollingPlan.status, 'failedVMs')) &&
        (field(rollingPlan.status, 'failedVMs') as unknown[]).some((vm) => hasValue(vmNames, vm)))

    if (linkedPlan || linkedVM) {
      addEntry('rollingmigrationplans', rollingPlan)
      addValue(rollingMigrationPlanNames, rollingPlan.metadata?.name)
      addValue(bmConfigNames, field(rollingPlan.spec, 'bmConfigRef', 'name'))
    }
  }

  for (const object of resourceLists.get('bmconfigs') || []) {
    if (hasValue(bmConfigNames, object.metadata?.name)) {
      addEntry('bmconfigs', object)
    }
  }

  for (const clusterMigration of resourceLists.get('clustermigrations') || []) {
    if (
      hasValue(
        rollingMigrationPlanNames,
        field(clusterMigration.spec, 'rollingMigrationPlanRef', 'name')
      )
    ) {
      addEntry('clustermigrations', clusterMigration)
    }
  }

  for (const esxiMigration of resourceLists.get('esximigrations') || []) {
    if (
      hasValue(
        rollingMigrationPlanNames,
        field(esxiMigration.spec, 'rollingMigrationPlanRef', 'name')
      ) ||
      hasValue(vmwareHostNames, field(esxiMigration.spec, 'esxiName'))
    ) {
      addEntry('esximigrations', esxiMigration)
    }
  }

  for (const sshCreds of resourceLists.get('esxisshcreds') || []) {
    if (isOwnedBy(sshCreds, migrationOwnerKeys)) {
      addEntry('esxisshcreds', sshCreds)
    }
  }

  return formatYamlBundle(Array.from(entries.values()), warnings)
}
