import { v4 as uuidv4 } from "uuid"

export const createMigrationPlanJson = (params) => {
  const {
    name,
    migrationTemplateName,
    retry = false,
    type = "hot",
    dataCopyStart,
    vmCutoverStart,
    vmCutoverEnd,
    virtualmachines,
  } = params || {}
  return {
    apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
    kind: "MigrationPlan",
    metadata: {
      name: name || uuidv4(),
    },
    spec: {
      migrationTemplate: migrationTemplateName,
      retry,
      migrationStrategy: {
        type,
        dataCopyStart,
        vmCutoverStart,
        vmCutoverEnd,
      },
      virtualmachines: [virtualmachines],
    },
  }
}
