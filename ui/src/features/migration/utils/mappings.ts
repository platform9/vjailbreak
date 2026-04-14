export type SourceTargetMapping = { source: string; target: string }

export const isMappingComplete = (
  sources: string[],
  mappings: SourceTargetMapping[] | undefined
): boolean => {
  if (!Array.isArray(sources) || sources.length === 0) return true
  const mapped = new Set((mappings || []).map((m) => m.source))
  return sources.every((s) => mapped.has(s))
}

export const getUnmappedCount = (
  sources: string[],
  mappings: SourceTargetMapping[] | undefined
): number => {
  if (!Array.isArray(sources) || sources.length === 0) return 0
  const mapped = new Set((mappings || []).map((m) => m.source))
  return sources.reduce((acc, s) => acc + (mapped.has(s) ? 0 : 1), 0)
}
