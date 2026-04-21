import { getMigrationPlan } from 'src/api/migration-plans/migrationPlans'
import { getMigrationTemplate } from 'src/api/migration-templates/migrationTemplates'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { getSecret } from 'src/api/secrets/secrets'

const REGION_LABEL_KEY = 'vjailbreak.k8s.pf9.io/region-name'

const regionByPlanCache = new Map<string, Promise<string | undefined>>()
const regionByTemplateCache = new Map<string, Promise<string | undefined>>()
const regionByOpenstackRefCache = new Map<string, Promise<string | undefined>>()

const normalizeRegion = (region: unknown) =>
  typeof region === 'string' && region.trim().length > 0 ? region.trim() : undefined

export const getRegionNameForOpenstackRef = (openstackRef?: string, namespace?: string) => {
  if (!openstackRef) return Promise.resolve(undefined)
  const key = `${namespace || ''}:${openstackRef}`

  const cached = regionByOpenstackRefCache.get(key)
  if (cached) return cached

  const regionPromise = (async () => {
    try {
      const openstackCred = await getOpenstackCredentials(openstackRef, namespace)
      const labeledRegion = normalizeRegion(openstackCred?.metadata?.labels?.[REGION_LABEL_KEY])
      if (labeledRegion) return labeledRegion

      const secretName = `${openstackRef}-openstack-secret`
      const secret = await getSecret(secretName, namespace)
      return normalizeRegion(secret?.data?.OS_REGION_NAME)
    } catch {
      return undefined
    }
  })()

  regionByOpenstackRefCache.set(key, regionPromise)
  return regionPromise
}

export const getRegionNameForMigrationTemplate = (templateName?: string, namespace?: string) => {
  if (!templateName) return Promise.resolve(undefined)
  const key = `${namespace || ''}:${templateName}`

  const cached = regionByTemplateCache.get(key)
  if (cached) return cached

  const regionPromise = (async () => {
    try {
      const template = await getMigrationTemplate(templateName, namespace)
      const openstackRef = template?.spec?.destination?.openstackRef
      return getRegionNameForOpenstackRef(openstackRef, namespace)
    } catch {
      return undefined
    }
  })()

  regionByTemplateCache.set(key, regionPromise)
  return regionPromise
}

export const getRegionNameForMigrationPlan = (planName?: string, namespace?: string) => {
  if (!planName) return Promise.resolve(undefined)
  const key = `${namespace || ''}:${planName}`

  const cached = regionByPlanCache.get(key)
  if (cached) return cached

  const regionPromise = (async () => {
    try {
      const plan = await getMigrationPlan(planName, namespace)
      const templateName = plan?.spec?.migrationTemplate
      return getRegionNameForMigrationTemplate(templateName, namespace)
    } catch {
      return undefined
    }
  })()

  regionByPlanCache.set(key, regionPromise)
  return regionPromise
}
