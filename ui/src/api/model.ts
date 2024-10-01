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
  apiHost?: string
  config?: AxiosRequestConfig
  options: IRequestOptions
}

export interface IBasicRequestPostParams {
  endpoint: string
  version?: string
  body?: unknown
  apiHost?: string
  config?: AxiosRequestConfig
  options: IRequestOptions
}

export interface IBasicRequestDeleteParams {
  endpoint: string
  version?: string
  params?: AxiosRequestConfig["params"]
  data?: AxiosRequestConfig["data"]
  apiHost?: string
  config?: AxiosRequestConfig
  options: IRequestOptions
}
