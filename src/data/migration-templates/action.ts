import ApiClient from "src/api/ApiClient"
import { createMigrationTemplateJson } from "./helpers"
import { MigrationTemplate } from "./model"

const { vjailbreak } = ApiClient.getInstance()

export const getMigrationTemplate = async (
  templateName: string
): Promise<MigrationTemplate> => {
  try {
    const data = await vjailbreak.getMigrationTemplate(templateName)
    return data
  } catch (error) {
    console.error(`Error getting MigrationTemplate`, {
      error,
      params: { templateName },
    })
    return {} as MigrationTemplate
  }
}

export const createMigrationTemplate = async (
  params
): Promise<MigrationTemplate> => {
  const body = createMigrationTemplateJson(params)
  try {
    const data = await vjailbreak.createMigrationTemplate(body)
    return data
  } catch (error) {
    console.error("Error creating MigrationTemplate", { error, params })
    return {} as MigrationTemplate
  }
}

export const updateMigrationTemplate = async (
  templateName: string,
  updatedParams = {}
): Promise<MigrationTemplate> => {
  try {
    const data = await vjailbreak.updateMigrationTemplate(
      templateName,
      updatedParams
    )
    return data
  } catch (error) {
    console.error("Error updating MigrationTemplate", {
      error,
      params: updatedParams,
    })
    return {} as MigrationTemplate
  }
}
