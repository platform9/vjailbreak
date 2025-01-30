import { Paper, styled, IconButton, Tooltip } from "@mui/material"
import { DataGrid, GridColDef } from "@mui/x-data-grid"
import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import { FIVE_SECONDS, THIRTY_SECONDS } from "src/constants"
import { useMigrationsQuery } from "src/hooks/api/useMigrationsQuery"
import MigrationProgressWithPopover from "./MigrationProgressWithPopover"
import { deleteMigration } from "src/api/migrations/migrations"
import { useQueryClient } from "@tanstack/react-query"
import { MIGRATIONS_QUERY_KEY } from "src/hooks/api/useMigrationsQuery"
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import DeleteConfirmationDialog from "./DeleteConfirmationDialog"
import { deleteMigrationPlan } from "src/api/migration-plans/migrationPlans"
import { deleteMigrationTemplate } from "src/api/migration-templates/migrationTemplates"

const DashboardContainer = styled("div")({
  display: "flex",
  justifyContent: "center",
  width: "100%",
  marginTop: "40px",
})


const columns: GridColDef[] = [
  {
    field: "name",
    headerName: "Name",
    valueGetter: (_, row) => row.metadata?.name,
    flex: 2,
  },
  {
    field: "status",
    headerName: "Status",
    valueGetter: (_, row) => row?.status?.phase || "Pending",
    flex: 1,
  },
  {
    field: "status.conditions",
    headerName: "Progress",
    valueGetter: (_, row) => row.status?.phase,
    flex: 2,
    renderCell: (params) => {
      const phase = params.row?.status?.phase
      const conditions = params.row?.status?.conditions
      return conditions ? (
        <MigrationProgressWithPopover
          phase={phase}
          conditions={params.row?.status?.conditions}
        />
      ) : null
    },
  },
  {
    field: "actions",
    headerName: "Actions",
    flex: 1,
    renderCell: (params) => {
      const phase = params.row?.status?.phase;
      const isDisabled = phase === "Running" || phase === "Pending";

      return (
        <Tooltip title={isDisabled ? "Cannot delete while migration is in progress" : "Delete migration"} >
          <span>
            <IconButton
              onClick={(e) => {
                e.stopPropagation();
                params.row.onDelete(params.row.metadata?.name);
              }}
              disabled={isDisabled}
              size="small"
              sx={{
                cursor: isDisabled ? 'not-allowed' : 'pointer',
                position: 'relative'
              }}
            >
              <DeleteIcon />
            </IconButton>
          </span>
        </Tooltip>
      );
    },
  },
]

const paginationModel = { page: 0, pageSize: 25 }

export default function Dashboard() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [deleteDialog, setDeleteDialog] = useState<{ open: boolean, migrationName: string | null }>({
    open: false,
    migrationName: null
  });
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const { data: migrations } = useMigrationsQuery(undefined, {
    refetchInterval: (query) => {
      const migrations = query?.state?.data || []
      const hasPendingMigration = !!migrations.find(
        (m) => m.status === undefined
      )
      return hasPendingMigration ? FIVE_SECONDS : THIRTY_SECONDS
    },
  })

  const handleDeleteClick = (migrationName: string) => {
    setDeleteError(null);
    setDeleteDialog({
      open: true,
      migrationName
    });
  };

  const handleDeleteClose = () => {
    setDeleteError(null);
    setDeleteDialog({
      open: false,
      migrationName: null
    });
  };

  const handleDeleteConfirm = async () => {
    if (deleteDialog.migrationName) {
      try {
        setIsDeleting(true);
        setDeleteError(null);

        // Get the migration to find related resources
        const migration = migrations?.find(m => m.metadata?.name === deleteDialog.migrationName);
        if (!migration) {
          throw new Error('Migration not found');
        }

        // Find and delete the migration plan
        const migrationPlanName = migration.metadata?.labels?.migrationplan;
        if (migrationPlanName) {
          try {
            await deleteMigrationPlan(migrationPlanName);
          } catch (error) {
            console.error('Failed to delete migration plan:', error);
          }
        }

        // Find and delete the migration template
        const migrationTemplateName = migration.metadata?.labels?.migrationtemplate;
        if (migrationTemplateName) {
          try {
            await deleteMigrationTemplate(migrationTemplateName);
          } catch (error) {
            console.error('Failed to delete migration template:', error);
          }
        }


        // Delete migration first
        await deleteMigration(deleteDialog.migrationName);

        // Refresh the migrations list
        queryClient.invalidateQueries({ queryKey: MIGRATIONS_QUERY_KEY });
        handleDeleteClose();
      } catch (error) {
        console.error('Failed to delete migration:', error);
        setDeleteError(error instanceof Error ? error.message : 'Failed to delete migration and related resources');
      } finally {
        setIsDeleting(false);
      }
    }
  };

  useEffect(() => {
    if (!!migrations && migrations.length === 0) {
      navigate("/onboarding")
    }
  }, [migrations, navigate])

  const migrationsWithActions = migrations?.map(migration => ({
    ...migration,
    onDelete: handleDeleteClick
  })) || []

  return (
    <DashboardContainer>
      <Paper sx={{ width: "95%", margin: "auto" }}>
        <DataGrid
          rows={migrationsWithActions}
          columns={columns}
          initialState={{ pagination: { paginationModel } }}
          pageSizeOptions={[25, 50, 100]}
          localeText={{ noRowsLabel: "No Migrations Available" }}
          getRowId={(row) => row.metadata?.name}
          slots={{
            toolbar: () => <CustomSearchToolbar title="Migrations" placeholder="Search by Name, Status, or Progress" />,
          }}
        />
      </Paper>
      <DeleteConfirmationDialog
        open={deleteDialog.open}
        migrationName={deleteDialog.migrationName}
        onClose={handleDeleteClose}
        onConfirm={handleDeleteConfirm}
        isDeleting={isDeleting}
        error={deleteError}
      />
    </DashboardContainer>
  )
}
