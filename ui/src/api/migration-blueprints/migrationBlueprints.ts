import axios from '../axios'
import { VJAILBREAK_API_BASE_PATH, VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'
import { MigrationBlueprint, MigrationBlueprintList } from './model'

const MIGRATION_BLUEPRINTS_RESOURCE = 'migrationblueprints'

export const getMigrationBlueprintsList = async (namespace = VJAILBREAK_DEFAULT_NAMESPACE) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${MIGRATION_BLUEPRINTS_RESOURCE}`
  const response = await axios.get<MigrationBlueprintList>({ endpoint })
  return response?.items ?? []
}

export const getMigrationBlueprint = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${MIGRATION_BLUEPRINTS_RESOURCE}/${name}`
  return axios.get<MigrationBlueprint>({ endpoint })
}

export const postMigrationBlueprint = async (
  body: Partial<MigrationBlueprint>,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${MIGRATION_BLUEPRINTS_RESOURCE}`
  return axios.post<MigrationBlueprint>({ endpoint, data: body })
}

export const deleteMigrationBlueprint = async (
  name: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
) => {
  const endpoint = `${VJAILBREAK_API_BASE_PATH}/namespaces/${namespace}/${MIGRATION_BLUEPRINTS_RESOURCE}/${name}`
  return axios.del<MigrationBlueprint>({ endpoint })
}
