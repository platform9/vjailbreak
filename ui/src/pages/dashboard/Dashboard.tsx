import { Paper, styled, IconButton, Tooltip, Button, Box, Typography, Tab, Tabs } from "@mui/material"
import { DataGrid, GridColDef, GridRowSelectionModel, GridToolbarContainer } from "@mui/x-data-grid"
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
import { getMigrationPlan, patchMigrationPlan } from "src/api/migration-plans/migrationPlans"
import { Migration } from "src/api/migrations/model"
import MigrationsTable from "./MigrationsTable"
import NodesTable from "./NodesTable"

const DashboardContainer = styled("div")({
  display: "flex",
  justifyContent: "center",
  width: "100%",
  height: "100%",
  padding: "40px 20px",
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
  const [deleteDialog, setDeleteDialog] = useState<{ open: boolean, migrationName: string | null, selectedMigrations?: Migration[] }>({
    open: false,
    migrationName: null
  });
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([]);
  const [activeTab, setActiveTab] = useState(0);

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

  const handleSelectionChange = (newSelection: GridRowSelectionModel) => {
    setSelectedRows(newSelection);
  };

  const handleDeleteSelected = () => {
    const selectedMigrations = migrations?.filter(
      m => selectedRows.includes(m.metadata?.name)
    );
    if (!selectedMigrations?.length) return;

    setDeleteDialog({
      open: true,
      migrationName: null,
      selectedMigrations
    });
  };

  const handleDeleteMigration = async (migrations: Migration[]) => {
    try {
      // Group VMs by migration plan
      const migrationPlanUpdates = migrations.reduce((acc, migration) => {
        const planId = migration.spec.migrationPlan;
        if (!acc[planId]) {
          acc[planId] = {
            vmsToRemove: new Set<string>(),
            migrationsToDelete: new Set<string>()
          };
        }
        acc[planId].vmsToRemove.add(migration.spec.vmName);
        acc[planId].migrationsToDelete.add(migration.metadata.name);
        return acc;
      }, {} as Record<string, { vmsToRemove: Set<string>, migrationsToDelete: Set<string> }>);

      // Update each migration plan once
      await Promise.all(
        Object.entries(migrationPlanUpdates).map(async ([planId, { vmsToRemove, migrationsToDelete }]) => {
          const migrationPlan = await getMigrationPlan(planId);
          const updatedVirtualMachines = migrationPlan.spec.virtualmachines[0].filter(
            vm => !vmsToRemove.has(vm)
          );

          await patchMigrationPlan(planId, {
            spec: {
              virtualmachines: [updatedVirtualMachines]
            }
          });

          // Delete all migrations for this plan
          await Promise.all(
            Array.from(migrationsToDelete).map(migrationName =>
              deleteMigration(migrationName)
            )
          );
        })
      );

    } catch (error) {
      console.error("Error removing VMs from migration plan", error);
      throw error;
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deleteDialog.selectedMigrations?.length) return;

    setIsDeleting(true);
    setDeleteError(null);

    try {
      await handleDeleteMigration(deleteDialog.selectedMigrations);
      queryClient.invalidateQueries({ queryKey: MIGRATIONS_QUERY_KEY });
    } catch (error) {
      setDeleteError(
        error instanceof Error ? error.message : 'Failed to delete migrations'
      );
    } finally {
      setIsDeleting(false);
      handleDeleteClose();
      setSelectedRows([]);
    }
  };

  useEffect(() => {
    if (!!migrations && migrations.length === 0) {
      navigate("/onboarding")
    }
  }, [migrations, navigate])

  const handleTabChange = (event: React.SyntheticEvent, newValue: number) => {
    setActiveTab(newValue);
  };

  return (
    <DashboardContainer>
      <StyledPaper>
        <Tabs
          value={activeTab}
          onChange={handleTabChange}
          sx={{ borderBottom: 1, borderColor: 'divider' }}
        >
          <Tab label="Migrations" />
          <Tab label="Nodes" />
        </Tabs>

        {activeTab === 0 ? (
          <MigrationsTable
            migrations={migrations || []}
            onDeleteMigration={handleDeleteClick}
            onDeleteSelected={(selectedMigrations) => {
              setDeleteDialog({
                open: true,
                migrationName: null,
                selectedMigrations
              });
            }}
          />
        ) : (
          <NodesTable />
        )}
      </StyledPaper>

      <DeleteConfirmationDialog
        open={deleteDialog.open}
        migrationName={deleteDialog.migrationName}
        selectedMigrations={deleteDialog.selectedMigrations}
        onClose={handleDeleteClose}
        onConfirm={handleDeleteConfirm}
        isDeleting={isDeleting}
        error={deleteError}
      />
    </DashboardContainer>
  )
}
