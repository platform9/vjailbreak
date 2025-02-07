import axios from "../axios"
import {
  VJAILBREAK_API_BASE_PATH,
  VJAILBREAK_DEFAULT_NAMESPACE,
} from "../constants"
import { NodeList, NodeItem as Node } from "./model"

export const getNodes = async (namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vjailbreaknodes`
  const response = await axios.get<NodeList>({
    endpoint,
  })
  return response?.items
}

export const deleteNode = async (
  nodeName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/vjailbreaknodes/${nodeName}`
  const response = await axios.del<Node>({
    endpoint,
  })
  return response
}
