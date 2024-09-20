import { AxiosRequestConfig } from "axios"

interface IRequestOptions {
  clsName: string
  mthdName: string
  config?: unknown
}

export interface IBasicRequestGetParams {
  endpoint: string
  version?: string
  params?: AxiosRequestConfig["params"]
  baseUrl?: string
  config?: AxiosRequestConfig
  options: IRequestOptions
}

export interface IBasicRequestPostParams {
  endpoint: string
  version?: string
  body?: unknown
  baseUrl?: string
  config?: AxiosRequestConfig
  options: IRequestOptions
}

export interface IBasicRequestDeleteParams {
  endpoint: string
  version?: string
  params?: AxiosRequestConfig["params"]
  data?: AxiosRequestConfig["data"]
  baseUrl?: string
  config?: AxiosRequestConfig
  options: IRequestOptions
}
