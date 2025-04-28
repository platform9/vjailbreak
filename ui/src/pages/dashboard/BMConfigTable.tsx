import {
    DataGrid,
    GridColDef,
    GridToolbarContainer,
    GridActionsCellItem,
    GridRowParams,
    GridLoadingOverlay
} from "@mui/x-data-grid";
import {
    Button,
    Typography,
    Box,
    Chip
} from "@mui/material";
import AddIcon from '@mui/icons-material/Add';
import InfoIcon from '@mui/icons-material/Info';
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import { useState, useCallback, useEffect } from "react";
import { useBMConfigsQuery } from "src/hooks/api/useBMConfigQuery";
import BMConfigDetailsModal from "./BMConfigDetailsModal";
import { isBMConfigValid } from "src/api/bmconfig/helpers";

interface CustomToolbarProps {
    onRefresh: () => void;
    onAddBMConfig: () => void;
}

const CustomToolbar = ({
    onRefresh,
    onAddBMConfig
}: CustomToolbarProps) => {
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
                    Bare Metal Configurations
                </Typography>
            </div>
            <Box sx={{ display: 'flex', gap: 2 }}>
                <Button
                    variant="outlined"
                    color="primary"
                    startIcon={<AddIcon />}
                    onClick={onAddBMConfig}
                    sx={{ height: 40 }}
                >
                    Add BMConfig
                </Button>
                <CustomSearchToolbar
                    placeholder="Search by Name"
                    onRefresh={onRefresh}
                />
            </Box>
        </GridToolbarContainer>
    );
};

export default function BMConfigTable({ onAddBMConfig }) {
    const { data: bmConfigs, isLoading, refetch } = useBMConfigsQuery(
        undefined,
        {
            staleTime: 0,
            refetchOnMount: true
        }
    );

    const [detailsModalOpen, setDetailsModalOpen] = useState(false);
    const [selectedConfig, setSelectedConfig] = useState<string | null>(null);

    useEffect(() => {
        refetch();
    }, [refetch]);

    const handleRefresh = useCallback(() => {
        refetch();
    }, [refetch]);

    const handleViewDetails = (configName: string) => {
        setSelectedConfig(configName);
        setDetailsModalOpen(true);
    };

    const handleCloseDetailsModal = () => {
        setDetailsModalOpen(false);
        setSelectedConfig(null);
    };

    const columns: GridColDef[] = [
        {
            field: "name",
            headerName: "Name",
            flex: 1,
        },
        {
            field: "providerType",
            headerName: "Provider Type",
            flex: 1,
        },
        {
            field: "os",
            headerName: "OS",
            flex: 1,
        },
        {
            field: "status",
            headerName: "Status",
            flex: 1,
            renderCell: (params) => (
                <Chip
                    label={params.value}
                    variant="outlined"
                    color={params.value === 'validated' || params.value === 'valid' ? 'success' : 'error'}
                    size="small"
                />
            ),
        },
        {
            field: 'actions',
            type: 'actions',
            headerName: 'Actions',
            width: 100,
            getActions: (params: GridRowParams) => [
                <GridActionsCellItem
                    icon={<InfoIcon />}
                    label="View Details"
                    onClick={() => handleViewDetails(params.row.name)}
                />
            ],
        },
    ];

    const rows = bmConfigs ? bmConfigs.map(config => ({
        id: config.metadata.name,
        name: config.metadata.name,
        providerType: config.spec.providerType || 'Unknown',
        os: config.spec.os || 'Not specified',
        status: config.status?.validationStatus || 'Unknown',
        isValid: isBMConfigValid(config)
    })) : [];

    return (
        <div style={{ height: 'calc(100vh - 180px)', width: '100%', overflow: 'hidden' }}>
            <DataGrid
                rows={rows}
                columns={columns}
                loading={isLoading}
                disableRowSelectionOnClick
                initialState={{
                    sorting: {
                        sortModel: [{ field: 'name', sort: 'asc' }],
                    },
                    pagination: {
                        paginationModel: {
                            pageSize: 25,
                        },
                    },
                }}
                slots={{
                    toolbar: () => (
                        <CustomToolbar
                            onRefresh={handleRefresh}
                            onAddBMConfig={onAddBMConfig}
                        />
                    ),
                    loadingOverlay: GridLoadingOverlay
                }}
                pageSizeOptions={[10, 25, 50, 100]}
                sx={{
                    '& .MuiDataGrid-main': {
                        overflow: 'auto'
                    },
                    border: 1,
                    borderColor: 'divider',
                    '& .MuiDataGrid-cell:focus': {
                        outline: 'none',
                    },
                }}
            />

            {selectedConfig && (
                <BMConfigDetailsModal
                    open={detailsModalOpen}
                    onClose={handleCloseDetailsModal}
                    configName={selectedConfig}
                />
            )}
        </div>
    );
} 