import ApiClient from "src/api/ApiClient"
import { createMigrationTemplateJson } from "./helpers"
import { NetworkMapping } from "./model"

const { vjailbreak } = ApiClient.getInstance()

export const createNetworkMapping = async (params): Promise<NetworkMapping> => {
  const body = createMigrationTemplateJson(params)
  try {
    const data = await vjailbreak.createNetworkMapping(body)
    return data
  } catch (error) {
    console.error("Error creating NetworkMapping", { error, params })
    return {} as NetworkMapping
  }
}
