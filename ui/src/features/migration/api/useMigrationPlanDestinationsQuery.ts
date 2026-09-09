import { useMemo } from 'react'
import { useQuery, UseQueryResult } from '@tanstack/react-query'

import { getMigrationPlan } from 'src/api/migration-plans/migrationPlans'
import { getMigrationTemplate } from 'src/api/migration-templates/migrationTemplates'
import type { MigrationTemplateStatus } from 'src/api/migration-templates/model'
import { getOpenstackCredentialsList } from 'src/api/openstack-creds/openstackCreds'
import { getPCDClusters } from 'src/api/pcd-clusters/pcdClusters'
import type { Migration } from './migrations'
import { buildDestinationTenantResolver } from '../utils/pcdClusterLookup'

export type MigrationPlanDestination = {
  destinationCluster: string
  destinationTenant: string
  sourceVmwareRef: string
  sourceDatacenter: string
  destinationOpenstackRef: string
  vmOsByName: Record<string, string>
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

          // One resolver per namespace: the credential ref is authoritative, cluster-name
          // lookups stay as fallbacks. See buildDestinationTenantResolver.
          return [
            namespace,
            buildDestinationTenantResolver(openstackCredsList, pcdClustersList?.items),
          ] as const
        })
      )

      const lookupByNamespace = new Map(namespaceLookups)

      const resolveDestinationTenant = (
        namespace: string,
        openstackRef: string,
        destinationCluster: string
      ): string => lookupByNamespace.get(namespace)?.(openstackRef, destinationCluster) ?? 'N/A'

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
          const destinationOpenstackRef = (templateSpec?.destination?.openstackRef as string) || 'N/A'
          // The ref identifies the tenant exactly; resolving from the cluster name alone
          // returns another credential's tenant when two share a cluster name.
          const destinationTenant = resolveDestinationTenant(
            namespace,
            destinationOpenstackRef,
            destinationCluster
          )
          const sourceVmwareRef = (templateSpec?.source?.vmwareRef as string) || 'N/A'
          const sourceDatacenter = (templateSpec?.source?.datacenter as string) || 'N/A'
          const templateStatus = template?.status as MigrationTemplateStatus | undefined
          const vmOsByName: Record<string, string> = {}
          for (const vm of templateStatus?.vmware || []) {
            if (vm?.name && vm?.osFamily) vmOsByName[String(vm.name)] = String(vm.osFamily)
          }
          return [
            key,
            {
              destinationCluster,
              destinationTenant,
              sourceVmwareRef,
              sourceDatacenter,
              destinationOpenstackRef,
              vmOsByName
            }
          ] as const
        })
      )

      return Object.fromEntries(results)
    }
  })
}
