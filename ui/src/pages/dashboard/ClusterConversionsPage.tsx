import RollingMigrationsTable from "./RollingMigrationsTable"
import { useMigrationsQuery } from "src/hooks/api/useMigrationsQuery"
import { useClusterMigrationsQuery, CLUSTER_MIGRATIONS_QUERY_KEY } from "src/hooks/api/useClusterMigrationsQuery"
import { useESXIMigrationsQuery, ESXI_MIGRATIONS_QUERY_KEY } from "src/hooks/api/useESXIMigrationsQuery"
import { useRollingMigrationPlansQuery } from "src/hooks/api/useRollingMigrationPlansQuery"
import { THIRTY_SECONDS } from "src/constants"

export default function ClusterConversionsPage() {
  const { data: migrations, refetch: refetchMigrations } = useMigrationsQuery()

  const { data: clusterMigrations } = useClusterMigrationsQuery({
    queryKey: CLUSTER_MIGRATIONS_QUERY_KEY,
    refetchInterval: THIRTY_SECONDS,
    staleTime: 0,
    refetchOnMount: true
  })

  const { data: esxiMigrations } = useESXIMigrationsQuery({
    queryKey: ESXI_MIGRATIONS_QUERY_KEY,
    refetchInterval: THIRTY_SECONDS,
    staleTime: 0,
    refetchOnMount: true
  })

  const { data: rollingMigrationPlans, refetch: refetchRollingMigrationPlans } = useRollingMigrationPlansQuery({
    refetchInterval: THIRTY_SECONDS,
    staleTime: 0,
    refetchOnMount: true
  })

  return (
    <RollingMigrationsTable
      rollingMigrationPlans={rollingMigrationPlans || []}
      clusterMigrations={clusterMigrations || []}
      esxiMigrations={esxiMigrations || []}
      migrations={migrations || []}
      refetchRollingMigrationPlans={refetchRollingMigrationPlans}
      refetchMigrations={refetchMigrations}
    />
  )
}