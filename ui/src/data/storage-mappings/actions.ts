// import ApiClient from "src/api/ApiClient"
// import { createStorageMappingJson } from "../../api/storage-mappings/helpers"
// import { StorageMapping } from "../../api/storage-mappings/model"

// const { vjailbreak } = ApiClient.getInstance()

// export const createStorageMapping = async (params): Promise<StorageMapping> => {
//   const body = createStorageMappingJson(params)
//   try {
//     const data = await vjailbreak.createStorageMapping(body)
//     return data
//   } catch (error) {
//     console.error("Error creating StorageMapping", { error, params })
//     return {} as StorageMapping
//   }
// }
