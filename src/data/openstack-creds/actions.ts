import ApiClient from "src/api/ApiClient"
import { createOpenstackCredsJson } from "./helpers"
import { OpenstackCreds } from "./model"

const { vjailbreak } = ApiClient.getInstance()

export const createOpenstackCreds = async (params): Promise<OpenstackCreds> => {
  const body = createOpenstackCredsJson(params)
  try {
    const data = await vjailbreak.createOpenstackCredentials(body)
    return data
  } catch (error) {
    console.error("Error creating OpenstackCreds", error)
    return {} as OpenstackCreds
  }
}

export const getOpenstackCreds = async (
  name: string
): Promise<OpenstackCreds> => {
  try {
    const data = await vjailbreak.getOpenstackCredentials(name)
    return data
  } catch (error) {
    console.error("Error getting OpenstackCreds", error)
    return {} as OpenstackCreds
  }
}
