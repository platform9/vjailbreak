import type { BundleEntry, KubernetesObject, Metadata, UnknownRecord } from './types'
import { record } from './objectUtils'

const isScalar = (input: unknown): boolean =>
  input === null || ['string', 'number', 'boolean'].includes(typeof input)

const isEmptyObject = (input: unknown): boolean =>
  Boolean(input && typeof input === 'object' && !Array.isArray(input)) &&
  Object.keys(input as UnknownRecord).length === 0

const plainStringPattern = /^[A-Za-z0-9_.@/:=,-]+$/
const plainKeyPattern = /^[A-Za-z_][A-Za-z0-9_.-]*$/
const numberLikePattern = /^[+-]?\d+(\.\d+)?$/
const timestampPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/
const yamlAmbiguousValues = new Set(['true', 'false', 'null', 'yes', 'no', 'on', 'off'])

const formatKey = (key: string): string => (plainKeyPattern.test(key) ? key : JSON.stringify(key))

const formatScalar = (input: unknown, indent: number): string => {
  if (input === null) return 'null'
  if (typeof input === 'number' || typeof input === 'boolean') return String(input)
  if (typeof input !== 'string') return JSON.stringify(input)
  if (input === '') return '""'

  if (input.includes('\n')) {
    const padding = ' '.repeat(indent + 2)
    const lines = input.replace(/\n$/, '').split('\n')
    return `|\n${lines
      .map((line) => (line === '' ? '' : `${padding}${line}`))
      .join('\n')}`
  }

  const lower = input.toLowerCase()
  if (
    plainStringPattern.test(input) &&
    !yamlAmbiguousValues.has(lower) &&
    !numberLikePattern.test(input) &&
    !timestampPattern.test(input)
  ) {
    return input
  }

  if (
    (input.startsWith('{') && input.endsWith('}')) ||
    (input.startsWith('[') && input.endsWith(']'))
  ) {
    return `'${input.replace(/'/g, "''")}'`
  }

  return JSON.stringify(input)
}

export const unknownToYaml = (input: unknown, indent = 0): string => {
  const padding = ' '.repeat(indent)

  if (isScalar(input)) {
    return `${padding}${formatScalar(input, indent)}`
  }

  if (Array.isArray(input)) {
    if (input.length === 0) return `${padding}[]`

    return input
      .map((item) => {
        if (isScalar(item)) {
          return `${padding}- ${formatScalar(item, indent)}`
        }

        if (Array.isArray(item) && item.length === 0) {
          return `${padding}- []`
        }

        if (isEmptyObject(item)) {
          return `${padding}- {}`
        }

        const lines = unknownToYaml(item, indent + 2).split('\n')
        return `${padding}- ${lines[0].trimStart()}${lines.length > 1 ? `\n${lines.slice(1).join('\n')}` : ''}`
      })
      .join('\n')
  }

  const object = record(input)
  const entries = Object.entries(object)
  if (entries.length === 0) return `${padding}{}`

  return entries
    .map(([key, item]) => {
      if (isScalar(item)) {
        return `${padding}${formatKey(key)}: ${formatScalar(item, indent)}`
      }

      if (Array.isArray(item) && item.length === 0) {
        return `${padding}${formatKey(key)}: []`
      }

      if (isEmptyObject(item)) {
        return `${padding}${formatKey(key)}: {}`
      }

      return `${padding}${formatKey(key)}:\n${unknownToYaml(item, indent + 2)}`
    })
    .join('\n')
}

const omitUndefined = (object: UnknownRecord): UnknownRecord =>
  Object.fromEntries(Object.entries(object).filter(([, item]) => item !== undefined))

const orderMetadata = (metadata?: Metadata): UnknownRecord | undefined => {
  if (!metadata) return undefined

  const knownKeys = new Set([
    'annotations',
    'creationTimestamp',
    'finalizers',
    'generateName',
    'generation',
    'labels',
    'name',
    'namespace',
    'ownerReferences',
    'resourceVersion',
    'uid'
  ])
  const extra = Object.fromEntries(
    Object.entries(metadata as UnknownRecord).filter(([key]) => !knownKeys.has(key))
  )

  return omitUndefined({
    annotations: (metadata as UnknownRecord).annotations,
    creationTimestamp: (metadata as UnknownRecord).creationTimestamp,
    finalizers: (metadata as UnknownRecord).finalizers,
    generateName: (metadata as UnknownRecord).generateName,
    generation: (metadata as UnknownRecord).generation,
    labels: (metadata as UnknownRecord).labels,
    name: metadata.name,
    namespace: metadata.namespace,
    ownerReferences: metadata.ownerReferences,
    resourceVersion: (metadata as UnknownRecord).resourceVersion,
    uid: metadata.uid,
    ...extra
  })
}

const removeManagedFields = (object: KubernetesObject): KubernetesObject => {
  const metadata = object.metadata
    ? ({
        ...object.metadata,
        managedFields: undefined
      } as Metadata)
    : undefined

  if (metadata) {
    delete (metadata as UnknownRecord).managedFields
  }

  return {
    ...object,
    metadata
  }
}

const withKubectlLikeOrder = (plural: string, object: KubernetesObject): UnknownRecord => {
  const clean = removeManagedFields(object)

  if (plural === 'configmaps') {
    return omitUndefined({
      apiVersion: clean.apiVersion || 'v1',
      data: clean.data,
      kind: clean.kind || 'ConfigMap',
      metadata: orderMetadata(clean.metadata)
    })
  }

  if (plural === 'pods') {
    return omitUndefined({
      apiVersion: clean.apiVersion || 'v1',
      kind: clean.kind || 'Pod',
      metadata: orderMetadata(clean.metadata),
      spec: clean.spec,
      status: clean.status
    })
  }

  return omitUndefined({
    apiVersion: clean.apiVersion,
    kind: clean.kind,
    metadata: orderMetadata(clean.metadata),
    spec: clean.spec,
    status: clean.status,
    data: clean.data
  })
}

export const formatYamlBundle = (entries: BundleEntry[], warnings: string[]): string => {
  let output = ''
  const separator = `${'='.repeat(80)}\n`

  for (const [index, entry] of entries.sort((a, b) => a.path.localeCompare(b.path)).entries()) {
    if (index > 0) output += '\n'
    output += separator
    output += `FILE: ${entry.path}\n`
    output += separator
    output += `${unknownToYaml(withKubectlLikeOrder(entry.path.split('/')[1] || '', entry.object))}\n`
  }

  if (warnings.length > 0) {
    if (output) output += '\n'
    output += separator
    output += 'FILE: collection-warnings.txt\n'
    output += separator
    output += warnings.join('\n')
    output += '\n'
  }

  return output
}
