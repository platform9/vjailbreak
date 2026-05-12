import type { KubernetesObject } from './types'

export const value = (input: unknown): string => (typeof input === 'string' ? input.trim() : '')

export const record = (input: unknown) =>
  input && typeof input === 'object' && !Array.isArray(input)
    ? (input as Record<string, unknown>)
    : {}

export const field = (input: unknown, ...keys: string[]): unknown =>
  keys.reduce<unknown>((current, key) => record(current)[key], input)

export const hasValue = (set: Set<string>, input: unknown): boolean => {
  const next = value(input)
  return Boolean(next && set.has(next))
}

export const addValue = (set: Set<string>, input: unknown) => {
  const next = value(input)
  if (next) set.add(next)
}

export const addValues = (set: Set<string>, inputs: unknown) => {
  if (!Array.isArray(inputs)) return
  inputs.forEach((input) => addValue(set, input))
}

export const objectName = (object: KubernetesObject): string => value(object.metadata?.name)

export const isOwnedBy = (object: KubernetesObject, owners: Set<string>): boolean => {
  const ownerReferences = object.metadata?.ownerReferences || []
  return ownerReferences.some(
    (owner) => hasValue(owners, owner.uid) || hasValue(owners, owner.name)
  )
}
