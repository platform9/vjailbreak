import config from "app-config"
import ApiService from "./ApiService"

class vJailbreak extends ApiService {
  public getClassName(): string {
    return "vJailbreak"
  }

  protected async getEndpoint() {
    return Promise.resolve(config.apiHost)
  }

  get baseEndpoint() {
    return "/apis/vjailbreak.k8s.pf9.io/v1alpha1"
  }

  get defaultNamespace() {
    return "migration-system"
  }

  getOpenstackCredentials = async (
    openstackCredsName,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/openstackcreds/${openstackCredsName}`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getOpenstackCredentials",
      },
    })
    return response
  }

  getOpenstackCredentialsList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/openstackcreds`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getOpenstackCredentialsList",
      },
    })
    return response
  }

  createOpenstackCredentials = async (
    body,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/openstackcreds`
    const response = await this.client.post({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createOpenstackCredentials",
      },
    })
    return response
  }

  getVmwareCredentials = async (
    vmwareCredsName,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/vmwarecreds/${vmwareCredsName}`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getVmwareCredentials",
      },
    })
    return response
  }

  getVmwareCredentialsList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/vmwarecreds`
    const baseUrl = window.location.hostname
    const response = await this.client.get({
      endpoint,
      baseUrl,
      options: {
        clsName: this.getClassName(),
        mthdName: "getVmwareCredentialsList",
      },
    })
    return response
  }

  createVmwareCredentials = async (body, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/vmwarecreds`
    const response = await this.client.post({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createVmwareCredentials",
      },
    })
    return response
  }

  getNetworkMapping = async (
    networkMappingName,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/networkmappings/${networkMappingName}`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getNetworkMapping",
      },
    })
    return response
  }

  getNetworkMappingList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/networkmappings`
    const baseUrl = window.location.hostname
    const response = await this.client.get({
      endpoint,
      baseUrl,
      options: {
        clsName: this.getClassName(),
        mthdName: "getNetworkMappingList",
      },
    })
    return response
  }

  createNetworkMapping = async (body, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/networkmappings`
    const response = await this.client.post({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createNetworkMapping",
      },
    })
    return response
  }

  getStorageMapping = async (
    storageMappingName,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/storagemappings/${storageMappingName}`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getStorageMapping",
      },
    })
    return response
  }

  getStorageMappingList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/storagemappings`
    const baseUrl = window.location.hostname
    const response = await this.client.get({
      endpoint,
      baseUrl,
      options: {
        clsName: this.getClassName(),
        mthdName: "getStorageMappingList",
      },
    })
    return response
  }

  createStorageMapping = async (body, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/storagemappings`
    const response = await this.client.post({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createStorageMapping",
      },
    })
    return response
  }

  getMigrationTemplate = async (
    templateName,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationtemplates/${templateName}`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getMigrationTemplate",
      },
    })
    return response
  }

  getMigrationTemplateList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationtemplates`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getMigrationTemplateList",
      },
    })
    return response
  }

  createMigrationTemplate = async (body, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationtemplates`
    const response = await this.client.post({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createMigrationTemplate",
      },
    })
    return response
  }

  getMigrationPlan = async (planName, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationplans/${planName}`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getMigrationPlan",
      },
    })
    return response
  }

  getMigrationPlanList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationplans`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getMigrationPlanList",
      },
    })
    return response
  }

  createMigrationPlan = async (body, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationplans`
    const response = await this.client.post({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createMigrationPlan",
      },
    })
    return response
  }

  getMigration = async (migrationName, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrations/${migrationName}`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getMigration",
      },
    })
    return response
  }

  getMigrationList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrations`
    const response = await this.client.get({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getMigrationList",
      },
    })
    return response
  }
}

export default vJailbreak
