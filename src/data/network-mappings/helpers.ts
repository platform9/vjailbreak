import { v4 as uuidv4 } from "uuid"
export const createMigrationTemplateJson = (params) => {
  const { name, namespace = "migration-system", networks = [] } = params || {}
  return {
    apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
    kind: "NetworkMapping",
    metadata: {
      name: name || uuidv4(),
      namespace: namespace,
    },
    spec: {
      networks,
    },
  }
}
