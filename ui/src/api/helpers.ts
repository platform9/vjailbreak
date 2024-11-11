import {
  deleteMigrationPlan,
  getMigrationPlansList,
} from "./migration-plans/migrationPlans"
import {
  deleteMigrationTemplate,
  getMigrationTemplatesList,
} from "./migration-templates/migrationTemplates"
import {
  deleteNetworkMapping,
  getNetworkMappingList,
} from "./network-mapping/networkMappings"
import {
  deleteOpenstackCredentials,
  getOpenstackCredentialsList,
} from "./openstack-creds/openstackCreds"
import {
  deleteStorageMapping,
  getStorageMappingsList,
} from "./storage-mappings/storageMappings"
import {
  deleteVmwareCredentials,
  getVmwareCredentialsList,
} from "./vmware-creds/vmwareCreds"

export const cleanupAllResources = async () => {
  // Clean up vmware creds
  try {
    const vmwareCredsResponse = await getVmwareCredentialsList()
    const vmwareCreds = vmwareCredsResponse?.items || []
    for (const vmwareCred of vmwareCreds) {
      await deleteVmwareCredentials(vmwareCred.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up vmware creds", e)
  }

  // Clean up openstack creds
  try {
    const openstackCredsResponse = await getOpenstackCredentialsList()
    const openstackCreds = openstackCredsResponse?.items || []
    for (const openstackCred of openstackCreds) {
      await deleteOpenstackCredentials(openstackCred.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up openstack creds", e)
  }

  // Clean up network mappings
  try {
    const networkMappingsResponse = await getNetworkMappingList()
    const networkMappings = networkMappingsResponse?.items || []
    for (const networkMapping of networkMappings) {
      await deleteNetworkMapping(networkMapping.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up network mappings", e)
  }

  // Clean up storage mappings
  try {
    const storageMappingsResponse = await getStorageMappingsList()
    const storageMappings = storageMappingsResponse?.items || []
    for (const storageMapping of storageMappings) {
      await deleteStorageMapping(storageMapping.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up storage mappings", e)
  }

  // Clean up migration templates
  try {
    const migrationTemplatesResponse = await getMigrationTemplatesList()
    const migrationTemplates = migrationTemplatesResponse?.items || []
    for (const migrationTemplate of migrationTemplates) {
      await deleteMigrationTemplate(migrationTemplate.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up migration templates", e)
  }

  // Clean up migration plans. This will also clean up migrations
  try {
    const migrationPlansResponse = await getMigrationPlansList()
    const migrationPlans = migrationPlansResponse?.items || []
    for (const migrationPlan of migrationPlans) {
      await deleteMigrationPlan(migrationPlan.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up migration plans", e)
  }
}
