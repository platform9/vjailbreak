import config from "app-config"
import {
  GetMigrationPlansList,
  MigrationPlan,
} from "src/data/migration-plan/model"
import {
  GetMigrationTemplatesList,
  MigrationTemplate,
} from "src/data/migration-templates/model"
import {
  GetNetworkMappingsList,
  NetworkMapping,
} from "src/data/network-mappings/model"
import {
  GetOpenstackCredsList,
  OpenstackCreds,
} from "src/data/openstack-creds/model"
import {
  GetStorageMappingsList,
  StorageMapping,
} from "src/data/storage-mappings/model"
import { GetVMWareCredsList, VMwareCreds } from "src/data/vmware-creds/model"
import ApiService from "./ApiService"

class vJailbreak extends ApiService {
  public getClassName(): string {
    return "vJailbreak"
  }

  protected getEndpoint() {
    const isProductionMode = import.meta.env.MODE === "production"
    const port = import.meta.env.VITE_API_PORT
    const host = config.apiHost
    if (isProductionMode && port) {
      return `${host}:${port}`
    }
    return host
  }

  get baseEndpoint() {
    return "/apis/vjailbreak.k8s.pf9.io/v1alpha1"
  }

  get defaultNamespace() {
    return "migration-system"
  }

  getOpenstackCredentialsList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/openstackcreds`
    const response = await this.client.get<GetOpenstackCredsList>({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getOpenstackCredentialsList",
      },
    })
    return response
  }

  getOpenstackCredentials = async (
    openstackCredsName,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/openstackcreds/${openstackCredsName}`
    const response = await this.client.get<OpenstackCreds>({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getOpenstackCredentials",
      },
    })
    return response
  }

  createOpenstackCredentials = async (
    body,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/openstackcreds`
    const response = await this.client.post<OpenstackCreds>({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createOpenstackCredentials",
      },
    })
    return response
  }

  getVmwareCredentialsList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/vmwarecreds`
    const baseUrl = window.location.hostname
    const response = await this.client.get<GetVMWareCredsList>({
      endpoint,
      baseUrl,
      options: {
        clsName: this.getClassName(),
        mthdName: "getVmwareCredentialsList",
      },
    })
    return response
  }

  getVmwareCredentials = async (
    vmwareCredsName,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/vmwarecreds/${vmwareCredsName}`
    const response = await this.client.get<VMwareCreds>({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getVmwareCredentials",
      },
    })
    return response
  }

  createVmwareCredentials = async (body, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/vmwarecreds`
    const response = await this.client.post<VMwareCreds>({
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
    const response = await this.client.get<NetworkMapping>({
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
    const response = await this.client.get<GetNetworkMappingsList>({
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
    const response = await this.client.post<NetworkMapping>({
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
    const response = await this.client.get<StorageMapping>({
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
    const response = await this.client.get<GetStorageMappingsList>({
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
    const response = await this.client.post<StorageMapping>({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createStorageMapping",
      },
    })
    return response
  }

  getMigrationTemplateList = async (namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationtemplates`
    const response = await this.client.get<GetMigrationTemplatesList>({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getMigrationTemplateList",
      },
    })
    return response
  }

  getMigrationTemplate = async (
    templateName,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationtemplates/${templateName}`
    const response = await this.client.get<MigrationTemplate>({
      endpoint,
      options: {
        clsName: this.getClassName(),
        mthdName: "getMigrationTemplate",
      },
    })
    return response
  }

  createMigrationTemplate = async (body, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationtemplates`
    const response = await this.client.post<MigrationTemplate>({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "createMigrationTemplate",
      },
    })
    return response
  }

  updateMigrationTemplate = async (
    templateName,
    body,
    namespace = this.defaultNamespace
  ) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationtemplates/${templateName}`
    const response = await this.client.patch<MigrationTemplate>({
      endpoint,
      body,
      options: {
        clsName: this.getClassName(),
        mthdName: "updateMigrationTemplate",
        config: {
          headers: {
            "Content-Type": "application/merge-patch+json",
          },
        },
      },
    })
    return response
  }

  getMigrationPlan = async (planName, namespace = this.defaultNamespace) => {
    const endpoint = `${this.baseEndpoint}/namespaces/${namespace}/migrationplans/${planName}`
    const response = await this.client.get<MigrationPlan>({
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
    const response = await this.client.get<GetMigrationPlansList>({
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
    const response = await this.client.post<MigrationPlan>({
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
