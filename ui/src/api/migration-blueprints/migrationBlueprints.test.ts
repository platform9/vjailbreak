import { describe, expect, it, vi } from 'vitest'
import {
  getMigrationBlueprintsList,
  getMigrationBlueprint,
  postMigrationBlueprint,
  deleteMigrationBlueprint
} from './migrationBlueprints'
import axios from '../axios'
import { MigrationBlueprint, MigrationBlueprintList } from './model'

vi.mock('../axios', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    del: vi.fn()
  }
}))

const mockedAxios = vi.mocked(axios, true)

const makeBlueprint = (name: string): MigrationBlueprint => ({
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'MigrationBlueprint',
  metadata: { name, namespace: 'migration-system' },
  spec: { displayName: name }
})

describe('getMigrationBlueprintsList', () => {
  it('lists blueprints in the default namespace', async () => {
    const list: MigrationBlueprintList = {
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'MigrationBlueprintList',
      metadata: { resourceVersion: '1' },
      items: [makeBlueprint('a'), makeBlueprint('b')]
    }
    mockedAxios.get.mockResolvedValue(list)

    const result = await getMigrationBlueprintsList()

    expect(mockedAxios.get).toHaveBeenCalledWith({
      endpoint: '/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints'
    })
    expect(result).toEqual(list.items)
  })

  it('returns an empty array when the response has no items', async () => {
    mockedAxios.get.mockResolvedValue(undefined)

    expect(await getMigrationBlueprintsList()).toEqual([])
  })
})

describe('getMigrationBlueprint', () => {
  it('fetches a single blueprint by name', async () => {
    const blueprint = makeBlueprint('my-template')
    mockedAxios.get.mockResolvedValue(blueprint)

    const result = await getMigrationBlueprint('my-template')

    expect(mockedAxios.get).toHaveBeenCalledWith({
      endpoint:
        '/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints/my-template'
    })
    expect(result).toEqual(blueprint)
  })
})

describe('postMigrationBlueprint', () => {
  it('posts the body to the collection endpoint', async () => {
    const body = { metadata: { name: 'x' }, spec: { displayName: 'x' } }
    mockedAxios.post.mockResolvedValue(makeBlueprint('x'))

    await postMigrationBlueprint(body)

    expect(mockedAxios.post).toHaveBeenCalledWith({
      endpoint: '/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints',
      data: body
    })
  })
})

describe('deleteMigrationBlueprint', () => {
  it('deletes a blueprint by name', async () => {
    mockedAxios.del.mockResolvedValue(undefined)

    await deleteMigrationBlueprint('my-template')

    expect(mockedAxios.del).toHaveBeenCalledWith({
      endpoint:
        '/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrationblueprints/my-template'
    })
  })
})
