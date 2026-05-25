// Add this prefix so that the requests are proxied to the backend server
// Proxy config is in vite.config.ts
export const VJAILBREAK_API_BASE_URL = import.meta.env.MODE === 'development' ? '/dev-api' : ''
export const VJAILBREAK_API_BASE_PATH = '/apis/vjailbreak.k8s.pf9.io/v1alpha1'
export const KUBERNETES_API_BASE_PATH = '/api/v1'
export const VJAILBREAK_DEFAULT_NAMESPACE = 'migration-system'

// K8s API calls for Secrets and Pods are routed through the vpwned proxy so that
// ui-manager-sa does not need direct secrets/pods RBAC permissions.
// The /dev-api/sdk prefix is handled by the nginx ingress rewrite in production
// (strips it and forwards to the vpwned service) and by the vite dev-server proxy
// in development (strips /dev-api and forwards to VITE_API_HOST which then hits the ingress).
export const K8S_PROXY_BASE_PATH = '/dev-api/sdk/vpw/v1/k8s/api/v1'
