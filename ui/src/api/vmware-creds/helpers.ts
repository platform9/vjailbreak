import { v4 as uuidv4 } from "uuid"

export const createVmwareCredsJson = (params) => {
  const {
    name,
    vcenterHost,
    username,
    password,
    namespace = "migration-system",
  } = params || {}
  return {
    apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
    kind: "VMwareCreds",
    metadata: {
      name: name || uuidv4(),
      namespace,
    },
    spec: {
      VCENTER_HOST: vcenterHost,
      VCENTER_INSECURE: true,
      VCENTER_PASSWORD: password,
      VCENTER_USERNAME: username,
    },
  }
}
