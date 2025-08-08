import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import { GetMigrationsList, Migration, TriggerAdminCutoverRequest, TriggerAdminCutoverResponse } from "./model"

export const getMigrations = async (
  migrationPlanName = "",
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<Migration[]> => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrations`
  const params = {
    ...(migrationPlanName
      ? { labelSelector: `migrationplan=${migrationPlanName}` }
      : {}),
  }
  const data = await axios.get<GetMigrationsList>({
    endpoint,
    config: { params },
  })
  return data?.items
}

export const getMigration = async (
  migrationName,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrations/${migrationName}`
  const response = await axios.get<Migration>({
    endpoint,
  })
  return response
}

export const deleteMigration = async (
  migrationName,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrations/${migrationName}`
  const response = await axios.del<Migration>({
    endpoint,
  })
  return response
}


export const triggerAdminCutover = async (
  namespace: string,
  migrationName: string
): Promise<{ success: boolean; message: string }> => {
  try {
    // First get the migration to find the podRef
    const migration = await getMigration(migrationName, namespace);
    const podRef = migration.spec?.podRef;
    
    if (!podRef) {
      throw new Error("PodRef is empty in migration object");
    }

    // Patch the pod directly with the startCutover label
    const patchPayload = {
      metadata: {
        labels: {
          startCutover: "yes"
        }
      }
    };

    const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/pods/${podRef}`;
    
    await axios.patch({
      endpoint,
      data: patchPayload,
      config: {
        headers: {
          "Content-Type": "application/merge-patch+json",
        },
      },
    });

    return {
      success: true,
      message: "Successfully triggered cutover"
    };
  } catch (error) {
    console.error("Failed to trigger cutover:", error);
    return {
      success: false,
      message: error instanceof Error ? error.message : "Failed to trigger cutover"
    };
  }
};
