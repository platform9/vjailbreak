import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import { GetMigrationsList, Migration } from "./model"

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
