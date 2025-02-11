import {
    DataGrid,
    GridColDef,
    GridToolbarContainer,
    GridRowParams
} from "@mui/x-data-grid";
import {
    Button,
    Typography,
    Box,
    IconButton,
    Tooltip,
    Alert,
    Snackbar
} from "@mui/material";
import AddIcon from '@mui/icons-material/Add';
import RemoveIcon from '@mui/icons-material/Remove';
import WarningIcon from '@mui/icons-material/Warning';
import RemoveCircleOutlineIcon from '@mui/icons-material/RemoveCircleOutline';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import HelpIcon from '@mui/icons-material/Help';
import ErrorIcon from '@mui/icons-material/Error';
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import { useState } from "react";
import ScaleUpDrawer from "./ScaleUpDrawer";
import ConfirmationDialog from "src/components/dialogs/ConfirmationDialog";
import { useNodesQuery } from "src/hooks/api/useNodesQuery"
import { deleteNode } from "src/api/nodes/nodeMappings";
import { useQueryClient } from "@tanstack/react-query";
import { NODES_QUERY_KEY } from "src/hooks/api/useNodesQuery";
import { NodeItem } from "src/api/nodes/model";

const columns: GridColDef[] = [
    {
        field: "name",
        headerName: "Name",
        flex: 2,
    },
    {
        field: 'status',
        headerName: 'Status',
        flex: 1,
        renderCell: (params) => (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                {params.value === 'Online' ? (
                    <CheckCircleIcon sx={{ color: 'success.main' }} />
                ) : params.value === 'Offline' ? (
                    <ErrorIcon sx={{ color: 'warning.main' }} />
                ) : (
                    <HelpIcon />
                )}
                {params.value}
            </Box>
        ),
    },
    {
        field: "phase",
        headerName: "Phase",
        flex: 1,
    },
    {
        field: "role",
        headerName: "Role",
        flex: 1,
        renderCell: (params) => (
            <Box sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                color: params.value === 'master' ? 'primary.main' : 'text.primary'
            }}>
                {params.value === 'master' ? 'Master' : params.value === 'worker' ? 'Worker' : 'Unknown'}
            </Box>
        ),
    },
    {
        field: "ipAddress",
        headerName: "IP Address",
        flex: 2,
    },
    {
        field: 'actions',
        headerName: 'Actions',
        flex: 1,
        width: 100,
        sortable: false,
        renderCell: (params) => (
            <Tooltip title={params.row.role === 'master' ? "Master node cannot be scaled down" : "Scale down node"}>
                <span> {/* Wrapper for disabled button tooltip */}
                    <IconButton
                        onClick={(e) => {
                            e.stopPropagation();
                            params.row.onDelete(params.row.metadata?.name);
                        }}
                        size="small"
                        color="warning"
                        aria-label="scale down node"
                        disabled={params.row.role === 'master'}
                    >
                        <RemoveCircleOutlineIcon />
                    </IconButton>
                </span>
            </Tooltip>
        ),
    },
];

interface NodesToolbarProps {
    onScaleUp: () => void;
    onScaleDown: () => void;
    disableScaleDown: boolean;
    loading: boolean;
    selectedCount: number;
    totalNodes: number;
}
interface NodeSelector {
    id: string
    name: string
    status: string
    ipAddress: string
    role: string
}
const NodesToolbar = ({
    onScaleUp,
    onScaleDown,
    disableScaleDown,
    loading,
    selectedCount,
    totalNodes
}: NodesToolbarProps) => {
    const getScaleDownTooltip = () => {
        if (loading) return "Operation in progress";
        if (selectedCount === 0) return "Select nodes to scale down";
        if (selectedCount === totalNodes) return "Cannot scale down all nodes. At least one node must remain";
        return "";
    };

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
                    Nodes
                </Typography>
            </div>
            <Box sx={{ display: 'flex', gap: 2 }}>
                <Tooltip title={loading ? "Operation in progress" : ""}>
                    <span> {/* Wrapper needed for disabled button tooltip */}
                        <Button
                            variant="outlined"
                            color="primary"
                            startIcon={<AddIcon />}
                            onClick={onScaleUp}
                            disabled={loading}
                        >
                            Scale Up
                        </Button>
                    </span>
                </Tooltip>
                <Tooltip title={getScaleDownTooltip()}>
                    <span> {/* Wrapper needed for disabled button tooltip */}
                        <Button
                            variant="outlined"
                            color="primary"
                            startIcon={<RemoveIcon />}
                            onClick={onScaleDown}
                            disabled={disableScaleDown || loading}
                        >
                            Scale Down {selectedCount > 0 && `(${selectedCount})`}
                        </Button>
                    </span>
                </Tooltip>
                <CustomSearchToolbar
                    placeholder="Search by Name, Status, or IP"
                />
            </Box>
        </GridToolbarContainer>
    );
};

