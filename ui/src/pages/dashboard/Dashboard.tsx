import { Paper, styled, Tab, Tabs } from "@mui/material"
import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { FIVE_SECONDS, THIRTY_SECONDS } from "src/constants"
import { useMigrationsQuery } from "src/hooks/api/useMigrationsQuery"
import { deleteMigration } from "src/api/migrations/migrations"
import { useQueryClient } from "@tanstack/react-query"
import { MIGRATIONS_QUERY_KEY } from "src/hooks/api/useMigrationsQuery"
import ConfirmationDialog from "src/components/dialogs/ConfirmationDialog"
import { getMigrationPlan, patchMigrationPlan } from "src/api/migration-plans/migrationPlans"
import { Migration } from "src/api/migrations/model"
import MigrationsTable from "./MigrationsTable"
import NodesTable from "./NodesTable"
import CredentialsTable from "./CredentialsTable"
import BMConfigForm from "./BMConfigForm"
import WarningIcon from '@mui/icons-material/Warning';
import { useNodesQuery } from "../../hooks/api/useNodesQuery"
import { useClusterMigrationsQuery } from "../../hooks/api/useClusterMigrationsQuery"
import { useESXIMigrationsQuery } from "../../hooks/api/useESXIMigrationsQuery"
import { deleteClusterMigration } from "src/api/clustermigrations/clustermigrations"
import { deleteESXIMigration } from "src/api/esximigrations/esximigrations"
import { CLUSTER_MIGRATIONS_QUERY_KEY } from "src/hooks/api/useClusterMigrationsQuery"
import { ESXI_MIGRATIONS_QUERY_KEY } from "src/hooks/api/useESXIMigrationsQuery"
import RollingMigrationsTable from "./RollingMigrationsTable"
import { ClusterMigration } from "src/api/clustermigrations/model"
import { ESXIMigration } from "src/api/esximigrations/model"


const DashboardContainer = styled("div")({
  display: "flex",
  justifyContent: "center",
  width: "100%",
  height: "100%",
  padding: "60px 20px",
  boxSizing: "border-box"
})

const StyledPaper = styled(Paper)({
  width: "100%",
  "& .MuiDataGrid-virtualScroller": {
    overflowX: "hidden"
  }
})


