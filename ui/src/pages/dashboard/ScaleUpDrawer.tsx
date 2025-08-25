import {
    Box,
    FormControl,
    FormLabel,
    InputLabel,
    MenuItem,
    Select,
    TextField,
    Typography,
    IconButton,
    Tooltip,
    CircularProgress,
    InputAdornment,
} from "@mui/material";
import React, {
    useState, useCallback, useEffect 
} from "react";
import Step from "src/components/forms/Step";
import { StyledDrawer, DrawerContent } from "src/components/forms/StyledDrawer";
import Header from "src/components/forms/Header";
import Footer from "src/components/forms/Footer";
import OpenstackCredentialsForm from "src/components/forms/OpenstackCredentialsForm";
import InfoIcon from '@mui/icons-material/Info';
import { getOpenstackCredentials } from "src/api/openstack-creds/openstackCreds";
import { OpenstackCreds } from "src/api/openstack-creds/model";
import { createNodes } from "src/api/nodes/nodeMappings";
import { ArrowDropDownIcon } from "@mui/x-date-pickers/icons";
import { OpenstackFlavor } from "src/api/openstack-creds/model";
import SearchIcon from '@mui/icons-material/Search';
import { NodeItem } from "src/api/nodes/model";
import { useOpenstackCredentialsQuery } from "src/hooks/api/useOpenstackCredentialsQuery";
import axios from "axios";
import { useKeyboardSubmit } from "src/hooks/ui/useKeyboardSubmit";

// Mock data - replace with actual data from API

interface ScaleUpDrawerProps {
    open: boolean;
    onClose: () => void;
    masterNode: NodeItem | null;
}

const StepHeader = ({ number, label, tooltip }: { number: string, label: string, tooltip: string }) => (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Step stepNumber={number} label={label} sx={{ mb: 0 }} />
        <Tooltip title={tooltip} arrow>
            <IconButton size="small" color="info">
                <InfoIcon />
            </IconButton>
        </Tooltip>
    </Box>
);

