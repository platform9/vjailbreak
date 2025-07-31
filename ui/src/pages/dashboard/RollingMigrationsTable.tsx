import { useState, useMemo } from "react";
import {
    Box,
    Typography,
    Chip,
    Drawer,
    styled,
    Button,
    LinearProgress,
    IconButton,
    Tooltip
} from "@mui/material";
import {
    DataGrid,
    GridColDef,
    GridToolbarContainer,
    GridRowSelectionModel
} from "@mui/x-data-grid";
import VisibilityIcon from '@mui/icons-material/Visibility';
import CloseIcon from '@mui/icons-material/Close';
import DeleteIcon from '@mui/icons-material/Delete';
import ClusterIcon from '@mui/icons-material/Hub';
import RefreshIcon from '@mui/icons-material/Refresh';
import { QueryObserverResult } from "@tanstack/react-query";
import { RefetchOptions } from "@tanstack/react-query";
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import { ReactElement } from "react";
import WarningIcon from '@mui/icons-material/Warning';
import ConfirmationDialog from "src/components/dialogs/ConfirmationDialog";
import { deleteRollingMigrationPlan } from "src/api/rolling-migration-plans/rollingMigrationPlans";
import { useQueryClient } from "@tanstack/react-query";
import { ROLLING_MIGRATION_PLANS_QUERY_KEY } from "src/hooks/api/useRollingMigrationPlansQuery";

// Import CDS icons
import "@cds/core/icon/register.js";
import { ClarityIcons, buildingIcon, clusterIcon, hostIcon, vmIcon } from "@cds/core/icon";
import { ESXHost, ESXIMigration } from "src/api/esximigrations/model";
import { Migration, Phase } from "src/api/migrations/model";
import { RollingMigrationPlan } from "src/api/rolling-migration-plans/model";
import { getESXHosts } from "src/api/esximigrations/helper";
import MigrationsTable from "./MigrationsTable";
import { calculateTimeElapsed } from "src/utils";

// Register clarity icons
ClarityIcons.addIcons(buildingIcon, clusterIcon, hostIcon, vmIcon);



const CdsIconWrapper = styled('div')({
    marginRight: 8,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center'
});

const StyledDrawer = styled(Drawer)(() => ({
    "& .MuiDrawer-paper": {
        display: "grid",
        gridTemplateRows: "max-content 1fr max-content",
        width: "1200px",
        maxWidth: "90vw", // For responsiveness on smaller screens
    },
}));

const DrawerContent = styled("div")(({ theme }) => ({
    overflow: "auto",
    padding: theme.spacing(4, 6, 4, 4),
}));

const DrawerHeader = styled("div")(({ theme }) => ({
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    padding: theme.spacing(2, 4),
    borderBottom: `1px solid ${theme.palette.divider}`,
}));

const DrawerFooter = styled("div")(({ theme }) => ({
    display: "flex",
    alignItems: "center",
    justifyContent: "flex-end",
    padding: theme.spacing(2, 4),
    borderTop: `1px solid ${theme.palette.divider}`,
}));

function StatusChip({ status }: { status: string }) {
    let color: "success" | "info" | "warning" | "error" | "default" = "default";
    const icon: ReactElement | undefined = undefined;

    switch (status.toLowerCase()) {
        case "migrated":
        case "succeeded":
        case "completed":
        case "done":
            color = "success";
            break;
        case "in progress":
        case "running":
        case "active":
            color = "info";
            break;
        case "pending":
        case "queued":
            color = "warning";
            break;
        case "failed":
            color = "error";
            break;
        case "cordoned":
            color = "warning";
            break;
        default:
            color = "default";
            break;
    }

    return (
        <Chip
            size="small"
            label={status}
            variant="outlined"
            color={color}
            icon={icon}
            sx={{
                borderRadius: '4px',
                height: '24px'
            }}
        />
    );
}

