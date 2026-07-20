import type { MigrationBlueprintSpec } from 'src/api/migration-blueprints/model'

// Mirrors FormValues.dataCopyMethod ('hot' | 'cold' | 'mock') — the "Hot copy" / "Cold
// copy" / "Mock copy" tag shown on each template card.
export type DataCopyMethod = 'hot' | 'cold' | 'mock'

export interface SavedTemplateMapping {
  source: string
  target: string
}

// UI-facing, flattened view of a MigrationBlueprint for the Templates tab
// card/drawer. `spec` carries the full backend spec so clone/delete can
// round-trip fields this flattened shape doesn't surface.
export interface SavedTemplate {
  name: string // k8s object name (sanitized display name)
  displayName: string
  description?: string
  createdAt: string
  sourceVCenter: string
  destination: string
  targetCluster: string
  networkMappings: SavedTemplateMapping[]
  storageMappings: SavedTemplateMapping[]
  dataCopyMethod: DataCopyMethod
  cutoverOption: string // CUTOVER_TYPES value ('0' | '1' | '2')
  osFamily?: string
  useGPU?: boolean
  spec: MigrationBlueprintSpec
}

export interface SaveAsTemplateInput {
  displayName: string
  description?: string
  sourceVCenter: string
  destination: string
  targetCluster: string
  networkMappings: SavedTemplateMapping[]
  storageMappings: SavedTemplateMapping[]
  dataCopyMethod: DataCopyMethod
  cutoverOption: string // CUTOVER_TYPES value ('0' | '1' | '2')
  osFamily?: string
  useGPU?: boolean
}
