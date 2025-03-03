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
  getOpenstackCredentials,
  getOpenstackCredentialsList,
} from "./openstack-creds/openstackCreds"
import {
  deleteStorageMapping,
  getStorageMappingsList,
} from "./storage-mappings/storageMappings"
import {
  deleteVmwareCredentials,
  getVmwareCredentials,
  getVmwareCredentialsList,
} from "./vmware-creds/vmwareCreds"
import {
  createOpenstackCredsSecret,
  createVMwareCredsSecret,
  deleteSecret,
} from "./secrets/secrets"
import { createOpenstackCredsWithSecret } from "./openstack-creds/openstackCreds"
import { createVMwareCredsWithSecret } from "./vmware-creds/vmwareCreds"
import { VJAILBREAK_DEFAULT_NAMESPACE } from "./constants"

export const cleanupAllResources = async () => {
  // Clean up vmware creds
  try {
    const vmwareCreds = await getVmwareCredentialsList()
    for (const vmwareCred of vmwareCreds) {
      await deleteVmwareCredentials(vmwareCred.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up vmware creds", e)
  }

  // Clean up openstack creds
  try {
    const openstackCreds = await getOpenstackCredentialsList()
    for (const openstackCred of openstackCreds) {
      await deleteOpenstackCredentials(openstackCred.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up openstack creds", e)
  }

  // Clean up network mappings
  try {
    const networkMappings = await getNetworkMappingList()
    for (const networkMapping of networkMappings) {
      await deleteNetworkMapping(networkMapping.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up network mappings", e)
  }

  // Clean up storage mappings
  try {
    const storageMappings = await getStorageMappingsList()
    for (const storageMapping of storageMappings) {
      await deleteStorageMapping(storageMapping.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up storage mappings", e)
  }

  // Clean up migration templates
  try {
    const migrationTemplates = await getMigrationTemplatesList()
    for (const migrationTemplate of migrationTemplates) {
      await deleteMigrationTemplate(migrationTemplate.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up migration templates", e)
  }

  // Clean up migration plans. This will also clean up migrations
  try {
    const migrationPlans = await getMigrationPlansList()
    for (const migrationPlan of migrationPlans) {
      await deleteMigrationPlan(migrationPlan.metadata.name)
    }
  } catch (e) {
    console.error("Error cleaning up migration plans", e)
  }
}

// Create OpenStack credentials with secret
export const createOpenstackCredsWithSecretFlow = async (
  credName: string,
  credentials: {
    OS_USERNAME: string
    OS_PASSWORD: string
    OS_AUTH_URL: string
    OS_PROJECT_NAME?: string
    OS_TENANT_NAME?: string
    OS_DOMAIN_NAME: string
    OS_REGION_NAME?: string
  },
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  // Use the credName as part of the secret name instead of random UUID
  const secretName = `${credName}-openstack-secret`

  // Create the secret
  await createOpenstackCredsSecret(secretName, credentials, namespace)

  // Create the OpenStack credentials resource that references the secret
  return createOpenstackCredsWithSecret(credName, secretName, namespace)
}

// Create VMware credentials with secret
export const createVMwareCredsWithSecretFlow = async (
  credName: string,
  credentials: {
    VCENTER_HOST: string
    VCENTER_USERNAME: string
    VCENTER_DATACENTER: string
    VCENTER_PASSWORD: string
  },
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  // Use the credName as part of the secret name instead of random UUID
  const secretName = `${credName}-vmware-secret`

  // Add VCENTER_INSECURE:true to the credentials
  const vmwareCredentialsWithInsecure = {
    ...credentials,
    VCENTER_INSECURE: true,
  }

  // Create the secret with the added VCENTER_INSECURE flag
  await createVMwareCredsSecret(
    secretName,
    vmwareCredentialsWithInsecure,
    namespace
  )

  // Create the VMware credentials resource that references the secret
  return createVMwareCredsWithSecret(credName, secretName, namespace)
}

// Delete VMware credentials and associated secret
export const deleteVMwareCredsWithSecretFlow = async (
  credName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  try {
    const credential = await getVmwareCredentials(credName, namespace)

    await deleteVmwareCredentials(credName, namespace)

    // Check if the credential has a secretRef
    if (credential?.spec?.secretRef?.name) {
      // Delete the associated secret
      await deleteSecret(credential.spec.secretRef.name, namespace)
    } else {
      // For backward compatibility, try to delete using the new naming convention
      const secretName = `${credName}-vmware-secret`
      try {
        await deleteSecret(secretName, namespace)
      } catch (error) {
        // Ignore error if secret doesn't exist with this name
        console.log(`No secret found with name ${secretName} : ${error}`)
      }
    }

    return { success: true }
  } catch (error) {
    console.error(`Error deleting VMware credential ${credName}:`, error)
    throw error
  }
}

// Delete OpenStack credentials and associated secret
export const deleteOpenStackCredsWithSecretFlow = async (
  credName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  try {
    // First get the credential to retrieve the secret reference
    const credential = await getOpenstackCredentials(credName, namespace)

    // Delete the credential
    await deleteOpenstackCredentials(credName, namespace)

    // Check if the credential has a secretRef
    if (credential?.spec?.secretRef?.name) {
      // Delete the associated secret
      await deleteSecret(credential.spec.secretRef.name, namespace)
    } else {
      // For backward compatibility, try to delete using the new naming convention
      const secretName = `${credName}-openstack-secret`
      try {
        await deleteSecret(secretName, namespace)
      } catch (error) {
        // Ignore error if secret doesn't exist with this name
        console.log(`No secret found with name ${secretName} : ${error}`)
      }
    }

    return { success: true }
  } catch (error) {
    console.error(`Error deleting OpenStack credential ${credName}:`, error)
    throw error
  }
}
