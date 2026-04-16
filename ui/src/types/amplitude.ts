// Simplified event system - more flexible and extensible
export const AMPLITUDE_EVENTS = {
  // Migration Events
  MIGRATION_CREATED: 'Migration Created',
  MIGRATION_FAILED: 'Migration Failed', // For creation failures
  MIGRATION_CREATION_FAILED: 'Migration Creation Failed',
  MIGRATION_EXECUTION_FAILED: 'Migration Execution Failed', // For runtime failures
  MIGRATION_SUCCEEDED: 'Migration Succeeded', // For successful completions
  MIGRATION_DELETED: 'Migration Deleted',
  MIGRATION_DELETE_FAILED: 'Migration Delete Failed',

  // Agents
  AGENTS_SCALE_UP: 'Agents Scale Up',
  AGENTS_SCALE_UP_FAILED: 'Agents Scale Up Failed',
  AGENTS_SCALE_DOWN: 'Agents Scale Down',
  AGENTS_SCALE_DOWN_FAILED: 'Agents Scale Down Failed',

  // Storage Array Credentials
  STORAGE_ARRAY_CREDENTIALS_ADDED: 'Storage Array Credentials Added',
  STORAGE_ARRAY_CREDENTIALS_FAILED: 'Storage Array Credentials Failed',
  STORAGE_ARRAY_CREDENTIALS_UPDATED: 'Storage Array Credentials Updated',
  STORAGE_ARRAY_CREDENTIALS_UPDATE_FAILED: 'Storage Array Credentials Update Failed',
  STORAGE_ARRAY_CREDENTIALS_DELETED: 'Storage Array Credentials Deleted',
  STORAGE_ARRAY_CREDENTIALS_DELETE_FAILED: 'Storage Array Credentials Delete Failed',

  // ESXi SSH Credentials
  ESXI_SSH_CREDENTIALS_ADDED: 'ESXi SSH Credentials Added',
  ESXI_SSH_CREDENTIALS_FAILED: 'ESXi SSH Credentials Failed',
  ESXI_SSH_CREDENTIALS_UPDATED: 'ESXi SSH Credentials Updated',
  ESXI_SSH_CREDENTIALS_UPDATE_FAILED: 'ESXi SSH Credentials Update Failed',

  // Credential Events
  CREDENTIALS_ADDED: 'Credentials Added',
  CREDENTIALS_FAILED: 'Credentials Failed',
  VMWARE_CREDENTIALS_ADDED: 'VMware Credentials Added',
  VMWARE_CREDENTIALS_FAILED: 'VMware Credentials Failed',
  PCD_CREDENTIALS_ADDED: 'PCD Credentials Added',
  PCD_CREDENTIALS_FAILED: 'PCD Credentials Failed',
  VMWARE_CREDENTIALS_DELETED: 'VMware Credentials Deleted',
  VMWARE_CREDENTIALS_DELETE_FAILED: 'VMware Credentials Delete Failed',
  PCD_CREDENTIALS_DELETED: 'PCD Credentials Deleted',
  PCD_CREDENTIALS_DELETE_FAILED: 'PCD Credentials Delete Failed',
  ROLLING_MIGRATION_CREATED: 'Rolling Migration Created',
  ROLLING_MIGRATION_SUBMISSION_FAILED: 'Rolling Migration Submission Failed',

  // Cluster Migration Events
  CLUSTER_CONVERSION_TRIGGERED: 'Cluster Conversion Triggered',
  CLUSTER_CONVERSION_FAILED: 'Cluster Conversion Failed',
  CLUSTER_CONVERSION_EXECUTION_FAILED: 'Cluster Conversion Execution Failed', // For runtime failures
  CLUSTER_CONVERSION_SUCCEEDED: 'Cluster Conversion Succeeded' // For successful completions
} as const

export type AmplitudeEventName = (typeof AMPLITUDE_EVENTS)[keyof typeof AMPLITUDE_EVENTS] | string

// Minimal, flexible event properties - only truly universal properties
export interface EventProperties {
  // Universal context properties (always useful)
  component?: string
  userId?: string
  userEmail?: string
  namespace?: string

  // Universal error properties
  errorMessage?: string
  errorCode?: string

  // Flexible property bag - allows any key-value pairs
  [key: string]: unknown
}

// User context for tracking
export interface UserContext {
  userId?: string
  userEmail?: string
  userProperties?: Record<string, unknown>
}
