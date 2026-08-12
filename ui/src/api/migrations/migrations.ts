import axios from '../axios'
import { K8S_PROXY_BASE_PATH, VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'
import { GetMigrationsList, Migration } from './model'

export const getMigrations = async (
  migrationPlanName = '',
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<Migration[]> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrations`
  const params = {
    ...(migrationPlanName ? { labelSelector: `migrationplan=${migrationPlanName}` } : {})
  }
  const data = await axios.get<GetMigrationsList>({
    endpoint,
    config: { params }
  })
  return data?.items
}

export interface MigrationConfigMap {
  data?: Record<string, string>
}

export const getMigrationConfigMap = async (
  vmwareMachineName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<MigrationConfigMap> => {
  const endpoint = `${K8S_PROXY_BASE_PATH}/namespaces/${namespace}/configmaps/migration-config-${vmwareMachineName}`
  return axios.get<MigrationConfigMap>({ endpoint })
}

export const getMigration = async (migrationName, namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrations/${migrationName}`
  const response = await axios.get<Migration>({
    endpoint
  })
  return response
}

export const deleteMigration = async (migrationName, namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrations/${migrationName}`
  const response = await axios.del<Migration>({
    endpoint
  })
  return response
}

/**
 * Resolves the helper pod backing a migration.
 *
 * The migration records a podRef, but the pod's real name has a generated suffix,
 * so it has to be matched by prefix against the pods in the namespace. Throws
 * rather than returning null: every caller treats an unresolvable pod as a hard
 * failure, and the thrown message is what gets surfaced to the operator.
 */
export const resolveMigrationPod = async (
  namespace: string,
  migrationName: string
): Promise<string> => {
  const migration = await getMigration(migrationName, namespace)
  const podRef = migration.spec?.podRef

  if (!podRef) {
    throw new Error('PodRef is empty in migration object')
  }

  const podsEndpoint = `${K8S_PROXY_BASE_PATH}/namespaces/${namespace}/pods`
  const podsResponse = await axios.get<{
    items: Array<{
      metadata: {
        name: string
        namespace: string
      }
    }>
  }>({
    endpoint: podsEndpoint
  })

  if (!podsResponse?.items || podsResponse.items.length === 0) {
    throw new Error(`No pods found in namespace: ${namespace}`)
  }

  const matchingPod = podsResponse.items.find((pod) => pod.metadata.name.startsWith(podRef))

  if (!matchingPod) {
    throw new Error(`No pod found with name starting with: ${podRef}`)
  }

  return matchingPod.metadata.name
}

/**
 * Merge-patches labels onto a migration's helper pod.
 *
 * Both operator gates — admin cutover and the LDM boot gate — are answered by
 * setting a label the v2v-helper is watching, so they share this path rather than
 * each reimplementing resolve-then-patch.
 */
export const patchMigrationPodLabels = async (
  namespace: string,
  migrationName: string,
  labels: Record<string, string>
): Promise<void> => {
  const podName = await resolveMigrationPod(namespace, migrationName)

  await axios.patch({
    endpoint: `${K8S_PROXY_BASE_PATH}/namespaces/${namespace}/pods/${podName}`,
    data: {
      metadata: {
        labels
      }
    },
    config: {
      headers: {
        'Content-Type': 'application/merge-patch+json'
      }
    }
  })
}

export const triggerAdminCutover = async (
  namespace: string,
  migrationName: string
): Promise<{ success: boolean; message: string }> => {
  try {
    await patchMigrationPodLabels(namespace, migrationName, { startCutover: 'yes' })

    return {
      success: true,
      message: 'Successfully triggered cutover'
    }
  } catch (error) {
    console.error('Failed to trigger cutover:', error)
    return {
      success: false,
      message: error instanceof Error ? error.message : 'Failed to trigger cutover'
    }
  }
}

/**
 * Answer at the WaitingForLDMBootSuccess gate.
 *
 * Only reached by a guest whose system volume is on a Windows Dynamic Disk (LDM).
 * virt-v2v cannot convert such a guest, so it is created on an emulated SATA bus
 * with a scratch virtio volume attached; Windows installs the virtio storage
 * driver against that device on first boot. The admin confirms from the guest
 * whether that happened.
 *
 *   success - recreate the VM with its root disk on the virtio bus
 *   finish  - leave the VM on SATA, complete the migration successfully
 *   failed  - DESTRUCTIVE: fail the migration and run the standard cleanup
 *
 * Patches the pod label directly, the same path triggerAdminCutover uses, so no
 * new backend endpoint is needed. Deliberately a separate label from
 * startCutover so the cutover flow is untouched.
 */
export type LDMBootStatus = 'success' | 'finish' | 'failed'

export const setLDMBootStatus = async (
  namespace: string,
  migrationName: string,
  status: LDMBootStatus
): Promise<{ success: boolean; message: string }> => {
  try {
    await patchMigrationPodLabels(namespace, migrationName, { ldmBootStatus: status })

    return {
      success: true,
      message: `Recorded LDM boot status: ${status}`
    }
  } catch (error) {
    console.error('Failed to set LDM boot status:', error)
    return {
      success: false,
      message: error instanceof Error ? error.message : 'Failed to set LDM boot status'
    }
  }
}

