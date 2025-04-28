import { Box, Typography, FormControl, Select, MenuItem, ListSubheader, Drawer, styled, Paper, Tooltip, Button, Dialog, DialogTitle, DialogContent, DialogActions, FormLabel } from "@mui/material"
import { useState, useMemo, useEffect } from "react"
import { DataGrid, GridColDef, GridRowSelectionModel } from "@mui/x-data-grid"
import Footer from "../../components/forms/Footer"
import Header from "../../components/forms/Header"
import Step from "../../components/forms/Step"
import { DrawerContent } from "src/components/forms/StyledDrawer"
import { useKeyboardSubmit } from "src/hooks/ui/useKeyboardSubmit"
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward"
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward"
import VerticalAlignTopIcon from "@mui/icons-material/VerticalAlignTop"
import VerticalAlignBottomIcon from "@mui/icons-material/VerticalAlignBottom"
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism'
import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { getVmwareCredentialsList } from "src/api/vmware-creds/vmwareCreds"
import { getVMwareClusters } from "src/api/vmware-clusters/vmwareClusters"
import { getVMwareHosts } from "src/api/vmware-hosts/vmwareHosts"
import { getVMwareMachines } from "src/api/vmware-machines/vmwareMachines"
import { VMwareCreds } from "src/api/vmware-creds/model"
import { VMwareCluster } from "src/api/vmware-clusters/model"
import { VMwareHost } from "src/api/vmware-hosts/model"
import { VMwareMachine } from "src/api/vmware-machines/model"
import { VJAILBREAK_DEFAULT_NAMESPACE } from "src/api/constants"
import { getBMConfigList, getBMConfig } from "src/api/bmconfig/bmconfig"
import { BMConfig } from "src/api/bmconfig/model"
import MaasConfigDetailsModal from "src/pages/dashboard/BMConfigDetailsModal"
import { getOpenstackCredentialsList } from "src/api/openstack-creds/openstackCreds"
import NetworkAndStorageMappingStep, { ResourceMap } from "./NetworkAndStorageMappingStep"
import { createRollingMigrationPlanJson, postRollingMigrationPlan, VMSequence } from "src/api/rolling-migration-plans"
import DatacenterIcon from "@mui/icons-material/Storage"
import ClusterIcon from "@mui/icons-material/HubOutlined"
import VpnKeyIcon from "@mui/icons-material/VpnKey"

const StyledDrawer = styled(Drawer)(() => ({
    "& .MuiDrawer-paper": {
        display: "grid",
        gridTemplateRows: "max-content 1fr max-content",
        width: "1200px",
        maxWidth: "90vw",
    },
}))

interface PcdDataItem {
    name: string;
    credName: string;
}

const mockSecurityGroups = [
    { id: "default", name: "default" },
    { id: "web", name: "web" },
    { id: "app", name: "app" },
    { id: "db", name: "db" }
];

const mockTenants = [
    { id: "service", name: "service" },
    { id: "customer1", name: "customer1" },
    { id: "customer2", name: "customer2" }
];

interface ESXHost {
    id: string;
    name: string;
    ip: string;
    bmcIp: string;
    maasState: string;
    vms: number;
    state: string;
}

interface VM {
    id: string;
    name: string;
    ip: string;
    powerState: string;
    sg: string;
    tenant: string;
    esxHost: string;
    networks?: string[];
    datastores?: string[];
}

const esxColumns: GridColDef[] = [
    {
        field: "name",
        headerName: "ESX Name",
        flex: 1.5,
    },
    {
        field: "ip",
        headerName: "IP Address",
        flex: 1,
        valueGetter: (value) => value || "—",
    },
    {
        field: "bmcIp",
        headerName: "BMC IP Address",
        flex: 1,
        valueGetter: (value) => value || "—",
    },
    {
        field: "maasState",
        headerName: "MaaS State",
        flex: 0.5,
        valueGetter: (value) => value || "—",
    },
    {
        field: "vms",
        headerName: "# VMs",
        flex: 0.5,
        valueGetter: (value) => value || "—",
    },
    {
        field: "state",
        headerName: "State",
        flex: 0.5,
        renderHeader: () => (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                <div style={{ fontWeight: 500 }}>State</div>
            </Box>
        ),
    },
];

const vmColumns: GridColDef[] = [
    {
        field: "name",
        headerName: "VM Name",
        flex: 1.5,
        renderCell: (params) => (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Tooltip title={params.row.powerState === "powered-on" ? "Running" : "Stopped"}>
                    <Box
                        sx={{
                            width: 10,
                            height: 10,
                            borderRadius: '50%',
                            backgroundColor: params.row.powerState === "powered-on" ? 'success.main' : 'error.main',
                            display: 'inline-block'
                        }}
                    />
                </Tooltip>
                <Box sx={{ ml: 0.5 }}>{params.value}</Box>
            </Box>
        ),
    },
    {
        field: "ip",
        headerName: "IP Address",
        flex: 1,
        valueGetter: (value) => value || "—",
    },
    {
        field: "esxHost",
        headerName: "ESX Host",
        flex: 1,
        valueGetter: (value) => value || "—",
    },
    {
        field: "powerState",
        headerName: "Power State",
        flex: 1,
        valueGetter: (value) => value || "—",
    },
    {
        field: "sg",
        headerName: "SG",
        flex: 0.7,
        valueGetter: (value) => value || "—",
    },
    {
        field: "tenant",
        headerName: "Tenant",
        flex: 1,
        valueGetter: (value) => value || "—",
    },
];

const paginationModel = { page: 0, pageSize: 5 };

