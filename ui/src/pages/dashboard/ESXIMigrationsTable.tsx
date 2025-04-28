import {
    DataGrid,
    GridColDef,
    GridRowSelectionModel,
    GridToolbarContainer
} from "@mui/x-data-grid";
import {
    Button,
    Typography,
    Box,
    IconButton,
    Tooltip
} from "@mui/material";
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import { useState } from "react";
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import { ESXIMigration } from "src/api/esximigrations/model";
import { QueryObserverResult } from "@tanstack/react-query";
import { RefetchOptions } from "@tanstack/react-query";
import {
    getESXiName,
    getOpenstackCredsRefName,
    getVMwareCredsRefName
} from "src/api/esximigrations/helper";

const columns: GridColDef[] = [
    {
        field: "name",
        headerName: "Name",
        valueGetter: (_, row) => row.metadata?.name,
        flex: 1.5,
    },
    {
        field: "esxiName",
        headerName: "ESXi Host",
        valueGetter: (_, row) => getESXiName(row),
        flex: 1.5,
    },
    {
        field: "vmwareCredsRef",
        headerName: "VMware Credentials",
        valueGetter: (_, row) => getVMwareCredsRefName(row),
        flex: 1.5,
    },
    {
        field: "openstackCredsRef",
        headerName: "OpenStack Credentials",
        valueGetter: (_, row) => getOpenstackCredsRefName(row),
        flex: 1.5,
    },
    {
        field: "actions",
        headerName: "Actions",
        flex: 1,
        renderCell: (params) => {
            return (
                <Tooltip title="Delete ESXi migration">
                    <IconButton
                        onClick={(e) => {
                            e.stopPropagation();
                            params.row.onDelete(params.row.metadata?.name);
                        }}
                        size="small"
                    >
                        <DeleteIcon />
                    </IconButton>
                </Tooltip>
            );
        },
    },
]

interface CustomToolbarProps {
    numSelected: number;
    onDeleteSelected: () => void;
    refetchESXIMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<ESXIMigration[], Error>>;
}

const CustomToolbar = ({ numSelected, onDeleteSelected, refetchESXIMigrations }: CustomToolbarProps) => {
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
                    ESXi Migrations
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
                    placeholder="Search by Name or ESXi Host"
                    onRefresh={refetchESXIMigrations}
                />
            </Box>
        </GridToolbarContainer>
    );
};

interface ESXIMigrationsTableProps {
    esxiMigrations: ESXIMigration[];
    onDeleteESXIMigration: (name: string) => void;
    onDeleteSelected: (esxiMigrations: ESXIMigration[]) => void;
    refetchESXIMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<ESXIMigration[], Error>>;
}

export default function ESXIMigrationsTable({
    esxiMigrations,
    onDeleteESXIMigration,
    onDeleteSelected,
    refetchESXIMigrations
}: ESXIMigrationsTableProps) {
    const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([]);

    const handleSelectionChange = (newSelection: GridRowSelectionModel) => {
        setSelectedRows(newSelection);
    };

    const esxiMigrationsWithActions = esxiMigrations?.map(esxiMigration => ({
        ...esxiMigration,
        onDelete: onDeleteESXIMigration
    })) || [];

    return (
        <DataGrid
            rows={esxiMigrationsWithActions}
            columns={columns}
            initialState={{
                pagination: { paginationModel: { page: 0, pageSize: 25 } },
            }}
            pageSizeOptions={[25, 50, 100]}
            localeText={{ noRowsLabel: "No ESXi Migrations Available" }}
            getRowId={(row) => row.metadata?.name}
            checkboxSelection
            onRowSelectionModelChange={handleSelectionChange}
            rowSelectionModel={selectedRows}
            slots={{
                toolbar: () => (
                    <CustomToolbar
                        numSelected={selectedRows.length}
                        onDeleteSelected={() => {
                            const selectedESXIMigrations = esxiMigrations?.filter(
                                m => selectedRows.includes(m.metadata?.name)
                            );
                            onDeleteSelected(selectedESXIMigrations || []);
                        }}
                        refetchESXIMigrations={refetchESXIMigrations}
                    />
                ),
            }}
        />
    );
} 