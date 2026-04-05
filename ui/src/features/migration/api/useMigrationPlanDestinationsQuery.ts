import { useMemo } from 'react'
import { useQuery, UseQueryResult } from '@tanstack/react-query'

import { getMigrationPlan } from 'src/api/migration-plans/migrationPlans'
import { getMigrationTemplate } from 'src/api/migration-templates/migrationTemplates'
import { getOpenstackCredentialsList } from 'src/api/openstack-creds/openstackCreds'
import { getPCDClusters } from 'src/api/pcd-clusters/pcdClusters'
import type { Migration } from './migrations'

export type MigrationPlanDestination = {
  destinationCluster: string
  destinationTenant: string
}

export type MigrationPlanDestinationsByKey = Record<string, MigrationPlanDestination>

export const useMigrationPlanDestinationsQuery = (
  migrations: Migration[]
): UseQueryResult<MigrationPlanDestinationsByKey> => {
  const planQueryKey = useMemo(() => {
    const namespaces = Array.from(
      new Set(
        migrations
          .map((m) => m.metadata?.namespace)
          .filter(Boolean)
          .map(String)
      )
    )
      .sort()
      .join(',')

    const planNames = Array.from(
      new Set(
        migrations
          .map((m) => (m.spec as any)?.migrationPlan || (m.metadata as any)?.labels?.migrationplan)
          .filter(Boolean)
          .map(String)
      )
    )
      .sort()
      .join(',')

    return ['migration-plan-destinations', namespaces, planNames]
  }, [migrations])

  return useQuery({
    queryKey: planQueryKey,
    enabled: migrations.length > 0,
    refetchOnWindowFocus: false,
    staleTime: 60_000,
    queryFn: async () => {
      const safeGet = async <T>(fn: () => Promise<T>): Promise<T | null> => {
        try {
          return await fn()
        } catch (error) {
          console.error('Error in safeGet:', error)
          return null
        }
      }

      // Perf: for hover tooltips we avoid N network calls by fetching OpenStack creds list and PCD clusters
      // once per namespace, and deriving the tenant via destinationCluster -> PCDCluster label mapping.
      const namespaces = Array.from(
        new Set(
          migrations
            .map((m) => m.metadata?.namespace)
            .filter(Boolean)
            .map(String)
        )
      )

      const namespaceLookups = await Promise.all(
        namespaces.map(async (namespace) => {
          const [openstackCredsList, pcdClustersList] = await Promise.all([
            safeGet(() => getOpenstackCredentialsList(namespace)),
            safeGet(() => getPCDClusters(namespace))
          ])

          const openstackCredNameToProjectName = new Map<string, string>()
          for (const cred of openstackCredsList || []) {
            const name = String(cred?.metadata?.name || '').trim()
            const projectName = String((cred?.spec as any)?.projectName || '').trim()
            if (name && projectName) openstackCredNameToProjectName.set(name, projectName)
          }

          const destinationClusterToOpenstackCredName = new Map<string, string>()
          const pcdClusters = (pcdClustersList as any)?.items || []
          for (const cluster of pcdClusters || []) {
            const clusterName = String((cluster as any)?.spec?.clusterName || '').trim()
            const openstackCredName = String(
              (cluster as any)?.metadata?.labels?.['vjailbreak.k8s.pf9.io/openstackcreds'] || ''
            ).trim()
            if (clusterName && openstackCredName) {
              destinationClusterToOpenstackCredName.set(clusterName, openstackCredName)
            }
          }

          const destinationClusterToProjectNameFromHostConfig = new Map<string, string>()
          for (const cred of openstackCredsList || []) {
            const projectName = String((cred?.spec as any)?.projectName || '').trim()
            if (!projectName) continue
            const hostConfigs = ((cred?.spec as any)?.pcdHostConfig as any[]) || []
            for (const cfg of hostConfigs) {
              const clusterName = String((cfg as any)?.clusterName || '').trim()
              if (clusterName && !destinationClusterToProjectNameFromHostConfig.has(clusterName)) {
                destinationClusterToProjectNameFromHostConfig.set(clusterName, projectName)
              }
            }
          }

          return [
            namespace,
            {
              openstackCredNameToProjectName,
              destinationClusterToOpenstackCredName,
              destinationClusterToProjectNameFromHostConfig
            }
          ] as const
        })
      )

      const lookupByNamespace = new Map(namespaceLookups)

      const resolveDestinationTenant = (namespace: string, destinationCluster: string): string => {
        const lookup = lookupByNamespace.get(namespace)
        if (!lookup) return 'N/A'
        const clusterName = String(destinationCluster || '').trim()
        if (!clusterName || clusterName === 'N/A') return 'N/A'

        const mappedCredName = lookup.destinationClusterToOpenstackCredName.get(clusterName)
        if (mappedCredName) {
          const projectName = lookup.openstackCredNameToProjectName.get(mappedCredName)
          if (projectName) return projectName
        }

        const fromHostConfig = lookup.destinationClusterToProjectNameFromHostConfig.get(clusterName)
        if (fromHostConfig) return fromHostConfig

        return 'N/A'
      }

      const planKeys = Array.from(
        new Set(
          migrations
            .map((m) => {
              const namespace = m.metadata?.namespace
              const planName =
                (m.spec as any)?.migrationPlan || (m.metadata as any)?.labels?.migrationplan
              if (!namespace || !planName) return ''
              return `${namespace}::${planName}`
            })
            .filter(Boolean)
        )
      )

      const results = await Promise.all(
        planKeys.map(async (key) => {
          const [namespace, planName] = key.split('::')
          const plan = await safeGet(() => getMigrationPlan(planName, namespace))
          const templateName = (plan?.spec as any)?.migrationTemplate as string | undefined
          const template = templateName
            ? await safeGet(() => getMigrationTemplate(templateName, namespace))
            : null
          const templateSpec = (template?.spec as any) || {}
          const destinationCluster = (templateSpec?.targetPCDClusterName as string) || 'N/A'
          const destinationTenant = resolveDestinationTenant(namespace, destinationCluster)
          return [key, { destinationCluster, destinationTenant }] as const
        })
      )

      return Object.fromEntries(results)
    }
  })
}
