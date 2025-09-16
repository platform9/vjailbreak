import { VMwareMachineList, VMwareMachine } from "./model"
import { VmData } from "../migration-templates/model"
import { VJAILBREAK_API_BASE_PATH } from "../constants"
import { VJAILBREAK_DEFAULT_NAMESPACE } from "../constants"
import axios from "../axios"
import { VmNetworkInterface } from "./model"
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

  const response = await axios.get<VMwareMachineList>({
    endpoint,
    config,
  })

  // TODO: REMOVE MOCK DATA - Add mock RDM VMs for testing
  const mockRdmVMs: VMwareMachine[] = [
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "VmwareMachine",
      metadata: {
        name: "vm001-mock-rdm",
        namespace: namespace,
        creationTimestamp: "2024-01-15T10:00:00Z",
        labels: {
          "vjailbreak.k8s.pf9.io/is-shared-rdm": "true",
          "vjailbreak.k8s.pf9.io/vmware-cluster": "test-cluster",
          "vjailbreak.k8s.pf9.io/esxi-name": "esx-host-1",
        },
      },
      spec: {
        vms: {
          name: "vm001-mock-rdm",
          cpu: 4,
          memory: 8192,
          datastores: ["datastore1"],
          disks: ["disk1", "disk2"],
          networks: ["VM Network"],
          vmState: "stopped",
          ipAddress: "192.168.1.100",
          osFamily: "windowsGuest",
          networkInterfaces: [
            {
              mac: "00:50:56:12:34:56",
              network: "VM Network",
              ipAddress: "192.168.1.100",
            },
          ],
        },
        targetFlavorId: "",
        rdmDisks: ["rdm-disk-shared-1"],
      },
      status: {
        migrated: false,
        powerState: "notRunning",
      },
    },
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "VmwareMachine",
      metadata: {
        name: "vm002-mock-rdm",
        namespace: namespace,
        creationTimestamp: "2024-01-15T10:00:00Z",
        labels: {
          "vjailbreak.k8s.pf9.io/is-shared-rdm": "true",
          "vjailbreak.k8s.pf9.io/vmware-cluster": "test-cluster",
          "vjailbreak.k8s.pf9.io/esxi-name": "esx-host-2",
        },
      },
      spec: {
        vms: {
          name: "vm002-mock-rdm",
          cpu: 2,
          memory: 4096,
          datastores: ["datastore1"],
          disks: ["disk3", "disk4"],
          networks: ["VM Network"],
          vmState: "running",
          ipAddress: "192.168.1.101",
          osFamily: "linuxGuest",
          networkInterfaces: [
            {
              mac: "00:50:56:12:34:57",
              network: "VM Network",
              ipAddress: "192.168.1.101",
            },
          ],
        },
        targetFlavorId: "",
        rdmDisks: ["rdm-disk-shared-1"],
      },
      status: {
        migrated: false,
        powerState: "running",
      },
    },
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "VmwareMachine",
      metadata: {
        name: "vm003-mock-rdm",
        namespace: namespace,
        creationTimestamp: "2024-01-15T10:00:00Z",
        labels: {
          "vjailbreak.k8s.pf9.io/is-shared-rdm": "true",
          "vjailbreak.k8s.pf9.io/vmware-cluster": "test-cluster",
          "vjailbreak.k8s.pf9.io/esxi-name": "esx-host-3",
        },
      },
      spec: {
        vms: {
          name: "vm003-mock-rdm",
          cpu: 4,
          memory: 8192,
          datastores: ["datastore2"],
          disks: ["disk5", "disk6"],
          networks: ["VM Network"],
          vmState: "stopped",
          ipAddress: "192.168.1.102",
          osFamily: "windowsGuest",
          networkInterfaces: [
            {
              mac: "00:50:56:12:34:58",
              network: "VM Network",
              ipAddress: "192.168.1.102",
            },
          ],
        },
        targetFlavorId: "",
        rdmDisks: ["rdm-disk-shared-2"],
      },
      status: {
        migrated: false,
        powerState: "notRunning",
      },
    },
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "VmwareMachine",
      metadata: {
        name: "vm004-normal",
        namespace: namespace,
        creationTimestamp: "2024-01-15T10:00:00Z",
        labels: {
          "vjailbreak.k8s.pf9.io/vmware-cluster": "test-cluster",
          "vjailbreak.k8s.pf9.io/esxi-name": "esx-host-1",
        },
      },
      spec: {
        vms: {
          name: "vm004-normal",
          cpu: 2,
          memory: 4096,
          datastores: ["datastore1"],
          disks: ["disk7", "disk8"],
          networks: ["VM Network"],
          vmState: "running",
          ipAddress: "192.168.1.103",
          osFamily: "linuxGuest",
          networkInterfaces: [
            {
              mac: "00:50:56:12:34:59",
              network: "VM Network",
              ipAddress: "192.168.1.103",
            },
          ],
        },
        targetFlavorId: "",
      },

      status: {
        migrated: false,
        powerState: "running",
      },
    },
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "VmwareMachine",
      metadata: {
        name: "vm005-mock-rdm",
        namespace: namespace,
        creationTimestamp: "2024-01-15T10:00:00Z",
        labels: {
          "vjailbreak.k8s.pf9.io/is-shared-rdm": "true",
          "vjailbreak.k8s.pf9.io/vmware-cluster": "test-cluster",
          "vjailbreak.k8s.pf9.io/esxi-name": "esx-host-1",
        },
      },
      spec: {
        vms: {
          name: "vm005-mock-rdm",
          cpu: 2,
          memory: 4096,
          datastores: ["datastore1"],
          disks: ["disk7", "disk8"],
          networks: ["VM Network"],
          vmState: "running",
          ipAddress: "192.168.1.103",
          osFamily: "linuxGuest",
          networkInterfaces: [
            {
              mac: "00:50:56:12:34:59",
              network: "VM Network",
              ipAddress: "192.168.1.103",
            },
          ],
        },
        rdmDisks: ["rdm-disk-shared-2"],
        targetFlavorId: "",
      },

      status: {
        migrated: false,
        powerState: "running",
      },
    },
  ]

  // Add mock RDM VMs to the response
  const mockResponse: VMwareMachineList = {
    ...response,
    items: [...response.items, ...mockRdmVMs],
  }

  return mockResponse
}

/**
 * Update a VMware machine's properties
 * @param vmName - The name of the VM to update
 * @param payload - The payload containing fields to update
 * @param namespace - The namespace of the VM (defaults to migration-system)
 */

export const patchVMwareMachine = async (
  vmName: string,
  payload: {
    spec?: {
      targetFlavorId?: string
      vms?: {
        assignedIp?: string
        osFamily?: string
        networkInterfaces?: VmNetworkInterface[]
      }
    }
  },
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<VMwareMachine> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwaremachines/${vmName}`

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
    id: machine.spec.vms.name,
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
    osFamily: machine.spec.vms.osFamily,
    esxHost:
      machine.metadata?.labels?.[`vjailbreak.k8s.pf9.io/esxi-name`] || "",
    vmWareMachineName: machine.metadata.name,
    networkInterfaces: machine.spec.vms.networkInterfaces?.map((nic) => ({
      mac: nic.mac,
      network: nic.network,
      ipAddress: nic.ipAddress,
    })),
  }))
}

export const getVMwareMachine = async (
  vmName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<VMwareMachine> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vmwaremachines/${vmName}`

  return axios.get<VMwareMachine>({
    endpoint,
  })
}