export default function Dashboard() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedMigrations, setSelectedMigrations] = useState<Migration[]>([])
  const [selectedClusterMigrations, setSelectedClusterMigrations] = useState<ClusterMigration[]>([])
  const [selectedESXIMigrations, setSelectedESXIMigrations] = useState<ESXIMigration[]>([])
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [deleteType, setDeleteType] = useState<'migration' | 'clusterMigration' | 'esxiMigration'>('migration')
  const [activeTab, setActiveTab] = useState(0)

  const { data: migrations, refetch: refetchMigrations } = useMigrationsQuery(undefined, {
    refetchInterval: (query) => {
      const migrations = query?.state?.data || []
      const hasPendingMigration = !!migrations.find(
        (m) => m.status === undefined
      )
      return hasPendingMigration ? FIVE_SECONDS : THIRTY_SECONDS
    },
    staleTime: 0,
    refetchOnMount: true
  })

  const { data: clusterMigrations, refetch: refetchClusterMigrations } = useClusterMigrationsQuery({
    queryKey: CLUSTER_MIGRATIONS_QUERY_KEY,
    refetchInterval: THIRTY_SECONDS,
    staleTime: 0,
    refetchOnMount: true
  })

  const { data: esxiMigrations, refetch: refetchESXIMigrations } = useESXIMigrationsQuery({
    queryKey: ESXI_MIGRATIONS_QUERY_KEY,
    refetchInterval: THIRTY_SECONDS,
    staleTime: 0,
    refetchOnMount: true
  })

  const handleDeleteClick = (migrationName: string) => {
    const migration = migrations?.find(m => m.metadata.name === migrationName)
    if (migration) {
      setSelectedMigrations([migration])
      setDeleteType('migration')
      setDeleteDialogOpen(true)
    }
  }

  const handleDeleteClose = () => {
    setDeleteDialogOpen(false)
    setSelectedMigrations([])
    setSelectedClusterMigrations([])
    setSelectedESXIMigrations([])
    setDeleteError(null)
  }

  const handleDeleteSelected = (migrations: Migration[]) => {
    setSelectedMigrations(migrations)
    setDeleteType('migration')
    setDeleteDialogOpen(true)
  }

  const handleDeleteSelectedClusterMigrations = (clusterMigrations: ClusterMigration[]) => {
    setSelectedClusterMigrations(clusterMigrations)
    setDeleteType('clusterMigration')
    setDeleteDialogOpen(true)
  }

  const handleDeleteSelectedESXIMigrations = (esxiMigrations: ESXIMigration[]) => {
    setSelectedESXIMigrations(esxiMigrations)
    setDeleteType('esxiMigration')
    setDeleteDialogOpen(true)
  }

  const handleDeleteMigration = async (migrations: Migration[]) => {
    // Group VMs by migration plan
    const migrationPlanUpdates = migrations.reduce((acc, migration) => {
      const planId = migration.spec.migrationPlan
      if (!acc[planId]) {
        acc[planId] = {
          vmsToRemove: new Set<string>(),
          migrationsToDelete: new Set<string>()
        }
      }
      acc[planId].vmsToRemove.add(migration.spec.vmName)
      acc[planId].migrationsToDelete.add(migration.metadata.name)
      return acc
    }, {} as Record<string, { vmsToRemove: Set<string>, migrationsToDelete: Set<string> }>)

    // Update each migration plan once
    await Promise.all(
      Object.entries(migrationPlanUpdates).map(async ([planId, { vmsToRemove, migrationsToDelete }]) => {
        const migrationPlan = await getMigrationPlan(planId)
        const updatedVirtualMachines = migrationPlan.spec.virtualMachines?.[0]?.filter(
          vm => !vmsToRemove.has(vm)
        )

        await patchMigrationPlan(planId, {
          spec: {
            virtualMachines: [updatedVirtualMachines]
          }
        })

        // Delete all migrations for this plan
        await Promise.all(
          Array.from(migrationsToDelete).map(migrationName =>
            deleteMigration(migrationName)
          )
        )
      })
    )

    queryClient.invalidateQueries({ queryKey: MIGRATIONS_QUERY_KEY })
    handleDeleteClose()
  }

  const handleDeleteClusterMigration = async (clusterMigrations: ClusterMigration[]) => {
    await Promise.all(
      clusterMigrations.map(migration =>
        deleteClusterMigration(migration.metadata.name)
      )
    )
    queryClient.invalidateQueries({ queryKey: CLUSTER_MIGRATIONS_QUERY_KEY })
    handleDeleteClose()
  }

  const handleDeleteESXIMigration = async (esxiMigrations: ESXIMigration[]) => {
    await Promise.all(
      esxiMigrations.map(migration =>
        deleteESXIMigration(migration.metadata.name)
      )
    )
    queryClient.invalidateQueries({ queryKey: ESXI_MIGRATIONS_QUERY_KEY })
    handleDeleteClose()
  }

  const handleClusterMigrationDeleteClick = (migrationName: string) => {
    const migration = clusterMigrations?.find(m => m.metadata.name === migrationName)
    if (migration) {
      setSelectedClusterMigrations([migration])
      setDeleteType('clusterMigration')
      setDeleteDialogOpen(true)
    }
  }

  const handleESXIMigrationDeleteClick = (migrationName: string) => {
    const migration = esxiMigrations?.find(m => m.metadata.name === migrationName)
    if (migration) {
      setSelectedESXIMigrations([migration])
      setDeleteType('esxiMigration')
      setDeleteDialogOpen(true)
    }
  }

  const handleDeleteAction = (): Promise<void> => {
    if (deleteType === 'migration') {
      return handleDeleteMigration(selectedMigrations)
    } else if (deleteType === 'clusterMigration') {
      return handleDeleteClusterMigration(selectedClusterMigrations)
    } else {
      return handleDeleteESXIMigration(selectedESXIMigrations)
    }
  }

  const getDeleteDialogTitle = () => {
    if (deleteType === 'migration') {
      return "Confirm Delete Migration"
    } else if (deleteType === 'clusterMigration') {
      return "Confirm Delete Cluster Migration"
    } else if (deleteType === 'esxiMigration') {
      return "Confirm Delete ESXi Migration"
    }
    return "Confirm Delete"
  }

  const getDeleteDialogMessage = () => {
    if (deleteType === 'migration') {
      return selectedMigrations.length > 1
        ? "Are you sure you want to delete these migrations?"
        : `Are you sure you want to delete migration "${selectedMigrations[0]?.metadata.name}"?`
    } else if (deleteType === 'clusterMigration') {
      return selectedClusterMigrations.length > 1
        ? "Are you sure you want to delete these cluster migrations?"
        : `Are you sure you want to delete cluster migration "${selectedClusterMigrations[0]?.metadata.name}"?`
    } else if (deleteType === 'esxiMigration') {
      return selectedESXIMigrations.length > 1
        ? "Are you sure you want to delete these ESXi migrations?"
        : `Are you sure you want to delete ESXi migration "${selectedESXIMigrations[0]?.metadata.name}"?`
    }
    return "Are you sure you want to delete the selected items?"
  }

  const getDeleteDialogItems = () => {
    if (deleteType === 'migration') {
      return selectedMigrations.map(m => ({
        id: m.metadata.name,
        name: m.metadata.name
      }))
    } else if (deleteType === 'clusterMigration') {
      return selectedClusterMigrations.map(m => ({
        id: m.metadata.name,
        name: m.metadata.name
      }))
    } else if (deleteType === 'esxiMigration') {
      return selectedESXIMigrations.map(m => ({
        id: m.metadata.name,
        name: m.metadata.name
      }))
    }
    return []
  }

  const getCustomErrorMessage = (error: Error | string) => {
    const baseMessage = "Failed to delete migrations"
    if (error instanceof Error) {
      return `${baseMessage}: ${error.message}`
    }
    return baseMessage
  }

  const { data: nodes } = useNodesQuery()

  useEffect(() => {
    if (!!migrations && migrations.length === 0 && (!nodes || nodes.length === 0)) {
      navigate("/onboarding")
    }
  }, [migrations, navigate])

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setActiveTab(newValue)
  }

  return (
    <DashboardContainer>
      <StyledPaper>
        <Tabs
          value={activeTab}
          onChange={handleTabChange}
          sx={{ borderBottom: 1, borderColor: 'divider' }}
        >
          <Tab label="Migrations" />
          <Tab label="Rolling Migrations" />
          <Tab label="Agents" />
          <Tab label="Credentials" />
          <Tab label="Maas Config" />
        </Tabs>

        {activeTab === 0 ? (
          <MigrationsTable
            refetchMigrations={refetchMigrations}
            migrations={migrations || []}
            onDeleteMigration={handleDeleteClick}
            onDeleteSelected={handleDeleteSelected}
          />
        ) : activeTab === 1 ? (
          <RollingMigrationsTable
            clusterMigrations={clusterMigrations || []}
            esxiMigrations={esxiMigrations || []}
            onDeleteClusterMigration={handleClusterMigrationDeleteClick}
            onDeleteESXIMigration={handleESXIMigrationDeleteClick}
            onDeleteSelectedClusterMigrations={handleDeleteSelectedClusterMigrations}
            onDeleteSelectedESXIMigrations={handleDeleteSelectedESXIMigrations}
            refetchClusterMigrations={refetchClusterMigrations}
            refetchESXIMigrations={refetchESXIMigrations}
          />
        ) : activeTab === 2 ? (
          <NodesTable />
        ) : activeTab === 3 ? (
          <CredentialsTable />
        ) : (
          <BMConfigForm />
        )}
      </StyledPaper>

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={handleDeleteClose}
        title={getDeleteDialogTitle()}
        icon={<WarningIcon color="warning" />}
        message={getDeleteDialogMessage()}
        items={getDeleteDialogItems()}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleDeleteAction}
        customErrorMessage={getCustomErrorMessage}
        errorMessage={deleteError}
        onErrorChange={setDeleteError}
      />
    </DashboardContainer>
  )
}
