import { Box, Typography, Drawer, styled, Paper, Tooltip, Button, Dialog, DialogTitle, DialogContent, DialogActions, Alert, Select, MenuItem, GlobalStyles, FormLabel, Snackbar } from "@mui/material"
import ClusterIcon from "@mui/icons-material/Hub"
import React, { useState, useMemo, useEffect, useCallback } from "react"
import { DataGrid, GridColDef, GridRowSelectionModel, GridToolbarColumnsButton } from "@mui/x-data-grid"
import { useNavigate } from "react-router-dom"
import Footer from "../../components/forms/Footer"
import Header from "../../components/forms/Header"
import Step from "../../components/forms/Step"
import { DrawerContent } from "src/components/forms/StyledDrawer"
import { useKeyboardSubmit } from "src/hooks/ui/useKeyboardSubmit"
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism'
import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { getVMwareHosts, patchVMwareHost } from "src/api/vmware-hosts/vmwareHosts"
import { getVMwareMachines, patchVMwareMachine } from "src/api/vmware-machines/vmwareMachines"
import { VMwareHost } from "src/api/vmware-hosts/model"
import { VMwareMachine } from "src/api/vmware-machines/model"
import { VJAILBREAK_DEFAULT_NAMESPACE } from "src/api/constants"
import { getBMConfigList, getBMConfig } from "src/api/bmconfig/bmconfig"
import { BMConfig } from "src/api/bmconfig/model"
import MaasConfigDetailsModal from "src/pages/dashboard/BMConfigDetailsModal"
import { getOpenstackCredentials } from "src/api/openstack-creds/openstackCreds"
import { OpenstackCreds } from "src/api/openstack-creds/model"
import NetworkAndStorageMappingStep, { ResourceMap } from "./NetworkAndStorageMappingStep"
import { createRollingMigrationPlanJson, postRollingMigrationPlan, VMSequence, ClusterMapping } from "src/api/rolling-migration-plans"
import SourceDestinationClusterSelection from "./SourceDestinationClusterSelection"
// Import required APIs for creating migration resources
import { createNetworkMappingJson } from "src/api/network-mapping/helpers"
import { postNetworkMapping } from "src/api/network-mapping/networkMappings"
import { createStorageMappingJson } from "src/api/storage-mappings/helpers"
import { postStorageMapping } from "src/api/storage-mappings/storageMappings"
import { createMigrationTemplateJson } from "src/api/migration-templates/helpers"
import { postMigrationTemplate } from "src/api/migration-templates/migrationTemplates"
import useParams from "src/hooks/useParams"
import MigrationOptions from "./MigrationOptionsAlt"
import { CUTOVER_TYPES } from "./constants"
import WindowsIcon from "src/assets/windows_icon.svg";
import LinuxIcon from "src/assets/linux_icon.svg";
import WarningIcon from '@mui/icons-material/Warning';
import { useClusterData } from "./useClusterData"
import { useErrorHandler } from "src/hooks/useErrorHandler"

import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import { TextField, CircularProgress } from "@mui/material"
import { validateOpenstackIPs } from "src/api/openstack-creds/openstackCreds"

// Import CDS icons
import "@cds/core/icon/register.js"
import { ClarityIcons, buildingIcon, clusterIcon, hostIcon, vmIcon } from "@cds/core/icon"
import { getSecret } from "src/api/secrets/secrets"
import { useAmplitude } from "src/hooks/useAmplitude"
import { AMPLITUDE_EVENTS } from "src/types/amplitude"

// Define types for MigrationOptions
interface FormValues extends Record<string, unknown> {
    dataCopyMethod?: string;
    dataCopyStartTime?: string;
    cutoverOption?: string;
    cutoverStartTime?: string;
    cutoverEndTime?: string;
    postMigrationScript?: string;
    retryOnFailure?: boolean;
    osFamily?: string;
}

export interface SelectedMigrationOptionsType extends Record<string, unknown> {
    dataCopyMethod: boolean;
    dataCopyStartTime: boolean;
    cutoverOption: boolean;
    cutoverStartTime: boolean;
    cutoverEndTime: boolean;
    postMigrationScript: boolean;
    osFamily: boolean;
}

// Default state for checkboxes
const defaultMigrationOptions = {
    dataCopyMethod: false,
    dataCopyStartTime: false,
    cutoverOption: false,
    cutoverStartTime: false,
    cutoverEndTime: false,
    postMigrationScript: false,
    osFamily: false,
}

type FieldErrors = { [formId: string]: string };

// Register clarity icons
ClarityIcons.addIcons(buildingIcon, clusterIcon, hostIcon, vmIcon)

// Style for Clarity icons
const CdsIconWrapper = styled('div')({
    marginRight: 8,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center'
});

const StyledDrawer = styled(Drawer)(({ theme }) => ({
    "& .MuiDrawer-paper": {
        display: "grid",
        gridTemplateRows: "max-content 1fr max-content",
        width: "1400px",
        maxWidth: "90vw",
        zIndex: theme.zIndex.modal,
    },
    "& .hidden-column": {
        display: "none"
    },
}))

interface ESXHost {
    id: string;
    name: string;
    ip: string;
    bmcIp: string;
    maasState: string;
    vms: number;
    state: string;
    pcdHostConfigName?: string;
}

interface VM {
    id: string;
    name: string;
    ip: string;
    esxHost: string;
    networks?: string[];
    datastores?: string[];
    cpu?: number;
    memory?: number;
    powerState: string;
    osFamily?: string;
    flavor?: string;
    targetFlavorId?: string;
    ipValidationStatus?: 'pending' | 'valid' | 'invalid' | 'validating';
    ipValidationMessage?: string;
    networkInterfaces?: VmNetworkInterface[]
}

export interface VmNetworkInterface {
    mac: string
    network: string
    ipAddress: string
}

// ESX columns will be defined inside the component