const CustomToolbarWithActions = (props) => {
    const { rowSelectionModel, onAssignSG, onAssignTenant, onMoveUp, onMoveDown, onMoveToTop, onMoveToBottom, ...toolbarProps } = props;

    return (
        <Box sx={{ display: 'flex', justifyContent: 'flex-end', width: '100%', padding: '4px 8px' }}>
            {rowSelectionModel.length > 0 && (
                <>
                    <Button
                        variant="text"
                        color="primary"
                        onClick={onAssignSG}
                        size="small"
                        sx={{ ml: 1 }}
                    >
                        Assign SG ({rowSelectionModel.length})
                    </Button>
                    <Button
                        variant="text"
                        color="primary"
                        onClick={onAssignTenant}
                        size="small"
                        sx={{ ml: 1 }}
                    >
                        Assign Tenant ({rowSelectionModel.length})
                    </Button>
                    <Tooltip title="Move to Top">
                        <Button
                            variant="text"
                            color="primary"
                            onClick={onMoveToTop}
                            size="small"
                            sx={{ ml: 1, minWidth: 'auto' }}
                        >
                            <VerticalAlignTopIcon />
                        </Button>
                    </Tooltip>
                    <Tooltip title="Move Up">
                        <Button
                            variant="text"
                            color="primary"
                            onClick={onMoveUp}
                            size="small"
                            sx={{ ml: 1, minWidth: 'auto' }}
                        >
                            <ArrowUpwardIcon />
                        </Button>
                    </Tooltip>
                    <Tooltip title="Move Down">
                        <Button
                            variant="text"
                            color="primary"
                            onClick={onMoveDown}
                            size="small"
                            sx={{ ml: 1, minWidth: 'auto' }}
                        >
                            <ArrowDownwardIcon />
                        </Button>
                    </Tooltip>
                    <Tooltip title="Move to Bottom">
                        <Button
                            variant="text"
                            color="primary"
                            onClick={onMoveToBottom}
                            size="small"
                            sx={{ ml: 1, minWidth: 'auto' }}
                        >
                            <VerticalAlignBottomIcon />
                        </Button>
                    </Tooltip>
                </>
            )}
            <CustomSearchToolbar {...toolbarProps} />
        </Box>
    );
};

const CustomESXToolbarWithActions = (props) => {
    const { ...toolbarProps } = props;

    return (
        <Box sx={{ display: 'flex', justifyContent: 'flex-end', width: '100%', padding: '4px 8px' }}>
            <CustomSearchToolbar {...toolbarProps} />
        </Box>
    );
};

const MaasConfigDialog = styled(Dialog)({
    "& .MuiDialog-paper": {
        maxWidth: "900px",
        width: "100%"
    }
});

const ConfigSection = styled(Box)(({ theme }) => ({
    marginBottom: theme.spacing(3),
}));

const ConfigField = styled(Box)(({ theme }) => ({
    display: "flex",
    gap: theme.spacing(1),
    marginBottom: theme.spacing(1.5),
}));

const FieldLabel = styled(Typography)(({ theme }) => ({
    fontWeight: 500,
    minWidth: "140px",
    color: theme.palette.text.secondary,
}));

const FieldValue = styled(Typography)(({ theme }) => ({
    fontWeight: 400,
    color: theme.palette.text.primary,
}));

const CodeEditorContainer = styled(Box)(({ theme }) => ({
    border: `1px solid ${theme.palette.grey[300]}`,
    borderRadius: theme.shape.borderRadius,
    overflow: 'auto',
    position: 'relative',
    resize: 'vertical',
    minHeight: '250px',
    maxHeight: '400px',
    backgroundColor: theme.palette.common.white,
    '& pre': {
        margin: 0,
        borderRadius: 0,
        height: '100%',
        overflow: 'auto',
        fontSize: '14px',
    },
    '&::-webkit-scrollbar': {
        width: '8px',
        height: '8px',
    },
    '&::-webkit-scrollbar-thumb': {
        backgroundColor: theme.palette.grey[300],
        borderRadius: '4px',
    },
    '&::-webkit-scrollbar-track': {
        backgroundColor: theme.palette.grey[100],
    }
}));

interface SourceDataItem {
    credName: string;
    datacenter: string;
    clusters: {
        id: string;
        name: string;
    }[];
}

interface RollingMigrationFormDrawerProps {
    open: boolean;
    onClose: () => void;
}

