import { useState, useEffect, useMemo } from "react";
import {
    Box,
    Typography,
    IconButton,
    Tooltip,
    Chip,
    Button
} from "@mui/material";
import {
    DataGrid,
    GridColDef,
    GridRowSelectionModel,
    GridToolbarContainer,
    GridRenderCellParams
} from "@mui/x-data-grid";
import DeleteIcon from '@mui/icons-material/DeleteOutlined';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown';
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import { ClusterMigration } from "src/api/clustermigrations/model";
import { ESXIMigration } from "src/api/esximigrations/model";
import { QueryObserverResult } from "@tanstack/react-query";
import { RefetchOptions } from "@tanstack/react-query";
import {
    getClusterMigrationPhase,
    getClusterName,
    getESXiMigrationSequence,
    getCurrentESXi
} from "src/api/clustermigrations/helper";
import {
    getESXiName,
} from "src/api/esximigrations/helper";

// Phase ordering for sorting
const STATUS_ORDER = {
    'Running': 0,
    'Failed': 1,
    'Succeeded': 2,
    'Pending': 3
}

const getStatusColor = (status: string) => {
    switch (status) {
        case 'Succeeded':
            return 'success';
        case 'Failed':
            return 'error';
        case 'Pending':
            return 'warning';
        default:
            return 'primary';
    }
};

interface CustomToolbarProps {
    numSelected: number;
    onDeleteSelected: () => void;
    refetchData: (options?: RefetchOptions) => Promise<QueryObserverResult<ClusterMigration[], Error>>;
}

const CustomToolbar = ({ numSelected, onDeleteSelected, refetchData }: CustomToolbarProps) => {
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
                    Rolling Migrations
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
                    placeholder="Search by Name, Cluster, or ESXi"
                    onRefresh={refetchData}
                />
            </Box>
        </GridToolbarContainer>
    );
};

interface RollingMigrationsTableProps {
    clusterMigrations: ClusterMigration[];
    esxiMigrations: ESXIMigration[];
    onDeleteClusterMigration: (name: string) => void;
    onDeleteESXIMigration: (name: string) => void;
    onDeleteSelectedClusterMigrations: (clusterMigrations: ClusterMigration[]) => void;
    onDeleteSelectedESXIMigrations: (esxiMigrations: ESXIMigration[]) => void;
    refetchClusterMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<ClusterMigration[], Error>>;
    refetchESXIMigrations: (options?: RefetchOptions) => Promise<QueryObserverResult<ESXIMigration[], Error>>;
}

// Enhanced type that can represent either a ClusterMigration or ESXIMigration with UI properties
type EnhancedRow = (ClusterMigration | ESXIMigration) & {
    id: string;
    isESXiMigration: boolean;
    onDelete: (name: string) => void;
    relatedESXiCount?: number;
    esxiNames?: string;
    parentCluster?: string;
};

