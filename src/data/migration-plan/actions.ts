import ApiClient from "src/api/ApiClient"
import { createMigrationPlanJson } from "./helpers"
import { MigrationPlan } from "./model"

const { vjailbreak } = ApiClient.getInstance()

export const createMigrationPlan = async (params): Promise<MigrationPlan> => {
  const body = createMigrationPlanJson(params)
  try {
    const data = await vjailbreak.createMigrationPlan(body)
    return data
  } catch (error) {
    console.error("Error creating MigrationPlan", { error, params })
    return {} as MigrationPlan
  }
}
