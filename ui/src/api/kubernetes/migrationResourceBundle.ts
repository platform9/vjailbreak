import axios from '../axios'
import { KUBERNETES_API_BASE_PATH, VJAILBREAK_API_BASE_PATH } from '../constants'

type Metadata = {
  name?: string
  namespace?: string
  uid?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  ownerReferences?: Array<{
    apiVersion?: string
    kind?: string
    name?: string
    uid?: string
  }>
}

type UnknownRecord = Record<string, unknown>

type KubernetesObject = {
  apiVersion?: string
  kind?: string
  metadata?: Metadata
  spec?: UnknownRecord
  status?: UnknownRecord
  data?: Record<string, string>
}

type KubernetesList = {
  items?: KubernetesObject[]
}

type ResourceDefinition = {
  plural: string
}

type BundleEntry = {
  path: string
  object: KubernetesObject
}

const RELATED_CRD_RESOURCES: ResourceDefinition[] = [
  { plural: 'arraycreds' },
  { plural: 'arraycredsmappings' },
  { plural: 'bmconfigs' },
  { plural: 'clustermigrations' },
  { plural: 'esximigrations' },
  { plural: 'esxisshcreds' },
  { plural: 'migrationplans' },
  { plural: 'migrations' },
  { plural: 'migrationtemplates' },
  { plural: 'networkmappings' },
  { plural: 'openstackcreds' },
  { plural: 'pcdclusters' },
  { plural: 'pcdhosts' },
  { plural: 'rdmdisks' },
  { plural: 'rollingmigrationplans' },
  { plural: 'storagemappings' },
  { plural: 'vjailbreaknodes' },
  { plural: 'vmwareclusters' },
  { plural: 'vmwarecreds' },
  { plural: 'vmwarehosts' },
  { plural: 'vmwaremachines' },
  { plural: 'volumeimageprofiles' }
]

const value = (input: unknown): string => (typeof input === 'string' ? input.trim() : '')

const record = (input: unknown): UnknownRecord =>
  input && typeof input === 'object' && !Array.isArray(input) ? (input as UnknownRecord) : {}

const field = (input: unknown, ...keys: string[]): unknown =>
  keys.reduce<unknown>((current, key) => record(current)[key], input)

const hasValue = (set: Set<string>, input: unknown): boolean => {
  const next = value(input)
  return Boolean(next && set.has(next))
}

const addValue = (set: Set<string>, input: unknown) => {
  const next = value(input)
  if (next) set.add(next)
}

const addValues = (set: Set<string>, inputs: unknown) => {
  if (!Array.isArray(inputs)) return
  inputs.forEach((input) => addValue(set, input))
}

const objectName = (object: KubernetesObject): string => value(object.metadata?.name)

const getItems = async (
  plural: string,
  namespace: string,
  warnings: string[]
): Promise<KubernetesObject[]> => {
  try {
    const response = await axios.get<KubernetesList>({
      endpoint: `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${plural}`,
      config: { mock: false }
    })
    return Array.isArray(response?.items) ? response.items : []
  } catch (error) {
    warnings.push(
      `Failed to list ${plural}: ${error instanceof Error ? error.message : String(error)}`
    )
    return []
  }
}

const getCoreObject = async (
  plural: string,
  name: string,
  namespace: string,
  warnings: string[]
): Promise<KubernetesObject | null> => {
  try {
    return await axios.get<KubernetesObject>({
      endpoint: `${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/${plural}/${name}`,
      config: { mock: false }
    })
  } catch (error) {
    warnings.push(
      `Failed to get ${plural}/${name}: ${error instanceof Error ? error.message : String(error)}`
    )
    return null
  }
}