// Status summary component to show counts at the top
const StatusSummary = ({
    items,
    getStatus,
    title
}: {
    items: unknown[];
    getStatus: (item: unknown) => string;
    title: string
}) => {
    const counts = items.reduce<Record<string, number>>((acc, item) => {
        const status = getStatus(item);
        acc[status] = (acc[status] || 0) + 1;
        return acc;
    }, {});

    const total = items.length;
    const completed = counts['Completed'] || counts['Succeeded'] || counts['Done'] || counts['Migrated'] || 0;
    const inProgress = counts['In Progress'] || counts['Running'] || counts['Active'] || 0;
    const pending = counts['Pending'] || counts['Queued'] || 0;
    const failed = counts['Failed'] || 0;
    const cordoned = counts['Cordoned'] || 0;
    const progress = (completed / total) * 100;

    return (
        <Box sx={{ mb: 2 }}>
            <Typography variant="subtitle1" fontWeight="bold">{title}</Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                <Box sx={{ width: '100%', mr: 1 }}>
                    <LinearProgress
                        variant="determinate"
                        value={isNaN(progress) ? 0 : progress}
                        sx={{
                            height: 4,
                            borderRadius: 5,
                            backgroundColor: '#f5f5f5'
                        }}
                    />
                </Box>
                <Box sx={{ minWidth: 35 }}>
                    <Typography variant="body2" color="text.secondary">
                        {completed}/{total}
                    </Typography>
                </Box>
            </Box>
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
                {completed > 0 && <Chip label={`Completed: ${completed}`} size="small" color="success" variant="outlined" />}
                {inProgress > 0 && <Chip label={`In Progress: ${inProgress}`} size="small" color="info" variant="outlined" />}
                {pending > 0 && <Chip label={`Pending: ${pending}`} size="small" color="warning" variant="outlined" />}
                {failed > 0 && <Chip label={`Failed: ${failed}`} size="small" color="error" variant="outlined" />}
                {cordoned > 0 && <Chip label={`Cordoned: ${cordoned}`} size="small" color="info" variant="outlined" />}
            </Box>
        </Box>
    );
};