export default function RollingMigrationsTable({
    clusterMigrations,
    esxiMigrations,
    onDeleteClusterMigration,
    onDeleteESXIMigration,
    onDeleteSelectedClusterMigrations,
    onDeleteSelectedESXIMigrations,
    refetchClusterMigrations
}: RollingMigrationsTableProps) {
    const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([]);
    const [expandedClusterRows, setExpandedClusterRows] = useState<Record<string, boolean>>({});

    // Initialize all clusters to be expanded by default
    useEffect(() => {
        if (clusterMigrations.length > 0) {
            const expandedState: Record<string, boolean> = {};
            clusterMigrations.forEach(migration => {
                if (migration.metadata?.name) {
                    expandedState[migration.metadata.name] = true;
                }
            });
            setExpandedClusterRows(expandedState);
        }
    }, [clusterMigrations]);

    const handleSelectionChange = (newSelection: GridRowSelectionModel) => {
        setSelectedRows(newSelection);
    };

    const toggleClusterExpansion = (clusterName: string) => {
        setExpandedClusterRows(prev => ({
            ...prev,
            [clusterName]: !prev[clusterName]
        }));
    };

    // Helper function to get ESXi migration status
    const getESXiMigrationStatus = (esxiMigration: ESXIMigration): string => {
        // Check if the ESXi migration is referenced in a running cluster migration
        for (const cluster of clusterMigrations) {
            const phase = getClusterMigrationPhase(cluster);
            const currentESXi = cluster.status?.currentESXi;
            const esxiSequence = getESXiMigrationSequence(cluster);

            if (phase === "Running" && currentESXi === esxiMigration.spec?.esxiName) {
                return "In Progress";
            }

            if (esxiSequence.includes(esxiMigration.metadata?.name || "")) {
                if (phase === "Succeeded") {
                    return "Completed";
                } else if (phase === "Failed") {
                    return "Failed";
                } else if (phase === "Pending") {
                    return "Pending";
                }
            }
        }

        return "Queued";
    };

    // Create a map of ESXi migrations by name for quick lookup
    const esxiMigrationsByName = esxiMigrations.reduce((acc, migration) => {
        if (migration.metadata?.name) {
            acc[migration.metadata.name] = {
                ...migration,
                onDelete: onDeleteESXIMigration
            };
        }
        return acc;
    }, {} as Record<string, ESXIMigration & { onDelete: (name: string) => void }>);

    // Helper function to create a cluster migration row
    const createClusterRow = (clusterMigration: ClusterMigration): EnhancedRow => {
        const esxiSequence = getESXiMigrationSequence(clusterMigration);
        // Count related ESXi migrations
        const relatedESXiCount = esxiSequence.filter(name => esxiMigrationsByName[name]).length;

        return {
            ...clusterMigration,
            onDelete: onDeleteClusterMigration,
            id: clusterMigration.metadata?.name || `cluster-${Math.random()}`,
            isESXiMigration: false,
            relatedESXiCount,
            esxiNames: esxiSequence.join(", ")
        };
    };

    // Helper function to create ESXi migration rows for a cluster
    const createESXiRows = (clusterRow: EnhancedRow, esxiSequence: string[]): EnhancedRow[] => {
        return esxiSequence
            .filter(name => esxiMigrationsByName[name])
            .map(name => ({
                ...esxiMigrationsByName[name],
                id: `${clusterRow.id}-${name}`,
                isESXiMigration: true,
                parentCluster: clusterRow.id
            }));
    };

    // Process cluster migrations
    const clusterRows: EnhancedRow[] = clusterMigrations.map(createClusterRow);

    // Build the final rows based on expanded state
    const finalRows: EnhancedRow[] = useMemo(() => {
        const rows: EnhancedRow[] = [];

        clusterRows.forEach(clusterRow => {
            // Add the cluster row
            rows.push(clusterRow);

            // If expanded, add child ESXi rows
            if (expandedClusterRows[clusterRow.id]) {
                const esxiSequence = getESXiMigrationSequence(clusterRow as ClusterMigration);
                const relatedESXiMigrations = createESXiRows(clusterRow, esxiSequence);
                rows.push(...relatedESXiMigrations);
            }
        });

        return rows;
    }, [clusterRows, expandedClusterRows, esxiMigrationsByName]);

    const isRowSelectable = (params: { row: EnhancedRow }) => {
        if (params.row.isESXiMigration) {
            return true;
        }
        const phase = getClusterMigrationPhase(params.row as ClusterMigration);
        return !(!phase || phase === "Running" || phase === "Pending");
    };

    const columns: GridColDef[] = [
        {
            field: "name",
            headerName: "Name",
            flex: 1.5,
            renderCell: (params: GridRenderCellParams) => {
                if (!params.row) return null;
                if (!params.row.isESXiMigration && params.row.relatedESXiCount && params.row.relatedESXiCount > 0) {
                    return (
                        <Box sx={{ display: 'flex', alignItems: 'center' }}>
                            <IconButton
                                size="small"
                                onClick={(e) => {
                                    e.stopPropagation();
                                    toggleClusterExpansion(params.row.id as string);
                                }}
                                sx={{ p: 0.5, mr: 1 }}
                            >
                                {expandedClusterRows[params.row.id as string] ?
                                    <KeyboardArrowDownIcon fontSize="medium" sx={{ width: 24, height: 24 }} /> :
                                    <ChevronRightIcon fontSize="medium" sx={{ width: 24, height: 24 }} />
                                }
                            </IconButton>
                            {params.row.metadata?.name} ({params.row.relatedESXiCount} ESXi hosts)
                        </Box>
                    );
                }

                return params.row.isESXiMigration ?
                    <Box sx={{ pl: 6 }}>{params.row.metadata?.name}</Box> :
                    params.row.metadata?.name;
            }
        },
        {
            field: "type",
            headerName: "Type",
            renderCell: (params: { row: EnhancedRow }) => {
                if (!params?.row) return "";
                return params.row.isESXiMigration ? "ESXi Migration" : "Cluster Migration";
            },
            flex: 0.8,
        },
        {
            field: "clusterName",
            headerName: "Cluster",
            renderCell: (params: { row: EnhancedRow }) => {
                if (!params?.row) return "";
                if (params.row.isESXiMigration) {
                    // For ESXi migrations, show the parent cluster's name if available
                    const parentId = params.row.parentCluster;
                    if (parentId) {
                        const clusterRow = clusterRows.find(c => c.id === parentId);
                        if (clusterRow) {
                            return getClusterName(clusterRow as ClusterMigration);
                        }
                    }
                    return "";
                }
                return getClusterName(params.row as ClusterMigration);
            },
            flex: 1,
        },
        {
            field: "esxiHost",
            headerName: "ESXi Host",
            renderCell: (params: { row: EnhancedRow }) => {
                if (!params?.row) return "";
                if (params.row.isESXiMigration) {
                    return getESXiName(params.row as ESXIMigration);
                }
                // For cluster migrations, show the current ESXi being migrated
                return getCurrentESXi(params.row as ClusterMigration);
            },
            flex: 1,
        },
        {
            field: "status",
            headerName: "Status",
            flex: 0.8,
            renderCell: (params: GridRenderCellParams) => {
                const status = params?.value?.phase;
                if (!status) return null;
                return (
                    <Chip
                        label={status}
                        color={getStatusColor(status)}
                        size="small"
                        variant="outlined"
                    />
                );
            },
            sortComparator: (v1, v2) => {
                const order1 = STATUS_ORDER[v1] ?? Number.MAX_SAFE_INTEGER;
                const order2 = STATUS_ORDER[v2] ?? Number.MAX_SAFE_INTEGER;
                return order1 - order2;
            }
        },
        // {
        //     field: "vmwareCredentials",
        //     headerName: "VMware Credentials",
        //     valueGetter: (params: { row: EnhancedRow }) => {
        //         if (!params?.row) return "";
        //         if (params.row.isESXiMigration) {
        //             return getVMwareCredsRefName(params.row as ESXIMigration);
        //         }
        //         return ""; // Not applicable for cluster migrations
        //     },
        //     flex: 1,
        // },
        // {
        //     field: "openstackCredentials",
        //     headerName: "OpenStack Credentials",
        //     valueGetter: (params: { row: EnhancedRow }) => {
        //         if (!params?.row) return "";
        //         if (params.row.isESXiMigration) {
        //             return getOpenstackCredsRefName(params.row as ESXIMigration);
        //         }
        //         return ""; // Not applicable for cluster migrations
        //     },
        //     flex: 1,
        // },
        {
            field: "actions",
            headerName: "Actions",
            flex: 0.5,
            renderCell: (params: GridRenderCellParams) => {
                if (!params.row) return null;
                if (params.row.isESXiMigration) {
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
                } else {
                    const phase = getClusterMigrationPhase(params.row as ClusterMigration);
                    const isDisabled = !phase || phase === "Running" || phase === "Pending";

                    return (
                        <Tooltip title={isDisabled ? "Cannot delete while migration is in progress" : "Delete cluster migration"} >
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
                        </Tooltip>
                    );
                }
            },
        },
    ];

    return (
        <Box sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
            <DataGrid
                rows={finalRows}
                columns={columns}
                initialState={{
                    pagination: { paginationModel: { page: 0, pageSize: 25 } },
                    sorting: {
                        sortModel: [{ field: 'status', sort: 'asc' }],
                    },
                }}
                pageSizeOptions={[25, 50, 100]}
                localeText={{ noRowsLabel: "No Migrations Available" }}
                checkboxSelection
                isRowSelectable={isRowSelectable}
                onRowSelectionModelChange={handleSelectionChange}
                rowSelectionModel={selectedRows}
                slots={{
                    toolbar: () => (
                        <CustomToolbar
                            numSelected={selectedRows.length}
                            onDeleteSelected={() => {
                                const selectedClusterMigrations = clusterMigrations?.filter(
                                    m => selectedRows.includes(m.metadata?.name || "")
                                );
                                const selectedESXiMigrations = esxiMigrations?.filter(
                                    m => selectedRows.some(id => id.toString().includes(m.metadata?.name || ""))
                                );

                                if (selectedClusterMigrations.length > 0) {
                                    onDeleteSelectedClusterMigrations(selectedClusterMigrations);
                                }
                                if (selectedESXiMigrations.length > 0) {
                                    onDeleteSelectedESXIMigrations(selectedESXiMigrations);
                                }
                            }}
                            refetchData={refetchClusterMigrations}
                        />
                    ),
                }}
            />
        </Box>
    );
} 