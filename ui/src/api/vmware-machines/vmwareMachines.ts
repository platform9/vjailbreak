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
          labelSelector: `vmwarecreds.k8s.pf9.io-${vmwareCredName}=true`,
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
  let vmNameK8s = convertToK8sName(vmName).name

  if (convertToK8sName(vmName).error) {
    throw new Error(convertToK8sName(vmName).error)
  }

  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwaremachines/${vmNameK8s}`
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


function convertToK8sName(name: string, maxLength = 242) {
  // Convert to lowercase
  name = name.toLowerCase();

  // Replace underscores and spaces with hyphens
  name = name.replace(/[_\s]/g, '-');

  // Remove all characters not allowed in K8s names
  name = name.replace(/[^a-z0-9\-.]/g, '');

  // Remove leading and trailing hyphens
  name = name.replace(/^-+|-+$/g, '');

  // Truncate to the max allowed length
  if (name.length > maxLength) {
    name = name.substring(0, maxLength);
  }

  // Validate the name against Kubernetes DNS-1123 subdomain rules
  const k8sNameRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/;
  if (k8sNameRegex.test(name)) {
    return { name: name, error: "" };
  } else {
    return { name: name, error: `name '${name}' is not a valid K8s name` };
  }
}
