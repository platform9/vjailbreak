import ApiClient from "src/api/ApiClient"
import { Migration } from "./model"

const { vjailbreak } = ApiClient.getInstance()
export const getMigrationsList = async (): Promise<Migration[]> => {
  try {
    const data = await vjailbreak.getMigrationList()
    return data?.items
  } catch (error) {
    console.error("Error getting MigrationsList", { error })
    return []
  }
}
