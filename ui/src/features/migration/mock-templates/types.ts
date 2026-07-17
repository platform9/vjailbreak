// Mirrors FormValues.dataCopyMethod ('hot' | 'cold' | 'mock') — the "Hot copy" / "Cold
// copy" / "Mock copy" tag shown on each template card, per the mockup.
export type DataCopyMethod = 'hot' | 'cold' | 'mock'

export interface SavedTemplateMapping {
  source: string
  target: string
}

/**
 * UI-side shape for a saved Migration Template card/drawer. Field names mirror what
 * plan.md's MigrationTemplateSpec/Status extension will eventually carry, so swapping
 * the mock store in useMigrationTemplatesQuery.ts for real API calls later requires no
 * changes to any component in components/templates/.
 */
export interface SavedTemplate {
  name: string // k8s-style unique id (sanitized display name)
  displayName: string
  description?: string
  createdAt: string
  timesUsed: number
  lastUsedAt?: string
  sourceVCenter: string
  destination: string
  tenantProject: string
  targetCluster: string
  networkMappings: SavedTemplateMapping[]
  storageMappings: SavedTemplateMapping[]
  dataCopyMethod: DataCopyMethod
  cutoverOption: string // CUTOVER_TYPES value ('0' | '1' | '2')
  vmwareCluster?: string
  pcdCluster?: string
  osFamily?: string
  useGPU?: boolean
}

export interface SaveAsTemplateInput {
  displayName: string
  description?: string
  sourceVCenter: string
  destination: string
  tenantProject: string
  targetCluster: string
  networkMappings: SavedTemplateMapping[]
  storageMappings: SavedTemplateMapping[]
  dataCopyMethod: DataCopyMethod
  cutoverOption: string // CUTOVER_TYPES value ('0' | '1' | '2')
  vmwareCluster?: string
  pcdCluster?: string
  osFamily?: string
  useGPU?: boolean
}
