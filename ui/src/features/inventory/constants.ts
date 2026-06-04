// Inventory / Migration Planner constants.

/** Route path for the Inventory page (registered in App.tsx). */
export const INVENTORY_ROUTE = '/dashboard/inventory'

/** react-query key base for MigrationBucket resources (Phase 2). */
export const MIGRATION_BUCKETS_QUERY_KEY = 'migration-buckets'

/** Display name for the auto-created default bucket. */
export const DEFAULT_BUCKET_LABEL = 'Default Bucket'

/**
 * Label set on a bucket when its migrations are triggered from the planner. Lets the status
 * derivation show progress immediately (InProgress) before the per-VM Migration objects appear.
 */
export const BUCKET_TRIGGERED_LABEL = 'vjailbreak.k8s.pf9.io/triggered'

/** Resource name (metadata.name) of the auto-created default bucket. */
export const DEFAULT_BUCKET_NAME = 'default-bucket'

/**
 * Placeholder capacity inputs for the agent-count recommendation (DESIGN §9.1).
 * Real values are sourced from the vjailbreak-settings ConfigMap + node allocatable +
 * VjailbreakNode list during backend/integration phases (T043/T047); open question Q8.
 */
export const DEFAULT_AGENT_PARAMS = {
  /** CPU request (cores) per v2v migration pod — `C`. */
  cpuPerMigration: 2,
  /** Free cores on the master — `m`. */
  masterFreeCores: 0,
  /** Total free cores across existing agents — `ΣΔ`. */
  agentFreeCores: 0,
  /** Schedulable cores a fresh agent adds — `F`. */
  freshAgentCores: 8,
  /** Ceiling on new agents — `A_max`. */
  maxAgents: 10
} as const