const CustomToolbarWithActions = (props) => {
    const { rowSelectionModel, onAssignFlavor, ...toolbarProps } = props;

    return (
        <Box sx={{ display: 'flex', justifyContent: 'space-between', width: '100%', padding: '4px 8px' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <GridToolbarColumnsButton />
            </Box>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                {rowSelectionModel && rowSelectionModel.length > 0 && (
                    <Button
                        variant="text"
                        color="primary"
                        onClick={onAssignFlavor}
                        size="small"
                    >
                        Assign Flavor ({rowSelectionModel.length})
                    </Button>
                )}
                <CustomSearchToolbar {...toolbarProps} />
            </Box>
        </Box>
    );
};

const CustomESXToolbarWithActions = (props) => {
    const { onAssignHostConfig, ...toolbarProps } = props;

    return (
        <Box sx={{ display: 'flex', justifyContent: 'flex-end', width: '100%', padding: '4px 8px' }}>
            <Button
                variant="text"
                color="primary"
                onClick={onAssignHostConfig}
                size="small"
                sx={{ ml: 1 }}
            >
                Assign Host Config
            </Button>
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


interface RollingMigrationFormDrawerProps {
    open: boolean;
    onClose: () => void;
}

export default function RollingMigrationFormDrawer({
    open,
    onClose,
}: RollingMigrationFormDrawerProps) {
    const navigate = useNavigate();
    const { reportError } = useErrorHandler({ component: "RollingMigrationForm" });
    const { track } = useAmplitude({ component: "RollingMigrationForm" });
    const [sourceCluster, setSourceCluster] = useState("");
    const [destinationPCD, setDestinationPCD] = useState("");
    const [submitting, setSubmitting] = useState(false);

    const [selectedVMwareCredName, setSelectedVMwareCredName] = useState("");

    const [selectedPcdCredName, setSelectedPcdCredName] = useState("");

    const [selectedVMs, setSelectedVMs] = useState<GridRowSelectionModel>([]);
    const [pcdHostConfigDialogOpen, setPcdHostConfigDialogOpen] = useState(false);
    const [selectedPcdHostConfig, setSelectedPcdHostConfig] = useState("");
    const [updatingPcdMapping, setUpdatingPcdMapping] = useState(false);

    const [loadingHosts, setLoadingHosts] = useState(false);
    const [loadingVMs, setLoadingVMs] = useState(false);

    const [orderedESXHosts, setOrderedESXHosts] = useState<ESXHost[]>([]);
    const [vmsWithAssignments, setVmsWithAssignments] = useState<VM[]>([]);

    const [maasConfigDialogOpen, setMaasConfigDialogOpen] = useState(false);
    const [maasConfigs, setMaasConfigs] = useState<BMConfig[]>([]);
    const [selectedMaasConfig, setSelectedMaasConfig] = useState<BMConfig | null>(null);
    const [loadingMaasConfig, setLoadingMaasConfig] = useState(false);
    const [maasDetailsModalOpen, setMaasDetailsModalOpen] = useState(false);

    const [networkMappings, setNetworkMappings] = useState<ResourceMap[]>([]);
    const [storageMappings, setStorageMappings] = useState<ResourceMap[]>([]);
    const [networkMappingError, setNetworkMappingError] = useState<string>("");
    const [storageMappingError, setStorageMappingError] = useState<string>("");

    const [openstackCredData, setOpenstackCredData] = useState<OpenstackCreds | null>(null);
    const [loadingOpenstackDetails, setLoadingOpenstackDetails] = useState(false);

    // IP editing and validation state - updated for multiple interfaces
    const [editingIpFor, setEditingIpFor] = useState<string | null>(null);
    const [editingInterfaceIndex, setEditingInterfaceIndex] = useState<number | null>(null);
    const [tempIpValue, setTempIpValue] = useState<string>("");

    // Modal state for multi-NIC IP editing
    const [ipEditModalOpen, setIpEditModalOpen] = useState(false);
    const [editingVm, setEditingVm] = useState<VM | null>(null);
    const [modalIpValues, setModalIpValues] = useState<Record<string, string>>({});
    const [modalValidationStatus, setModalValidationStatus] = useState<Record<string, 'pending' | 'valid' | 'invalid' | 'validating'>>({});
    const [modalValidationMessages, setModalValidationMessages] = useState<Record<string, string>>({});

    // OS assignment state
    const [vmOSAssignments, setVmOSAssignments] = useState<Record<string, string>>({});
    const [osValidationError, setOsValidationError] = useState<string>("");

    // Migration Options state
    const { params, getParamsUpdater } = useParams<FormValues>({});
    const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
    const getFieldErrorsUpdater = useCallback((key: string | number) => (value: string) => {
        setFieldErrors(prev => ({ ...prev, [key]: value }));
    }, []);
    const { params: selectedMigrationOptions, getParamsUpdater: updateSelectedMigrationOptions } =
        useParams<SelectedMigrationOptionsType>(defaultMigrationOptions);

    const { sourceData, pcdData, loadingVMware: loading, loadingPCD } = useClusterData();
    const [assigningIPs, setAssigningIPs] = useState(false);

    // IP validation error state
    const [vmIpValidationError, setVmIpValidationError] = useState<string>("");

    // ESX host config validation error state
    const [esxHostConfigValidationError, setEsxHostConfigValidationError] = useState<string>("");

    // Bulk IP editing state - updated for multiple interfaces
    const [bulkEditDialogOpen, setBulkEditDialogOpen] = useState(false);
    const [bulkEditIPs, setBulkEditIPs] = useState<Record<string, Record<number, string>>>({});
    const [bulkValidationStatus, setBulkValidationStatus] = useState<Record<string, Record<number, 'empty' | 'valid' | 'invalid' | 'validating'>>>({});
    const [bulkValidationMessages, setBulkValidationMessages] = useState<Record<string, Record<number, string>>>({}); // Updated for multiple interfaces

    // Flavor assignment state
    const [flavorDialogOpen, setFlavorDialogOpen] = useState(false);
    const [selectedFlavor, setSelectedFlavor] = useState("");
    const [updating, setUpdating] = useState(false);

    // Toast notification state
    const [toastOpen, setToastOpen] = useState(false);
    const [toastMessage, setToastMessage] = useState("");
    const [toastSeverity, setToastSeverity] = useState<"success" | "error" | "warning" | "info">("success");

    const paginationModel = { page: 0, pageSize: 5 };

    // Toast notification helper
    const showToast = useCallback((message: string, severity: "success" | "error" | "warning" | "info" = "success") => {
        setToastMessage(message);
        setToastSeverity(severity);
        setToastOpen(true);
    }, []);

    const handleCloseToast = useCallback((_event?: React.SyntheticEvent | Event, reason?: string) => {
        if (reason === 'clickaway') {
            return;
        }
        setToastOpen(false);
    }, []);

    // Clear selection when component is closed
    useEffect(() => {
        if (!open) {
            setSelectedVMs([]);
        }
    }, [open]);

    useEffect(() => {
        if (open) {
            fetchMaasConfigs();
        }
    }, [open]);

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
        }
    }, [sourceCluster]);

    const fetchClusterHosts = async () => {
        if (!sourceCluster) return;

        setLoadingHosts(true);
        try {
            const parts = sourceCluster.split(":");
            const credName = parts[0];

            const sourceItem = sourceData.find(item => item.credName === credName);
            const clusterObj = sourceItem?.clusters.find(cluster =>
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

            const sourceItem = sourceData.find(item => item.credName === credName);
            const clusterObj = sourceItem?.clusters.find(cluster =>
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
                const clusterLabel = vm.metadata?.labels?.[`vjailbreak.k8s.pf9.io/vmware-cluster`];
                return clusterLabel === clusterName;
            });

            const mappedVMs: VM[] = filteredVMs.map((vm: VMwareMachine) => {
                const esxiHost = vm.metadata?.labels?.[`vjailbreak.k8s.pf9.io/esxi-name`] || "";

                // Get flavor information from the VM spec  
                const targetFlavorId = vm.spec.targetFlavorId || "";
                // We'll resolve flavor names later when openstackFlavors is available
                const flavorName = targetFlavorId || "auto-assign";

                if (vm.spec.vms.name == "nvidia-bcm-router") {
                    console.log(vm.spec.vms.networkInterfaces);
                }

                // Get all IP addresses from network interfaces in comma-separated format
                const allIPs = vm.spec.vms.networkInterfaces && vm.spec.vms.networkInterfaces.length > 0
                    ? vm.spec.vms.networkInterfaces
                        .map(nic => nic.ipAddress)
                        .filter(ip => ip && ip.trim() !== "") // Filter out empty/null IPs
                        .join(", ")
                    : vm.spec.vms.ipAddress || vm.spec.vms.assignedIp || "—";

                return {
                    id: vm.metadata.name,
                    name: vm.spec.vms.name || vm.metadata.name,
                    ip: allIPs || "—",
                    esxHost: esxiHost,
                    networks: vm.spec.vms.networks,
                    datastores: vm.spec.vms.datastores,
                    cpu: vm.spec.vms.cpu,
                    memory: vm.spec.vms.memory,
                    osFamily: vm.spec.vms.osFamily,
                    flavor: flavorName,
                    targetFlavorId: targetFlavorId,
                    powerState: vm.status.powerState === "running" ? "powered-on" : "powered-off",
                    ipValidationStatus: 'pending',
                    ipValidationMessage: '',
                    networkInterfaces: vm.spec.vms.networkInterfaces
                };
            });

            setVmsWithAssignments(mappedVMs);

            // Clean up persistent selection - remove VMs that no longer exist
            const availableVmIds = new Set(mappedVMs.map(vm => vm.id));
            const cleanedSelection = selectedVMs.filter(vmId => availableVmIds.has(String(vmId)));

            if (cleanedSelection.length !== selectedVMs.length) {
                setSelectedVMs(cleanedSelection);
            }
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

    const handleSourceClusterChange = (value) => {
        setSourceCluster(value);

        if (value) {
            const parts = value.split(":");
            const credName = parts[0];
            setSelectedVMwareCredName(credName);
        } else {
            setSelectedVMwareCredName("");
        }
    };

    const handleDestinationPCDChange = (value) => {
        setDestinationPCD(value);

        if (value) {
            const selectedPCD = pcdData.find(p => p.id === value);
            if (selectedPCD) {
                setSelectedPcdCredName(selectedPCD.openstackCredName);
                fetchOpenstackCredentialDetails(selectedPCD.openstackCredName);
            }
        } else {
            setSelectedPcdCredName("");
            setOpenstackCredData(null);
        }
    };

    const fetchOpenstackCredentialDetails = async (credName) => {
        if (!credName) return;

        setLoadingOpenstackDetails(true);
        try {
            const response = await getOpenstackCredentials(credName);
            setOpenstackCredData(response);
        } catch (error) {
            console.error("Failed to fetch OpenStack credential details:", error);
        } finally {
            setLoadingOpenstackDetails(false);
        }
    };

    // IP validation and editing functions
    const isValidIPAddress = (ip: string): boolean => {
        const ipRegex = /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
        return ipRegex.test(ip);
    };

    const getOpenstackAccessInfo = async (openstackCredData: OpenstackCreds) => {
        const spec = openstackCredData.spec;

        // If there's a secretRef, fetch the credentials from the secret
        if (spec.secretRef?.name) {
            try {
                const secret = await getSecret(spec.secretRef.name, VJAILBREAK_DEFAULT_NAMESPACE);
                if (secret && secret.data) {
                    return {
                        authUrl: secret.data.OS_AUTH_URL || '',
                        domainName: secret.data.OS_DOMAIN_NAME || 'default',
                        insecure: secret.data.OS_INSECURE === 'true' || true,
                        password: secret.data.OS_PASSWORD || '',
                        regionName: secret.data.OS_REGION_NAME || '',
                        tenantName: secret.data.OS_TENANT_NAME || '',
                        username: secret.data.OS_USERNAME || '',
                    };
                }
            } catch (error) {
                console.error('Failed to fetch OpenStack credentials from secret:', error);
                throw new Error('Failed to fetch OpenStack credentials from secret');
            }
        }

        return {
            authUrl: '',
            domainName: 'default',
            insecure: true,
            password: '',
            regionName: '',
            tenantName: '',
            username: '',
        };
    };



    const handleCancelEditingIP = () => {
        setEditingIpFor(null);
        setEditingInterfaceIndex(null);
        setTempIpValue("");
    };

    // Modal functions for multi-NIC IP editing
    const handleOpenIpEditModal = (vm: VM) => {
        setEditingVm(vm);
        // Initialize modal values with current IP addresses
        const ipValues: Record<string, string> = {};
        const validationStatus: Record<string, 'pending' | 'valid' | 'invalid' | 'validating'> = {};

        if (vm.networkInterfaces) {
            vm.networkInterfaces.forEach((nic, index) => {
                const key = `interface-${index}`;
                ipValues[key] = nic.ipAddress || "";
                validationStatus[key] = nic.ipAddress ? 'valid' : 'pending';
            });
        }

        setModalIpValues(ipValues);
        setModalValidationStatus(validationStatus);
        setModalValidationMessages({});
        setIpEditModalOpen(true);
    };

    const handleCloseIpEditModal = () => {
        setIpEditModalOpen(false);
        setEditingVm(null);
        setModalIpValues({});
        setModalValidationStatus({});
        setModalValidationMessages({});
    };



    const handleModalIpChange = (interfaceIndex: number, value: string) => {
        const key = `interface-${interfaceIndex}`;
        setModalIpValues(prev => ({
            ...prev,
            [key]: value
        }));

        // Reset validation state for this field
        setModalValidationStatus(prev => ({
            ...prev,
            [key]: 'pending'
        }));
        setModalValidationMessages(prev => ({
            ...prev,
            [key]: ""
        }));
    };

    const handleSaveModalIPs = async () => {
        if (!editingVm || !editingVm.networkInterfaces) {
            handleCloseIpEditModal();
            return;
        }

        try {
            // Validate all IP addresses first
            const ipsToValidate: string[] = [];
            const validIpMap: Record<number, string> = {};
            let hasValidationErrors = false;

            for (let i = 0; i < editingVm.networkInterfaces.length; i++) {
                const key = `interface-${i}`;
                const ipValue = modalIpValues[key]?.trim();

                if (ipValue && ipValue !== "—") {
                    if (!isValidIPAddress(ipValue)) {
                        setModalValidationMessages(prev => ({
                            ...prev,
                            [key]: "Invalid IP address format"
                        }));
                        setModalValidationStatus(prev => ({
                            ...prev,
                            [key]: 'invalid'
                        }));
                        hasValidationErrors = true;
                    } else {
                        validIpMap[i] = ipValue;
                        ipsToValidate.push(ipValue);
                        setModalValidationStatus(prev => ({
                            ...prev,
                            [key]: 'validating'
                        }));
                    }
                }
            }

            if (hasValidationErrors) {
                return;
            }

            // Validate IPs with backend if there are any to validate
            if (ipsToValidate.length > 0 && openstackCredData) {
                try {
                    const accessInfo = await getOpenstackAccessInfo(openstackCredData);
                    await validateOpenstackIPs({
                        ip: ipsToValidate,
                        accessInfo
                    });
                    // Mark all as valid if validation passes
                    Object.keys(validIpMap).forEach(interfaceIndex => {
                        const key = `interface-${interfaceIndex}`;
                        setModalValidationStatus(prev => ({
                            ...prev,
                            [key]: 'valid'
                        }));
                    });
                } catch (error) {
                    console.error("IP validation failed:", error);
                    Object.keys(validIpMap).forEach(interfaceIndex => {
                        const key = `interface-${interfaceIndex}`;
                        setModalValidationMessages(prev => ({
                            ...prev,
                            [key]: "IP validation failed"
                        }));
                        setModalValidationStatus(prev => ({
                            ...prev,
                            [key]: 'invalid'
                        }));
                    });
                    return;
                }
            }

            // Prepare updated network interfaces
            const updatedInterfaces = editingVm.networkInterfaces.map((nic, index) => {
                const key = `interface-${index}`;
                const newIpValue = modalIpValues[key]?.trim();

                return {
                    ...nic,
                    ipAddress: newIpValue || nic.ipAddress
                };
            });

            // Patch the VM with updated network interfaces via API
            await patchVMwareMachine(editingVm.id, {
                spec: {
                    vms: {
                        networkInterfaces: updatedInterfaces
                    }
                }
            }, VJAILBREAK_DEFAULT_NAMESPACE);

            // Update local state after successful API call
            const updatedVMs = vmsWithAssignments.map(vmItem => {
                if (vmItem.id === editingVm.id) {
                    // Recalculate comma-separated IP string
                    const allIPs = updatedInterfaces
                        .map(nic => nic.ipAddress)
                        .filter(ip => ip && ip.trim() !== "")
                        .join(", ");

                    return {
                        ...vmItem,
                        networkInterfaces: updatedInterfaces,
                        ip: allIPs || "—"
                    };
                }
                return vmItem;
            });

            setVmsWithAssignments(updatedVMs);
            handleCloseIpEditModal();

            // Track the analytics event [[memory:4507259]]
            track('ip_addresses_updated', {
                vm_id: editingVm.id,
                vm_name: editingVm.name,
                interface_count: editingVm.networkInterfaces.length,
                action: 'modal_multi_ip_update'
            });

            // Show success toast
            const updatedIpsCount = Object.values(modalIpValues).filter(ip => ip && ip.trim() !== "").length;
            showToast(`Successfully updated ${updatedIpsCount} IP address${updatedIpsCount === 1 ? '' : 'es'} for VM "${editingVm.name}"`);

        } catch (error) {
            console.error("Failed to update IPs:", error);
            reportError(error as Error, {
                context: 'modal-multi-ip-update',
                metadata: {
                    vm_id: editingVm.id,
                    vm_name: editingVm.name,
                    modalIpValues: modalIpValues,
                    action: 'modal-multi-ip-update'
                }
            });
            showToast(`Failed to update IP addresses for VM "${editingVm?.name}": ${error instanceof Error ? error.message : 'Unknown error'}`, "error");
        }
    };



    const handleSaveIP = async (vmId: string, interfaceIndex?: number) => {
        const vm = vmsWithAssignments.find(v => v.id === vmId);
        if (!vm || !tempIpValue.trim()) {
            handleCancelEditingIP();
            return;
        }

        if (!isValidIPAddress(tempIpValue.trim())) {
            console.error('Invalid IP address format:', tempIpValue.trim());
            return;
        }

        try {
            if (openstackCredData && interfaceIndex !== undefined) {
                const accessInfo = await getOpenstackAccessInfo(openstackCredData);
                const validationResult = await validateOpenstackIPs({
                    ip: [tempIpValue.trim()],
                    accessInfo
                });

                const isValid = validationResult.isValid[0];
                const reason = validationResult.reason[0];

                if (!isValid) {
                    console.error('IP validation failed:', reason);
                    return;
                }
            }

            // Update the VM with new IP for specific interface
            if (interfaceIndex !== undefined && vm.networkInterfaces) {
                const updatedInterfaces = [...vm.networkInterfaces];
                updatedInterfaces[interfaceIndex] = {
                    ...updatedInterfaces[interfaceIndex],
                    ipAddress: tempIpValue.trim()
                };

                await patchVMwareMachine(vm.id, {
                    spec: {
                        vms: {
                            networkInterfaces: updatedInterfaces
                        }
                    }
                }, VJAILBREAK_DEFAULT_NAMESPACE);

                // Update local state - recalculate comma-separated IP string
                const updatedVMs = vmsWithAssignments.map(v => {
                    if (v.id === vmId) {
                        const allIPs = updatedInterfaces
                            .map(nic => nic.ipAddress)
                            .filter(ip => ip && ip.trim() !== "")
                            .join(", ");
                        return {
                            ...v,
                            networkInterfaces: updatedInterfaces,
                            ip: allIPs || "—"
                        };
                    }
                    return v;
                });
                setVmsWithAssignments(updatedVMs);
            } else {
                // Fallback for single IP assignment
                await patchVMwareMachine(vm.id, {
                    spec: {
                        vms: {
                            assignedIp: tempIpValue.trim()
                        }
                    }
                }, VJAILBREAK_DEFAULT_NAMESPACE);

                // Update local state
                const updatedVMs = vmsWithAssignments.map(v =>
                    v.id === vmId ? { ...v, ip: tempIpValue.trim() } : v
                );
                setVmsWithAssignments(updatedVMs);
            }

            handleCancelEditingIP();

            // Show success toast
            showToast(`IP address successfully updated for VM "${vm?.name}"`);

        } catch (error) {
            console.error("Failed to update IP:", error);
            reportError(error as Error, {
                context: 'ip-assignment',
                metadata: { vmId, interfaceIndex, action: 'ip-assignment' }
            });
            showToast(`Failed to update IP address: ${error instanceof Error ? error.message : 'Unknown error'}`, "error");
        }
    };

    // OS assignment handler
    const handleOSAssignment = async (vmId: string, osFamily: string) => {
        try {
            setVmOSAssignments(prev => ({ ...prev, [vmId]: osFamily }));

            await patchVMwareMachine(vmId, {
                spec: {
                    vms: {
                        osFamily: osFamily
                    }
                }
            }, VJAILBREAK_DEFAULT_NAMESPACE);

            const updatedVMs = vmsWithAssignments.map(v =>
                v.id === vmId ? { ...v, osFamily: osFamily } : v
            );
            setVmsWithAssignments(updatedVMs);

        } catch (error) {
            console.error("Failed to assign OS family:", error);
            reportError(error as Error, {
                context: 'os-family-assignment',
                metadata: {
                    vmId: vmId,
                    osFamily: osFamily,
                    action: 'os-family-assignment'
                }
            });
            // Revert local state on error
            setVmOSAssignments(prev => {
                const newState = { ...prev };
                delete newState[vmId];
                return newState;
            });
        }
    };


    const availableVmwareNetworks = useMemo(() => {
        if (!vmsWithAssignments.length || !selectedVMs.length) return [];

        const selectedVMsData = vmsWithAssignments.filter(vm =>
            selectedVMs.includes(vm.id));

        const extractedNetworks = selectedVMsData
            .filter(vm => vm.networks)
            .flatMap(vm => vm.networks || []);

        if (extractedNetworks.length > 0) {
            return extractedNetworks.sort(); // Remove Array.from(new Set()) to keep duplicates
        }
        return [];

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
        return [];

    }, [vmsWithAssignments, selectedVMs]);

    // Define ESX columns inside component to access state and functions
    const esxColumns: GridColDef[] = [
        {
            field: "name",
            headerName: "ESX Name",
            flex: 2,
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
            field: "pcdHostConfigName",
            headerName: "Host Config",
            flex: 1,
            renderCell: (params) => {
                const hostId = params.row.id;
                const currentConfig = params.value || "";

                return (
                    <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
                        <Select
                            size="small"
                            value={currentConfig}
                            onChange={(e) => handleIndividualHostConfigChange(hostId, e.target.value)}
                            displayEmpty
                            sx={{
                                width: 250,
                                '& .MuiSelect-select': {
                                    padding: '4px 8px',
                                    fontSize: '0.875rem'
                                }
                            }}
                            renderValue={(selected) => {
                                if (!selected) {
                                    return (
                                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}>
                                            <WarningIcon sx={{ fontSize: 16 }} />
                                            <em>Select Host Config</em>
                                        </Box>
                                    );
                                }
                                return <Typography variant="body2">{selected}</Typography>;
                            }}
                        >
                            <MenuItem value="">
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}>
                                    <WarningIcon sx={{ fontSize: 16 }} />
                                    <em>Select Host Config</em>
                                </Box>
                            </MenuItem>
                            {(openstackCredData?.spec?.pcdHostConfig || []).map((config) => (
                                <MenuItem key={config.id} value={config.name}>
                                    <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                                        <Typography variant="body2">{config.name}</Typography>
                                        <Typography variant="caption" color="text.secondary">
                                            {config.mgmtInterface}
                                        </Typography>
                                    </Box>
                                </MenuItem>
                            ))}
                        </Select>
                    </Box>
                );
            },
        },
    ];

    // Calculate ESX host to PCD config mapping status
    const esxHostMappingStatus = useMemo(() => {
        const mappedHostsCount = orderedESXHosts.filter(host => host.pcdHostConfigName).length;
        return {
            mapped: mappedHostsCount,
            total: orderedESXHosts.length,
            fullyMapped: mappedHostsCount === orderedESXHosts.length
        };
    }, [orderedESXHosts]);

    const openstackNetworks = useMemo(() => {
        if (!openstackCredData) return [];

        const networks = openstackCredData?.status?.openstack?.networks || [];
        return networks.sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
    }, [openstackCredData]);

    const openstackVolumeTypes = useMemo(() => {
        if (!openstackCredData) return [];

        const volumeTypes = openstackCredData?.status?.openstack?.volumeTypes || [];
        return volumeTypes.sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
    }, [openstackCredData]);

    const openstackFlavors = useMemo(() => {
        if (!openstackCredData) return [];

        return openstackCredData?.spec?.flavors || [];
    }, [openstackCredData]);

    // Update VM flavor names when OpenStack flavors become available
    useEffect(() => {
        if (openstackFlavors.length > 0 && vmsWithAssignments.length > 0) {
            const updatedVMs = vmsWithAssignments.map(vm => {
                if (vm.targetFlavorId) {
                    const flavorObj = openstackFlavors.find(f => f.id === vm.targetFlavorId);
                    if (flavorObj && vm.flavor !== flavorObj.name) {
                        return { ...vm, flavor: flavorObj.name };
                    }
                }
                return vm;
            });

            // Only update if there are actual changes
            const hasChanges = updatedVMs.some((vm, index) =>
                vm.flavor !== vmsWithAssignments[index]?.flavor
            );

            if (hasChanges) {
                setVmsWithAssignments(updatedVMs);
            }
        }
    }, [openstackFlavors, vmsWithAssignments]);

    const handleMappingsChange = (key: string) => (value: ResourceMap[]) => {
        if (key === "networkMappings") {
            setNetworkMappings(value);
            setNetworkMappingError("");
        } else if (key === "storageMappings") {
            setStorageMappings(value);
            setStorageMappingError("");
        }
    };

    // Validate IP addresses for selected VMs
    const vmIpValidation = useMemo(() => {
        if (selectedVMs.length === 0) {
            setVmIpValidationError("Please select VMs to assign IP addresses.");
            return { hasError: true, vmsWithoutIPs: [] };
        }

        const selectedVMsData = vmsWithAssignments.filter(vm => selectedVMs.includes(vm.id));
        const vmsWithoutIPs = selectedVMsData.filter(vm => vm.ip === "—" || !vm.ip);

        if (vmsWithoutIPs.length > 0) {
            const errorMessage = `Cannot proceed with Migration: ${vmsWithoutIPs.length} selected VM${vmsWithoutIPs.length === 1 ? '' : 's'} do not have IP addresses assigned. Please assign IP addresses to all selected VMs before continuing.`;
            setVmIpValidationError(errorMessage);
            return { hasError: true, vmsWithoutIPs };
        } else {
            setVmIpValidationError("");
            return { hasError: false, vmsWithoutIPs: [] };
        }
    }, [selectedVMs, vmsWithAssignments]);

    // Validate ESX host configs for all hosts
    const esxHostConfigValidation = useMemo(() => {
        if (orderedESXHosts.length === 0) {
            setEsxHostConfigValidationError("Please select VMs to migrate.");
            return { hasError: true, hostsWithoutConfigs: [] };
        }

        const hostsWithoutConfigs = orderedESXHosts.filter(host => !host.pcdHostConfigName);

        if (hostsWithoutConfigs.length > 0) {
            const errorMessage = `Cannot proceed with Migration: ${hostsWithoutConfigs.length} ESXi host${hostsWithoutConfigs.length === 1 ? '' : 's'} do not have Host Config assigned. Please assign Host Config to all ESXi hosts before continuing.`;
            setEsxHostConfigValidationError(errorMessage);
            return { hasError: true, hostsWithoutConfigs };
        } else {
            setEsxHostConfigValidationError("");
            return { hasError: false, hostsWithoutConfigs: [] };
        }
    }, [orderedESXHosts]);

    // Validate OS assignment for selected powered-off VMs
    const osValidation = useMemo(() => {
        if (selectedVMs.length === 0) {
            setOsValidationError("Please select VMs to assign OS.");
            return { hasError: true, vmsWithoutOS: [] };
        }

        const selectedVMsData = vmsWithAssignments.filter(vm => selectedVMs.includes(vm.id));
        const poweredOffVMsWithoutOS = selectedVMsData.filter(vm => {
            const assignedOS = vmOSAssignments[vm.id];
            const currentOS = assignedOS || vm.osFamily;
            return vm.powerState === "powered-off" && (!currentOS || currentOS === "Unknown");
        });

        if (poweredOffVMsWithoutOS.length > 0) {
            const errorMessage = `Cannot proceed with Migration: ${poweredOffVMsWithoutOS.length} powered-off VM${poweredOffVMsWithoutOS.length === 1 ? '' : 's'} do not have Operating System assigned. Please assign OS to all powered-off VMs before continuing.`;
            setOsValidationError(errorMessage);
            return { hasError: true, vmsWithoutOS: poweredOffVMsWithoutOS };
        } else {
            setOsValidationError("");
            return { hasError: false, vmsWithoutOS: [] };
        }
    }, [selectedVMs, vmsWithAssignments, vmOSAssignments]);

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

            const sourceItem = sourceData.find(item => item.credName === credName);
            const clusterObj = sourceItem?.clusters.find(cluster =>
                cluster.id === sourceCluster
            );
            const clusterName = clusterObj?.name || "";

            const selectedVMsData = vmsWithAssignments
                .filter(vm => selectedVMs.includes(vm.id))
                .map(vm => ({
                    vmName: vm.name,
                })) as VMSequence[];

            // Create cluster mapping between VMware cluster and PCD cluster
            const selectedPCD = pcdData.find(p => p.id === destinationPCD);
            const pcdClusterName = selectedPCD?.name || "";
            const targetPCDClusterName = selectedPCD?.name;

            const clusterMapping: ClusterMapping[] = [{
                vmwareClusterName: clusterName,
                pcdClusterName: pcdClusterName
            }];

            // Update VMware hosts with their host config IDs
            const hostsToUpdate = orderedESXHosts.filter(host => host.pcdHostConfigName);

            for (const host of hostsToUpdate) {
                try {
                    // Find the config ID from the name
                    const availablePcdHostConfigs = openstackCredData?.spec?.pcdHostConfig || [];
                    const selectedPcdConfig = availablePcdHostConfigs.find(config => config.name === host.pcdHostConfigName);
                    const hostConfigId = selectedPcdConfig ? selectedPcdConfig.id : host.pcdHostConfigName;

                    if (hostConfigId) {
                        console.log(`Updating host ${host.name} with hostConfigId: ${hostConfigId}`);
                        await patchVMwareHost(host.id, hostConfigId, VJAILBREAK_DEFAULT_NAMESPACE);
                    }
                } catch (error) {
                    console.error(`Failed to update host config for ${host.name}:`, error);
                    reportError(error as Error, {
                        context: 'host-config-update',
                        metadata: {
                            hostId: host.id,
                            hostName: host.name,
                            hostConfigId: host.pcdHostConfigName,
                            action: 'host-config-update'
                        }
                    });
                    // Continue with other hosts even if one fails
                }
            }

            // 1. Create network mapping
            const networkMappingJson = createNetworkMappingJson({
                networkMappings: networkMappings.map(mapping => ({
                    source: mapping.source,
                    target: mapping.target
                }))
            });
            const networkMappingResponse = await postNetworkMapping(networkMappingJson);

            // 2. Create storage mapping
            const storageMappingJson = createStorageMappingJson({
                storageMappings: storageMappings.map(mapping => ({
                    source: mapping.source,
                    target: mapping.target
                }))
            });
            const storageMappingResponse = await postStorageMapping(storageMappingJson);

            // 3. Create migration template
            const migrationTemplateJson = createMigrationTemplateJson({
                vmwareRef: selectedVMwareCredName,
                openstackRef: selectedPcdCredName,
                networkMapping: networkMappingResponse.metadata.name,
                storageMapping: storageMappingResponse.metadata.name,
                targetPCDClusterName: targetPCDClusterName,
            });
            const migrationTemplateResponse = await postMigrationTemplate(migrationTemplateJson);

            // 4. Create rolling migration plan with the template
            const migrationPlanJson = createRollingMigrationPlanJson({
                clusterName,
                vms: selectedVMsData,
                clusterMapping,
                bmConfigRef: {
                    name: selectedMaasConfig?.metadata.name || "",
                },
                migrationStrategy: {
                    adminInitiatedCutOver: selectedMigrationOptions.cutoverOption &&
                        params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED,
                    healthCheckPort: "443",
                    performHealthChecks: false,
                    type: selectedMigrationOptions.dataCopyMethod ?
                        (params.dataCopyMethod as string) : "cold",
                    ...(selectedMigrationOptions.dataCopyStartTime && params.dataCopyStartTime && {
                        dataCopyStart: params.dataCopyStartTime
                    }),
                    ...(selectedMigrationOptions.cutoverOption &&
                        params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
                        params.cutoverStartTime && {
                        vmCutoverStart: params.cutoverStartTime
                    }),
                    ...(selectedMigrationOptions.cutoverOption &&
                        params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
                        params.cutoverEndTime && {
                        vmCutoverEnd: params.cutoverEndTime
                    }),
                },
                migrationTemplate: migrationTemplateResponse.metadata.name,
                namespace: VJAILBREAK_DEFAULT_NAMESPACE
            });

            await postRollingMigrationPlan(migrationPlanJson, VJAILBREAK_DEFAULT_NAMESPACE);

            console.log("Submitted rolling migration plan", migrationPlanJson);

            // Track successful cluster conversion creation
            track(AMPLITUDE_EVENTS.ROLLING_MIGRATION_CREATED, {
                clusterMigrationName: clusterName,
                sourceCluster: clusterObj?.name,
                destinationCluster: selectedPCD?.name,
                vmwareCredential: selectedVMwareCredName,
                pcdCredential: selectedPcdCredName,
                maasConfig: selectedMaasConfig?.metadata.name,
                virtualMachineCount: selectedVMsData?.length || 0,
                esxHostCount: orderedESXHosts?.length || 0,
                networkMappingCount: networkMappings?.length || 0,
                storageMappingCount: storageMappings?.length || 0,
                migrationType: params.dataCopyMethod || "cold",
                hasAdminInitiatedCutover: selectedMigrationOptions.cutoverOption &&
                    params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED,
                hasTimedCutover: selectedMigrationOptions.cutoverOption &&
                    params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW,
                migrationTemplate: migrationTemplateResponse.metadata.name,
                namespace: VJAILBREAK_DEFAULT_NAMESPACE,
            });

            onClose();
            navigate("/dashboard/cluster-conversions");
        } catch (error) {
            console.error("Failed to submit rolling migration plan:", error);

            // Track cluster conversion failure
            const parts = sourceCluster.split(":");
            const credName = parts[0];
            const sourceItem = sourceData.find(item => item.credName === credName);
            const clusterObj = sourceItem?.clusters.find(cluster =>
                cluster.id === sourceCluster
            );
            const selectedPCD = pcdData.find(p => p.id === destinationPCD);
            const selectedVMsData = vmsWithAssignments
                .filter(vm => selectedVMs.includes(vm.id));

            track(AMPLITUDE_EVENTS.ROLLING_MIGRATION_SUBMISSION_FAILED, {
                clusterMigrationName: clusterObj?.name,
                sourceCluster: clusterObj?.name,
                destinationCluster: selectedPCD?.name,
                vmwareCredential: selectedVMwareCredName,
                pcdCredential: selectedPcdCredName,
                virtualMachineCount: selectedVMsData?.length || 0,
                esxHostCount: orderedESXHosts?.length || 0,
                errorMessage: error instanceof Error ? error.message : String(error),
                stage: "creation",
            });

            reportError(error as Error, {
                context: 'rolling-migration-plan-submission',
                metadata: {
                    sourceCluster: sourceCluster,
                    destinationPCD: destinationPCD,
                    selectedVMwareCredName: selectedVMwareCredName,
                    selectedPcdCredName: selectedPcdCredName,
                    action: 'rolling-migration-plan-submission'
                }
            });
            const errorMessage = error instanceof Error ? error.message : "Unknown error";
            alert(`Failed to submit rolling migration plan: ${errorMessage}`);
        } finally {
            setSubmitting(false);
        }
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
            !selectedVMs.length ||
            submitting;


        const mappingsValid = !(availableVmwareNetworks.some(network =>
            !networkMappings.some(mapping => mapping.source === network)) ||
            availableVmwareDatastores.some(datastore =>
                !storageMappings.some(mapping => mapping.source === datastore)));

        // Migration options validation
        const migrationOptionValidated = Object.keys(selectedMigrationOptions).every((key) => {
            if (selectedMigrationOptions[key]) {
                if (
                    key === "cutoverOption" &&
                    params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW
                ) {
                    return (
                        params.cutoverStartTime &&
                        params.cutoverEndTime &&
                        !fieldErrors["cutoverStartTime"] &&
                        !fieldErrors["cutoverEndTime"]
                    )
                }
                return params?.[key] && !fieldErrors[key];
            }
            return true;
        });

        // PCD host config validation - not needed anymore since validation is handled by esxHostConfigValid

        // ESX host config validation - ensure all ESX hosts have host configs assigned
        const esxHostConfigValid = !esxHostConfigValidation.hasError;

        // IP validation - ensure all selected VMs have IP addresses assigned
        const ipValidationPassed = !vmIpValidation.hasError;

        // OS validation - ensure all selected powered-off VMs have OS assigned
        const osValidationPassed = !osValidation.hasError;

        return basicRequirementsMissing || !mappingsValid || !migrationOptionValidated || !esxHostConfigValid || !ipValidationPassed || !osValidationPassed;
    }, [
        sourceCluster,
        destinationPCD,
        selectedMaasConfig,
        submitting,
        selectedVMs,
        availableVmwareNetworks,
        networkMappings,
        availableVmwareDatastores,
        storageMappings,
        selectedMigrationOptions,
        params,
        fieldErrors,
        orderedESXHosts,
        vmIpValidation.hasError,
        esxHostConfigValidation.hasError,
        osValidation.hasError
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

    const handleOpenPcdHostConfigDialog = () => {
        setPcdHostConfigDialogOpen(true);
    };

    const handleClosePcdHostConfigDialog = () => {
        setPcdHostConfigDialogOpen(false);
        setSelectedPcdHostConfig("");
    };

    const handlePcdHostConfigChange = (event) => {
        setSelectedPcdHostConfig(event.target.value);
    };

    const handleApplyPcdHostConfig = async () => {
        if (!selectedPcdHostConfig) {
            handleClosePcdHostConfigDialog();
            return;
        }

        setUpdatingPcdMapping(true);

        try {
            const availablePcdHostConfigs = openstackCredData?.spec?.pcdHostConfig || [];
            const selectedPcdConfig = availablePcdHostConfigs.find(config => config.id === selectedPcdHostConfig);
            const pcdConfigName = selectedPcdConfig ? selectedPcdConfig.name : selectedPcdHostConfig;

            // Update ALL ESX hosts with the selected host config
            const updatedESXHosts = orderedESXHosts.map(host => ({
                ...host,
                pcdHostConfigName: pcdConfigName
            }));

            setOrderedESXHosts(updatedESXHosts);

            handleClosePcdHostConfigDialog();
        } catch (error) {
            console.error("Error updating PCD host config mapping:", error);
            reportError(error as Error, {
                context: 'pcd-host-config-mapping',
                metadata: {
                    selectedPcdHostConfig: selectedPcdHostConfig,
                    action: 'update-pcd-host-config-mapping'
                }
            });
        } finally {
            setUpdatingPcdMapping(false);
        }
    };

    // Define VM columns inside component to access state
    const vmColumns: GridColDef[] = [
        {
            field: "name",
            headerName: "VM Name",
            flex: 1.3,
            minWidth: 150,
            hideable: false,
            renderCell: (params) => (
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Tooltip title={params.row.powerState === "powered-on" ? "Powered On" : "Powered Off"}>
                        <CdsIconWrapper>
                            {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                            {/* @ts-ignore */}
                            <cds-icon shape="vm" size="md" badge={params.row.powerState === "powered-on" ? "success" : "danger"}></cds-icon>
                        </CdsIconWrapper>
                    </Tooltip>
                    <Box sx={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{params.value}</Box>
                </Box>
            ),
        },
        {
            field: "ip",
            headerName: "IP Address(es)",
            flex: 1,
            hideable: true,
            renderCell: (params) => {
                const vm = params.row as VM;
                const vmId = vm.id;
                const isSelected = selectedVMs.includes(vmId);
                const powerState = vm.powerState;

                // For powered-off VMs with multiple network interfaces - Modal Design
                if (powerState === "powered-off" && vm.networkInterfaces && vm.networkInterfaces.length > 1) {
                    const ipSummary = vm.networkInterfaces.map(nic => nic.ipAddress || "—").join(", ");
                    const tooltipContent = vm.networkInterfaces.map((nic) =>
                        `${nic.network}: ${nic.ipAddress || "—"}, `
                    ).join("\n");

                    const content = (
                        <Box sx={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                            width: '100%',
                            height: '100%',
                            gap: 1
                        }}>
                            <Typography variant="body2" sx={{
                                fontSize: '0.875rem',
                                flex: 1,
                                whiteSpace: 'nowrap',
                                overflow: 'hidden',
                                textOverflow: 'ellipsis'
                            }}>
                                {ipSummary}
                            </Typography>
                            {isSelected && (
                                <Button
                                    size="small"
                                    variant="outlined"
                                    onClick={() => handleOpenIpEditModal(vm)}
                                    sx={{
                                        minWidth: 'auto',
                                        px: 1.5,
                                        py: 0.5,
                                        fontSize: '0.75rem',
                                        height: 28
                                    }}
                                >
                                    Edit
                                </Button>
                            )}
                        </Box>
                    );

                    // Only show tooltip when row is selected
                    return isSelected ? (
                        <Tooltip title={tooltipContent} arrow placement="top">
                            {content}
                        </Tooltip>
                    ) : content;
                }

                // For single interface or when not using multi-interface modal
                const currentIp = vm.ip || "—";
                const isEditing = editingIpFor === vmId && editingInterfaceIndex === null;
                // Only allow editing if VM is powered off
                if ((isSelected || isEditing) && powerState === "powered-off") {
                    return (
                        <Box sx={{ display: 'flex', alignItems: 'center', width: '100%', height: '100%' }}>
                            <TextField
                                value={isEditing ? tempIpValue : currentIp === "—" ? "" : currentIp}
                                onChange={(e) => {
                                    if (!isEditing) {
                                        setTempIpValue(e.target.value);
                                        setEditingIpFor(vmId);
                                        setEditingInterfaceIndex(null);
                                    } else {
                                        setTempIpValue(e.target.value);
                                    }
                                }}
                                onBlur={() => {
                                    if (isEditing) {
                                        handleSaveIP(vmId, vm.networkInterfaces ? 0 : undefined);
                                    }
                                }}
                                onKeyDown={(e) => {
                                    if (e.key === 'Enter') {
                                        handleSaveIP(vmId, vm.networkInterfaces ? 0 : undefined);
                                    } else if (e.key === 'Escape') {
                                        handleCancelEditingIP();
                                    }
                                }}
                                size="small"
                                sx={{
                                    minWidth: 120,
                                    '& .MuiInputBase-root': {
                                        height: '32px'
                                    },
                                    '& .MuiInputBase-input': {
                                        padding: '4px 8px',
                                        fontSize: '0.875rem'
                                    }
                                }}
                                placeholder="Enter IP address"
                            />
                        </Box>
                    );
                }

                // For powered-on VMs, show IP but indicate it's not editable
                if ((isSelected || isEditing) && powerState === "powered-on") {
                    return (
                        <Tooltip title="IP assignment is only available for powered-off VMs" arrow>
                            <Box sx={{
                                display: 'flex',
                                alignItems: 'center',
                                width: '100%',
                                height: '100%',
                            }}>
                                <Typography variant="body2">
                                    {currentIp}
                                </Typography>
                            </Box>
                        </Tooltip>
                    );
                }

                return (
                    <Box sx={{
                        display: 'flex',
                        alignItems: 'center',
                        width: '100%',
                        height: '100%'
                    }}>
                        <Typography variant="body2">
                            {currentIp}
                        </Typography>
                    </Box>
                );
            },
        },
        {
            field: "osFamily",
            headerName: "Operating System",
            flex: 1,
            hideable: true,
            renderCell: (params) => {
                const vmId = params.row.id;
                const isSelected = selectedVMs.includes(vmId);
                const powerState = params.row?.powerState;
                const detectedOsFamily = params.row?.osFamily;
                const assignedOsFamily = vmOSAssignments[vmId];
                const currentOsFamily = assignedOsFamily || detectedOsFamily;


                // Show dropdown for ALL powered-off VMs (allows changing selection)
                if (isSelected && powerState === "powered-off") {
                    return (
                        <Box sx={{ display: 'flex', alignItems: 'center', height: '100%' }}>
                            <Select
                                size="small"
                                value={(() => {
                                    if (!currentOsFamily || currentOsFamily === "Unknown") return "";
                                    const osLower = currentOsFamily.toLowerCase();
                                    if (osLower.includes("windows")) return "windowsGuest";
                                    if (osLower.includes("linux")) return "linuxGuest";
                                    return "";
                                })()}
                                onChange={(e) => handleOSAssignment(vmId, e.target.value)}
                                displayEmpty
                                sx={{
                                    minWidth: 120,
                                    '& .MuiSelect-select': {
                                        padding: '4px 8px',
                                        fontSize: '0.875rem'
                                    }
                                }}
                            >
                                <MenuItem value="">
                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}>
                                        <WarningIcon sx={{ fontSize: 16 }} />
                                        <em>Select OS</em>
                                    </Box>
                                </MenuItem>
                                <MenuItem value="windowsGuest">
                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                        <img src={WindowsIcon} alt="Windows" style={{ width: 16, height: 16 }} />
                                        Windows
                                    </Box>
                                </MenuItem>
                                <MenuItem value="linuxGuest">
                                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                        <img src={LinuxIcon} alt="Linux" style={{ width: 16, height: 16 }} />
                                        Linux
                                    </Box>
                                </MenuItem>
                            </Select>
                        </Box>
                    );
                }

                // Show OS with icon for assigned/detected OS
                let displayValue = currentOsFamily || "Unknown";
                let icon: React.ReactNode = null;

                if (currentOsFamily && currentOsFamily.toLowerCase().includes("windows")) {
                    displayValue = "Windows";
                    icon = <img src={WindowsIcon} alt="Windows" style={{ width: 20, height: 20 }} />;
                } else if (currentOsFamily && currentOsFamily.toLowerCase().includes("linux")) {
                    displayValue = "Linux";
                    icon = <img src={LinuxIcon} alt="Linux" style={{ width: 20, height: 20 }} />;
                } else if (currentOsFamily && currentOsFamily !== "Unknown") {
                    displayValue = "Other";
                }

                return (
                    <Tooltip title={powerState === "powered-off" ?
                        ((!currentOsFamily || currentOsFamily === "Unknown") ?
                            "OS assignment required for powered-off VMs" :
                            "Click to change OS selection") :
                        displayValue}>
                        <Box sx={{
                            display: 'flex',
                            alignItems: 'center',
                            height: '100%',
                            gap: 1
                        }}>
                            {icon}
                            {powerState === "powered-off" && (!currentOsFamily || currentOsFamily === "Unknown") && (
                                <WarningIcon sx={{ color: 'warning.main', fontSize: 16 }} />
                            )}
                            <Typography variant="body2" sx={{
                                color: (!currentOsFamily || currentOsFamily === "Unknown") ? 'text.secondary' : 'text.primary'
                            }}>
                                {displayValue}
                            </Typography>
                        </Box>
                    </Tooltip>
                );
            },
        },
        {
            field: "networks",
            headerName: "Network Interface(s)",
            flex: 1,
            hideable: true,
            valueGetter: (value) => value || "—",
        },
        {
            field: "cpu",
            headerName: "CPU",
            flex: 0.3,
            hideable: true,
            valueGetter: (value) => value || "- ",
        },
        {
            field: "memory",
            headerName: "Memory (MB)",
            flex: 0.8,
            hideable: true,
            valueGetter: (value) => value || "—",
        },
        {
            field: "esxHost",
            headerName: "ESX Host",
            flex: 1,
            hideable: true,
            valueGetter: (value) => value || "—",
        },
        {
            field: "flavor",
            headerName: "Flavor",
            flex: 1,
            hideable: true,
            renderCell: (params) => {
                const vmId = params.row.id;
                const isSelected = selectedVMs.includes(vmId);
                const currentFlavor = params.value || "auto-assign";

                if (isSelected) {
                    return (
                        <Box sx={{ display: 'flex', alignItems: 'center', height: '100%', width: '100%' }}>
                            <Select
                                size="small"
                                value={(() => {
                                    if (currentFlavor === "auto-assign") return "auto-assign";
                                    const flavorByName = openstackFlavors.find(f => f.name === currentFlavor);
                                    const flavorById = openstackFlavors.find(f => f.id === currentFlavor);
                                    return flavorByName?.id || flavorById?.id || currentFlavor;
                                })()}
                                onChange={(e) => handleIndividualFlavorChange(vmId, e.target.value)}
                                displayEmpty
                                sx={{
                                    minWidth: 120,
                                    width: '100%',
                                    '& .MuiSelect-select': {
                                        padding: '4px 8px',
                                        fontSize: '0.875rem'
                                    }
                                }}
                            >
                                <MenuItem value="auto-assign">
                                    <Typography variant="body2">Auto Assign</Typography>
                                </MenuItem>
                                {openstackFlavors.map((flavor) => (
                                    <MenuItem key={flavor.id} value={flavor.id}>
                                        <Typography variant="body2">{flavor.name}</Typography>
                                    </MenuItem>
                                ))}
                            </Select>
                        </Box>
                    );
                }

                return (
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                        <Typography variant="body2">
                            {currentFlavor}
                        </Typography>
                    </Box>
                );
            },
        },
        {
            field: "powerState",
            headerName: "Power State",
            hideable: true,
            flex: 0.8,
            valueGetter: (value) => value || "—",
        }
    ];


    const handleCloseBulkEditDialog = () => {
        setBulkEditDialogOpen(false);
        setBulkEditIPs({});
        setBulkValidationStatus({});
        setBulkValidationMessages({});
    };

    const handleBulkIpChange = (vmId: string, interfaceIndex: number, value: string) => {
        setBulkEditIPs(prev => ({
            ...prev,
            [vmId]: { ...prev[vmId], [interfaceIndex]: value }
        }));

        if (!value.trim()) {
            setBulkValidationStatus(prev => ({
                ...prev,
                [vmId]: { ...prev[vmId], [interfaceIndex]: 'empty' }
            }));
            setBulkValidationMessages(prev => ({
                ...prev,
                [vmId]: { ...prev[vmId], [interfaceIndex]: '' }
            }));
        } else if (!isValidIPAddress(value.trim())) {
            setBulkValidationStatus(prev => ({
                ...prev,
                [vmId]: { ...prev[vmId], [interfaceIndex]: 'invalid' }
            }));
            setBulkValidationMessages(prev => ({
                ...prev,
                [vmId]: { ...prev[vmId], [interfaceIndex]: 'Invalid IP format' }
            }));
        } else {
            setBulkValidationStatus(prev => ({
                ...prev,
                [vmId]: { ...prev[vmId], [interfaceIndex]: 'empty' }
            }));
            setBulkValidationMessages(prev => ({
                ...prev,
                [vmId]: { ...prev[vmId], [interfaceIndex]: '' }
            }));
        }
    };

    const handleClearAllIPs = () => {
        const clearedIPs: Record<string, Record<number, string>> = {};
        const clearedStatus: Record<string, Record<number, 'empty' | 'valid' | 'invalid' | 'validating'>> = {};

        Object.keys(bulkEditIPs).forEach(vmId => {
            clearedIPs[vmId] = {};
            clearedStatus[vmId] = {};

            Object.keys(bulkEditIPs[vmId]).forEach(interfaceIndexStr => {
                const interfaceIndex = parseInt(interfaceIndexStr);
                clearedIPs[vmId][interfaceIndex] = "";
                clearedStatus[vmId][interfaceIndex] = 'empty';
            });
        });

        setBulkEditIPs(clearedIPs);
        setBulkValidationStatus(clearedStatus);
        setBulkValidationMessages({});
    };

    const handleApplyBulkIPs = async () => {
        // Collect all IPs to apply with their VM and interface info
        const ipsToApply: Array<{ vmId: string, interfaceIndex: number, ip: string }> = [];

        Object.entries(bulkEditIPs).forEach(([vmId, interfaces]) => {
            Object.entries(interfaces).forEach(([interfaceIndexStr, ip]) => {
                if (ip.trim() !== "") {
                    ipsToApply.push({
                        vmId,
                        interfaceIndex: parseInt(interfaceIndexStr),
                        ip: ip.trim()
                    });
                }
            });
        });

        if (ipsToApply.length === 0) return;

        setAssigningIPs(true);

        try {
            // Batch validation before applying any changes
            if (openstackCredData) {
                const accessInfo = await getOpenstackAccessInfo(openstackCredData);
                const ipList = ipsToApply.map(item => item.ip);

                // Set validating status for all IPs
                setBulkValidationStatus(prev => {
                    const newStatus = { ...prev };
                    ipsToApply.forEach(({ vmId, interfaceIndex }) => {
                        if (!newStatus[vmId]) newStatus[vmId] = {};
                        newStatus[vmId][interfaceIndex] = 'validating';
                    });
                    return newStatus;
                });

                const validationResult = await validateOpenstackIPs({
                    ip: ipList,
                    accessInfo
                });

                // Process validation results
                const validIPs: Array<{ vmId: string, interfaceIndex: number, ip: string }> = [];
                let hasInvalidIPs = false;

                ipsToApply.forEach((item, index) => {
                    const isValid = validationResult.isValid[index];
                    const reason = validationResult.reason[index];

                    if (isValid) {
                        validIPs.push(item);
                        setBulkValidationStatus(prev => ({
                            ...prev,
                            [item.vmId]: { ...prev[item.vmId], [item.interfaceIndex]: 'valid' }
                        }));
                        setBulkValidationMessages(prev => ({
                            ...prev,
                            [item.vmId]: { ...prev[item.vmId], [item.interfaceIndex]: 'Valid' }
                        }));
                    } else {
                        hasInvalidIPs = true;
                        setBulkValidationStatus(prev => ({
                            ...prev,
                            [item.vmId]: { ...prev[item.vmId], [item.interfaceIndex]: 'invalid' }
                        }));
                        setBulkValidationMessages(prev => ({
                            ...prev,
                            [item.vmId]: { ...prev[item.vmId], [item.interfaceIndex]: reason }
                        }));
                    }
                });

                // Only proceed if ALL IPs are valid
                if (hasInvalidIPs) {
                    setAssigningIPs(false);
                    return;
                }

                // Apply the valid IPs to VMs
                const updatePromises = validIPs.map(async ({ vmId, interfaceIndex, ip }) => {
                    try {
                        const vm = vmsWithAssignments.find(v => v.id === vmId);
                        if (!vm) throw new Error('VM not found');

                        // Update network interfaces
                        if (vm.networkInterfaces && vm.networkInterfaces[interfaceIndex]) {
                            const updatedInterfaces = [...vm.networkInterfaces];
                            updatedInterfaces[interfaceIndex] = {
                                ...updatedInterfaces[interfaceIndex],
                                ipAddress: ip
                            };

                            await patchVMwareMachine(vmId, {
                                spec: {
                                    vms: {
                                        networkInterfaces: updatedInterfaces
                                    }
                                }
                            }, VJAILBREAK_DEFAULT_NAMESPACE);
                        } else {
                            // Fallback for single IP assignment
                            await patchVMwareMachine(vmId, {
                                spec: {
                                    vms: {
                                        assignedIp: ip
                                    }
                                }
                            }, VJAILBREAK_DEFAULT_NAMESPACE);
                        }

                        return { success: true, vmId, interfaceIndex, ip };
                    } catch (error) {
                        setBulkValidationStatus(prev => ({
                            ...prev,
                            [vmId]: { ...prev[vmId], [interfaceIndex]: 'invalid' }
                        }));
                        setBulkValidationMessages(prev => ({
                            ...prev,
                            [vmId]: { ...prev[vmId], [interfaceIndex]: error instanceof Error ? error.message : 'Failed to apply IP' }
                        }));
                        return { success: false, vmId, interfaceIndex, error };
                    }
                });

                const results = await Promise.all(updatePromises);

                // Check if any updates failed
                const failedUpdates = results.filter(result => !result.success);
                if (failedUpdates.length > 0) {
                    setAssigningIPs(false);
                    return; // Don't close modal if any updates failed
                }

                // Update local VM state
                const updatedVMs = vmsWithAssignments.map(vm => {
                    const vmUpdates = validIPs.filter(item => item.vmId === vm.id);
                    if (vmUpdates.length === 0) return vm;

                    const updatedVM = { ...vm };

                    if (vm.networkInterfaces) {
                        const updatedInterfaces = [...vm.networkInterfaces];
                        vmUpdates.forEach(({ interfaceIndex, ip }) => {
                            if (updatedInterfaces[interfaceIndex]) {
                                updatedInterfaces[interfaceIndex] = {
                                    ...updatedInterfaces[interfaceIndex],
                                    ipAddress: ip
                                };
                            }
                        });
                        updatedVM.networkInterfaces = updatedInterfaces;

                        // Recalculate comma-separated IP string
                        const allIPs = updatedInterfaces
                            .map(nic => nic.ipAddress)
                            .filter(ip => ip && ip.trim() !== "")
                            .join(", ");
                        updatedVM.ip = allIPs || "—";
                    } else {
                        // Fallback for single IP
                        const firstUpdate = vmUpdates[0];
                        if (firstUpdate) {
                            updatedVM.ip = firstUpdate.ip;
                        }
                    }

                    return updatedVM;
                });
                setVmsWithAssignments(updatedVMs);

                // Update bulk validation status
                const newBulkValidationStatus = { ...bulkValidationStatus };
                const newBulkValidationMessages = { ...bulkValidationMessages };

                validIPs.forEach(({ vmId, interfaceIndex }) => {
                    if (!newBulkValidationStatus[vmId]) newBulkValidationStatus[vmId] = {};
                    if (!newBulkValidationMessages[vmId]) newBulkValidationMessages[vmId] = {};

                    newBulkValidationStatus[vmId][interfaceIndex] = 'valid';
                    newBulkValidationMessages[vmId][interfaceIndex] = 'IP validated and applied successfully';
                });

                setBulkValidationStatus(newBulkValidationStatus);
                setBulkValidationMessages(newBulkValidationMessages);

                handleCloseBulkEditDialog();
            }

        } catch (error) {
            console.error("Error in bulk IP validation/assignment:", error);
            reportError(error as Error, {
                context: 'bulk-ip-validation-assignment',
                metadata: {
                    bulkEditIPs: bulkEditIPs,
                    action: 'bulk-ip-validation-assignment'
                }
            });
        } finally {
            setAssigningIPs(false);
        }
    };

    // Flavor assignment handlers
    const handleOpenFlavorDialog = () => {
        if (selectedVMs.length === 0) return;
        setFlavorDialogOpen(true);
    };

    const handleCloseFlavorDialog = () => {
        setFlavorDialogOpen(false);
        setSelectedFlavor("");
    };

    const handleFlavorChange = (event) => {
        setSelectedFlavor(event.target.value);
    };

    const handleIndividualFlavorChange = async (vmId: string, flavorValue: string) => {
        try {
            const isAutoAssign = flavorValue === "auto-assign";
            const selectedFlavorObj = !isAutoAssign ? openstackFlavors.find(f => f.id === flavorValue) : null;
            const flavorName = isAutoAssign ? "auto-assign" : (selectedFlavorObj ? selectedFlavorObj.name : flavorValue);

            // Update VM via API
            const payload = {
                spec: {
                    targetFlavorId: isAutoAssign ? "" : flavorValue
                }
            };

            await patchVMwareMachine(vmId, payload, VJAILBREAK_DEFAULT_NAMESPACE);

            // Update local state
            const updatedVMs = vmsWithAssignments.map(vm => {
                if (vm.id === vmId) {
                    return {
                        ...vm,
                        flavor: flavorName,
                        targetFlavorId: isAutoAssign ? "" : flavorValue
                    };
                }
                return vm;
            });
            setVmsWithAssignments(updatedVMs);

            console.log(`Successfully assigned flavor "${flavorName}" to VM ${vmId}`);

        } catch (error) {
            console.error(`Failed to update flavor for VM ${vmId}:`, error);
            reportError(error as Error, {
                context: 'individual-vm-flavor-update',
                metadata: {
                    vmId: vmId,
                    flavorValue: flavorValue,
                    isAutoAssign: flavorValue === "auto-assign",
                    action: 'individual-vm-flavor-update'
                }
            });
            alert(`Failed to assign flavor to VM: ${error instanceof Error ? error.message : 'Unknown error'}`);
        }
    };

    const handleIndividualHostConfigChange = async (hostId: string, configName: string) => {
        try {
            // Update the ESX host with the selected host config
            const updatedESXHosts = orderedESXHosts.map(host => {
                if (host.id === hostId) {
                    return {
                        ...host,
                        pcdHostConfigName: configName
                    };
                }
                return host;
            });

            setOrderedESXHosts(updatedESXHosts);

            console.log(`Successfully assigned host config "${configName}" to ESX host ${hostId}`);

        } catch (error) {
            console.error(`Failed to update host config for ESX host ${hostId}:`, error);
            reportError(error as Error, {
                context: 'individual-host-config-update',
                metadata: {
                    hostId: hostId,
                    configName: configName,
                    action: 'individual-host-config-update'
                }
            });
            alert(`Failed to assign host config to ESX host: ${error instanceof Error ? error.message : 'Unknown error'}`);
        }
    };

    const handleApplyFlavor = async () => {
        if (!selectedFlavor) {
            handleCloseFlavorDialog();
            return;
        }

        setUpdating(true);

        try {
            const isAutoAssign = selectedFlavor === "auto-assign";
            const selectedFlavorObj = !isAutoAssign ? openstackFlavors.find(f => f.id === selectedFlavor) : null;
            const flavorName = isAutoAssign ? "auto-assign" : (selectedFlavorObj ? selectedFlavorObj.name : selectedFlavor);

            // Update VMs via API
            const updatePromises = selectedVMs.map(async (vmId) => {
                try {
                    const payload = {
                        spec: {
                            targetFlavorId: isAutoAssign ? "" : selectedFlavor
                        }
                    };

                    await patchVMwareMachine(vmId as string, payload, VJAILBREAK_DEFAULT_NAMESPACE);
                    return { success: true, vmId };
                } catch (error) {
                    console.error(`Failed to update flavor for VM ${vmId}:`, error);
                    return { success: false, vmId, error };
                }
            });

            const results = await Promise.all(updatePromises);
            const failedUpdates = results.filter(result => !result.success);

            if (failedUpdates.length > 0) {
                console.error(`Failed to update flavor for ${failedUpdates.length} VMs`);
                reportError(new Error(`Failed to update flavor for ${failedUpdates.length} VMs`), {
                    context: 'vm-flavor-batch-update-failures',
                    metadata: {
                        failedUpdates: failedUpdates,
                        totalVMs: selectedVMs.length,
                        successCount: results.length - failedUpdates.length,
                        failedCount: failedUpdates.length,
                        action: 'vm-flavor-batch-update'
                    }
                });
                alert(`Failed to assign flavor to ${failedUpdates.length} VM${failedUpdates.length > 1 ? 's' : ''}`);
            } else {
                // Update local state only if all API calls succeeded
                const updatedVMs = vmsWithAssignments.map(vm => {
                    if (selectedVMs.includes(vm.id)) {
                        return {
                            ...vm,
                            flavor: flavorName,
                            targetFlavorId: isAutoAssign ? "" : selectedFlavor
                        };
                    }
                    return vm;
                });
                setVmsWithAssignments(updatedVMs);

                const actionText = isAutoAssign ? "cleared flavor assignment for" : "assigned flavor to";
                console.log(`Successfully ${actionText} ${selectedVMs.length} VM${selectedVMs.length > 1 ? 's' : ''}`);

                // Refresh VM list to get updated flavor information from API
                await fetchClusterVMs();
            }

            handleCloseFlavorDialog();
        } catch (error) {
            console.error("Error updating flavors:", error);
            reportError(error as Error, {
                context: 'vm-flavor-assignment',
                metadata: {
                    selectedVMs: selectedVMs,
                    selectedFlavor: selectedFlavor,
                    action: 'vm-flavor-assignment'
                }
            });
            alert("Failed to assign flavor to VMs");
        } finally {
            setUpdating(false);
        }
    };

    return (
        <>
            <GlobalStyles
                styles={{
                    '.MuiDataGrid-columnsManagement, .MuiDataGrid-columnsManagementPopover': {
                        '& .MuiFormControlLabel-label': {
                            fontSize: '0.875rem !important',
                        },
                        '& .MuiCheckbox-root': {
                            padding: '4px !important',
                        },
                        '& .MuiListItem-root': {
                            fontSize: '0.875rem !important',
                            minHeight: '32px !important',
                            padding: '2px 8px !important',
                        },
                        '& .MuiTypography-root': {
                            fontSize: '0.875rem !important',
                        },
                        '& .MuiInputBase-input': {
                            fontSize: '0.875rem !important',
                        },
                        '& .MuiTextField-root .MuiInputBase-input': {
                            fontSize: '0.875rem !important',
                        }
                    }
                }}
            />
            <StyledDrawer
                anchor="right"
                open={open}
                onClose={handleClose}
                ModalProps={{
                    keepMounted: false,
                    style: { zIndex: 1300 }
                }}
            >
                <Header title="Cluster Conversion" icon={<ClusterIcon />} />


                <DrawerContent>
                    <Box sx={{ display: "grid", gap: 4 }}>
                        <SourceDestinationClusterSelection
                            onChange={() => () => { }}
                            errors={{}}
                            stepNumber="1"
                            stepLabel="Source & Destination"
                            onVmwareClusterChange={handleSourceClusterChange}
                            onPcdClusterChange={handleDestinationPCDChange}
                            vmwareCluster={sourceCluster}
                            pcdCluster={destinationPCD}
                            loadingVMware={loading}
                            loadingPCD={loadingPCD}
                        />
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
                                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                                    <Typography variant="body2" color="text.secondary">
                                        Select ESXi hosts and assign PCD host configurations
                                    </Typography>
                                    {esxHostMappingStatus.fullyMapped && esxHostMappingStatus.total > 0 ? (
                                        <Typography variant="body2" color="success.main">
                                            All hosts mapped ✓
                                        </Typography>
                                    ) :
                                        <Typography variant="body2" color="warning.main">
                                            {esxHostMappingStatus.mapped} of {esxHostMappingStatus.total} hosts unmapped
                                        </Typography>
                                    }
                                </Box>
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
                                            toolbar: (props) => (
                                                <CustomESXToolbarWithActions
                                                    {...props}
                                                    onAssignHostConfig={handleOpenPcdHostConfigDialog}
                                                />
                                            )
                                        }}
                                        disableColumnMenu
                                        disableColumnFilter
                                        loading={loadingHosts}
                                    />
                                </Paper>
                                {esxHostConfigValidationError && (
                                    <Alert severity="warning" sx={{ mt: 2 }}>
                                        {esxHostConfigValidationError}
                                    </Alert>
                                )}
                            </Box>
                        </Box>

                        <Box>
                            <Step stepNumber="4" label="Select Virtual Machines to Migrate" />
                            <Box sx={{ ml: 5, mt: 2 }}>
                                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                                    <Typography variant="body2" color="text.secondary">
                                        Select VMs to migrate
                                    </Typography>
                                </Box>
                                <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block', mb: 1 }}>
                                    💡 Tip: Powered-off VMs require IP Address and OS assignment for proper migration configuration
                                </Typography>
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
                                        rowHeight={52}
                                        checkboxSelection
                                        onRowSelectionModelChange={(selectedRowIds) => {
                                            setSelectedVMs(selectedRowIds);
                                        }}
                                        rowSelectionModel={selectedVMs.filter(vmId =>
                                            vmsWithAssignments.some(vm => vm.id === vmId)
                                        )}
                                        slots={{
                                            toolbar: (props) => (
                                                <CustomToolbarWithActions
                                                    {...props}
                                                    rowSelectionModel={selectedVMs.filter(vmId =>
                                                        vmsWithAssignments.some(vm => vm.id === vmId)
                                                    )}
                                                    onAssignFlavor={handleOpenFlavorDialog}
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
                                        disableColumnFilter
                                        disableRowSelectionOnClick
                                        loading={loadingVMs}
                                    />
                                </Paper>
                                {vmIpValidationError && (
                                    <Alert severity="warning" >
                                        {vmIpValidationError}
                                    </Alert>
                                )}
                                {osValidationError && (
                                    <Alert severity="warning" >
                                        {osValidationError}
                                    </Alert>
                                )}
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
                                        loading={loadingOpenstackDetails}
                                    />
                                    <MigrationOptions
                                        stepNumber="6"
                                        params={params}
                                        onChange={getParamsUpdater}
                                        selectedMigrationOptions={selectedMigrationOptions}
                                        updateSelectedMigrationOptions={updateSelectedMigrationOptions}
                                        errors={fieldErrors}
                                        getErrorsUpdater={getFieldErrorsUpdater}
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

                {/* PCD Host Config Assignment Dialog */}
                <Dialog
                    open={pcdHostConfigDialogOpen}
                    onClose={handleClosePcdHostConfigDialog}
                    fullWidth
                    maxWidth="sm"
                >
                    <DialogTitle>
                        Assign Host Config to All ESXi Hosts
                    </DialogTitle>
                    <DialogContent>
                        <Box sx={{ my: 2 }}>
                            <Typography variant="body2" gutterBottom>
                                Select Host Configuration
                            </Typography>
                            <Select
                                fullWidth
                                value={selectedPcdHostConfig}
                                onChange={handlePcdHostConfigChange}
                                size="small"
                                sx={{ mt: 1 }}
                                displayEmpty
                            >
                                <MenuItem value="">
                                    <em>Select a  host configuration</em>
                                </MenuItem>
                                {(openstackCredData?.spec?.pcdHostConfig || []).map((config) => (
                                    <MenuItem key={config.id} value={config.id}>
                                        <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                                            <Typography variant="body1">{config.name}</Typography>
                                            <Typography variant="caption" color="text.secondary">
                                                Management Interface: {config.mgmtInterface}
                                            </Typography>
                                        </Box>
                                    </MenuItem>
                                ))}
                            </Select>
                        </Box>
                    </DialogContent>
                    <DialogActions>
                        <Button onClick={handleClosePcdHostConfigDialog}>Cancel</Button>
                        <Button
                            onClick={handleApplyPcdHostConfig}
                            variant="contained"
                            color="primary"
                            disabled={!selectedPcdHostConfig || updatingPcdMapping}
                        >
                            {updatingPcdMapping ? "Applying..." : "Apply to all hosts"}
                        </Button>
                    </DialogActions>
                </Dialog>

                {/* Bulk IP Editor Dialog */}
                <Dialog
                    open={bulkEditDialogOpen}
                    onClose={handleCloseBulkEditDialog}
                    maxWidth="md"
                >
                    <DialogTitle>
                        Edit IP Addresses for {selectedVMs.length} {selectedVMs.length === 1 ? 'VM' : 'VMs'}
                    </DialogTitle>
                    <DialogContent>
                        <Box sx={{ my: 2 }}>
                            {/* Quick Actions */}
                            <Box sx={{ mb: 3, display: 'flex', justifyContent: 'flex-end' }}>
                                <Button size="small" variant="outlined" onClick={handleClearAllIPs}>
                                    Clear All
                                </Button>
                            </Box>

                            {/* IP Editor Fields */}
                            <Box sx={{ maxHeight: 400, overflowY: 'auto' }}>
                                {Object.entries(bulkEditIPs).map(([vmId, interfaces]) => {
                                    const vm = vmsWithAssignments.find(v => v.id === vmId);
                                    if (!vm) return null;

                                    return (
                                        <Box key={vmId} sx={{ mb: 3, p: 2, border: '1px solid #e0e0e0', borderRadius: 1 }}>
                                            <Typography variant="body2" sx={{ fontWeight: 500, mb: 1 }}>
                                                {vm.name}
                                            </Typography>

                                            {Object.entries(interfaces).map(([interfaceIndexStr, ip]) => {
                                                const interfaceIndex = parseInt(interfaceIndexStr);
                                                const networkInterface = vm.networkInterfaces?.[interfaceIndex];
                                                const status = bulkValidationStatus[vmId]?.[interfaceIndex];
                                                const message = bulkValidationMessages[vmId]?.[interfaceIndex];

                                                return (
                                                    <Box key={interfaceIndex} sx={{ mb: 2, display: 'flex', alignItems: 'center', gap: 2 }}>
                                                        <Box sx={{ width: 120, flexShrink: 0 }}>
                                                            <Typography variant="caption" color="text.secondary">
                                                                {networkInterface?.network || `Interface ${interfaceIndex + 1}`}:
                                                            </Typography>
                                                            <Typography variant="caption" display="block" color="text.secondary">
                                                                Current: {networkInterface?.ipAddress || vm.ip || "—"}
                                                            </Typography>
                                                        </Box>
                                                        <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
                                                            <TextField
                                                                value={ip}
                                                                onChange={(e) => handleBulkIpChange(vmId, interfaceIndex, e.target.value)}
                                                                placeholder="Enter IP address"
                                                                size="small"
                                                                sx={{ flex: 1 }}
                                                                error={status === 'invalid'}
                                                                helperText={message}
                                                            />
                                                            <Box sx={{ width: 24, display: 'flex' }}>
                                                                {status === 'validating' && <CircularProgress size={20} />}
                                                                {status === 'valid' && <CheckCircleIcon color="success" fontSize="small" />}
                                                                {status === 'invalid' && <ErrorIcon color="error" fontSize="small" />}
                                                            </Box>
                                                        </Box>
                                                    </Box>
                                                );
                                            })}
                                        </Box>
                                    );
                                })}
                            </Box>
                        </Box>
                    </DialogContent>
                    <DialogActions>
                        <Button onClick={handleCloseBulkEditDialog}>Cancel</Button>
                        <Button
                            onClick={handleApplyBulkIPs}
                            variant="contained"
                            color="primary"
                            disabled={Object.values(bulkEditIPs).every(ip => !ip) || assigningIPs}
                        >
                            {assigningIPs ? "Applying..." : "Apply Changes"}
                        </Button>
                    </DialogActions>
                </Dialog>



                {/* Flavor Assignment Dialog */}
                <Dialog
                    open={flavorDialogOpen}
                    onClose={handleCloseFlavorDialog}
                    fullWidth
                    maxWidth="sm"
                >
                    <DialogTitle>
                        Assign Flavor to {selectedVMs.length} {selectedVMs.length === 1 ? 'VM' : 'VMs'}
                    </DialogTitle>
                    <DialogContent>
                        <Box sx={{ my: 2 }}>
                            <FormLabel>Select Flavor</FormLabel>
                            <Select
                                fullWidth
                                value={selectedFlavor}
                                onChange={handleFlavorChange}
                                size="small"
                                sx={{ mt: 1 }}
                                displayEmpty
                            >
                                <MenuItem value="">
                                    <em>Select a flavor</em>
                                </MenuItem>
                                <MenuItem value="auto-assign">
                                    <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                                        <Typography variant="body1">Auto Assign</Typography>
                                        <Typography variant="caption" color="text.secondary">
                                            Let OpenStack automatically assign the most suitable flavor
                                        </Typography>
                                    </Box>
                                </MenuItem>
                                {openstackFlavors.map((flavor) => (
                                    <MenuItem key={flavor.id} value={flavor.id}>
                                        <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                                            <Typography variant="body1">{flavor.name}</Typography>
                                            <Typography variant="caption" color="text.secondary">
                                                {flavor.vcpus} vCPU, {flavor.ram / 1024}GB RAM, {flavor.disk}GB Storage
                                            </Typography>
                                        </Box>
                                    </MenuItem>
                                ))}
                            </Select>
                        </Box>
                    </DialogContent>
                    <DialogActions>
                        <Button onClick={handleCloseFlavorDialog}>Cancel</Button>
                        <Button
                            onClick={handleApplyFlavor}
                            variant="contained"
                            color="primary"
                            disabled={!selectedFlavor || updating}
                        >
                            {updating ? "Applying..." : "Apply to selected VMs"}
                        </Button>
                    </DialogActions>
                </Dialog>

                {/* IP Address Editor Modal */}
                <Dialog
                    open={ipEditModalOpen}
                    onClose={handleCloseIpEditModal}
                    maxWidth="sm"
                    PaperProps={{
                        sx: {
                            width: 'auto',
                            maxWidth: '500px'
                        }
                    }}
                >
                    <DialogTitle>
                        <Typography variant="h6">
                            Edit IP Addresses for "{editingVm?.name}"
                        </Typography>
                        <Typography variant="body2">
                            ({editingVm?.networkInterfaces?.length || 0} Network Interfaces)
                        </Typography>
                    </DialogTitle>
                    <DialogContent>
                        <Box sx={{ mt: 2 }}>
                            {editingVm?.networkInterfaces?.map((nic, index) => {
                                const key = `interface-${index}`;
                                const currentValue = modalIpValues[key] || "";
                                const validationStatus = modalValidationStatus[key] || 'pending';
                                const validationMessage = modalValidationMessages[key] || "";

                                return (
                                    <Box key={index} sx={{ py: 2 }}>
                                        <Box sx={{
                                            display: 'flex',
                                            alignItems: 'flex-start',
                                            gap: 3,
                                            flexDirection: { xs: 'column', sm: 'row' }
                                        }}>
                                            <Box sx={{
                                                minWidth: { sm: 200 },
                                                display: 'flex',
                                                flexDirection: 'column',
                                                gap: 0.5
                                            }}>
                                                <Typography variant="subtitle2" sx={{ fontWeight: 500 }}>
                                                    Network Interface {index + 1}
                                                </Typography>
                                                <Typography variant="body2" color="text.secondary">
                                                    Network:<strong>{nic.network}</strong>
                                                </Typography>
                                                {nic.mac && (
                                                    <Typography variant="caption" color="text.secondary">
                                                        MAC:<strong>{nic.mac}</strong>
                                                    </Typography>
                                                )}
                                            </Box>
                                            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                                                <TextField
                                                    label="IP Address"
                                                    value={currentValue}
                                                    onChange={(e) => handleModalIpChange(index, e.target.value)}
                                                    size="small"
                                                    placeholder="192.168.1.100"
                                                    sx={{
                                                        width: '200px',
                                                        '& .MuiInputBase-input': {
                                                            fontFamily: 'monospace'
                                                        }
                                                    }}
                                                    error={validationStatus === 'invalid'}
                                                    helperText={validationMessage || ""}
                                                    InputProps={{
                                                        endAdornment: validationStatus === 'validating' ? (
                                                            <Box sx={{ display: 'flex', alignItems: 'center', mr: 1 }}>
                                                                <Typography variant="caption" color="text.secondary">
                                                                    Validating...
                                                                </Typography>
                                                            </Box>
                                                        ) : validationStatus === 'valid' ? (
                                                            <Box sx={{ display: 'flex', alignItems: 'center', mr: 1 }}>
                                                                <Typography variant="caption" color="success.main">
                                                                    ✓ Valid
                                                                </Typography>
                                                            </Box>
                                                        ) : null
                                                    }}
                                                />
                                            </Box>
                                        </Box>
                                    </Box>
                                );
                            })}
                        </Box>
                    </DialogContent>
                    <DialogActions sx={{ px: 3, pb: 3 }}>
                        <Button onClick={handleCloseIpEditModal} variant="outlined">
                            Cancel
                        </Button>
                        <Button
                            onClick={handleSaveModalIPs}
                            variant="contained"
                            color="primary"
                            disabled={(() => {
                                // Check if any IPs are assigned and valid
                                const hasAnyIPs = Object.values(modalIpValues).some(ip => ip && ip.trim() !== "");
                                const hasInvalidIPs = Object.values(modalValidationStatus).some(status => status === 'invalid');
                                const isValidating = Object.values(modalValidationStatus).some(status => status === 'validating');

                                // Disable if: no IPs assigned OR any invalid IPs OR currently validating
                                return !hasAnyIPs || hasInvalidIPs || isValidating;
                            })()}
                        >
                            Save IP Addresses
                        </Button>
                    </DialogActions>
                </Dialog>

                {/* Toast Notification */}
                <Snackbar
                    open={toastOpen}
                    autoHideDuration={4000}
                    onClose={handleCloseToast}
                    anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
                >
                    <Alert
                        onClose={handleCloseToast}
                        severity={toastSeverity}
                        sx={{ width: '100%' }}
                        variant="standard"
                    >
                        {toastMessage}
                    </Alert>
                </Snackbar>
            </StyledDrawer>
        </>
    );
} 
