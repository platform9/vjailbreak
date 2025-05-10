import { VMwareMachineList, VMwareMachine } from "./model"
import { VmData } from "../migration-templates/model"
import { VJAILBREAK_API_BASE_PATH } from "../constants"
import { VJAILBREAK_DEFAULT_NAMESPACE } from "../constants"
import axios from "../axios"

export const getVMwareMachines = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE,
  vmwareCredName?: string
): Promise<VMwareMachineList> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwaremachines`

  // Use label selector if vmwareCredName is provided
  const config = vmwareCredName
    ? {
        params: {
          labelSelector: `vjailbreak.k8s.pf9.io/vmwarecreds=${vmwareCredName}`,
        },
      }
    : undefined

  return axios.get<VMwareMachineList>({
    endpoint,
    config,
  })
}

/**
 * Update a VMware machine's target flavor ID
 * @param vmName - The name of the VM to update
 * @param flavorId - The ID of the flavor to assign
 * @param namespace - The namespace of the VM (defaults to migration-system)
 */
export const patchVMwareMachine = async (
  vmName: string,
  flavorId: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<VMwareMachine> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwaremachines/${vmName}`
  const payload = {
    spec: {
      targetFlavorId: flavorId,
    },
  }

  return axios.patch<VMwareMachine>({
    endpoint,
    data: payload,
    config: {
      headers: {
        "Content-Type": "application/merge-patch+json",
      },
    },
  })
}
export const mapToVmData = (machines: VMwareMachine[]): VmData[] => {
  return machines.map((machine) => ({
    name: machine.spec.vms.name,
    vmState: machine.status.powerState === "running" ? "running" : "stopped",
    ipAddress: machine.spec.vms.ipAddress,
    networks: machine.spec.vms.networks || [],
    datastores: machine.spec.vms.datastores || [],
    memory: machine.spec.vms.memory,
    cpuCount: machine.spec.vms.cpu,
    isMigrated: machine.status.migrated,
    disks: machine.spec.vms.disks || [],
    targetFlavorId: machine.spec.targetFlavorId,
    labels: machine.metadata.labels,
    osType: machine.spec.vms.osType,

  }))
}
