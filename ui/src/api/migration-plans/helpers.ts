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
    virtualMachines,
    adminInitiatedCutOver,
    postMigrationAction,
    disconnectSourceNetwork = false,
    securityGroups,
    fallbackToDHCP = false,
    postMigrationScript,
  } = params || {}

  const spec: Record<string, unknown> = {
    migrationTemplate: migrationTemplateName,
    retry,
    migrationStrategy: {
      type,
      dataCopyStart,
      adminInitiatedCutOver,
      vmCutoverStart,
      vmCutoverEnd,
      disconnectSourceNetwork,
    },
    virtualMachines: [virtualMachines],
    fallbackToDHCP,
  }

  // Add firstBootScript if postMigrationScript is provided
  if (postMigrationScript && postMigrationScript.trim()) {
    spec.firstBootScript = postMigrationScript
  }

  if (
    postMigrationAction &&
    (postMigrationAction.renameVm || postMigrationAction.moveToFolder)
  ) {
    spec.postMigrationAction = {
      renameVm: postMigrationAction.renameVm || false,
      suffix: postMigrationAction.suffix || "",
      moveToFolder: postMigrationAction.moveToFolder || false,
      folderName: postMigrationAction.folderName || "vjailbreakedVMs",
    }
  }

  if (securityGroups && securityGroups.length > 0) {
    spec.securityGroups = securityGroups
  }

  return {
    apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
    kind: "MigrationPlan",
    metadata: {
      name: name || uuidv4(),
    },
    spec,
  }
}
