import ApiClient from "../ApiClient"

abstract class ApiService {
  private readonly className: string
  protected apiEndpoint: string = ""
  public abstract getClassName(): string
  protected abstract getEndpoint(): Promise<string>

  constructor(protected client: ApiClient) {
    this.className = this.getClassName()
  }

  async initialize(): Promise<string> {
    this.apiEndpoint = await this.getEndpoint()
    return this.apiEndpoint
  }

  async getApiEndpoint(): Promise<string> {
    if (this.apiEndpoint) {
      return this.apiEndpoint
    }
    return this.initialize()
  }
}

export default ApiService