function ClusterDetailsDrawer({ open, onClose, esxHosts, migrations, refetchMigrations, rollingMigrationPlan, refetchESXMigrations }) {
    // ESX Columns for the table
    const esxColumns: GridColDef[] = [
        {
            field: 'name',
            headerName: 'ESX Name',
            flex: 1,
            renderCell: (params) => (
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <CdsIconWrapper>
                        {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                        {/* @ts-ignore */}
                        <cds-icon shape="host" size="md" badge="info"></cds-icon>
                    </CdsIconWrapper>
                    {params.value}
                </Box>
            ),
        },
        {
            field: 'state',
            headerName: 'State',
            flex: 1,
            renderCell: (params) => <StatusChip status={params.value as string} />
        },
        {
            field: 'timeElapsed',
            headerName: 'Time Elapsed',
            flex: 0.8,
            valueGetter: (_, row) => calculateTimeElapsed(row.creationTimestamp, row.status),
        },
        {
            field: 'ip',
            headerName: 'Current IP',
            flex: 1,
            valueGetter: (value: string) => value || "—"
        },
        {
            field: 'vms',
            headerName: '# VMs',
            flex: 0.5,
            valueGetter: (value: string) => value || "—"
        }
    ];

    // Get display name from RollingMigrationPlan cluster sequence or fallback to plan name
    const displayName = rollingMigrationPlan?.spec?.clusterSequence?.[0]?.clusterName ||
        rollingMigrationPlan?.metadata?.name ||
        'Migration Details';

    // // VM Columns for the table
    // const vmColumns: GridColDef[] = [
    //     {
    //         field: 'name',
    //         headerName: 'VM Name',
    //         flex: 1.5,
    //         renderCell: (params) => (
    //             <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
    //                 <Tooltip title={params.row.powerState === "powered-on" ? "Powered On" : "Powered Off"}>
    //                     <CdsIconWrapper>
    //                         {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
    //                         {/* @ts-ignore */}
    //                         <cds-icon shape="vm" size="md" badge={params.row.powerState === "powered-on" ? "success" : "danger"}></cds-icon>
    //                     </CdsIconWrapper>
    //                 </Tooltip>
    //                 <Box>{params.value}</Box>
    //             </Box>
    //         ),
    //     },
    //     {
    //         field: 'status',
    //         headerName: 'Status',
    //         flex: 0.8,
    //         renderCell: (params) => <StatusChip status={params.value as string} />
    //     },
    //     {
    //         field: 'ip',
    //         headerName: 'Current IP',
    //         flex: 1,
    //         valueGetter: (value: string) => value || "—"
    //     },
    //     {
    //         field: 'networks',
    //         headerName: 'Network Interface(s)',
    //         flex: 1,
    //         renderCell: (params) => {
    //             const networks = (params.row as VM).networks;
    //             return networks ? networks.join(", ") : "—";
    //         }
    //     },
    //     {
    //         field: 'memory',
    //         headerName: 'Memory (MB)',
    //         flex: 0.8,
    //         valueGetter: (value: string) => value || "—"
    //     },
    //     {
    //         field: 'esxHost',
    //         headerName: 'ESX Host',
    //         flex: 1,
    //         valueGetter: (value: string) => value || "—"
    //     },
    // ];

    return (
        <StyledDrawer
            anchor="right"
            open={open}
            onClose={onClose}
        >
            <DrawerHeader>
                <Typography variant="h6">
                    {displayName} Details
                </Typography>
                <IconButton onClick={onClose}>
                    <CloseIcon />
                </IconButton>
            </DrawerHeader>
            <DrawerContent>
                <Box sx={{ p: 2 }}>
                    <Box sx={{ mb: 4 }}>
                        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                            <Typography variant="h6" fontWeight="bold">
                                ESX Migrations
                            </Typography>
                            <Tooltip title="Refresh ESX Migrations">
                                <IconButton
                                    onClick={() => refetchESXMigrations?.()}
                                    size="small"
                                    color="primary"
                                >
                                    <RefreshIcon />
                                </IconButton>
                            </Tooltip>
                        </Box>
                        <StatusSummary
                            items={esxHosts}
                            getStatus={(esxi) => (esxi as ESXHost).state}
                            title=""
                        />
                        <Box sx={{ height: 300, width: '100%' }}>
                            <DataGrid
                                rows={esxHosts}
                                columns={esxColumns}
                                disableRowSelectionOnClick
                                initialState={{
                                    pagination: { paginationModel: { pageSize: 10 } },
                                }}
                                pageSizeOptions={[10, 25, 50]}
                                sx={{
                                    '& .MuiDataGrid-columnHeaders': {
                                        backgroundColor: 'rgba(0, 0, 0, 0.04)'
                                    }
                                }}
                            />
                        </Box>
                    </Box>

                    <Box sx={{ mt: 4 }}>
                        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                            <Typography variant="h6" fontWeight="bold">
                                VM Migrations
                            </Typography>
                            <Tooltip title="Refresh VM Migrations">
                                <IconButton
                                    onClick={() => refetchMigrations?.()}
                                    size="small"
                                    color="primary"
                                >
                                    <RefreshIcon />
                                </IconButton>
                            </Tooltip>
                        </Box>
                        <StatusSummary
                            items={migrations}
                            getStatus={(migration) => (migration as Migration).status?.phase}
                            title=""
                        />
                        <Box sx={{ height: 300, width: '100%' }}>
                            <MigrationsTable
                                refetchMigrations={refetchMigrations}
                                migrations={migrations || []}
                            // onDeleteMigration={handleDeleteClick}
                            // onDeleteSelected={handleDeleteSelected}
                            />
                        </Box>
                    </Box>
                </Box>
            </DrawerContent>
            <DrawerFooter>
                <Button variant="outlined" onClick={onClose}>Close</Button>
            </DrawerFooter>
        </StyledDrawer>
    );
}

interface CustomToolbarProps {
    refetchClusterMigrations?: (options?: RefetchOptions) => Promise<QueryObserverResult<RollingMigrationPlan[], Error>>;
    selectedCount: number;
    onDeleteSelected: () => void;
}

const CustomToolbar = ({ refetchClusterMigrations, selectedCount, onDeleteSelected }: CustomToolbarProps) => {
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
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <ClusterIcon />
                    <Typography variant="h6" component="h2">
                        Cluster Conversions
                    </Typography>
                </Box>
                {selectedCount > 0 && (
                    <Typography variant="body2" color="text.secondary">
                        {selectedCount} {selectedCount === 1 ? 'row' : 'rows'} selected
                    </Typography>
                )}
            </div>
            <Box sx={{ display: 'flex', gap: 2 }}>
                {selectedCount > 0 && (
                    <Button
                        variant="outlined"
                        color="error"
                        startIcon={<DeleteIcon />}
                        onClick={onDeleteSelected}
                        size="small"
                    >
                        Delete Selected
                    </Button>
                )}
                <CustomSearchToolbar
                    placeholder="Search by Cluster Name, Status, or ESXi Host"
                    onRefresh={refetchClusterMigrations}
                />
            </Box>
        </GridToolbarContainer>
    );
};