const listCoreObjects = async (
  plural: string,
  namespace: string,
  warnings: string[]
): Promise<KubernetesObject[]> => {
  try {
    const response = await axios.get<KubernetesList>({
      endpoint: `${KUBERNETES_API_BASE_PATH}/namespaces/${namespace}/${plural}`,
      config: { mock: false }
    })
    return Array.isArray(response?.items) ? response.items : []
  } catch (error) {
    warnings.push(
      `Failed to list ${plural}: ${error instanceof Error ? error.message : String(error)}`
    )
    return []
  }
}

const isOwnedBy = (object: KubernetesObject, owners: Set<string>): boolean => {
  const ownerReferences = object.metadata?.ownerReferences || []
  return ownerReferences.some(
    (owner) => hasValue(owners, owner.uid) || hasValue(owners, owner.name)
  )
}

const isScalar = (input: unknown): boolean =>
  input === null || ['string', 'number', 'boolean'].includes(typeof input)

const isEmptyObject = (input: unknown): boolean =>
  Boolean(input && typeof input === 'object' && !Array.isArray(input)) &&
  Object.keys(input as UnknownRecord).length === 0

const plainStringPattern = /^[A-Za-z0-9_.@/:=,-]+$/
const plainKeyPattern = /^[A-Za-z_][A-Za-z0-9_.-]*$/
const numberLikePattern = /^[+-]?\d+(\.\d+)?$/
const timestampPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/
const yamlAmbiguousValues = new Set(['true', 'false', 'null', 'yes', 'no', 'on', 'off'])

const formatKey = (key: string): string => (plainKeyPattern.test(key) ? key : JSON.stringify(key))

const formatScalar = (input: unknown, indent: number): string => {
  if (input === null) return 'null'
  if (typeof input === 'number' || typeof input === 'boolean') return String(input)
  if (typeof input !== 'string') return JSON.stringify(input)
  if (input === '') return '""'

  if (input.includes('\n')) {
    const padding = ' '.repeat(indent + 2)
    const lines = input.replace(/\n$/, '').split('\n')
    return `|\n${lines.map((line) => `${padding}${line}`).join('\n')}`
  }

  const lower = input.toLowerCase()
  if (
    plainStringPattern.test(input) &&
    !yamlAmbiguousValues.has(lower) &&
    !numberLikePattern.test(input) &&
    !timestampPattern.test(input)
  ) {
    return input
  }

  if (
    (input.startsWith('{') && input.endsWith('}')) ||
    (input.startsWith('[') && input.endsWith(']'))
  ) {
    return `'${input.replace(/'/g, "''")}'`
  }

  return JSON.stringify(input)
}

const toYaml = (input: unknown, indent = 0): string => {
  const padding = ' '.repeat(indent)

  if (isScalar(input)) {
    return `${padding}${formatScalar(input, indent)}`
  }

  if (Array.isArray(input)) {
    if (input.length === 0) return `${padding}[]`

    return input
      .map((item) => {
        if (isScalar(item)) {
          return `${padding}- ${formatScalar(item, indent)}`
        }

        if (Array.isArray(item) && item.length === 0) {
          return `${padding}- []`
        }

        if (isEmptyObject(item)) {
          return `${padding}- {}`
        }

        const lines = toYaml(item, indent + 2).split('\n')
        return `${padding}- ${lines[0].trimStart()}${lines.length > 1 ? `\n${lines.slice(1).join('\n')}` : ''}`
      })
      .join('\n')
  }

  const object = record(input)
  const entries = Object.entries(object)
  if (entries.length === 0) return `${padding}{}`

  return entries
    .map(([key, item]) => {
      if (isScalar(item)) {
        return `${padding}${formatKey(key)}: ${formatScalar(item, indent)}`
      }

      if (Array.isArray(item) && item.length === 0) {
        return `${padding}${formatKey(key)}: []`
      }

      if (isEmptyObject(item)) {
        return `${padding}${formatKey(key)}: {}`
      }

      return `${padding}${formatKey(key)}:\n${toYaml(item, indent + 2)}`
    })
    .join('\n')
}

