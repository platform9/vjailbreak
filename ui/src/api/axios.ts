import axios from "axios"
import { pathJoin } from "src/utils"

import { AxiosRequestConfig } from "axios"

const getHeaders = () => {
  const authToken = import.meta.env.VITE_API_TOKEN
  const headers = {
    common: {
      "Content-Type": "application/json;charset=UTF-8",
      ...(authToken && { Authorization: `Bearer ${authToken}` }),
    },
  }
  return headers
}

const axiosInstance = axios.create({
  headers: getHeaders(),
})

const getDefaultBaseUrl = () => {
  if (import.meta.env.VITE_USE_MOCK_API === "true") {
    return "http://localhost:3001/mock-server"
  }

  if (import.meta.env.MODE === "development") {
    return "/dev-api"
  }
  return ""
}

interface AxiosGetRequestParams {
  endpoint: string
  baseUrl?: string
  config?: AxiosRequestConfig
}

interface AxiosPostRequestParams {
  endpoint: string
  baseUrl?: string
  data: unknown
  config?: AxiosRequestConfig
}

interface AxiosPutRequestParams {
  endpoint: string
  baseUrl?: string
  data: unknown
  config?: AxiosRequestConfig
}

interface AxiosPatchRequestParams {
  endpoint: string
  baseUrl?: string
  data: unknown
  config?: AxiosRequestConfig
}

interface AxiosDeleteRequestParams {
  endpoint: string
  baseUrl?: string
  config?: AxiosRequestConfig
}

// Wrappers for axios methods
export const get = async <T>({
  endpoint,
  baseUrl,
  config,
}: AxiosGetRequestParams) => {
  const url = pathJoin(baseUrl || getDefaultBaseUrl(), endpoint)
  const response = await axiosInstance.get<T>(url, config)
  return response.data
}

export const post = async <T>({
  endpoint,
  baseUrl,
  data,
  config = {},
}: AxiosPostRequestParams) => {
  const url = pathJoin(baseUrl || getDefaultBaseUrl(), endpoint)
  const response = await axiosInstance.post<T>(url, data, config)
  return response.data
}

export const put = async <T>({
  endpoint,
  baseUrl,
  data,
  config = {},
}: AxiosPutRequestParams) => {
  const url = pathJoin(baseUrl || getDefaultBaseUrl(), endpoint)
  const response = await axiosInstance.put<T>(url, data, config)
  return response.data
}

const patch = async <T>({
  endpoint,
  baseUrl,
  data,
  config = {},
}: AxiosPatchRequestParams) => {
  const url = pathJoin(baseUrl || getDefaultBaseUrl(), endpoint)
  const response = await axiosInstance.patch<T>(url, data, config)
  return response.data
}

export const del = async <T>({
  endpoint,
  baseUrl,
  config = {},
}: AxiosDeleteRequestParams) => {
  const url = pathJoin(baseUrl || getDefaultBaseUrl(), endpoint)
  const response = await axiosInstance.delete<T>(url, config)
  return response.data
}

export default {
  get,
  post,
  put,
  patch,
  del,
}
