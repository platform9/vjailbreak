import { v4 as uuidv4 } from "uuid"



// Helper function to parse OS_INSECURE from string to boolean
const getBooleanValue = (value: string | undefined): boolean | undefined => {
  if (value === undefined) return undefined;
  return value.toLowerCase() === "true";
};



export const createOpenstackCredsJson = (params) => {
  const {
    name,
    namespace = "migration-system",
    OS_AUTH_URL,
    OS_DOMAIN_NAME,
    OS_USERNAME,
    OS_PASSWORD,
    OS_REGION_NAME,
    OS_TENANT_NAME,
    OS_INSECURE,
  } = params || {}
  return {
    apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
    kind: "OpenstackCreds",
    metadata: {
      name: name || uuidv4(),
      namespace,
    },
    spec: {
      OS_AUTH_URL,
      OS_DOMAIN_NAME,
      OS_USERNAME,
      OS_PASSWORD,
      OS_REGION_NAME,
      OS_TENANT_NAME,
      OS_INSECURE: getBooleanValue(OS_INSECURE), 
    },
  }
}
