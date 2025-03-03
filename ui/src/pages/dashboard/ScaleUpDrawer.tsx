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
} from "@mui/material";
import { useState, useEffect } from "react";
import Step from "src/components/forms/Step";
import { StyledDrawer, DrawerContent } from "src/components/forms/StyledDrawer";
import Header from "src/components/forms/Header";
import Footer from "src/components/forms/Footer";
import OpenstackCredentialsForm from "src/components/forms/OpenstackCredentialsForm";
import InfoIcon from '@mui/icons-material/Info';
import { getOpenstackCredentials } from "src/api/openstack-creds/openstackCreds";
import { OpenstackCreds } from "src/api/openstack-creds/model";
import { createNodes, getMasterNode } from "src/api/nodes/nodeMappings";
import { ArrowDropDownIcon } from "@mui/x-date-pickers/icons";
import { OpenstackFlavor } from "src/api/nodes/model";
import { NodeItem } from "src/api/nodes/model";
import { useOpenstackCredentialsQuery } from "src/hooks/api/useOpenstackCredentialsQuery";
import { createOpenstackCredsWithSecretFlow } from "src/api/helpers";
import { useInterval } from "src/hooks/useInterval";
import { THREE_SECONDS } from "src/constants";
import axios from "axios";

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
    const [validatingOpenstackCreds, setValidatingOpenstackCreds] = useState(false);
    const [selectedOpenstackCred, setSelectedOpenstackCred] = useState<string | null>(null);
    const [openstackError, setOpenstackError] = useState<string | null>(null);

    const [flavors, setFlavors] = useState<Array<OpenstackFlavor>>([]);
    const [selectedFlavor, setSelectedFlavor] = useState('');
    const [loadingFlavors, setLoadingFlavors] = useState(false);
    const [flavorsError, setFlavorsError] = useState<string | null>(null);

    // Fetch credentials list
    const { data: openstackCredsList = [], isLoading: loadingOpenstackCreds, refetch: refetchOpenstackCreds } = useOpenstackCredentialsQuery();


    const openstackCredsValidated = openstackCredentials?.status?.openstackValidationStatus === "Succeeded";

    // Polling condition for OpenStack credentials
    const shouldPollOpenstackCreds =
        !!openstackCredentials?.metadata?.name &&
        openstackCredentials?.status === undefined;

    // Reset state when drawer closes
    const handleClose = () => {
        clearStates();
        onClose();
    };

    const clearStates = () => {
        setOpenstackCredentials(null);
        setSelectedOpenstackCred(null);
        setValidatingOpenstackCreds(false);
        setOpenstackError(null);
        setNodeCount(1);
        setError(null);
        setSelectedFlavor('');
        setFlavors([]);
        setLoadingFlavors(false);
        setFlavorsError(null);
    }

    const handleOpenstackCredChange = (values: Record<string, string | number | boolean>) => {
        setOpenstackError(null);

        // If this is a new credential being created
        if ('credentialName' in values) {
            handleCreateOpenstackCred(values);
        }
    };

    const handleCreateOpenstackCred = async (values: Record<string, string | number | boolean>) => {
        setValidatingOpenstackCreds(true);
        try {
            // Format the values for the secret-based flow
            const response = await createOpenstackCredsWithSecretFlow(
                values.credentialName as string,
                {
                    OS_AUTH_URL: values.OS_AUTH_URL as string,
                    OS_DOMAIN_NAME: values.OS_DOMAIN_NAME as string,
                    OS_USERNAME: values.OS_USERNAME as string,
                    OS_PASSWORD: values.OS_PASSWORD as string,
                    OS_REGION_NAME: values.OS_REGION_NAME as string,
                    OS_TENANT_NAME: values.OS_TENANT_NAME as string,
                }
            );
            setOpenstackCredentials(response);
            // Only set validatingOpenstackCreds to false if status is already defined
            if (response?.status) {
                setValidatingOpenstackCreds(false);
                if (response?.status?.openstackValidationStatus !== "Succeeded") {
                    setOpenstackError("Error Validating OpenStack credentials");
                }
            }
        } catch (error) {
            console.error("Error creating OpenStack credentials:", error);
            setValidatingOpenstackCreds(false);
            setOpenstackError(
                "Error creating OpenStack credentials: " + (axios.isAxiosError(error) ? error?.response?.data?.message : error)
            );
        }
    };

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
                    const response = await getMasterNode();
                    console.log(response);
                    const flavours = response?.spec.availableflavors;
                    console.log(flavours);

                    if (!flavours) {
                        // retry for 3 times in a interval of 5 seconds
                        let retries = 0;
                        const interval = setInterval(async () => {
                            const response = await getMasterNode();
                            console.log(response);
                            const flavours = response?.spec.availableflavors
                            console.log(flavours);
                            if (flavours) {
                                clearInterval(interval);
                                setFlavors(flavours || []);
                            } else {
                                retries++;
                                if (retries >= 3) {
                                    clearInterval(interval);
                                    setFlavorsError('Failed to fetch OpenStack flavors');
                                }
                            }
                        }, 5000);
                    }
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

    // Add polling for OpenStack credentials status
    useInterval(
        async () => {
            if (shouldPollOpenstackCreds) {
                try {
                    const response = await getOpenstackCredentials(
                        openstackCredentials?.metadata?.name
                    );
                    setOpenstackCredentials(response);
                    const validationStatus = response?.status?.openstackValidationStatus;
                    if (validationStatus) {
                        setValidatingOpenstackCreds(false);
                        if (validationStatus !== "Succeeded") {
                            setOpenstackError(
                                response?.status?.openstackValidationMessage || "Error validating OpenStack credentials"
                            );
                        }
                    }
                } catch (err) {
                    console.error("Error validating OpenStack credentials", err);
                    setOpenstackError(
                        "Error validating OpenStack credentials"
                    );
                    setValidatingOpenstackCreds(false);
                }
            }
        },
        THREE_SECONDS,
        shouldPollOpenstackCreds
    );

    const handleSubmit = async () => {
        if (!masterNode?.spec.imageid || !selectedFlavor || !nodeCount || !openstackCredentials?.metadata?.name) {
            setError('Please fill in all required fields');
            return;
        }

        try {
            setLoading(true);
            await createNodes({
                imageId: masterNode.spec.imageid,
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
                            tooltip="Select existing OpenStack credentials or create new ones to authenticate with the OpenStack platform where new nodes will be created."
                        />
                        <Box sx={{ ml: 6, mt: 2 }}>
                            <FormControl fullWidth error={!!openstackError} required>
                                <OpenstackCredentialsForm
                                    fullWidth={true}
                                    size="medium"
                                    credentialsList={openstackCredsList}
                                    loadingCredentials={loadingOpenstackCreds}
                                    refetchCredentials={refetchOpenstackCreds}
                                    validatingCredentials={validatingOpenstackCreds}
                                    credentialsValidated={openstackCredsValidated}
                                    error={openstackError || ""}
                                    onChange={handleOpenstackCredChange}
                                    onCredentialSelect={handleOpenstackCredSelect}
                                    selectedCredential={selectedOpenstackCred}
                                    showCredentialNameField={true}
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
                                />
                            </FormControl>
                            <FormControl fullWidth>
                                <Typography variant="body1" style={{ color: 'red' }}>
                                    ⚠️ Please select a flavor with a disk size greater than 16GB.
                                </Typography>
                            </FormControl>
                            <FormControl error={!!flavorsError} fullWidth>
                                <InputLabel>{loadingFlavors ? "Loading Flavors..." : "Flavor"}</InputLabel>
                                <Select
                                    value={selectedFlavor}
                                    label="Flavor"
                                    onChange={(e) => setSelectedFlavor(e.target.value)}
                                    required
                                    disabled={loadingFlavors || !openstackCredsValidated || !openstackCredentials}
                                    IconComponent={
                                        loadingFlavors
                                            ? () => <CircularProgress size={24} sx={{ marginRight: 2, display: 'flex', alignItems: 'center' }} />
                                            : ArrowDropDownIcon
                                    }
                                >
                                    {flavors.map((flavor) => (
                                        <MenuItem key={flavor.id} value={flavor.id}>
                                            {`${flavor.name} (${flavor.vcpus} vCPU, ${flavor.ram / 1024}GB RAM, ${flavor.disk}GB disk)`}
                                        </MenuItem>
                                    ))}
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