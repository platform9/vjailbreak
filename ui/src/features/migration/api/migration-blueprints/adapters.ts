import type {
  MigrationBlueprint,
  MigrationBlueprintSpec,
  MigrationBlueprintStrategy
} from 'src/api/migration-blueprints/model'
import { CUTOVER_TYPES } from '../../constants'
import type { SavedTemplate, SaveAsTemplateInput } from './types'

const ZERO_TIME = '0001-01-01T00:00:00Z'
const isSetTime = (value?: string) => Boolean(value && value !== ZERO_TIME)

// Mirrors useRetryPrefill.ts's cutoverOption derivation from a MigrationStrategy so
// the Templates tab and the retry flow agree on what "admin cutover" / "time window"
// mean given the same underlying strategy shape.
function deriveCutoverOption(strategy?: MigrationBlueprintStrategy): string {
  if (strategy?.adminInitiatedCutOver) return CUTOVER_TYPES.ADMIN_INITIATED
  if (isSetTime(strategy?.vmCutoverStart) || isSetTime(strategy?.vmCutoverEnd)) {
    return CUTOVER_TYPES.TIME_WINDOW
  }
  return CUTOVER_TYPES.IMMEDIATE
}

export function blueprintToSavedTemplate(blueprint: MigrationBlueprint): SavedTemplate {
  const { spec, metadata } = blueprint
  return {
    name: metadata.name,
    displayName: spec.displayName,
    description: spec.description,
    createdAt: metadata.creationTimestamp || new Date(0).toISOString(),
    sourceVCenter: spec.vmwareRef || '',
    destination: spec.pcdRef || '',
    targetCluster: spec.targetPCDClusterName || '',
    networkMappings: spec.networkMappings || [],
    storageMappings: spec.storageMappings || [],
    dataCopyMethod: (spec.migrationStrategy?.type || 'cold') as SavedTemplate['dataCopyMethod'],
    cutoverOption: deriveCutoverOption(spec.migrationStrategy),
    osFamily: spec.osFamily,
    useGPU: spec.useGPUFlavor,
    spec
  }
}

export function savedTemplateInputToBlueprintSpec(
  input: SaveAsTemplateInput
): MigrationBlueprintSpec {
  return {
    displayName: input.displayName,
    ...(input.description && { description: input.description }),
    ...(input.sourceVCenter && { vmwareRef: input.sourceVCenter }),
    ...(input.destination && { pcdRef: input.destination }),
    ...(input.targetCluster && { targetPCDClusterName: input.targetCluster }),
    networkMappings: input.networkMappings,
    storageMappings: input.storageMappings,
    migrationStrategy: {
      type: input.dataCopyMethod,
      adminInitiatedCutOver: input.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED
    },
    ...(input.osFamily && { osFamily: input.osFamily as MigrationBlueprintSpec['osFamily'] }),
    useGPUFlavor: input.useGPU || false
  }
}

export function sanitizeTemplateName(displayName: string): string {
  return (
    displayName
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/(^-|-$)/g, '') || 'template'
  )
}

// Appends -2, -3, ... until the candidate k8s object name doesn't collide with an
// existing one — mirrors k8s object-name uniqueness without racing the API server.
export function uniqueTemplateName(base: string, existingNames: string[]): string {
  const taken = new Set(existingNames)
  let candidate = base
  let suffix = 2
  while (taken.has(candidate)) {
    candidate = `${base}-${suffix}`
    suffix += 1
  }
  return candidate
}
