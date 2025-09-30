import { get } from "../axios";
import {
  VjailbreakSettings
} from "./model";

const VERSION_CONFIG_MAP_NAME = "vjailbreak-settings";
const VERSION_NAMESPACE = "migration-system";

export const getSettingsConfigMap = async (
  namespace: string = VERSION_NAMESPACE,
): Promise<VjailbreakSettings> => {
  const endpoint = `/api/v1/namespaces/${namespace}/configmaps/${VERSION_CONFIG_MAP_NAME}`;
  return get<VjailbreakSettings>({
    endpoint,
    config: { mock: false }, // Force real API call, not mock
  });
};


export const getDeploymentName = async (namespace:string = VERSION_NAMESPACE) : Promise<string> =>{
try {
    const configMap = await getSettingsConfigMap(namespace);
    return configMap.data.DEPLOYMENT_NAME;
} catch (error) {
  console.error("Failed to fetch header name",error)
  throw error;
}
}

