import { v4 as uuidv4 } from "uuid"

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
    },
  }
}
