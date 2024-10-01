import { v4 as uuidv4 } from "uuid"

export const createMigrationTemplateJson = (params) => {
  const {
    name,
    networkMapping = "",
    storageMapping = "",
    virtioWinDriver,
    osType,
    datacenter,
    vmwareRef,
    openstackRef,
  } = params || {}
  return {
    apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
    kind: "MigrationTemplate",
    metadata: {
      name: name || uuidv4(),
    },
    spec: {
      networkMapping: networkMapping,
      storageMapping: storageMapping,
      virtioWinDriver: virtioWinDriver,
      osType: osType,
      source: {
        datacenter: datacenter,
        vmwareRef: vmwareRef,
      },
      destination: {
        openstackRef: openstackRef,
      },
    },
  }
}