export default function NodesTable() {
    const { data: nodes, isLoading } = useNodesQuery();
    const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
    const [scaleUpOpen, setScaleUpOpen] = useState(false);
    const [scaleDownDialogOpen, setScaleDownDialogOpen] = useState(false);
    const [loading, setLoading] = useState(false);
    const [successMessage, setSuccessMessage] = useState<string | null>(null);
    const queryClient = useQueryClient();
    const [scaleDownError, setScaleDownError] = useState<string | null>(null);

    const masterNode = nodes?.find(node => node.spec.noderole === 'master') || null;

    const transformedNodes: NodeSelector[] = nodes?.map((node: NodeItem) => ({
        id: node.metadata.name,
        name: node.metadata.name,
        status: node.status?.status || 'Unknown',
        phase: node.status?.phase || 'Unknown',
        ipAddress: node.status?.vmip || '-',
        role: node.spec.noderole,
    })) || [];

    const handleSelectionChange = (newSelection) => {
        setSelectedNodes(newSelection);
    };


    const handleScaleUp = () => {
        setScaleUpOpen(true);
    };

    const handleScaleDown = async () => {
        setScaleDownDialogOpen(true);
    };

    const handleSingleNodeScaleDown = (node: NodeSelector) => {
        setSelectedNodes([node.name]);
        setScaleDownDialogOpen(true);
    };


    const confirmScaleDown = async () => {
        try {
            setLoading(true);
            setScaleDownError(null);

            // Delete nodes sequentially to handle errors better
            for (const nodeName of selectedNodes) {
                await deleteNode(nodeName);
            }

            setSelectedNodes([]);
            setScaleDownDialogOpen(false);
            setSuccessMessage(`Successfully scaled down ${selectedNodes.length} node(s)`);

            // Refresh nodes list
            queryClient.invalidateQueries({ queryKey: NODES_QUERY_KEY });

        } catch (error) {
            console.error('Error scaling down nodes:', error);
            setScaleDownError('Failed to scale down nodes. Please try again.');
        } finally {
            setLoading(false);
        }
    };

    const handleCloseScaleUp = () => {
        setScaleUpOpen(false);
        setSelectedNodes([]);
    };

    const remainingNodesAfterScaleDown = transformedNodes.length - selectedNodes.length;

    const nodesWithActions = transformedNodes.map(node => ({
        ...node,
        onDelete: () => handleSingleNodeScaleDown(node)
    }));

    const isRowSelectable = (params: GridRowParams) => {
        return params.row.role === 'worker';
    };

    return (
        <>
            <DataGrid
                rows={nodesWithActions}
                columns={columns}
                initialState={{
                    pagination: { paginationModel: { page: 0, pageSize: 25 } },
                }}
                pageSizeOptions={[25, 50, 100]}
                checkboxSelection
                isRowSelectable={isRowSelectable}
                onRowSelectionModelChange={handleSelectionChange}
                rowSelectionModel={selectedNodes}
                loading={isLoading}
                slots={{
                    toolbar: () => (
                        <NodesToolbar
                            onScaleUp={handleScaleUp}
                            onScaleDown={handleScaleDown}
                            disableScaleDown={selectedNodes.length === 0 || remainingNodesAfterScaleDown < 1}
                            loading={loading}
                            selectedCount={selectedNodes.length}
                            totalNodes={transformedNodes.length}
                        />
                    ),
                    noRowsOverlay: () => (
                        <Box sx={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            height: '100%'
                        }}>
                            <Typography>No Nodes</Typography>
                        </Box>
                    ),
                }}
            />

            <ConfirmationDialog
                open={scaleDownDialogOpen}
                onClose={() => {
                    setScaleDownDialogOpen(false);
                    setSelectedNodes([]);
                    setScaleDownError(null);
                }}
                title="Confirm Scale Down"
                icon={<WarningIcon color="warning" />}
                message={
                    selectedNodes.length > 1
                        ? "Are you sure you want to scale down these nodes?"
                        : "Are you sure you want to scale down this node?"
                }
                items={transformedNodes
                    .filter(node => selectedNodes.includes(node.name))
                    .map(node => ({
                        id: node.name,
                        name: node.name
                    }))
                }
                actionLabel="Scale Down"
                actionColor="error"
                actionVariant="outlined"
                onConfirm={confirmScaleDown}
                errorMessage={scaleDownError}
                onErrorChange={setScaleDownError}
            />

            <ScaleUpDrawer
                open={scaleUpOpen}
                onClose={handleCloseScaleUp}
                masterNode={masterNode}
            />

            <Snackbar
                open={!!successMessage}
                autoHideDuration={6000}
                onClose={() => setSuccessMessage(null)}
            >
                <Alert
                    onClose={() => setSuccessMessage(null)}
                    severity="success"
                >
                    {successMessage}
                </Alert>
            </Snackbar>
        </>
    );
} 