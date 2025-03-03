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
import WarningIcon from '@mui/icons-material/Warning';
import { useNodesQuery } from "../../hooks/api/useNodesQuery"


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
  const [deleteError, setDeleteError] = useState<string | null>(null)
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

  const handleDeleteClick = (migrationName: string) => {
    const migration = migrations?.find(m => m.metadata.name === migrationName)
    if (migration) {
      setSelectedMigrations([migration])
      setDeleteDialogOpen(true)
    }
  }

  const handleDeleteClose = () => {
    setDeleteDialogOpen(false)
    setSelectedMigrations([])
    setDeleteError(null)
  }

  const handleDeleteSelected = (migrations: Migration[]) => {
    setSelectedMigrations(migrations)
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
        const updatedVirtualMachines = migrationPlan.spec.virtualmachines?.[0]?.filter(
          vm => !vmsToRemove.has(vm)
        )

        await patchMigrationPlan(planId, {
          spec: {
            virtualmachines: [updatedVirtualMachines]
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
          <Tab label="Agents" />
          <Tab label="Credentials" />
        </Tabs>

        {activeTab === 0 ? (
          <MigrationsTable
            refetchMigrations={refetchMigrations}
            migrations={migrations || []}
            onDeleteMigration={handleDeleteClick}
            onDeleteSelected={handleDeleteSelected}
          />
        ) : activeTab === 1 ? (
          <NodesTable />
        ) : (
          <CredentialsTable />
        )}
      </StyledPaper>

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={handleDeleteClose}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={selectedMigrations.length > 1
          ? "Are you sure you want to delete these migrations?"
          : `Are you sure you want to delete migration "${selectedMigrations[0]?.metadata.name}"?`
        }
        items={selectedMigrations.map(m => ({
          id: m.metadata.name,
          name: m.metadata.name
        }))}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={() => handleDeleteMigration(selectedMigrations)}
        customErrorMessage={getCustomErrorMessage}
        errorMessage={deleteError}
        onErrorChange={setDeleteError}
      />
    </DashboardContainer>
  )
}
