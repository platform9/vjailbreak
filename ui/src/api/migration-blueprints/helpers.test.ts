import { describe, expect, it } from 'vitest'
import { createMigrationBlueprintJson } from './helpers'

describe('createMigrationBlueprintJson', () => {
  it('omits resourceVersion when not provided (create/POST)', () => {
    const body = createMigrationBlueprintJson('my-template', { displayName: 'My Template' })

    expect(body).toEqual({
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'MigrationBlueprint',
      metadata: { name: 'my-template' },
      spec: { displayName: 'My Template' }
    })
    expect(body.metadata).not.toHaveProperty('resourceVersion')
  })

  it('includes resourceVersion when provided (update/PUT)', () => {
    const body = createMigrationBlueprintJson(
      'my-template',
      { displayName: 'My Template' },
      '42'
    )

    expect(body.metadata).toEqual({ name: 'my-template', resourceVersion: '42' })
  })
})
