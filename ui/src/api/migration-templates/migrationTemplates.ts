import {
  GetMigrationTemplatesList,
  MigrationTemplate,
} from "src/api/migration-templates/model"
import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"

export const getMigrationTemplateList = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrationtemplates`
  const response = await axios.get<GetMigrationTemplatesList>({
    endpoint,
  })
  return response
}

export const getMigrationTemplate = async (
  templateName,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrationtemplates/${templateName}`
  const response = await axios.get<MigrationTemplate>({
    endpoint,
  })
  return response
}

export const createMigrationTemplate = async (
  body,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrationtemplates`
  const response = await axios.post<MigrationTemplate>({
    endpoint,
    data: body,
  })
  return response
}

export const updateMigrationTemplate = async (
  templateName,
  body,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrationtemplates/${templateName}`
  const response = await axios.patch<MigrationTemplate>({
    endpoint,
    data: body,
    config: {
      headers: {
        "Content-Type": "application/merge-patch+json",
      },
    },
  })
  return response
}