interface RollingMigrationsTableProps {
    rollingMigrationPlans: RollingMigrationPlan[];
    esxiMigrations: ESXIMigration[];
    migrations: Migration[];
    refetchRollingMigrationPlans?: (options?: RefetchOptions) => Promise<QueryObserverResult<RollingMigrationPlan[], Error>>;
    refetchESXIMigrations?: (options?: RefetchOptions) => Promise<QueryObserverResult<ESXIMigration[], Error>>;
    refetchMigrations?: (options?: RefetchOptions) => Promise<QueryObserverResult<Migration[], Error>>;
}

export default function RollingMigrationsTable({
    refetchRollingMigrationPlans,
    esxiMigrations,
    rollingMigrationPlans,
    migrations,
    refetchESXIMigrations,
    refetchMigrations,
}: RollingMigrationsTableProps) {
    const queryClient = useQueryClient();
    const [selectedPlan, setSelectedPlan] = useState<RollingMigrationPlan | null>(null);
    const [drawerOpen, setDrawerOpen] = useState(false);
    const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([]);
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
    const [deleteError, setDeleteError] = useState<string | null>(null);



    const esxHostsByPlan = useMemo(() => {
        const result: Record<string, ESXHost[]> = {};

        rollingMigrationPlans.forEach(plan => {
            if (plan.metadata?.name) {
                const filteredESXIMigrations = esxiMigrations.filter(esxi =>
                    esxi.metadata?.labels?.['vjailbreak.k8s.pf9.io/rollingmigrationplan'] === plan.metadata.name
                );

                const esxHosts = getESXHosts(filteredESXIMigrations);
                result[plan.metadata.name] = esxHosts;
            }
        });

        return result;
    }, [rollingMigrationPlans, esxiMigrations]);

    // Map from RollingMigrationPlan to VM migrations
    const migrationsByPlan = useMemo(() => {
        const result: Record<string, Migration[]> = {};

        rollingMigrationPlans.forEach(plan => {
            if (plan.metadata?.name) {
                result[plan.metadata.name] = migrations.filter(migration =>
                    migration.metadata?.labels?.['vjailbreak.k8s.pf9.io/rollingmigrationplan']?.includes(plan.metadata.name)
                );
            }
        });

        return result;
    }, [rollingMigrationPlans, migrations]);

    const handleOpenDetails = (plan: RollingMigrationPlan) => {
        setSelectedPlan(plan);
        setDrawerOpen(true);
    };
    const handleCloseDrawer = () => {
        setDrawerOpen(false);
    };

    const handleDeleteSelected = () => {
        setDeleteDialogOpen(true);
    };

    const handleDeleteClose = () => {
        setDeleteDialogOpen(false);
        setDeleteError(null);
    };


    const handleConfirmDelete = async () => {
        try {
            const selectedRollingMigrationPlans = rollingMigrationPlans.filter(plan =>
                selectedRows.includes(plan.metadata?.name || '')
            );

            await Promise.all(
                selectedRollingMigrationPlans.map(async (plan) => {
                    await deleteRollingMigrationPlan(plan.metadata?.name || '');
                })
            );

            queryClient.invalidateQueries({ queryKey: ROLLING_MIGRATION_PLANS_QUERY_KEY });

            setSelectedRows([]);
        } catch (error) {
            console.error("Failed to delete rolling migration plans:", error);
            setDeleteError(error instanceof Error ? error.message : "Failed to delete rolling migration plans");
            throw error;
        }
    };

    const handleSelectionChange = (newSelection: GridRowSelectionModel) => {
        setSelectedRows(newSelection);
    };

    // Status ordering for sorting
    const STATUS_ORDER = {
        'Running': 0,
        'In Progress': 0,
        'Active': 0,
        'Failed': 1,
        'Succeeded': 2,
        'Completed': 2,
        'Migrated': 2,
        'Done': 2,
        'Pending': 3,
        'Queued': 3
    };

    // ClusterMigration columns for the main table
    const clusterColumns: GridColDef[] = [
        {
            field: 'clusterName',
            headerName: 'Cluster Name',
            display: 'flex',
            flex: 1,
            renderCell: (params) => {
                const plan = params.row as RollingMigrationPlan;
                // Get cluster name directly from RollingMigrationPlan cluster sequence
                const clusterName = plan.spec?.clusterSequence?.[0]?.clusterName || 'Unknown';

                return (
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                        <CdsIconWrapper>
                            {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                            {/* @ts-ignore */}
                            <cds-icon shape="cluster" size="md"></cds-icon>
                        </CdsIconWrapper>
                        <Typography variant="body2">{clusterName}</Typography>
                    </Box>
                );
            },
        },
        {
            field: 'status',
            headerName: 'Migration Status',
            flex: 0.5,
            renderCell: (params) => {
                const plan = params.row as RollingMigrationPlan;
                const status = plan.status?.phase || 'Unknown';
                return <StatusChip status={status} />;
            },
            sortComparator: (v1, v2) => {
                const order1 = STATUS_ORDER[v1] ?? Number.MAX_SAFE_INTEGER;
                const order2 = STATUS_ORDER[v2] ?? Number.MAX_SAFE_INTEGER;
                return order1 - order2;
            }
        },
        {
            field: 'timeElapsed',
            headerName: 'Time Elapsed',
            flex: 0.5,
            valueGetter: (_, row) => calculateTimeElapsed(row.metadata?.creationTimestamp, row.status),
        },
        {
            field: 'esxCount',
            headerName: 'ESX Hosts',
            flex: 0.5,
            renderCell: (params) => {
                const plan = params.row as RollingMigrationPlan;
                const planName = plan.metadata?.name || '';
                return esxHostsByPlan[planName]?.length || 0;
            },
        },
        {
            field: 'vmCount',
            headerName: 'VMs',
            flex: 0.5,
            renderCell: (params) => {
                const plan = params.row as RollingMigrationPlan;
                const planName = plan.metadata?.name || '';
                return migrationsByPlan[planName]?.length || 0;
            },
        },
        {
            field: 'progress',
            headerName: 'Migration Progress',
            flex: 1,
            renderCell: (params) => {
                const plan = params.row as RollingMigrationPlan;
                const planName = plan.metadata?.name || '';

                // ESX Hosts progress
                const esxHosts = esxHostsByPlan[planName] || [];
                const totalEsx = esxHosts.length;
                const migratedEsx = esxHosts.filter(host => host.state === Phase.Succeeded).length;
                const esxProgress = totalEsx > 0 ? (migratedEsx / totalEsx) * 100 : 0;

                // VMs progress
                const migrations = migrationsByPlan[planName] || [];
                const totalVms = migrations.length;
                const migratedVms = migrations.filter(migration => migration.status?.phase === Phase.Succeeded).length;
                const vmProgress = totalVms > 0 ? (migratedVms / totalVms) * 100 : 0;

                return (
                    <Box sx={{ width: '100%' }}>
                        {/* ESX Hosts progress - single line layout */}
                        <Box sx={{ display: 'flex', alignItems: 'center', mt: 1 }}>
                            <Typography variant="caption" color="text.secondary" sx={{ width: 70 }}>
                                ESX Hosts:
                            </Typography>
                            <Box sx={{ flex: 1, mx: 1, display: 'flex', flexDirection: 'row', alignItems: 'center' }}>
                                <LinearProgress
                                    variant="determinate"
                                    value={esxProgress}
                                    sx={{
                                        width: 150,
                                        borderRadius: 1,
                                        backgroundColor: 'rgba(0, 0, 0, 0.08)'
                                    }}
                                />
                                <Typography variant="caption" sx={{ width: 35, textAlign: 'right' }}>
                                    {migratedEsx}/{totalEsx}
                                </Typography>
                            </Box>
                        </Box>

                        {/* VMs progress - single line layout */}
                        <Box sx={{ display: 'flex', alignItems: 'center' }}>
                            <Typography variant="caption" color="text.secondary" sx={{ width: 70 }}>
                                VMs:
                            </Typography>
                            <Box sx={{ flex: 1, mx: 1, display: 'flex', flexDirection: 'row', alignItems: 'center' }}>
                                <LinearProgress
                                    variant="determinate"
                                    value={vmProgress}
                                    sx={{
                                        width: 150,
                                        borderRadius: 1,
                                        backgroundColor: 'rgba(0, 0, 0, 0.08)'
                                    }}
                                />
                                <Typography variant="caption" sx={{ width: 35, textAlign: 'right' }}>
                                    {migratedVms}/{totalVms}
                                </Typography>
                            </Box>
                        </Box>
                    </Box>
                );
            }
        },
        {
            field: 'actions',
            headerName: 'Actions',
            flex: 1,
            renderCell: (params) => (
                <Button
                    variant="text"
                    size="small"
                    startIcon={<VisibilityIcon />}
                    onClick={() => handleOpenDetails(params.row as RollingMigrationPlan)}
                >
                    Details
                </Button>
            ),
        },
    ];

    return (
        <Box sx={{ width: '100%', height: '100%' }}>
            <DataGrid
                rows={rollingMigrationPlans}
                columns={clusterColumns}
                getRowId={(row: RollingMigrationPlan) => row.metadata?.name || ''}
                initialState={{
                    pagination: { paginationModel: { pageSize: 25 } },
                    sorting: {
                        sortModel: [{ field: 'status', sort: 'asc' }],
                    },
                }}
                pageSizeOptions={[5, 10, 25]}
                localeText={{ noRowsLabel: "No Migrations Available" }}
                slots={{
                    toolbar: () => (
                        <CustomToolbar
                            refetchClusterMigrations={refetchRollingMigrationPlans}
                            selectedCount={selectedRows.length}
                            onDeleteSelected={handleDeleteSelected}
                        />
                    ),
                }}
                checkboxSelection
                onRowSelectionModelChange={handleSelectionChange}
                rowSelectionModel={selectedRows}
                disableRowSelectionOnClick
            />

            {selectedPlan && (
                <ClusterDetailsDrawer
                    open={drawerOpen}
                    onClose={handleCloseDrawer}
                    esxHosts={esxHostsByPlan[selectedPlan.metadata?.name || ''] || []}
                    migrations={migrationsByPlan[selectedPlan.metadata?.name || ''] || []}
                    refetchMigrations={refetchMigrations}
                    rollingMigrationPlan={selectedPlan}
                    refetchESXMigrations={refetchESXIMigrations}
                />
            )}

            <ConfirmationDialog
                open={deleteDialogOpen}
                onClose={handleDeleteClose}
                title="Confirm Delete"
                icon={<WarningIcon color="warning" />}
                message={selectedRows.length > 1
                    ? "Are you sure you want to delete these rolling migration plans?"
                    : `Are you sure you want to delete the selected rolling migration plan?`
                }
                items={rollingMigrationPlans
                    .filter(plan => selectedRows.includes(plan.metadata?.name || ''))
                    .map(plan => ({
                        id: plan.metadata?.name || '',
                        name: plan.metadata?.name || ''
                    }))}
                actionLabel="Delete"
                actionColor="error"
                actionVariant="outlined"
                onConfirm={handleConfirmDelete}
                errorMessage={deleteError}
                onErrorChange={setDeleteError}
            />
        </Box>
    );
}
