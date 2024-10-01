import ApiClient from "../ApiClient"

abstract class ApiService {
  protected apiHost: string = ""
  public abstract getClassName(): string
  abstract getApiHost(): string

  constructor(protected client: ApiClient) {}

  async initialize(): Promise<string> {
    this.apiHost = await this.getApiHost()
    return this.apiHost
  }

  async getApiEndpoint(): Promise<string> {
    if (this.apiHost) {
      return this.apiHost
    }
    return this.initialize()
  }
}

export default ApiService
