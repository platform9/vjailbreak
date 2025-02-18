import { Paper, styled, IconButton, Tooltip, Button, Box, Typography } from "@mui/material"
import { DataGrid, GridColDef, GridRowSelectionModel, GridToolbarContainer } from "@mui/x-data-grid"
import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import { FIVE_SECONDS, THIRTY_SECONDS } from "src/constants"
import { useMigrationsQuery } from "src/hooks/api/useMigrationsQuery"
import { deleteMigration } from "src/api/migrations/migrations"
import { useQueryClient } from "@tanstack/react-query"
import { MIGRATIONS_QUERY_KEY } from "src/hooks/api/useMigrationsQuery"
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import DeleteConfirmationDialog from "./DeleteConfirmationDialog"
import { getMigrationPlan, patchMigrationPlan } from "src/api/migration-plans/migrationPlans"
import { Migration } from "src/api/migrations/model"
import { Phase } from "src/api/migrations/model"
import MigrationProgress from "./MigrationProgress"

const STATUS_ORDER = {
  'Running': 0,
  'Failed': 1,
  'Succeeded': 2,
  'Pending': 3
}

const PHASE_STEPS = {
  [Phase.Pending]: 1,
  [Phase.Validating]: 2,
  [Phase.AwaitingDataCopyStart]: 3,
  [Phase.CopyingBlocks]: 4,
  [Phase.CopyingChangedBlocks]: 5,
  [Phase.ConvertingDisk]: 6,
  [Phase.AwaitingCutOverStartTime]: 7,
  [Phase.AwaitingAdminCutOver]: 8,
  [Phase.Succeeded]: 9,
  [Phase.Failed]: 9,
}

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

const getProgressText = (phase: Phase | undefined, conditions: Condition[] | undefined) => {
  if (!phase || phase === Phase.Unknown) {
    return "Unknown Status";
  }

  const stepNumber = PHASE_STEPS[phase] || 0;
  const totalSteps = 9;

  // Get the most recent condition's message
  const latestCondition = conditions?.sort((a, b) =>
    new Date(b.lastTransitionTime).getTime() - new Date(a.lastTransitionTime).getTime()
  )[0];

  const message = latestCondition?.message || phase;

  if (phase === Phase.Failed || phase === Phase.Succeeded) {
    return `${phase} - ${message}`;
  }

  return `STEP ${stepNumber}/${totalSteps}: ${phase} - ${message}`;
}

const columns: GridColDef[] = [
  {
    field: "name",
    headerName: "Name",
    valueGetter: (_, row) => row.metadata?.name,
    flex: 1.5,
  },
  {
    field: "status",
    headerName: "Status",
    valueGetter: (_, row) => row?.status?.phase || "Pending",
    flex: 1,
    sortComparator: (v1, v2) => {
      const order1 = STATUS_ORDER[v1] ?? Number.MAX_SAFE_INTEGER;
      const order2 = STATUS_ORDER[v2] ?? Number.MAX_SAFE_INTEGER;
      return order1 - order2;
    }
  },
  {
    field: "status.conditions",
    headerName: "Progress",
    valueGetter: (_, row) => getProgressText(row.status?.phase, row.status?.conditions),
    flex: 3,
    renderCell: (params) => {
      const phase = params.row?.status?.phase
      const conditions = params.row?.status?.conditions
      return conditions ? (
        <MigrationProgress
          phase={phase}
          progressText={getProgressText(phase, conditions)}
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
      const isDisabled = !phase || phase === "Running" || phase === "Pending";

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

interface CustomToolbarProps {
  numSelected: number;
  onDeleteSelected: () => void;
}

const CustomToolbar = ({ numSelected, onDeleteSelected }: CustomToolbarProps) => {
  return (
    <GridToolbarContainer
      sx={{
        p: 2,
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}
    >
      <div>
        <Typography variant="h6" component="h2">
          Migrations
        </Typography>
      </div>
      <Box sx={{ display: 'flex', gap: 2 }}>
        {numSelected > 0 && (
          <Button
            variant="outlined"
            color="error"
            startIcon={<DeleteIcon />}
            onClick={onDeleteSelected}
            sx={{ height: 40 }}
          >
            Delete Selected ({numSelected})
          </Button>
        )}
        <CustomSearchToolbar
          placeholder="Search by Name, Status, or Progress"
        />
      </Box>
    </GridToolbarContainer>
  );
};

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

  const migrationsWithActions = migrations?.map(migration => ({
    ...migration,
    onDelete: handleDeleteClick
  })) || []

  const isRowSelectable = (params) => {
    const phase = params.row?.status?.phase;
    return !(!phase || phase === "Running" || phase === "Pending");
  };

  return (
    <DashboardContainer>
      <StyledPaper>
        <DataGrid
          rows={migrationsWithActions}
          columns={columns}
          initialState={{
            pagination: { paginationModel },
            sorting: {
              sortModel: [{ field: 'status', sort: 'asc' }],
            },
          }}
          pageSizeOptions={[25, 50, 100]}
          localeText={{ noRowsLabel: "No Migrations Available" }}
          getRowId={(row) => row.metadata?.name}
          checkboxSelection
          isRowSelectable={isRowSelectable}
          onRowSelectionModelChange={handleSelectionChange}
          rowSelectionModel={selectedRows}
          slots={{
            toolbar: () => (
              <CustomToolbar
                numSelected={selectedRows.length}
                onDeleteSelected={handleDeleteSelected}
              />
            ),
          }}
        />
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
