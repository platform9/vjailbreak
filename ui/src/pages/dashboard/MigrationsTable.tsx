import { DataGrid, GridColDef, GridRowSelectionModel, GridToolbarContainer } from "@mui/x-data-grid";
import { Button, Typography, Box, IconButton, Tooltip } from "@mui/material";
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import { useState } from "react";
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import MigrationProgressWithPopover from "./MigrationProgressWithPopover";
import { Migration } from "src/api/migrations/model";

// Move the STATUS_ORDER and columns from Dashboard.tsx to here
const STATUS_ORDER = {
    'Running': 0,
    'Failed': 1,
    'Succeeded': 2,
    'Pending': 3
}

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
        sortComparator: (v1, v2) => {
            const order1 = STATUS_ORDER[v1] ?? Number.MAX_SAFE_INTEGER;
            const order2 = STATUS_ORDER[v2] ?? Number.MAX_SAFE_INTEGER;
            return order1 - order2;
        }
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
];

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

interface MigrationsTableProps {
    migrations: Migration[];
    onDeleteMigration: (name: string) => void;
    onDeleteSelected: (migrations: Migration[]) => void;
}

export default function MigrationsTable({
    migrations,
    onDeleteMigration,
    onDeleteSelected
}: MigrationsTableProps) {
    const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([]);

    const handleSelectionChange = (newSelection: GridRowSelectionModel) => {
        setSelectedRows(newSelection);
    };

    const migrationsWithActions = migrations?.map(migration => ({
        ...migration,
        onDelete: onDeleteMigration
    })) || [];

    const isRowSelectable = (params) => {
        const phase = params.row?.status?.phase;
        return !(!phase || phase === "Running" || phase === "Pending");
    };

    return (
        <DataGrid
            rows={migrationsWithActions}
            columns={columns}
            initialState={{
                pagination: { paginationModel: { page: 0, pageSize: 25 } },
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
                        onDeleteSelected={() => {
                            const selectedMigrations = migrations?.filter(
                                m => selectedRows.includes(m.metadata?.name)
                            );
                            onDeleteSelected(selectedMigrations || []);
                        }}
                    />
                ),
            }}
        />
    );
} 