import ApiClient from "src/api/ApiClient"
import { createStorageMappingJson } from "./helpers"
import { StorageMapping } from "./model"

const { vjailbreak } = ApiClient.getInstance()

export const createStorageMapping = async (params): Promise<StorageMapping> => {
  const body = createStorageMappingJson(params)
  try {
    const data = await vjailbreak.createStorageMapping(body)
    return data
  } catch (error) {
    console.error("Error creating StorageMapping", { error, params })
    return {} as StorageMapping
  }
}
