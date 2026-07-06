import { useState } from 'react'
import { Box, Divider, Typography } from '@mui/material'
import RollingMigrationsTable from '../components/RollingMigrationsTable'
import BatchesTable from '../components/BatchesTable'
import CreateBatchDialog from '../components/CreateBatchDialog'
import BatchDetailDrawer from '../components/BatchDetailDrawer'
import { useMigrationsQuery } from 'src/hooks/api/useMigrationsQuery'
import {
  useESXIMigrationsQuery,
  ESXI_MIGRATIONS_QUERY_KEY
} from 'src/hooks/api/useESXIMigrationsQuery'
import { useRollingMigrationPlansQuery } from 'src/hooks/api/useRollingMigrationPlansQuery'
import { useClusterConversionBatchesQuery } from 'src/hooks/api/useClusterConversionBatchesQuery'
import { THIRTY_SECONDS } from 'src/constants'
import { useRollingMigrationsStatusMonitor } from 'src/hooks/useRollingMigrationsStatusMonitor'
import { ClusterConversionBatch } from 'src/api/cluster-conversion-batches/model'

export default function ClusterConversionsPage() {
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [selectedBatch, setSelectedBatch] = useState<ClusterConversionBatch | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)

  const { data: migrations, refetch: refetchMigrations } = useMigrationsQuery()

  const { data: esxiMigrations, refetch: refetchESXIMigrations } = useESXIMigrationsQuery({
    queryKey: ESXI_MIGRATIONS_QUERY_KEY,
    refetchInterval: THIRTY_SECONDS,
    staleTime: 0,
    refetchOnMount: true
  })

  const { data: rollingMigrationPlans, refetch: refetchRollingMigrationPlans } =
    useRollingMigrationPlansQuery({
      refetchInterval: THIRTY_SECONDS,
      staleTime: 0,
      refetchOnMount: true
    })

  const { data: batches, refetch: refetchBatches } = useClusterConversionBatchesQuery({
    refetchInterval: THIRTY_SECONDS,
    staleTime: 0,
    refetchOnMount: true
  })

  useRollingMigrationsStatusMonitor(rollingMigrationPlans)

  const handleViewDetails = (batch: ClusterConversionBatch) => {
    setSelectedBatch(batch)
    setDrawerOpen(true)
  }

  const handleCloseDrawer = () => {
    setDrawerOpen(false)
    setSelectedBatch(null)
  }

  const handleRefreshDrawer = () => {
    refetchBatches()
  }

  return (
    <Box sx={{ width: '100%', height: '100%', display: 'flex', flexDirection: 'column', gap: 2 }}>
      <BatchesTable
        batches={batches || []}
        refetchBatches={refetchBatches}
        onCreateBatch={() => setCreateDialogOpen(true)}
        onViewDetails={handleViewDetails}
      />

      {(rollingMigrationPlans || []).length > 0 && (
        <>
          <Divider />
          <Box sx={{ px: 2 }}>
            <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
              Legacy Rolling Migration Plans (in-flight)
            </Typography>
          </Box>
          <RollingMigrationsTable
            rollingMigrationPlans={rollingMigrationPlans || []}
            esxiMigrations={esxiMigrations || []}
            migrations={migrations || []}
            refetchRollingMigrationPlans={refetchRollingMigrationPlans}
            refetchESXIMigrations={refetchESXIMigrations}
            refetchMigrations={refetchMigrations}
          />
        </>
      )}

      <CreateBatchDialog open={createDialogOpen} onClose={() => setCreateDialogOpen(false)} />

      <BatchDetailDrawer
        open={drawerOpen}
        onClose={handleCloseDrawer}
        batch={selectedBatch}
        onRefresh={handleRefreshDrawer}
      />
    </Box>
  )
}