export default function RollingMigrationFormDrawer({
    open,
    onClose,
}: RollingMigrationFormDrawerProps) {
    const [sourceCluster, setSourceCluster] = useState("");
    const [destinationPCD, setDestinationPCD] = useState("");
    const [submitting, setSubmitting] = useState(false);

    const [sourceData, setSourceData] = useState<SourceDataItem[]>([]);
    const [loading, setLoading] = useState(false);
    const [selectedVMwareCredName, setSelectedVMwareCredName] = useState("");

    const [pcdData, setPcdData] = useState<PcdDataItem[]>([]);
    const [loadingPCD, setLoadingPCD] = useState(false);
    const [selectedPcdCredName, setSelectedPcdCredName] = useState("");

    const [selectedVMs, setSelectedVMs] = useState<GridRowSelectionModel>([]);

    const [loadingHosts, setLoadingHosts] = useState(false);

    const [loadingVMs, setLoadingVMs] = useState(false);

    const [orderedESXHosts, setOrderedESXHosts] = useState<ESXHost[]>([]);
    const [vmsWithAssignments, setVmsWithAssignments] = useState<VM[]>([]);

    const [sgDialogOpen, setSgDialogOpen] = useState(false);
    const [tenantDialogOpen, setTenantDialogOpen] = useState(false);
    const [selectedSG, setSelectedSG] = useState("");
    const [selectedTenant, setSelectedTenant] = useState("");

    const [maasConfigDialogOpen, setMaasConfigDialogOpen] = useState(false);
    const [maasConfigs, setMaasConfigs] = useState<BMConfig[]>([]);
    const [selectedMaasConfig, setSelectedMaasConfig] = useState<BMConfig | null>(null);
    const [loadingMaasConfig, setLoadingMaasConfig] = useState(false);
    const [maasDetailsModalOpen, setMaasDetailsModalOpen] = useState(false);

    const [networkMappings, setNetworkMappings] = useState<ResourceMap[]>([]);
    const [storageMappings, setStorageMappings] = useState<ResourceMap[]>([]);
    const [networkMappingError, setNetworkMappingError] = useState<string>("");
    const [storageMappingError, setStorageMappingError] = useState<string>("");

    useEffect(() => {
        if (open) {
            fetchSourceData();
            fetchMaasConfigs();
            fetchPcdData();
        }
    }, [open]);

    const fetchSourceData = async () => {
        setLoading(true);
        try {
            const vmwareCreds = await getVmwareCredentialsList(VJAILBREAK_DEFAULT_NAMESPACE);

            if (!vmwareCreds || vmwareCreds.length === 0) {
                setSourceData([]);
                setLoading(false);
                return;
            }
            const sourceDataPromises = vmwareCreds.map(async (cred: VMwareCreds) => {
                const credName = cred.metadata.name;
                const datacenter = cred.spec.datacenter || credName;

                const clustersResponse = await getVMwareClusters(
                    VJAILBREAK_DEFAULT_NAMESPACE,
                    credName
                );

                const clusters = clustersResponse.items.map((cluster: VMwareCluster) => ({
                    id: `${credName}:${cluster.metadata.name}`,
                    name: cluster.spec.name
                }));

                return {
                    credName,
                    datacenter,
                    clusters
                };
            });

            const newSourceData = await Promise.all(sourceDataPromises);
            setSourceData(newSourceData.filter(item => item.clusters.length > 0));
        } catch (error) {
            console.error("Failed to fetch source data:", error);
        } finally {
            setLoading(false);
        }
    };

    const fetchPcdData = async () => {
        setLoadingPCD(true);
        try {
            const openstackCreds = await getOpenstackCredentialsList(VJAILBREAK_DEFAULT_NAMESPACE);

            if (!openstackCreds || openstackCreds.length === 0) {
                setPcdData([]);
                setLoadingPCD(false);
                return;
            }

            const filteredPcds = openstackCreds
                .filter(cred => {
                    const metadata = cred.metadata as {
                        name: string,
                        namespace: string,
                        labels?: Record<string, string>
                    };
                    return metadata?.labels?.["vjailbreak.k8s.pf9.io/is-pcd"] === "true";
                })
                .map(cred => {
                    const credName = cred.metadata.name;
                    return {
                        name: credName,
                        credName: credName
                    };
                });

            setPcdData(filteredPcds);
        } catch (error) {
            console.error("Failed to fetch PCD data:", error);
        } finally {
            setLoadingPCD(false);
        }
    };

    const fetchMaasConfigs = async () => {
        try {
            setLoadingMaasConfig(true);
            const configs = await getBMConfigList(VJAILBREAK_DEFAULT_NAMESPACE);
            if (configs && configs.length > 0) {
                setMaasConfigs(configs);
                try {
                    const config = await getBMConfig(configs[0].metadata.name, VJAILBREAK_DEFAULT_NAMESPACE);
                    setSelectedMaasConfig(config);
                } catch (error) {
                    console.error(`Failed to fetch MAAS config:`, error);
                }
            }
        } catch (error) {
            console.error("Failed to fetch MAAS configs:", error);
        } finally {
            setLoadingMaasConfig(false);
        }
    };

    useEffect(() => {
        if (sourceCluster) {
            fetchClusterHosts();
            fetchClusterVMs();
        } else {
            setOrderedESXHosts([]);
            setVmsWithAssignments([]);
        }
    }, [sourceCluster]);

    const fetchClusterHosts = async () => {
        if (!sourceCluster) return;

        setLoadingHosts(true);
        try {
            const parts = sourceCluster.split(":");
            const credName = parts[0];

            const sourceDataItem = sourceData.find(item => item.credName === credName);
            const clusterObj = sourceDataItem?.clusters.find(cluster =>
                cluster.id === sourceCluster
            );
            const clusterName = clusterObj?.name;

            if (!clusterName) {
                setOrderedESXHosts([]);
                setLoadingHosts(false);
                return;
            }

            const hostsResponse = await getVMwareHosts(
                VJAILBREAK_DEFAULT_NAMESPACE,
                // credName,
                "",
                clusterName
            );

            const mappedHosts: ESXHost[] = hostsResponse.items.map((host: VMwareHost) => ({
                id: host.metadata.name,
                name: host.spec.name,
                ip: "",
                bmcIp: "",
                maasState: "Unknown",
                vms: 0,
                state: "Active"
            }));

            setOrderedESXHosts(mappedHosts);
        } catch (error) {
            console.error("Failed to fetch cluster hosts:", error);
        } finally {
            setLoadingHosts(false);
        }
    };

    const fetchClusterVMs = async () => {
        if (!sourceCluster) return;

        setLoadingVMs(true);
        try {
            const parts = sourceCluster.split(":");
            const credName = parts[0];

            const sourceDataItem = sourceData.find(item => item.credName === credName);
            const clusterObj = sourceDataItem?.clusters.find(cluster =>
                cluster.id === sourceCluster
            );
            const clusterName = clusterObj?.name;

            if (!clusterName) {
                setVmsWithAssignments([]);
                setLoadingVMs(false);
                return;
            }

            const vmsResponse = await getVMwareMachines(
                VJAILBREAK_DEFAULT_NAMESPACE,
                credName
            );

            const filteredVMs = vmsResponse.items.filter((vm: VMwareMachine) => {
                const clusterLabel = vm.metadata?.labels?.[`vjailbreak.k8s.pf9.io/cluster-name`];
                return clusterLabel === clusterName;
            });

            const mappedVMs: VM[] = filteredVMs.map((vm: VMwareMachine) => {
                const esxiHost = vm.metadata?.labels?.[`vjailbreak.k8s.pf9.io/esxi-name`] || "";

                return {
                    id: vm.metadata.name,
                    name: vm.spec.vms.name || vm.metadata.name,
                    ip: vm.spec.vms.ipAddress || "—",
                    powerState: vm.status.powerState === "running" ? "powered-on" : "powered-off",
                    sg: "default",
                    tenant: "service",
                    esxHost: esxiHost,
                    networks: vm.spec.vms.networks,
                    datastores: vm.spec.vms.datastores
                };
            });

            setVmsWithAssignments(mappedVMs);
        } catch (error) {
            console.error("Failed to fetch cluster VMs:", error);
            setVmsWithAssignments([]);
        } finally {
            setLoadingVMs(false);
        }
    };

    useEffect(() => {
        if (orderedESXHosts.length > 0 && vmsWithAssignments.length > 0) {
            const esxHostOrder = new Map();
            orderedESXHosts.forEach((host, index) => {
                esxHostOrder.set(host.id, index);
            });

            const sortedVMs = [...vmsWithAssignments].sort((a, b) => {
                const aHostIndex = esxHostOrder.get(a.esxHost) ?? 999;
                const bHostIndex = esxHostOrder.get(b.esxHost) ?? 999;
                return aHostIndex - bHostIndex;
            });

            setVmsWithAssignments(sortedVMs);
        }
    }, [orderedESXHosts]);

    const handleCloseMaasConfig = () => {
        setMaasConfigDialogOpen(false);
    };

    const handleSourceClusterChange = (event) => {
        const value = event.target.value;
        setSourceCluster(value);

        if (value) {
            const parts = value.split(":");
            const credName = parts[0];
            setSelectedVMwareCredName(credName);
        } else {
            setSelectedVMwareCredName("");
        }
    };

    const handleDestinationPCDChange = (event) => {
        const value = event.target.value;
        setDestinationPCD(value);
        setSelectedPcdCredName(value);
    };

    const findItemIndices = <T extends { id: string | number }>(items: T[], selectedIds: readonly (string | number)[]) => {
        const indices: number[] = [];
        for (const id of selectedIds) {
            const index = items.findIndex(item => item.id === id);
            if (index !== -1) {
                indices.push(index);
            }
        }
        return indices.sort((a, b) => a - b);
    };

    const availableVmwareNetworks = useMemo(() => {
        if (!vmsWithAssignments.length || !selectedVMs.length) return [];

        const selectedVMsData = vmsWithAssignments.filter(vm =>
            selectedVMs.includes(vm.id));

        const extractedNetworks = selectedVMsData
            .filter(vm => vm.networks)
            .flatMap(vm => vm.networks || []);

        if (extractedNetworks.length > 0) {
            return Array.from(new Set(extractedNetworks)).sort();
        }

        return Array.from(new Set(["VM Network", "Management Network", "Storage Network"]));
    }, [vmsWithAssignments, selectedVMs]);

    const availableVmwareDatastores = useMemo(() => {
        if (!vmsWithAssignments.length || !selectedVMs.length) return [];

        const selectedVMsData = vmsWithAssignments.filter(vm =>
            selectedVMs.includes(vm.id));

        const extractedDatastores = selectedVMsData
            .filter(vm => vm.datastores)
            .flatMap(vm => vm.datastores || []);

        if (extractedDatastores.length > 0) {
            return Array.from(new Set(extractedDatastores)).sort();
        }

        return Array.from(new Set(["datastore1", "datastore2", "ssd-storage"]));
    }, [vmsWithAssignments, selectedVMs]);

    const openstackNetworks = useMemo(() => {
        if (!destinationPCD) return [];

        const selectedPcd = pcdData.find(item => item.credName === destinationPCD);

        const networks = selectedPcd ? ["private", "public", "storage"] : [];
        return networks;
    }, [destinationPCD, pcdData]);

    const openstackVolumeTypes = useMemo(() => {
        if (!destinationPCD) return [];

        const selectedPcd = pcdData.find(item => item.credName === destinationPCD);

        const volumeTypes = selectedPcd ? ["ceph", "local", "ssd"] : [];
        return volumeTypes;
    }, [destinationPCD, pcdData]);

    const handleMappingsChange = (key: string) => (value: ResourceMap[]) => {
        if (key === "networkMappings") {
            setNetworkMappings(value);
            setNetworkMappingError("");
        } else if (key === "storageMappings") {
            setStorageMappings(value);
            setStorageMappingError("");
        }
    };

    const handleSubmit = async () => {
        setSubmitting(true);

        if (selectedVMs.length > 0) {
            if (availableVmwareNetworks.some(network =>
                !networkMappings.some(mapping => mapping.source === network))) {
                setNetworkMappingError("All networks from selected VMs must be mapped");
                setSubmitting(false);
                return;
            }

            if (availableVmwareDatastores.some(datastore =>
                !storageMappings.some(mapping => mapping.source === datastore))) {
                setStorageMappingError("All datastores from selected VMs must be mapped");
                setSubmitting(false);
                return;
            }
        } else if (sourceCluster && destinationPCD) {
            alert("Please select at least one VM to migrate");
            setSubmitting(false);
            return;
        }

        try {
            const parts = sourceCluster.split(":");
            const credName = parts[0];

            const sourceDataItem = sourceData.find(item => item.credName === credName);
            const clusterObj = sourceDataItem?.clusters.find(cluster =>
                cluster.id === sourceCluster
            );
            const clusterName = clusterObj?.name || "";

            const selectedVMsData = vmsWithAssignments
                .filter(vm => selectedVMs.includes(vm.id))
                .map(vm => ({
                    vmName: vm.name,
                    esxiName: vm.esxHost
                })) as VMSequence[];

            const migrationPlanJson = createRollingMigrationPlanJson({
                clusterName,
                vms: selectedVMsData,
                vmwareCredsRef: {
                    name: selectedVMwareCredName,
                },
                openstackCredsRef: {
                    name: selectedPcdCredName,
                },
                bmConfigRef: {
                    name: selectedMaasConfig?.metadata.name || "",
                },
                networkMappings: networkMappings.map(mapping => ({
                    source: mapping.source,
                    destination: mapping.target
                })),
                storageMappings: storageMappings.map(mapping => ({
                    source: mapping.source,
                    destination: mapping.target
                })),
                namespace: VJAILBREAK_DEFAULT_NAMESPACE
            });

            await postRollingMigrationPlan(migrationPlanJson, VJAILBREAK_DEFAULT_NAMESPACE);

            console.log("Submitted rolling migration plan", migrationPlanJson);
            onClose();
        } catch (error) {
            console.error("Failed to submit rolling migration plan:", error);
            const errorMessage = error instanceof Error ? error.message : "Unknown error";
            alert(`Failed to submit rolling migration plan: ${errorMessage}`);
        } finally {
            setSubmitting(false);
        }
    };

    const handleVMMoveUp = () => {
        if (selectedVMs.length === 0) return;

        const newOrderedVMs = [...vmsWithAssignments];
        const indices = findItemIndices(newOrderedVMs, selectedVMs);

        for (const index of indices) {
            if (index > 0) {
                [newOrderedVMs[index - 1], newOrderedVMs[index]] =
                    [newOrderedVMs[index], newOrderedVMs[index - 1]];
            }
        }

        setVmsWithAssignments(newOrderedVMs);
    };

    const handleVMMoveDown = () => {
        if (selectedVMs.length === 0) return;

        const newOrderedVMs = [...vmsWithAssignments];
        const indices = findItemIndices(newOrderedVMs, selectedVMs);

        for (let i = indices.length - 1; i >= 0; i--) {
            const index = indices[i];
            if (index < newOrderedVMs.length - 1) {
                [newOrderedVMs[index], newOrderedVMs[index + 1]] =
                    [newOrderedVMs[index + 1], newOrderedVMs[index]];
            }
        }

        setVmsWithAssignments(newOrderedVMs);
    };

    const handleVMMoveToTop = () => {
        if (selectedVMs.length === 0) return;

        const selectedItems = vmsWithAssignments.filter(item =>
            selectedVMs.includes(item.id as string | number)
        );

        const remainingItems = vmsWithAssignments.filter(item =>
            !selectedVMs.includes(item.id as string | number)
        );

        setVmsWithAssignments([...selectedItems, ...remainingItems]);
    };

    const handleVMMoveToBottom = () => {
        if (selectedVMs.length === 0) return;

        const selectedItems = vmsWithAssignments.filter(item =>
            selectedVMs.includes(item.id as string | number)
        );

        const remainingItems = vmsWithAssignments.filter(item =>
            !selectedVMs.includes(item.id as string | number)
        );

        setVmsWithAssignments([...remainingItems, ...selectedItems]);
    };

    const handleOpenSGDialog = () => {
        if (selectedVMs.length === 0) return;
        setSgDialogOpen(true);
    };

    const handleCloseSGDialog = () => {
        setSgDialogOpen(false);
        setSelectedSG("");
    };

    const handleSGChange = (event) => {
        setSelectedSG(event.target.value);
    };

    const handleApplySG = () => {
        if (!selectedSG) {
            handleCloseSGDialog();
            return;
        }

        const updatedVMs = vmsWithAssignments.map(vm => {
            if (selectedVMs.includes(vm.id)) {
                return {
                    ...vm,
                    sg: selectedSG
                };
            }
            return vm;
        });

        setVmsWithAssignments(updatedVMs);
        handleCloseSGDialog();
    };

    const handleOpenTenantDialog = () => {
        if (selectedVMs.length === 0) return;
        setTenantDialogOpen(true);
    };

    const handleCloseTenantDialog = () => {
        setTenantDialogOpen(false);
        setSelectedTenant("");
    };

    const handleTenantChange = (event) => {
        setSelectedTenant(event.target.value);
    };

    const handleApplyTenant = () => {
        if (!selectedTenant) {
            handleCloseTenantDialog();
            return;
        }

        const updatedVMs = vmsWithAssignments.map(vm => {
            if (selectedVMs.includes(vm.id)) {
                return {
                    ...vm,
                    tenant: selectedTenant
                };
            }
            return vm;
        });

        setVmsWithAssignments(updatedVMs);
        handleCloseTenantDialog();
    };

    const handleClose = () => {
        if (!submitting) {
            onClose();
        }
    };

    const isSubmitDisabled = useMemo(() => {
        const basicRequirementsMissing = !sourceCluster ||
            !destinationPCD ||
            !selectedMaasConfig ||
            submitting;

        if (selectedVMs.length === 0) {
            return basicRequirementsMissing;
        }

        const mappingsValid = !(availableVmwareNetworks.some(network =>
            !networkMappings.some(mapping => mapping.source === network)) ||
            availableVmwareDatastores.some(datastore =>
                !storageMappings.some(mapping => mapping.source === datastore)));

        return basicRequirementsMissing || !mappingsValid;
    }, [
        sourceCluster,
        destinationPCD,
        selectedMaasConfig,
        submitting,
        selectedVMs,
        availableVmwareNetworks,
        networkMappings,
        availableVmwareDatastores,
        storageMappings
    ]);

    useKeyboardSubmit({
        open,
        isSubmitDisabled: isSubmitDisabled,
        onSubmit: handleSubmit,
        onClose: handleClose
    });

    const handleViewMaasConfig = () => {
        setMaasDetailsModalOpen(true);
    };

    const handleCloseMaasDetailsModal = () => {
        setMaasDetailsModalOpen(false);
    };

    return (
        <StyledDrawer
            anchor="right"
            open={open}
            onClose={handleClose}
            ModalProps={{ keepMounted: false }}
        >
            <Header title="Rolling Migration Plan" />
            <DrawerContent>
                <Box sx={{ display: "grid", gap: 4 }}>
                    <Box>
                        <Step stepNumber="1" label="Source & Destination" />
                        <Box sx={{ ml: 5, mt: 2 }}>
                            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 4 }}>
                                <Box>
                                    <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: "500" }}>Source</Typography>
                                    <FormControl fullWidth variant="outlined" size="small">
                                        <Select
                                            value={sourceCluster}
                                            onChange={handleSourceClusterChange}
                                            displayEmpty
                                            disabled={loading}
                                            renderValue={(selected) => {
                                                if (!selected) return <em>Select Cluster</em>;
                                                const parts = selected.split(":");
                                                const credName = parts[0];
                                                const sourceDataItem = sourceData.find(item => item.credName === credName);
                                                const cluster = sourceDataItem?.clusters.find(c => c.id === selected);
                                                return `${sourceDataItem?.credName} - ${sourceDataItem?.datacenter || ""} - ${cluster?.name || ""}`;
                                            }}
                                            MenuProps={{
                                                PaperProps: {
                                                    style: {
                                                        maxHeight: 300
                                                    }
                                                }
                                            }}
                                        >
                                            <MenuItem value="" disabled><em>Select Cluster</em></MenuItem>

                                            {loading ? (
                                                <MenuItem disabled>Loading...</MenuItem>
                                            ) : sourceData.length === 0 ? (
                                                <MenuItem disabled>No clusters found</MenuItem>
                                            ) : (
                                                Object.entries(
                                                    sourceData.reduce((acc, item) => {
                                                        if (!acc[item.credName]) {
                                                            acc[item.credName] = {};
                                                        }
                                                        acc[item.credName][item.datacenter] = item.clusters;
                                                        return acc;
                                                    }, {} as Record<string, Record<string, { id: string; name: string }[]>>)
                                                ).map(([credName, datacenters]) => [
                                                    <ListSubheader key={credName} sx={{ fontWeight: 700 }}>
                                                        <Box sx={{ display: 'flex', alignItems: 'center' }}>
                                                            <VpnKeyIcon sx={{ mr: 1 }} />
                                                            {credName}
                                                        </Box>
                                                    </ListSubheader>,
                                                    ...Object.entries(datacenters).map(([datacenterName, clusters]) => [
                                                        <ListSubheader key={`${credName}-${datacenterName}`} sx={{ fontWeight: 600, pl: 4 }}>
                                                            <Box sx={{ display: 'flex', alignItems: 'center' }}>
                                                                <DatacenterIcon sx={{ mr: 1 }} fontSize="small" />
                                                                {datacenterName}
                                                            </Box>
                                                        </ListSubheader>,
                                                        ...clusters.map((cluster) => (
                                                            <MenuItem
                                                                key={cluster.id}
                                                                value={cluster.id}
                                                                sx={{ pl: 7 }}
                                                            >
                                                                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                                                                    <ClusterIcon sx={{ mr: 1 }} fontSize="small" />
                                                                    {cluster.name}
                                                                </Box>
                                                            </MenuItem>
                                                        ))
                                                    ])
                                                ]).flat()
                                            )}
                                        </Select>
                                    </FormControl>
                                </Box>
                                <Box>
                                    <Typography variant="subtitle2" sx={{ mb: 1, fontWeight: "500" }}>Destination</Typography>
                                    <FormControl fullWidth variant="outlined" size="small">
                                        <Select
                                            value={destinationPCD}
                                            onChange={handleDestinationPCDChange}
                                            displayEmpty
                                            disabled={!sourceCluster || loadingPCD}
                                            renderValue={(selected) => {
                                                if (!selected) return <em>Select Destination</em>;
                                                const pcd = pcdData.find(p => p.credName === selected);
                                                return pcd?.name || selected;
                                            }}
                                            MenuProps={{
                                                PaperProps: {
                                                    style: {
                                                        maxHeight: 300
                                                    }
                                                }
                                            }}
                                        >
                                            <MenuItem value="" disabled><em>Select Destination</em></MenuItem>

                                            {loadingPCD ? (
                                                <MenuItem disabled>Loading...</MenuItem>
                                            ) : pcdData.length === 0 ? (
                                                <MenuItem disabled>No PCD destinations found</MenuItem>
                                            ) : (
                                                pcdData.map((pcd) => (
                                                    <MenuItem
                                                        key={pcd.credName}
                                                        value={pcd.credName}
                                                    >
                                                        {pcd.name}
                                                    </MenuItem>
                                                ))
                                            )}
                                        </Select>
                                    </FormControl>
                                </Box>
                            </Box>
                        </Box>
                    </Box>

                    <Box>
                        <Step stepNumber="2" label="MAAS Config (Verify the configuration)" />
                        <Box sx={{ ml: 5, mt: 1 }}>
                            {loadingMaasConfig ? (
                                <Typography variant="body2">Loading MAAS Config...</Typography>
                            ) : maasConfigs.length === 0 ? (
                                <Typography variant="body2">No MAAS Config available</Typography>
                            ) : (
                                <Typography
                                    variant="subtitle2"
                                    component="a"
                                    sx={{
                                        color: 'primary.main',
                                        textDecoration: 'underline',
                                        cursor: 'pointer',
                                        fontWeight: "500"
                                    }}
                                    onClick={handleViewMaasConfig}
                                >
                                    View MAAS Config Details
                                </Typography>
                            )}
                        </Box>
                    </Box>

                    <Box>
                        <Step stepNumber="3" label="ESXi Hosts" />
                        <Box sx={{ ml: 5, mt: 2 }}>
                            <Paper sx={{ width: "100%", height: 389 }}>
                                <DataGrid
                                    rows={orderedESXHosts}
                                    columns={esxColumns}
                                    initialState={{
                                        pagination: { paginationModel },
                                        columns: {
                                            columnVisibilityModel: {}
                                        }
                                    }}
                                    pageSizeOptions={[5, 10, 25]}
                                    rowHeight={45}
                                    slots={{
                                        toolbar: CustomESXToolbarWithActions
                                    }}
                                    disableColumnMenu
                                    disableColumnFilter
                                    loading={loadingHosts}
                                />
                            </Paper>
                        </Box>
                    </Box>


                    <Box>
                        <Step stepNumber="4" label="VM Migration Sequence" />
                        <Box sx={{ ml: 5, mt: 2 }}>
                            <Paper sx={{ width: "100%", height: 389 }}>
                                <DataGrid
                                    rows={vmsWithAssignments}
                                    columns={vmColumns}
                                    initialState={{
                                        pagination: { paginationModel },
                                        columns: {
                                            columnVisibilityModel: {}
                                        }
                                    }}
                                    pageSizeOptions={[5, 10, 25]}
                                    rowHeight={45}
                                    checkboxSelection
                                    onRowSelectionModelChange={setSelectedVMs}
                                    rowSelectionModel={selectedVMs}
                                    slots={{
                                        toolbar: (props) => (
                                            <CustomToolbarWithActions
                                                {...props}
                                                rowSelectionModel={selectedVMs}
                                                onAssignSG={handleOpenSGDialog}
                                                onAssignTenant={handleOpenTenantDialog}
                                                onMoveUp={handleVMMoveUp}
                                                onMoveDown={handleVMMoveDown}
                                                onMoveToTop={handleVMMoveToTop}
                                                onMoveToBottom={handleVMMoveToBottom}
                                            />
                                        ),
                                        noRowsOverlay: () => (
                                            <Box sx={{
                                                display: 'flex',
                                                flexDirection: 'column',
                                                alignItems: 'center',
                                                justifyContent: 'center',
                                                height: '100%',
                                                p: 2
                                            }}>
                                                <Typography variant="body1" color="text.secondary" align="center" sx={{ mb: 1 }}>
                                                    {loadingVMs ? 'Loading VMs...' : 'No VMs found for the selected cluster.'}
                                                </Typography>
                                            </Box>
                                        )
                                    }}
                                    disableColumnMenu
                                    disableColumnFilter
                                    loading={loadingVMs}
                                />
                            </Paper>
                        </Box>
                    </Box>

                    <Box>
                        {sourceCluster && destinationPCD ? (
                            <>
                                <NetworkAndStorageMappingStep
                                    vmwareNetworks={availableVmwareNetworks}
                                    vmWareStorage={availableVmwareDatastores}
                                    openstackNetworks={openstackNetworks}
                                    openstackStorage={openstackVolumeTypes}
                                    params={{
                                        networkMappings: networkMappings,
                                        storageMappings: storageMappings
                                    }}
                                    onChange={handleMappingsChange}
                                    networkMappingError={networkMappingError}
                                    storageMappingError={storageMappingError}
                                    stepNumber="5"
                                />
                            </>
                        ) : (
                            <Typography variant="body2" color="text.secondary">
                                Please select both source cluster and destination PCD to configure mappings.
                            </Typography>
                        )}
                    </Box>
                </Box>
            </DrawerContent>
            <Footer
                submitButtonLabel="Run"
                onClose={handleClose}
                onSubmit={handleSubmit}
                disableSubmit={isSubmitDisabled}
                submitting={submitting}
            />

            <Dialog
                open={sgDialogOpen}
                onClose={handleCloseSGDialog}
                fullWidth
                maxWidth="sm"
            >
                <DialogTitle>
                    Assign Security Group to {selectedVMs.length} {selectedVMs.length === 1 ? 'VM' : 'VMs'}
                </DialogTitle>
                <DialogContent>
                    <Box sx={{ my: 2 }}>
                        <FormLabel>Select Security Group</FormLabel>
                        <Select
                            fullWidth
                            value={selectedSG}
                            onChange={handleSGChange}
                            size="small"
                            sx={{ mt: 1 }}
                            displayEmpty
                        >
                            <MenuItem value="">
                                <em>Select a security group</em>
                            </MenuItem>
                            {mockSecurityGroups.map((sg) => (
                                <MenuItem key={sg.id} value={sg.id}>
                                    {sg.name}
                                </MenuItem>
                            ))}
                        </Select>
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={handleCloseSGDialog}>Cancel</Button>
                    <Button
                        onClick={handleApplySG}
                        variant="contained"
                        color="primary"
                        disabled={!selectedSG}
                    >
                        Apply to selected VMs
                    </Button>
                </DialogActions>
            </Dialog>

            <Dialog
                open={tenantDialogOpen}
                onClose={handleCloseTenantDialog}
                fullWidth
                maxWidth="sm"
            >
                <DialogTitle>
                    Assign Tenant to {selectedVMs.length} {selectedVMs.length === 1 ? 'VM' : 'VMs'}
                </DialogTitle>
                <DialogContent>
                    <Box sx={{ my: 2 }}>
                        <FormLabel>Select Tenant</FormLabel>
                        <Select
                            fullWidth
                            value={selectedTenant}
                            onChange={handleTenantChange}
                            size="small"
                            sx={{ mt: 1 }}
                            displayEmpty
                        >
                            <MenuItem value="">
                                <em>Select a tenant</em>
                            </MenuItem>
                            {mockTenants.map((tenant) => (
                                <MenuItem key={tenant.id} value={tenant.id}>
                                    {tenant.name}
                                </MenuItem>
                            ))}
                        </Select>
                    </Box>
                </DialogContent>
                <DialogActions>
                    <Button onClick={handleCloseTenantDialog}>Cancel</Button>
                    <Button
                        onClick={handleApplyTenant}
                        variant="contained"
                        color="primary"
                        disabled={!selectedTenant}
                    >
                        Apply to selected VMs
                    </Button>
                </DialogActions>
            </Dialog>

            <MaasConfigDialog
                open={maasConfigDialogOpen}
                onClose={handleCloseMaasConfig}
                aria-labelledby="maas-config-dialog-title"
            >
                <DialogTitle id="maas-config-dialog-title">
                    <Typography variant="h6">ESXi - MAAS Configuration</Typography>
                </DialogTitle>
                <DialogContent dividers>
                    {loadingMaasConfig ? (
                        <Typography>Loading configuration details...</Typography>
                    ) : !selectedMaasConfig ? (
                        <Typography>No configuration available</Typography>
                    ) : (
                        <>
                            <ConfigSection>
                                <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>Provider Configuration</Typography>
                                <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3, ml: 1 }}>
                                    <ConfigField>
                                        <FieldLabel>Provider Type:</FieldLabel>
                                        <FieldValue>{selectedMaasConfig.spec.providerType}</FieldValue>
                                    </ConfigField>
                                    <ConfigField>
                                        <FieldLabel>MAAS URL:</FieldLabel>
                                        <FieldValue>{selectedMaasConfig.spec.apiUrl}</FieldValue>
                                    </ConfigField>
                                    <ConfigField>
                                        <FieldLabel>Insecure:</FieldLabel>
                                        <FieldValue>{selectedMaasConfig.spec.insecure ? "Yes" : "No"}</FieldValue>
                                    </ConfigField>
                                    {selectedMaasConfig.spec.os && (
                                        <ConfigField>
                                            <FieldLabel>OS:</FieldLabel>
                                            <FieldValue>{selectedMaasConfig.spec.os}</FieldValue>
                                        </ConfigField>
                                    )}
                                    <ConfigField>
                                        <FieldLabel>Status:</FieldLabel>
                                        <FieldValue>
                                            {selectedMaasConfig.status?.validationStatus || "Pending validation"}
                                        </FieldValue>
                                    </ConfigField>
                                    {selectedMaasConfig.status?.validationMessage && (
                                        <ConfigField>
                                            <FieldLabel>Validation Message:</FieldLabel>
                                            <FieldValue>{selectedMaasConfig.status.validationMessage}</FieldValue>
                                        </ConfigField>
                                    )}
                                </Box>
                            </ConfigSection>

                            {selectedMaasConfig.spec.userDataSecretRef && (
                                <ConfigSection>
                                    <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>Cloud-Init Configuration</Typography>
                                    <Typography variant="caption" sx={{ mb: 1, display: 'block', color: 'text.secondary' }}>
                                        User data is stored in a secret: {selectedMaasConfig.spec.userDataSecretRef.name}
                                    </Typography>
                                    <CodeEditorContainer>
                                        <SyntaxHighlighter
                                            language="yaml"
                                            style={oneLight}
                                            showLineNumbers
                                            wrapLongLines
                                            customStyle={{
                                                margin: 0,
                                                maxHeight: '100%',
                                            }}
                                        >
                                            {`# Cloud-init configuration is stored in Kubernetes Secret: 
# ${selectedMaasConfig.spec.userDataSecretRef.name}
# in namespace: ${selectedMaasConfig.spec.userDataSecretRef.namespace || VJAILBREAK_DEFAULT_NAMESPACE}

# The cloud-init configuration includes:
# - package updates and installations
# - configuration files
# - commands to run on startup
# - network configuration
# - and other system setup parameters

# This will be used when provisioning ESXi hosts in the bare metal environment.`}
                                        </SyntaxHighlighter>
                                    </CodeEditorContainer>
                                </ConfigSection>
                            )}

                            <ConfigSection>
                                <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>Resource Information</Typography>
                                <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3, ml: 1 }}>
                                    <ConfigField>
                                        <FieldLabel>Name:</FieldLabel>
                                        <FieldValue>{selectedMaasConfig.metadata.name}</FieldValue>
                                    </ConfigField>
                                    <ConfigField>
                                        <FieldLabel>Namespace:</FieldLabel>
                                        <FieldValue>{selectedMaasConfig.metadata.namespace}</FieldValue>
                                    </ConfigField>
                                    <ConfigField>
                                        <FieldLabel>Created:</FieldLabel>
                                        <FieldValue>
                                            {new Date(selectedMaasConfig.metadata.creationTimestamp).toLocaleString()}
                                        </FieldValue>
                                    </ConfigField>
                                </Box>
                            </ConfigSection>
                        </>
                    )}
                </DialogContent>
                <DialogActions>
                    <Button variant="contained" onClick={handleCloseMaasConfig}>
                        Close
                    </Button>
                </DialogActions>
            </MaasConfigDialog>

            {
                maasConfigs && maasConfigs.length > 0 && (
                    <MaasConfigDetailsModal
                        open={maasDetailsModalOpen}
                        onClose={handleCloseMaasDetailsModal}
                        configName={maasConfigs[0].metadata.name}
                        namespace={maasConfigs[0].metadata.namespace}
                    />
                )
            }
        </StyledDrawer >
    );
} 