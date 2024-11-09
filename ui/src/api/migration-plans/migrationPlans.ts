import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import { GetMigrationPlansList, MigrationPlan } from "./model"

export const getMigrationPlanList = async (
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrationplans`
  const response = await axios.get<GetMigrationPlansList>({
    endpoint,
  })
  return response
}

export const getMigrationPlan = async (
  planName,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrationplans/${planName}`
  const response = await axios.get<MigrationPlan>({
    endpoint,
  })
  return response
}

export const createMigrationPlan = async (
  body,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/migrationplans`
  const response = await axios.post<MigrationPlan>({
    endpoint,
    data: body,
  })
  return response
}
