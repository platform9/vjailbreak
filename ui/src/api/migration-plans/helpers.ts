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
  } = params || {}
  
  const spec: any = {  
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
  }

 
  if (postMigrationAction && 
      (postMigrationAction.renameVm || postMigrationAction.moveToFolder)) {
    spec.postMigrationAction = {
      renameVm: postMigrationAction.renameVm || false,
      suffix: postMigrationAction.suffix || "",
      moveToFolder: postMigrationAction.moveToFolder || false,
      folderName: postMigrationAction.folderName || "vjailbreakedVMs"
    }
  }

  if (securityGroups && securityGroups.length > 0) {
    spec.securityGroups = securityGroups;
  }
  
  return {
    apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
    kind: "MigrationPlan",
    metadata: {
      name: name || uuidv4(),
    },
    spec
  }
}