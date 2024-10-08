import axios, { AxiosInstance } from "axios"
import { mergeDeepLeft } from "ramda"
import { pathJoin } from "src/utils"
import {
  IBasicRequestDeleteParams,
  IBasicRequestGetParams,
  IBasicRequestPostParams,
} from "./model"
import ApiService from "./services/ApiService"
import vJailbreakService from "./services/vJailbreak"

const addApiRequestMetadata = (apiClass, requestMethod) => {
  return { apiClassMetadata: { apiClass, requestMethod } }
}

declare global {
  interface Window {
    env: {
      [key: string]: string;
    };
  }
}

class ApiClient {
  private readonly axiosInstance: AxiosInstance
  private static instance: ApiClient
  private apiServices: { [key: string]: ApiService } = {}
  private token = ""

  // Define API Services here
  public vjailbreak: vJailbreakService

  static init() {
    if (!ApiClient.instance) {
      ApiClient.instance = new ApiClient()
    }
    return ApiClient.instance
  }

  static getInstance() {
    if (!ApiClient.instance) {
      console.warn(
        "ApiClient instance has not been initialized, please call ApiClient.init to instantiate it"
      )
      return {} as ApiClient
    }
    return ApiClient.instance
  }

  constructor() {
    this.axiosInstance = axios.create({
      timeout: 120000,
      headers: {
        common: {
          "Content-Type": "application/json;charset=UTF-8",
        },
      },
    })

    this.token = import.meta.env.VITE_API_TOKEN

    // Add API Services here
    this.vjailbreak = this.addApiService(new vJailbreakService(this))
  }

  addApiService = <T extends ApiService>(apiClientInstance: T) => {
    this.apiServices[apiClientInstance.getClassName()] = apiClientInstance
    return apiClientInstance
  }

  setToken = (token) => {
    this.token = token
  }

  getToken = () => {
    return this.token
  }

  getAuthHeaders = () => {
    if (!this.token) {
      return {}
    }

    const headers = {
      Authorization: `Bearer ${String(this.token)}`, // required for k8s proxy api
    }
    return { headers }
  }

  async getApiHost(clsName) {
    return this.apiServices[clsName].getApiHost()
  }

  get = async <T>({
    endpoint,
    apiHost = undefined,
    params = undefined,
    options: { clsName, mthdName },
  }: IBasicRequestGetParams) => {
    if (!apiHost) {
      apiHost = await this.getApiHost(clsName)
    }
    const response = await this.axiosInstance.get<T>(
      pathJoin(apiHost, endpoint),
      {
        params,
        ...this.getAuthHeaders(),
        ...addApiRequestMetadata(clsName, mthdName),
      }
    )
    return response?.data
  }

  post = async <T>({
    endpoint,
    apiHost = undefined,
    body = undefined,
    options: { clsName, mthdName },
  }: IBasicRequestPostParams) => {
    if (!apiHost) {
      apiHost = await this.getApiHost(clsName)
    }

    const response = await this.axiosInstance.post<T>(
      pathJoin(apiHost, endpoint),
      body,
      {
        ...this.getAuthHeaders(),
        ...addApiRequestMetadata(clsName, mthdName),
      }
    )
    return response?.data
  }

  patch = async <T>({
    endpoint,
    apiHost = undefined,
    body = undefined,
    options: { clsName, mthdName, config = {} },
  }: IBasicRequestPostParams) => {
    if (!apiHost) {
      apiHost = await this.getApiHost(clsName)
    }
    const response = await this.axiosInstance.patch<T>(
      pathJoin(apiHost, endpoint),
      body,
      mergeDeepLeft(this.getAuthHeaders(), {
        ...(config || {}),
        ...addApiRequestMetadata(clsName, mthdName),
      })
    )
    return response?.data
  }

  put = async <T>({
    endpoint,
    apiHost = undefined,
    body = undefined,
    options: { clsName, mthdName },
  }: IBasicRequestPostParams) => {
    if (!apiHost) {
      apiHost = await this.getApiHost(clsName)
    }
    const response = await this.axiosInstance.put<T>(
      pathJoin(apiHost, endpoint),
      body,
      {
        ...this.getAuthHeaders(),
        ...addApiRequestMetadata(clsName, mthdName),
      }
    )
    return response?.data
  }

  delete = async <T>({
    endpoint,
    apiHost = undefined,
    options: { clsName, mthdName },
    data = undefined,
  }: IBasicRequestDeleteParams) => {
    if (!apiHost) {
      apiHost = await this.getApiHost(clsName)
    }
    const response = await this.axiosInstance.delete<T>(
      pathJoin(apiHost, endpoint),
      {
        ...this.getAuthHeaders(),
        data,
        ...addApiRequestMetadata(clsName, mthdName),
      }
    )
    return response?.data
  }
}

export default ApiClient
