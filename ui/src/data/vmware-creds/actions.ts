import ApiClient from "src/api/ApiClient"
import { createVmwareCredsJson } from "./helpers"
import { VMwareCreds } from "./model"

const { vjailbreak } = ApiClient.getInstance()

export const createVmwareCreds = async (params): Promise<VMwareCreds> => {
  const body = createVmwareCredsJson(params)
  try {
    const data = await vjailbreak.createVmwareCredentials(body)
    return data
  } catch (error) {
    console.error("Error creating VMware creds", error)
    return {} as VMwareCreds
  }
}

export const getVmwareCreds = async (name: string): Promise<VMwareCreds> => {
  try {
    const data = await vjailbreak.getVmwareCredentials(name)
    return data
  } catch (error) {
    console.error("Error getting VMware creds", { params: { name }, error })
    return {} as VMwareCreds
  }
}
