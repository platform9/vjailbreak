import {
    DataGrid,
    GridColDef,
    GridToolbarContainer,
    GridRowSelectionModel,
    GridToolbarProps,
} from "@mui/x-data-grid";
import {
    Button,
    Typography,
    Box,
    IconButton,
    Tooltip,
    Chip
} from "@mui/material";
import DeleteIcon from '@mui/icons-material/Delete';
import WarningIcon from '@mui/icons-material/Warning';
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import { useState, useCallback, useEffect } from "react";
import { useVmwareCredentialsQuery } from "src/hooks/api/useVmwareCredentialsQuery";
import { useOpenstackCredentialsQuery } from "src/hooks/api/useOpenstackCredentialsQuery";
import { VmwareCredential } from "src/components/forms/VmwareCredentialsForm";
import { OpenstackCredential } from "src/components/forms/OpenstackCredentialsForm";
import ConfirmationDialog from "src/components/dialogs/ConfirmationDialog";
import { useQueryClient } from "@tanstack/react-query";
import { VMWARE_CREDS_QUERY_KEY } from "src/hooks/api/useVmwareCredentialsQuery";
import { OPENSTACK_CREDS_QUERY_KEY } from "src/hooks/api/useOpenstackCredentialsQuery";
import { deleteVMwareCredsWithSecretFlow, deleteOpenStackCredsWithSecretFlow } from "src/api/helpers";

interface CredentialItem {
    id: string;
    name: string;
    type: 'VMware' | 'OpenStack';
    status: string;
    credObject: VmwareCredential | OpenstackCredential;
}

const columns: GridColDef[] = [
    {
        field: "name",
        headerName: "Name",
        flex: 1,
    },
    {
        field: "type",
        headerName: "Type",
        flex: 1,
        renderCell: (params) => (
            <Chip
                label={params.value}
                color={params.value === 'VMware' ? 'primary' : 'secondary'}
                variant="outlined"
                size="small"
            />
        ),
    },
    {
        field: "status",
        headerName: "Status",
        flex: 1,
        renderCell: (params) => (
            <Chip
                label={params.value}
                variant="outlined"
                color={params.value === 'Succeeded' ? 'success' : 'error'}
                size="small"
            />
        ),
    },
    {
        field: 'actions',
        headerName: 'Actions',
        flex: 1,
        width: 100,
        sortable: false,
        renderCell: (params) => (
            <Tooltip title="Delete credential">
                <IconButton
                    onClick={(e) => {
                        e.stopPropagation();
                        if (params.row.onDelete) {
                            params.row.onDelete(params.row.id, params.row.type);
                        }
                    }}
                    size="small"
                    color="error"
                    aria-label="delete credential"
                >
                    <DeleteIcon />
                </IconButton>
            </Tooltip>
        ),
    },
];

// Define a type that extends GridToolbarProps with our custom props
interface CustomToolbarProps {
    selectedCount?: number;
    onDeleteSelected?: () => void;
    loading?: boolean;
    onRefresh?: () => void;
}

// Custom toolbar component
const CustomToolbar = (props: GridToolbarProps) => {
    // Access our custom props from the component's context
    const { selectedCount = 0, onDeleteSelected, loading = false, onRefresh } =
        props.getSlotsParameters?.() as CustomToolbarProps || {};

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
                    Credentials
                </Typography>
            </div>
            <Box sx={{ display: 'flex', gap: 2 }}>
                {selectedCount > 0 && (
                    <Button
                        variant="outlined"
                        color="error"
                        startIcon={<DeleteIcon />}
                        onClick={onDeleteSelected}
                        disabled={loading}
                    >
                        Delete Selected ({selectedCount})
                    </Button>
                )}
                <CustomSearchToolbar
                    placeholder="Search by Name or Type"
                    onRefresh={onRefresh}
                />
            </Box>
        </GridToolbarContainer>
    );
};

