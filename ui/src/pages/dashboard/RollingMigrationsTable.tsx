import { useState, useMemo } from "react";
import {
    Box,
    Typography,
    Tooltip,
    Chip,
    Drawer,
    styled,
    Button,
    LinearProgress,
    IconButton
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
import { QueryObserverResult } from "@tanstack/react-query";
import { RefetchOptions } from "@tanstack/react-query";
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import { ReactElement } from "react";

// Import CDS icons
import "@cds/core/icon/register.js";
import { ClarityIcons, buildingIcon, clusterIcon, hostIcon, vmIcon } from "@cds/core/icon";

// Register clarity icons
ClarityIcons.addIcons(buildingIcon, clusterIcon, hostIcon, vmIcon);

// Define interface for GridValueGetterParams to avoid import issues

// Define ClusterMigration and ESXIMigration types
interface ClusterMigration {
    apiVersion: string;
    kind: string;
    metadata: {
        name: string;
        namespace: string;
        creationTimestamp: string;
        finalizers: string[];
        generation: number;
        resourceVersion: string;
        uid: string;
        ownerReferences: { apiVersion: string; kind: string; name: string; uid: string }[];
    };
    spec: {
        clusterName: string;
        esxiMigrationSequence: string[];
        openstackCredsRef: { name: string };
        rollingMigrationPlanRef: { name: string };
        vmwareCredsRef: { name: string };
    };
    status: {
        currentESXi: string;
        message: string;
        phase: string;
    };
}

// interface ESXIMigration {
//     apiVersion: string;
//     kind: string;
//     metadata: {
//         name: string;
//         namespace: string;
//         creationTimestamp: string;
//         finalizers: string[];
//         generation: number;
//         resourceVersion: string;
//         uid: string;
//         ownerReferences: { apiVersion: string; kind: string; name: string; uid: string }[];
//     };
//     spec: {
//         esxiName: string;
//         openstackCredsRef: { name: string };
//         rollingMigrationPlanRef: { name: string };
//         vmwareCredsRef: { name: string };
//     };
// }

// VM model based on the UI display needs
interface VM {
    id: string;
    name: string;
    status: string;
    cluster: string;
    ip: string;
    esxHost: string;
    networks?: string[];
    datastores?: string[];
    cpu?: number;
    memory?: number;
    powerState: string;
}

// ESX host model
interface ESXHost {
    id: string;
    name: string;
    ip: string;
    bmcIp: string;
    maasState: string;
    vms: number;
    state: string;
}

// Style for icons
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

    return (
        <Box sx={{ mb: 2 }}>
            <Typography variant="subtitle1" fontWeight="bold">{title}</Typography>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                <Box sx={{ width: '100%', mr: 1 }}>
                    <LinearProgress
                        variant="determinate"
                        value={(completed / total) * 100}
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

// Generate ESX mock data
const generateESXHostsForCluster = (clusterName: string): ESXHost[] => {
    const hostCount = Math.floor(Math.random() * 5) + 6; // 6-10 hosts per cluster
    return Array.from({ length: hostCount }, (_, i) => ({
        id: `${clusterName}-esx-${i + 1}`,
        name: `esx-${i + 1}.${clusterName.toLowerCase()}.local`,
        ip: `10.0.${Math.floor(i / 3) + 1}.${10 + i}`,
        bmcIp: `10.1.${Math.floor(i / 3) + 1}.${10 + i}`,
        maasState: i % 5 === 0 ? "Cordoned" :
            i % 5 === 1 ? "Active" :
                i % 5 === 2 ? "Migrated" :
                    i % 5 === 3 ? "Pending" : "In Progress",
        vms: Math.floor(Math.random() * 8) + 3,
        state: i % 4 === 0 ? "Migrated" :
            i % 4 === 1 ? "In Progress" :
                i % 4 === 2 ? "Pending" : "Active"
    }));
};

// Mock VM data generator for the cluster
const generateVMsForCluster = (clusterName: string): VM[] => {
    const vmCount = Math.floor(Math.random() * 20) + 15; // 15-35 VMs per cluster
    return Array.from({ length: vmCount }, (_, i) => ({
        id: `${clusterName}-vm-${i + 1}`,
        name: `vm-${i + 1}.${clusterName.toLowerCase()}`,
        status: i % 6 === 0 ? "In Progress" :
            i % 6 === 1 ? "Done" :
                i % 6 === 2 ? "Queued" :
                    i % 6 === 3 ? "Failed" :
                        i % 6 === 4 ? "Pending" : "Migrated",
        cluster: clusterName,
        ip: `10.9.${Math.floor(i / 8) + 1}.${20 + i % 100}`,
        esxHost: `esx-${Math.floor(i / 4) % 10 + 1}.${clusterName.toLowerCase()}.local`,
        networks: [`Network ${i % 4 + 1}`, "Management Network"],
        datastores: [`datastore${i % 3 + 1}`],
        cpu: 2 + (i % 6),
        memory: 4096 + (i % 4) * 2048,
        powerState: i % 4 === 0 ? "powered-off" : "powered-on"
    }));
};

// Generate mock cluster data with more examples
const generateMockClusters = (): ClusterMigration[] => {
    const clusters = [
        "Prod-Finance",
        "Dev-Engineering",
        "QA-Testing",
        "Staging-Web",
        "Core-Infra",
        "Marketing-Web",
        "Sales-CRM",
        "HR-Internal",
        "Analytics-BI",
        "Cloud-Services",
        "Mobile-Backend",
        "IoT-Platform",
        "Database-Cluster",
        "Data-Warehouse"
    ];

    return clusters.map((name, index) => ({
        apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
        kind: "ClusterMigration",
        metadata: {
            name: `${name.toLowerCase().replace('-', '-')}-migration`,
            namespace: "vjailbreak-migration-system",
            creationTimestamp: new Date(Date.now() - index * 24 * 60 * 60 * 1000).toISOString(),
            finalizers: [],
            generation: 1,
            resourceVersion: `${123456 + index}`,
            uid: `${name}-uid-${index}`,
            ownerReferences: []
        },
        spec: {
            clusterName: name,
            esxiMigrationSequence: [],
            openstackCredsRef: { name: "openstack-creds" },
            rollingMigrationPlanRef: { name: "rolling-plan" },
            vmwareCredsRef: { name: "vmware-creds" }
        },
        status: {
            currentESXi: "",
            message: "",
            phase: index % 5 === 0 ? "Running" :
                index % 5 === 1 ? "Pending" :
                    index % 5 === 2 ? "Succeeded" :
                        index % 5 === 3 ? "Failed" : "Completed"
        }
    }));
};


// Component for the cluster details drawer
function ClusterDetailsDrawer({ open, onClose, clusterMigration, esxHosts, vms }) {
    // ESX Columns for the table
    const esxColumns: GridColDef[] = [
        {
            field: 'name',
            headerName: 'ESX Name',
            flex: 1.5,
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
            flex: 0.6,
            renderCell: (params) => <StatusChip status={params.value as string} />
        },
        {
            field: 'ip',
            headerName: 'Current IP',
            flex: 1,
            valueGetter: (value: string) => value || "—"
        },
        {
            field: 'bmcIp',
            headerName: 'BMC IP Address',
            flex: 1,
            valueGetter: (value: string) => value || "—"
        },
        {
            field: 'maasState',
            headerName: 'MaaS State',
            flex: 0.6,
            valueGetter: (value: string) => value || "—"
        },
        {
            field: 'vms',
            headerName: '# VMs',
            flex: 0.5,
            valueGetter: (value: string) => value || "—"
        }
    ];

    // VM Columns for the table
    const vmColumns: GridColDef[] = [
        {
            field: 'name',
            headerName: 'VM Name',
            flex: 1.5,
            renderCell: (params) => (
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Tooltip title={params.row.powerState === "powered-on" ? "Powered On" : "Powered Off"}>
                        <CdsIconWrapper>
                            {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                            {/* @ts-ignore */}
                            <cds-icon shape="vm" size="md" badge={params.row.powerState === "powered-on" ? "success" : "danger"}></cds-icon>
                        </CdsIconWrapper>
                    </Tooltip>
                    <Box>{params.value}</Box>
                </Box>
            ),
        },
        {
            field: 'status',
            headerName: 'Status',
            flex: 0.8,
            renderCell: (params) => <StatusChip status={params.value as string} />
        },
        {
            field: 'ip',
            headerName: 'Current IP',
            flex: 1,
            valueGetter: (value: string) => value || "—"
        },
        {
            field: 'networks',
            headerName: 'Network Interface(s)',
            flex: 1,
            renderCell: (params) => {
                const networks = (params.row as VM).networks;
                return networks ? networks.join(", ") : "—";
            }
        },
        {
            field: 'memory',
            headerName: 'Memory (MB)',
            flex: 0.8,
            valueGetter: (value: string) => value || "—"
        },
        {
            field: 'esxHost',
            headerName: 'ESX Host',
            flex: 1,
            valueGetter: (value: string) => value || "—"
        },
    ];

    return (
        <StyledDrawer
            anchor="right"
            open={open}
            onClose={onClose}
        >
            <DrawerHeader>
                <Typography variant="h6">
                    {clusterMigration.spec.clusterName} Details
                </Typography>
                <IconButton onClick={onClose}>
                    <CloseIcon />
                </IconButton>
            </DrawerHeader>
            <DrawerContent>
                <Box sx={{ p: 2 }}>
                    <Box sx={{ mb: 4 }}>
                        <StatusSummary
                            items={esxHosts}
                            getStatus={(esxi) => (esxi as ESXHost).maasState}
                            title="ESX Migration"
                        />
                        <Box sx={{ height: 300, width: '100%' }}>
                            <DataGrid
                                rows={esxHosts}
                                columns={esxColumns}
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
                        <StatusSummary
                            items={vms}
                            getStatus={(vm) => (vm as VM).status}
                            title="VM Migrations"
                        />
                        <Box sx={{ height: 300, width: '100%' }}>
                            <DataGrid
                                rows={vms}
                                columns={vmColumns}
                                getRowId={(row) => row.id}
                                initialState={{
                                    pagination: { paginationModel: { pageSize: 10 } },
                                }}
                                pageSizeOptions={[10, 25, 50]}
                                checkboxSelection
                                sx={{
                                    '& .MuiDataGrid-columnHeaders': {
                                        backgroundColor: 'rgba(0, 0, 0, 0.04)'
                                    }
                                }}
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
    refetchClusterMigrations?: (options?: RefetchOptions) => Promise<QueryObserverResult<ClusterMigration[], Error>>;
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
                <Typography variant="h6" component="h2">
                    Cluster Migrations
                </Typography>
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
    clusterMigrations: ClusterMigration[];
    refetchClusterMigrations?: (options?: RefetchOptions) => Promise<QueryObserverResult<ClusterMigration[], Error>>;
}

export default function RollingMigrationsTable({
    clusterMigrations: propClusterMigrations,
    refetchClusterMigrations
}: RollingMigrationsTableProps) {
    // Generate mock data if no data is provided
    const mockClusters = useMemo(() => propClusterMigrations || generateMockClusters(), []);

    // Use provided data or mock data
    const clusterMigrations = mockClusters;

    const [selectedCluster, setSelectedCluster] = useState<ClusterMigration | null>(null);
    const [drawerOpen, setDrawerOpen] = useState(false);
    const [selectedRows, setSelectedRows] = useState<GridRowSelectionModel>([]);

    // Generate ESX hosts and VMs for each cluster
    const esxHostsByCluster = useMemo(() => {
        const result: Record<string, ESXHost[]> = {};

        clusterMigrations.forEach(cluster => {
            if (cluster.metadata?.name) {
                const clusterName = cluster.spec.clusterName || '';
                result[cluster.metadata.name] = generateESXHostsForCluster(clusterName);
            }
        });

        return result;
    }, [clusterMigrations]);

    const vmsByCluster = useMemo(() => {
        const result: Record<string, VM[]> = {};

        clusterMigrations.forEach(cluster => {
            if (cluster.metadata?.name) {
                const clusterName = cluster.spec.clusterName || '';
                result[cluster.metadata.name] = generateVMsForCluster(clusterName);
            }
        });

        return result;
    }, [clusterMigrations]);

    const handleOpenDetails = (cluster: ClusterMigration) => {
        setSelectedCluster(cluster);
        setDrawerOpen(true);
    };

    const handleCloseDrawer = () => {
        setDrawerOpen(false);
    };

    const handleDeleteSelected = () => {
        console.log("Delete selected clusters:", selectedRows);
        // Here you would implement actual deletion logic
        // And call any API endpoint to delete the selected clusters
        setSelectedRows([]);
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
            renderCell: (params) => (
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <CdsIconWrapper>
                        {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                        {/* @ts-ignore */}
                        <cds-icon shape="cluster" size="md"></cds-icon>
                    </CdsIconWrapper>
                    <Typography variant="body2">{(params.row as ClusterMigration).spec.clusterName || 'Unknown'}</Typography>
                </Box>
            ),
        },
        {
            field: 'status',
            headerName: 'Status',
            flex: 0.5,
            renderCell: (params) => {
                return <StatusChip status={(params.row as ClusterMigration).status?.phase || 'Pending'} />;
            },
            sortComparator: (v1, v2) => {
                const order1 = STATUS_ORDER[v1] ?? Number.MAX_SAFE_INTEGER;
                const order2 = STATUS_ORDER[v2] ?? Number.MAX_SAFE_INTEGER;
                return order1 - order2;
            }
        },
        {
            field: 'esxCount',
            headerName: 'ESX Hosts',
            flex: 0.5,
            renderCell: (params) => {
                const clusterId = (params.row as ClusterMigration).metadata?.name || '';
                return esxHostsByCluster[clusterId]?.length || 0;
            },
        },
        {
            field: 'vmCount',
            headerName: 'VMs',
            flex: 0.5,
            renderCell: (params) => {
                const clusterId = (params.row as ClusterMigration).metadata?.name || '';
                return vmsByCluster[clusterId]?.length || 0;
            },
        },
        {
            field: 'progress',
            headerName: 'Migration Progress',
            flex: 1,
            renderCell: (params) => {
                const clusterId = (params.row as ClusterMigration).metadata?.name || '';

                // ESX Hosts progress
                const esxHosts = esxHostsByCluster[clusterId] || [];
                const totalEsx = esxHosts.length;
                const migratedEsx = esxHosts.filter(host => host.state === 'Migrated').length;
                const esxProgress = totalEsx > 0 ? (migratedEsx / totalEsx) * 100 : 0;

                // VMs progress
                const vms = vmsByCluster[clusterId] || [];
                const totalVms = vms.length;
                const migratedVms = vms.filter(vm => vm.status === 'Migrated' || vm.status === 'Done').length;
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
                    onClick={() => handleOpenDetails(params.row as ClusterMigration)}
                >
                    Details
                </Button>
            ),
        },
    ];

    return (
        <Box sx={{ width: '100%' }}>
            <DataGrid
                rows={clusterMigrations}
                columns={clusterColumns}
                getRowId={(row: ClusterMigration) => row.metadata?.name || ''}
                initialState={{
                    pagination: { paginationModel: { pageSize: 10 } },
                    sorting: {
                        sortModel: [{ field: 'status', sort: 'asc' }],
                    },
                }}
                pageSizeOptions={[5, 10, 25]}
                slots={{
                    toolbar: () => (
                        <CustomToolbar
                            refetchClusterMigrations={refetchClusterMigrations}
                            selectedCount={selectedRows.length}
                            onDeleteSelected={handleDeleteSelected}
                        />
                    ),
                }}
                sx={{
                    border: 'none',
                    '& .MuiDataGrid-columnHeaders': {
                        backgroundColor: 'rgba(0, 0, 0, 0.04)'
                    }
                }}
                checkboxSelection
                onRowSelectionModelChange={handleSelectionChange}
                rowSelectionModel={selectedRows}
                disableRowSelectionOnClick
            />

            {selectedCluster && (
                <ClusterDetailsDrawer
                    open={drawerOpen}
                    onClose={handleCloseDrawer}
                    clusterMigration={selectedCluster}
                    esxHosts={esxHostsByCluster[selectedCluster.metadata?.name || ''] || []}
                    vms={vmsByCluster[selectedCluster.metadata?.name || ''] || []}
                />
            )}
        </Box>
    );
}