export default function ScaleUpDrawer({ open, onClose, masterNode }: ScaleUpDrawerProps) {
    const [openstackCredentials, setOpenstackCredentials] = useState<OpenstackCreds | null>(null);
    const [nodeCount, setNodeCount] = useState(1);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [selectedOpenstackCred, setSelectedOpenstackCred] = useState<string | null>(null);
    const [openstackError, setOpenstackError] = useState<string | null>(null);

    const [flavors, setFlavors] = useState<Array<OpenstackFlavor>>([]);
    const [selectedFlavor, setSelectedFlavor] = useState('');
    const [loadingFlavors, setLoadingFlavors] = useState(false);
    const [flavorsError, setFlavorsError] = useState<string | null>(null);
    const [flavorSearchTerm, setFlavorSearchTerm] = useState('');
    
    // Filter flavors based on search term
    const filteredFlavors = React.useMemo(() => {
        return flavors.filter(flavor => 
            flavor.name.toLowerCase().includes(flavorSearchTerm.toLowerCase()) || 
            `${flavor.vcpus} vCPU`.toLowerCase().includes(flavorSearchTerm.toLowerCase()) || 
            `${flavor.ram / 1024}GB RAM`.toLowerCase().includes(flavorSearchTerm.toLowerCase()) || 
            `${flavor.disk}GB disk`.toLowerCase().includes(flavorSearchTerm.toLowerCase())
        );
    }, [flavors, flavorSearchTerm]);

    // Fetch credentials list
    const { data: openstackCredsList = [], isLoading: loadingOpenstackCreds } = useOpenstackCredentialsQuery();

    const openstackCredsValidated = openstackCredentials?.status?.openstackValidationStatus === "Succeeded";

    const clearStates = () => {
        setOpenstackCredentials(null);
        setSelectedOpenstackCred(null);
        setOpenstackError(null);
        setNodeCount(1);
        setError(null);
        setSelectedFlavor('');
        setFlavors([]);
        setLoadingFlavors(false);
        setFlavorsError(null);
        setFlavorSearchTerm('');
    }

    // Reset state when drawer closes
    const handleClose = useCallback(() => {
        clearStates();
        onClose();
    }, [onClose]);

    const handleOpenstackCredSelect = async (credId: string | null) => {
        setSelectedOpenstackCred(credId);

        if (credId) {
            try {
                const response = await getOpenstackCredentials(credId);
                setOpenstackCredentials(response);
            } catch (error) {
                console.error("Error fetching OpenStack credentials:", error);
                setOpenstackError(
                    "Error fetching OpenStack credentials: " + (axios.isAxiosError(error) ? error?.response?.data?.message : error)
                );
            }
        } else {
            setOpenstackCredentials(null);
        }
    };

    useEffect(() => {
        const fetchFlavours = async () => {
            if (openstackCredsValidated || openstackCredentials) {
                setLoadingFlavors(true);
                try {
                    const flavours = openstackCredentials?.spec.flavors;
                    console.log(flavours);
                    setFlavors(flavours || []);

                } catch (error) {
                    console.error('Failed to fetch flavors:', error);
                    setFlavorsError('Failed to fetch OpenStack flavors');
                } finally {
                    setLoadingFlavors(false);
                }
            }
        };
        fetchFlavours();
    }, [openstackCredsValidated, openstackCredentials]);

    const handleSubmit = async () => {
        if (!masterNode?.spec.openstackImageID || !selectedFlavor || !nodeCount || !openstackCredentials?.metadata?.name) {
            setError('Please fill in all required fields');
            return;
        }

        try {
            setLoading(true);
            await createNodes({
                imageId: masterNode.spec.openstackImageID,
                openstackCreds: {
                    kind: "openstackcreds" as const,
                    name: openstackCredentials.metadata.name,
                    namespace: "migration-system"
                },
                count: nodeCount,
                flavorId: selectedFlavor
            });

            handleClose();
        } catch (error) {
            console.error('Error scaling up nodes:', error);
            setError(error instanceof Error ? error.message : 'Failed to scale up nodes');
        } finally {
            setLoading(false);
        }
    };

    useKeyboardSubmit({
        open,
        isSubmitDisabled: !masterNode || !selectedFlavor || loading || !openstackCredsValidated,
        onSubmit: handleSubmit,
        onClose: handleClose
    });

    return (
        <StyledDrawer
            anchor="right"
            open={open}
            onClose={handleClose}
        >
            <Header title="Scale Up Agents" />
            <DrawerContent>
                <Box sx={{ display: "grid", gap: 4 }}>
                    {/* Step 1: OpenStack Credentials */}
                    <div>
                        <StepHeader
                            number="1"
                            label="OpenStack Credentials"
                            tooltip="Select existing OpenStack credentials to authenticate with the OpenStack platform where new nodes will be created."
                        />
                        <Box sx={{ ml: 6, mt: 2 }}>
                            <FormControl fullWidth error={!!openstackError} required>
                                <OpenstackCredentialsForm
                                    fullWidth={true}
                                    size="small"
                                    credentialsList={openstackCredsList}
                                    loadingCredentials={loadingOpenstackCreds}
                                    error={openstackError || ""}
                                    onCredentialSelect={handleOpenstackCredSelect}
                                    selectedCredential={selectedOpenstackCred}
                                    showCredentialSelector={true}
                                />
                            </FormControl>
                        </Box>
                    </div>

                    {/* Step 2: Agent Template */}
                    <div>
                        <StepHeader
                            number="2"
                            label="Agent Template"
                            tooltip="Configure the specification for the new nodes."
                        />
                        <Box sx={{ ml: 6, mt: 2, display: 'grid', gap: 3 }}>
                            <FormControl fullWidth>
                                <TextField
                                    label="Master Agent Image"
                                    value={'Image selected from the first vjailbreak node'}
                                    disabled
                                    fullWidth
                                    size="small"
                                />
                            </FormControl>
                            <FormControl fullWidth>
                                <Typography variant="body1" style={{ color: 'red' }}>
                                    ⚠️ Please select a flavor with a disk size greater than 16GB.
                                </Typography>
                            </FormControl>
                            <FormControl error={!!flavorsError} fullWidth>
                                <InputLabel size="small">{loadingFlavors ? "Loading Flavors..." : "Flavor"}</InputLabel>
                                <Select
                                    value={selectedFlavor}
                                    label="Flavor"
                                    onChange={(e) => setSelectedFlavor(e.target.value)}
                                    required
                                    size="small"
                                    disabled={loadingFlavors || !openstackCredsValidated || !openstackCredentials}
                                    IconComponent={
                                        loadingFlavors
                                            ? () => <CircularProgress size={24} sx={{ marginRight: 2, display: 'flex', alignItems: 'center' }} />
                                            : ArrowDropDownIcon
                                    }
                                    MenuProps={{
                                        PaperProps: {
                                            style: {
                                                maxHeight: 300
                                            }
                                        }
                                    }}
                                >
                                    <Box sx={{ p: 1, position: 'sticky', top: 0, bgcolor: 'background.paper', zIndex: 1 }}>
                                        <TextField
                                            size="small"
                                            placeholder="Search flavors"
                                            fullWidth
                                            value={flavorSearchTerm}
                                            onChange={(e) => {
                                                e.stopPropagation();
                                                setFlavorSearchTerm(e.target.value);
                                            }}
                                            onClick={(e) => e.stopPropagation()}
                                            onKeyDown={(e) => e.stopPropagation()}
                                            autoFocus
                                            InputProps={{
                                                startAdornment: (
                                                    <InputAdornment position="start">
                                                        <SearchIcon fontSize="small" />
                                                    </InputAdornment>
                                                ),
                                            }}
                                        />
                                    </Box>
                                    {flavors.length === 0 ? (
                                        <MenuItem disabled>No flavors available</MenuItem>
                                    ) : filteredFlavors.length === 0 ? (
                                        <MenuItem disabled>No matching flavors found</MenuItem>
                                    ) : (
                                        filteredFlavors.map((flavor) => (
                                            <MenuItem key={flavor.id} value={flavor.id}>
                                                {`${flavor.name} (${flavor.vcpus} vCPU, ${flavor.ram / 1024}GB RAM, ${flavor.disk}GB disk)`}
                                            </MenuItem>
                                        ))
                                    )}
                                </Select>
                                {flavorsError && (
                                    <FormLabel error sx={{ mt: 1, fontSize: '0.75rem' }}>
                                        {flavorsError}
                                    </FormLabel>
                                )}
                            </FormControl>
                        </Box>
                    </div>

                    {/* Step 3: Node Count */}
                    <div>
                        <StepHeader
                            number="3"
                            label="Agent Count"
                            tooltip="Specify how many new nodes to create based on the above node template."
                        />
                        <Box sx={{ ml: 6, mt: 2 }}>
                            <TextField
                                type="number"
                                label="Number of Agents"
                                value={nodeCount}
                                onChange={(e) => {
                                    const value = parseInt(e.target.value);
                                    if (value >= 1 && value <= 5) {
                                        setNodeCount(value);
                                    }
                                }}
                                inputProps={{ min: 1, max: 5 }}
                                fullWidth
                                size="small"
                                helperText="Min: 1, Max: 5 nodes"
                            />
                        </Box>
                    </div>

                    {error && (
                        <Typography color="error" sx={{ ml: 6 }}>
                            {error}
                        </Typography>
                    )}
                </Box>
            </DrawerContent>
            <Footer
                submitButtonLabel="Scale Up"
                onClose={handleClose}
                onSubmit={handleSubmit}
                disableSubmit={!masterNode || !selectedFlavor || loading || !openstackCredsValidated}
                submitting={loading}
            />
        </StyledDrawer>
    );
} 