import axios from "../axios"

import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import { RdmDisk, RdmDiskList } from "./model"

export const getRdmDisksList = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/rdmdisks`

  try {
    const response = await axios.get<RdmDiskList>({
      endpoint,
    })

    // TODO: REMOVE MOCK DATA - Add mock RDM disks for testing
    const mockRdmDisks: RdmDisk[] = [
      {
        apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
        kind: "RDMDisk",
        metadata: {
          name: "rdm-disk-shared-1",
          namespace: namespace,
          creationTimestamp: "2024-01-15T10:00:00Z",
          labels: {
            "vjailbreak.k8s.pf9.io/rdm-type": "shared",
          },
        },
        spec: {
          diskName: "rdm-disk-shared-1",
          diskSize: 1099511627776, // 1TB in bytes
          displayName: "Shared RDM Disk 1",
          importToCinder: true,
          ownerVMs: ["vm001-mock-rdm", "vm002-mock-rdm"],
          uuid: "6000C298-1234-5678-9ABC-DEF012345678",
          openstackVolumeRef: {
            cinderBackendPool: "",
            volumeType: "",
            source: {},
          },
        },
        status: {
          phase: "Available",
          conditions: [
            {
              lastTransitionTime: "2024-01-15T10:00:00Z",
              message: "RDM disk is available for configuration",
              reason: "Available",
              status: "True",
              type: "Ready",
            },
          ],
        },
      },
      {
        apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
        kind: "RDMDisk",
        metadata: {
          name: "rdm-disk-shared-2",
          namespace: namespace,
          creationTimestamp: "2024-01-15T10:00:00Z",
          labels: {
            "vjailbreak.k8s.pf9.io/rdm-type": "shared",
          },
        },
        spec: {
          diskName: "rdm-disk-shared-2",
          diskSize: 2199023255552, // 2TB in bytes
          displayName: "Shared RDM Disk 2",
          importToCinder: true,
          ownerVMs: ["vm003-mock-rdm", "vm005-mock-rdm"],
          uuid: "6000C298-ABCD-EFGH-1234-567890ABCDEF",
          openstackVolumeRef: {
            cinderBackendPool: "",
            volumeType: "",
            source: {},
          },
        },
        status: {
          phase: "Available",
          conditions: [
            {
              lastTransitionTime: "2024-01-15T10:00:00Z",
              message: "RDM disk is available for configuration",
              reason: "Available",
              status: "True",
              type: "Ready",
            },
          ],
        },
      },
    ]

    return [...(response?.items || []), ...mockRdmDisks]
  } catch (error) {

    // Return only mock data if API call fails
    const mockRdmDisks: RdmDisk[] = [
      {
        apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
        kind: "RDMDisk",
        metadata: {
          name: "rdm-disk-shared-1",
          namespace: namespace,
          creationTimestamp: "2024-01-15T10:00:00Z",
          labels: {
            "vjailbreak.k8s.pf9.io/rdm-type": "shared",
          },
        },
        spec: {
          diskName: "rdm-disk-shared-1",
          diskSize: 1099511627776,
          displayName: "Shared RDM Disk 1",
          importToCinder: true,
          ownerVMs: ["vm001-mock-rdm", "vm002-mock-rdm"],
          uuid: "6000C298-1234-5678-9ABC-DEF012345678",
          openstackVolumeRef: {
            cinderBackendPool: "",
            volumeType: "",
            source: {},
          },
        },
        status: {
          phase: "Available",
          conditions: [
            {
              lastTransitionTime: "2024-01-15T10:00:00Z",
              message: "RDM disk is available for configuration",
              reason: "Available",
              status: "True",
              type: "Ready",
            },
          ],
        },
      },
      {
        apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
        kind: "RDMDisk",
        metadata: {
          name: "rdm-disk-shared-2",
          namespace: namespace,
          creationTimestamp: "2024-01-15T10:00:00Z",
          labels: {
            "vjailbreak.k8s.pf9.io/rdm-type": "shared",
          },
        },
        spec: {
          diskSize: 2199023255552,
          displayName: "Shared RDM Disk 2",
          importToCinder: true,
          ownerVMs: ["vm003-mock-rdm", "vm005-mock-rdm"],
          uuid: "6000C298-ABCD-EFGH-1234-567890ABCDEF",
          diskName: "rdm-disk-shared-2",
          openstackVolumeRef: {
            cinderBackendPool: "",
            volumeType: "",
            source: {},
          },
        },
        status: {
          phase: "Available",
          conditions: [
            {
              lastTransitionTime: "2024-01-15T10:00:00Z",
              message: "RDM disk is available for configuration",
              reason: "Available",
              status: "True",
              type: "Ready",
            },
          ],
        },
      },
    ]

    return mockRdmDisks
  }
}

export const getRdmDisk = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/rdmdisks/${name}`
  const response = await axios.get<RdmDisk>({
    endpoint,
  })
  return response
}

export const patchRdmDisk = async (
  name: string,
  data: Partial<RdmDisk>,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/rdmdisks/${name}`
  const response = await axios.patch<RdmDisk>({
    endpoint,
    data,
    config: {
      headers: {
        'Content-Type': 'application/merge-patch+json'
      }
    }
  })
  return response
}
