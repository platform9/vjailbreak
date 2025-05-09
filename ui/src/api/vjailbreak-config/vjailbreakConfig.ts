import axios from "../axios"
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from "../constants"
import { VjailbreakConfig } from "./model"

export const getVjailbreakConfig = async (namespace: string = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vjailbreakconfig`
  const response = await axios.get<VjailbreakConfig>({
    endpoint,
  })
  return response?.spec?.debug
}

export const updateVjailbreakConfig = async (body: VjailbreakConfig, namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vjailbreakconfig`
  const response = await axios.put<VjailbreakConfig>({
    endpoint,
    data: body,
  })
  return response
}