export default function CredentialsTable() {
    const queryClient = useQueryClient();

    // Fetch credentials with options to always get fresh data
    const { data: vmwareCredentials, isLoading: loadingVmware, refetch: refetchVmware } = useVmwareCredentialsQuery(
        undefined,
        {
            staleTime: 0,
            refetchOnMount: true
        }
    );

    const { data: openstackCredentials, isLoading: loadingOpenstack, refetch: refetchOpenstack } = useOpenstackCredentialsQuery(
        undefined,
        {
            staleTime: 0,
            refetchOnMount: true
        }
    );

    const [selectedIds, setSelectedIds] = useState<string[]>([]);
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
    const [selectedForDeletion, setSelectedForDeletion] = useState<CredentialItem[]>([]);
    const [deleteError, setDeleteError] = useState<string | null>(null);
    const [deleting, setDeleting] = useState(false);

    // Force refetch when the component mounts
    useEffect(() => {
        refetchVmware();
        refetchOpenstack();
    }, [refetchVmware, refetchOpenstack]);

    const handleRefresh = useCallback(() => {
        refetchVmware();
        refetchOpenstack();
    }, [refetchVmware, refetchOpenstack]);

    // Handle deletion of a credential
    const handleDeleteCredential = (id: string, type: 'VMware' | 'OpenStack') => {
        const credential = type === 'VMware'
            ? vmwareCredentials?.find(cred => cred.metadata.name === id)
            : openstackCredentials?.find(cred => cred.metadata.name === id);

        if (credential) {
            const credItem: CredentialItem = {
                id,
                name: credential.metadata.name,
                type,
                status: type === 'VMware'
                    ? (credential as VmwareCredential).status?.vmwareValidationStatus || 'Unknown'
                    : (credential as OpenstackCredential).status?.openstackValidationStatus || 'Unknown',
                credObject: credential
            };
            setSelectedForDeletion([credItem]);
            setDeleteDialogOpen(true);
        }
    };

    const handleSelectionChange = (rowSelectionModel: GridRowSelectionModel) => {
        setSelectedIds(rowSelectionModel as string[]);
    };

    const handleDeleteSelected = () => {
        const selectedCreds = allCredentials.filter(cred => selectedIds.includes(cred.id));
        setSelectedForDeletion(selectedCreds);
        setDeleteDialogOpen(true);
    };

    const handleDeleteClose = () => {
        setDeleteDialogOpen(false);
        setSelectedForDeletion([]);
        setDeleteError(null);
    };

    const handleConfirmDelete = async () => {
        setDeleting(true);
        try {
            // Group credentials by type for batch deletion
            const vmwareCreds = selectedForDeletion.filter(cred => cred.type === 'VMware');
            const openstackCreds = selectedForDeletion.filter(cred => cred.type === 'OpenStack');

            // Delete VMware credentials with their secrets
            await Promise.all(
                vmwareCreds.map(cred => deleteVMwareCredsWithSecretFlow(cred.id))
            );

            // Delete OpenStack credentials with their secrets
            await Promise.all(
                openstackCreds.map(cred => deleteOpenStackCredsWithSecretFlow(cred.id))
            );

            // Refresh data
            queryClient.invalidateQueries({ queryKey: VMWARE_CREDS_QUERY_KEY });
            queryClient.invalidateQueries({ queryKey: OPENSTACK_CREDS_QUERY_KEY });

            // Clear selections
            setSelectedIds([]);
            handleDeleteClose();
        } catch (error) {
            console.error("Error deleting credentials:", error);
            setDeleteError(error instanceof Error ? error.message : "Unknown error occurred");
        } finally {
            setDeleting(false);
        }
    };

    const getCustomErrorMessage = useCallback((error: Error | string) => {
        const baseMessage = "Failed to delete credentials";
        if (error instanceof Error) {
            return `${baseMessage}: ${error.message}`;
        }
        return `${baseMessage}: ${error}`;
    }, []);

    // Transform VMware credentials to the common format
    const vmwareItems: CredentialItem[] = vmwareCredentials?.map((cred: VmwareCredential) => ({
        id: cred.metadata.name,
        name: cred.metadata.name,
        type: 'VMware' as const,
        status: cred.status?.vmwareValidationStatus || 'Unknown',
        credObject: cred,
    })) || [];

    // Transform OpenStack credentials to the common format
    const openstackItems: CredentialItem[] = openstackCredentials?.map((cred: OpenstackCredential) => ({
        id: cred.metadata.name,
        name: cred.metadata.name,
        type: 'OpenStack' as const,
        status: cred.status?.openstackValidationStatus || 'Unknown',
        credObject: cred,
    })) || [];

    // Combine both credential types
    const allCredentials = [...vmwareItems, ...openstackItems];

    const rowsWithActions = allCredentials.map(row => ({
        ...row,
        onDelete: handleDeleteCredential
    }));

    const isLoading = loadingVmware || loadingOpenstack || deleting;

    return (
        <div style={{ height: 'calc(100vh - 180px)', width: '100%', overflow: 'hidden' }}>
            <DataGrid
                rows={rowsWithActions}
                columns={columns}
                disableRowSelectionOnClick
                checkboxSelection
                rowSelectionModel={selectedIds}
                onRowSelectionModelChange={handleSelectionChange}
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
                    toolbar: CustomToolbar,
                }}
                slotProps={{
                    toolbar: {
                        selectedCount: selectedIds.length,
                        onDeleteSelected: handleDeleteSelected,
                        loading: isLoading,
                        onRefresh: handleRefresh,
                    },
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

            <ConfirmationDialog
                open={deleteDialogOpen}
                onClose={handleDeleteClose}
                title="Confirm Delete"
                icon={<WarningIcon color="warning" />}
                message={selectedForDeletion.length > 1
                    ? "Are you sure you want to delete these credentials?"
                    : `Are you sure you want to delete ${selectedForDeletion[0]?.type} credential "${selectedForDeletion[0]?.name}"?`
                }
                items={selectedForDeletion.map(cred => ({
                    id: cred.id,
                    name: cred.name
                }))}
                actionLabel="Delete"
                actionColor="error"
                actionVariant="outlined"
                onConfirm={handleConfirmDelete}
                customErrorMessage={getCustomErrorMessage}
                errorMessage={deleteError}
                onErrorChange={setDeleteError}
            />
        </div>
    );
}