const removeManagedFields = (object: KubernetesObject): KubernetesObject => {
  const metadata = object.metadata
    ? ({
        ...object.metadata,
        managedFields: undefined
      } as Metadata)
    : undefined

  if (metadata) {
    delete (metadata as UnknownRecord).managedFields
  }

  return {
    ...object,
    metadata
  }
}

const omitUndefined = (object: UnknownRecord): UnknownRecord =>
  Object.fromEntries(Object.entries(object).filter(([, item]) => item !== undefined))

const orderMetadata = (metadata?: Metadata): UnknownRecord | undefined => {
  if (!metadata) return undefined

  const knownKeys = new Set([
    'annotations',
    'creationTimestamp',
    'finalizers',
    'generateName',
    'generation',
    'labels',
    'name',
    'namespace',
    'ownerReferences',
    'resourceVersion',
    'uid'
  ])
  const extra = Object.fromEntries(
    Object.entries(metadata as UnknownRecord).filter(([key]) => !knownKeys.has(key))
  )

  return omitUndefined({
    annotations: (metadata as UnknownRecord).annotations,
    creationTimestamp: (metadata as UnknownRecord).creationTimestamp,
    finalizers: (metadata as UnknownRecord).finalizers,
    generateName: (metadata as UnknownRecord).generateName,
    generation: (metadata as UnknownRecord).generation,
    labels: (metadata as UnknownRecord).labels,
    name: metadata.name,
    namespace: metadata.namespace,
    ownerReferences: metadata.ownerReferences,
    resourceVersion: (metadata as UnknownRecord).resourceVersion,
    uid: metadata.uid,
    ...extra
  })
}

const withKubectlLikeOrder = (plural: string, object: KubernetesObject): UnknownRecord => {
  const clean = removeManagedFields(object)

  if (plural === 'configmaps') {
    return omitUndefined({
      apiVersion: clean.apiVersion || 'v1',
      data: clean.data,
      kind: clean.kind || 'ConfigMap',
      metadata: orderMetadata(clean.metadata)
    })
  }

  if (plural === 'pods') {
    return omitUndefined({
      apiVersion: clean.apiVersion || 'v1',
      kind: clean.kind || 'Pod',
      metadata: orderMetadata(clean.metadata),
      spec: clean.spec,
      status: clean.status
    })
  }

  return omitUndefined({
    apiVersion: clean.apiVersion,
    kind: clean.kind,
    metadata: orderMetadata(clean.metadata),
    spec: clean.spec,
    status: clean.status,
    data: clean.data
  })
}

const formatEntries = (entries: BundleEntry[], warnings: string[]): string => {
  let output = ''
  const separator = `${'='.repeat(80)}\n`

  for (const [index, entry] of entries.sort((a, b) => a.path.localeCompare(b.path)).entries()) {
    if (index > 0) output += '\n'
    output += separator
    output += `FILE: ${entry.path}\n`
    output += separator
    output += `${toYaml(withKubectlLikeOrder(entry.path.split('/')[1] || '', entry.object))}\n`
  }

  if (warnings.length > 0) {
    if (output) output += '\n'
    output += separator
    output += 'FILE: collection-warnings.txt\n'
    output += separator
    output += warnings.join('\n')
    output += '\n'
  }

  return output
}

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
    RELATED_CRD_RESOURCES.map(async (resource) => {
      resourceLists.set(resource.plural, await getItems(resource.plural, namespace, warnings))
    })
  )

  const migrations = resourceLists.get('migrations') || []
  const migration =
    migrations.find((item) => objectName(item) === migrationName) ||
    migrations.find((item) => value(field(item.spec, 'podRef')) === podName)

  if (!migration) {
    warnings.push(`Migration resource not found for migrationName=${migrationName || '<empty>'}`)
    return formatEntries([], warnings)
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

  return formatEntries(Array.from(entries.values()), warnings)
}
