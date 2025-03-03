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
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import { useState } from "react";
import ScaleUpDrawer from "./ScaleUpDrawer";
import ConfirmationDialog from "src/components/dialogs/ConfirmationDialog";
import { useNodesQuery } from "src/hooks/api/useNodesQuery"
import { deleteNode } from "src/api/nodes/nodeMappings";
import { useQueryClient } from "@tanstack/react-query";
import { NODES_QUERY_KEY } from "src/hooks/api/useNodesQuery";
import { NodeItem } from "src/api/nodes/model";
import { intervalToDuration } from 'date-fns';

// Update helper for formatting duration
const formatAge = (date: Date) => {
    const duration = intervalToDuration({
        start: date,
        end: new Date()
    });

    if (duration.days) {
        return `${duration.days} day${duration.days > 1 ? 's' : ''}`;
    }
    if (duration.hours) {
        return `${duration.hours} hour${duration.hours > 1 ? 's' : ''}`;
    }
    if (duration.minutes) {
        return `${duration.minutes} minute${duration.minutes > 1 ? 's' : ''}`;
    }

    return 'just now';
};

const columns: GridColDef[] = [
    {
        field: "name",
        headerName: "Name",
        flex: 2,
    },
    // {
    //     field: 'status',
    //     headerName: 'Status',
    //     flex: 1,
    //     renderCell: (params) => (
    //         <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
    //             {params.value === 'Online' ? (
    //                 <CheckCircleIcon sx={{ color: 'success.main' }} />
    //             ) : params.value === 'Offline' ? (
    //                 <ErrorIcon sx={{ color: 'warning.main' }} />
    //             ) : (
    //                 <HelpIcon />
    //             )}
    //             {params.value}
    //         </Box>
    //     ),
    // },
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
        sortComparator: (v1, v2) => {
            if (v1 === 'master') return -1;
            if (v2 === 'master') return 1;
            return v1.localeCompare(v2);
        }
    },
    {
        field: "age",
        headerName: "Age",
        flex: 1,
        valueGetter: (_, params) => new Date(params?.creationTimestamp),
        renderCell: (params) => (
            <Tooltip title={`Created on ${params.value.toLocaleString()}`}>
                <span>{formatAge(params.value)}</span>
            </Tooltip>
        ),
        sortComparator: (v1, v2) => {
            return new Date(v1).getTime() - new Date(v2).getTime();
        }
    },
    {
        field: "ipAddress",
        headerName: "IP Address",
        flex: 1,
    },
    // {
    //     field: "activeMigrations",
    //     headerName: "Active Migrations",
    //     flex: 2,
    //     valueGetter: (_, params) => params?.activeMigrations || [],
    //     renderCell: (params) => {
    //         const migrations = params.value as string[];
    //         if (!migrations || migrations.length === 0) {
    //             return "-";
    //         }
    //         return migrations.join(", ");
    //     },
    // },
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
    onRefresh?: () => void;
}
interface NodeSelector {
    id: string
    name: string
    status: string
    ipAddress: string
    role: string,
    activeMigrations: string[]
}
const NodesToolbar = ({
    onScaleUp,
    onScaleDown,
    disableScaleDown,
    loading,
    selectedCount,
    totalNodes,
    onRefresh
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
                    Agents
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
                    onRefresh={onRefresh}
                />
            </Box>
        </GridToolbarContainer>
    );
};

export default function NodesTable() {
    const { data: nodes, isLoading, refetch: refreshNodes } = useNodesQuery();
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
        // status: node.status?.status || 'Unknown',
        status: 'Unknown',
        phase: node.status?.phase || 'Unknown',
        ipAddress: node.status?.vmip || '-',
        role: node.spec.noderole,
        creationTimestamp: node.metadata.creationTimestamp,
        activeMigrations: node.status?.activeMigrations || []
    })) || [];

    const handleSelectionChange = (newSelection) => {
        setSelectedNodes(newSelection);
    };

    const handleRefresh = () => {
        refreshNodes()
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
        queryClient.invalidateQueries({ queryKey: NODES_QUERY_KEY });
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
                    sorting: {
                        sortModel: [
                            { field: 'age', sort: 'desc' },
                        ],
                    },
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
                            onRefresh={handleRefresh}
                        />
                    ),
                    noRowsOverlay: () => (
                        <Box sx={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            height: '100%'
                        }}>
                            <Typography>No Agents</Typography>
                        </Box>
                    ),
                }}
                slotProps={{
                    toolbar: {
                        onScaleUp: handleScaleUp,
                        onScaleDown: handleScaleDown,
                        disableScaleDown: selectedNodes.length === 0 || selectedNodes.length === transformedNodes.length,
                        loading: loading || isLoading,
                        selectedCount: selectedNodes.length,
                        totalNodes: transformedNodes.length,
                        onRefresh: handleRefresh
                    },
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