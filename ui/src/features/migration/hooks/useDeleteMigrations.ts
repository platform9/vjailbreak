import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Migration, deleteMigration } from '../api/migrations'
import { getMigrationPlan, patchMigrationPlan } from '../api/migrationPlans'
import { MIGRATIONS_QUERY_KEY } from './useMigrationsQuery'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'
import { getRegionNameForMigrationPlan } from 'src/utils/regionNameResolver'

export function useDeleteMigrations() {
  const queryClient = useQueryClient()
  const { track } = useAmplitude({ component: 'useDeleteMigrations' })
  const [isDeleting, setIsDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const deleteMigrations = async (migrations: Migration[]): Promise<boolean> => {
    setIsDeleting(true)
    setError(null)
    const snapshot = [...migrations]

    try {
      // Group by migration plan, collect migrations without a plan separately
      const byPlan: Record<string, { vmsToRemove: Set<string>; migrationsToDelete: Set<string> }> = {}
      const noPlanNames: string[] = []

      for (const m of snapshot) {
        const planId = m.spec?.migrationPlan
        if (!planId) {
          noPlanNames.push(m.metadata.name)
          continue
        }
        if (!byPlan[planId]) {
          byPlan[planId] = { vmsToRemove: new Set(), migrationsToDelete: new Set() }
        }
        const vmKey =
          m.metadata?.annotations?.['vjailbreak.k8s.pf9.io/original-vm-name'] ||
          m.metadata?.labels?.['vjailbreak.k8s.pf9.io/vm-key'] ||
          m.spec.vmName
        byPlan[planId].vmsToRemove.add(vmKey)
        byPlan[planId].migrationsToDelete.add(m.metadata.name)
      }

      await Promise.all([
        ...Object.entries(byPlan).map(async ([planId, { vmsToRemove, migrationsToDelete }]) => {
          const plan = await getMigrationPlan(planId)
          const updatedVMs = plan.spec.virtualMachines?.[0]?.filter((vm) => !vmsToRemove.has(vm)) ?? []
          await patchMigrationPlan(planId, { spec: { virtualMachines: [updatedVMs] } })
          await Promise.all(Array.from(migrationsToDelete).map((name) => deleteMigration(name)))
        }),
        ...noPlanNames.map((name) => deleteMigration(name)),
      ])

      snapshot.forEach((m) => {
        void (async () => {
          const regionName = await getRegionNameForMigrationPlan(
            m.spec?.migrationPlan,
            m.metadata?.namespace
          )
          track(AMPLITUDE_EVENTS.MIGRATION_DELETED, {
            migrationName: m.metadata?.name,
            migrationPlan: m.spec?.migrationPlan,
            vmName: m.spec?.vmName,
            regionName,
            namespace: m.metadata?.namespace,
          })
        })()
      })

      queryClient.invalidateQueries({ queryKey: MIGRATIONS_QUERY_KEY })
      return true
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)

      snapshot.forEach((m) => {
        void (async () => {
          const regionName = await getRegionNameForMigrationPlan(
            m.spec?.migrationPlan,
            m.metadata?.namespace
          )
          track(AMPLITUDE_EVENTS.MIGRATION_DELETE_FAILED, {
            migrationName: m.metadata?.name,
            migrationPlan: m.spec?.migrationPlan,
            vmName: m.spec?.vmName,
            regionName,
            namespace: m.metadata?.namespace,
            errorMessage: msg,
          })
        })()
      })

      return false
    } finally {
      setIsDeleting(false)
    }
  }

  return { deleteMigrations, isDeleting, error, setError }
}
