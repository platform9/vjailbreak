import ApiClient from "src/api/ApiClient"
import { Migration } from "./model"

const { vjailbreak } = ApiClient.getInstance()
export const getMigrationsList = async (
  migrationPlanName = "",
  namespace = undefined
): Promise<Migration[]> => {
  try {
    const data = await vjailbreak.getMigrationList(migrationPlanName, namespace)
    return data?.items
  } catch (error) {
    console.error("Error getting MigrationsList", { error })
    return []
  }
}
