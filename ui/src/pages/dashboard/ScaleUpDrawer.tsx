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
import { useState, useCallback, useEffect } from "react";
import Step from "src/components/forms/Step";
import { StyledDrawer, DrawerContent } from "src/components/forms/StyledDrawer";
import Header from "src/components/forms/Header";
import Footer from "src/components/forms/Footer";
import OpenstackRCFileUpload from "src/components/forms/OpenstackRCFileUpload";
import InfoIcon from '@mui/icons-material/Info';
import CheckIcon from '@mui/icons-material/Check';
import { v4 as uuidv4 } from "uuid";
import { createOpenstackCredsJson } from "src/api/openstack-creds/helpers";
import { postOpenstackCredentials, getOpenstackCredentials, deleteOpenstackCredentials, generateOpenstackToken } from "src/api/openstack-creds/openstackCreds";
import { debounce } from "src/utils";
import { OpenstackCreds } from "src/api/openstack-creds/model";

// Mock data - replace with actual data from API
const FLAVORS = [
    { id: 'standard-2vcpu-4gb', name: 'Standard 2vCPU 4GB RAM' },
    { id: 'performance-4vcpu-8gb', name: 'Performance 4vCPU 8GB RAM' },
    { id: 'highcpu-8vcpu-16gb', name: 'High CPU 8vCPU 16GB RAM' },
];

interface ScaleUpDrawerProps {
    open: boolean;
    onClose: () => void;
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

export default function ScaleUpDrawer({ open, onClose }: ScaleUpDrawerProps) {
    const [openstackCreds, setOpenstackCreds] = useState<OpenstackCreds | null>(null);
    const [flavor, setFlavor] = useState('');
    const [nodeCount, setNodeCount] = useState(1);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [validatingOpenstackCreds, setValidatingOpenstackCreds] = useState(false);
    const [openstackCredsValidated, setOpenstackCredsValidated] = useState(false);
    const [openstackError, setOpenstackError] = useState<string | null>(null);
    const [openstackCredsId, setOpenstackCredsId] = useState<string>("");

    // Reset state when drawer closes
    const handleClose = () => {
        setOpenstackCreds(null);
        setOpenstackCredsId("");
        setOpenstackCredsValidated(false);
        setOpenstackError(null);
        setValidatingOpenstackCreds(false);
        setFlavor('');
        setNodeCount(1);
        setError(null);

        // Delete credentials if they were created
        if (openstackCredsId) {
            deleteOpenstackCredentials(openstackCredsId).catch(console.error);
        }

        onClose();
    };

    const validateOpenstackCreds = useCallback(async (creds: OpenstackCreds) => {
        try {
            setValidatingOpenstackCreds(true);
            setOpenstackError(null);

            // Show loader while creating credentials
            const credsId = uuidv4();
            const credsJson = createOpenstackCredsJson({
                name: credsId,
                ...creds
            });

            try {
                await postOpenstackCredentials(credsJson);
                setOpenstackCredsId(credsId);

                // Generate OpenStack token after credentials are validated
                const token = await generateOpenstackToken(creds);
                console.log('OpenStack token generated:', token);

            } catch (error) {
                console.error("Error creating OpenStack credentials:", error);
                setOpenstackError("Failed to create credentials");
                setValidatingOpenstackCreds(false);
                return;
            }

            // Poll for validation status
            const pollValidation = async () => {
                try {
                    const response = await getOpenstackCredentials(credsId);
                    const status = response?.status?.openstackValidationStatus;
                    const message = response?.status?.openstackValidationMessage;

                    if (status === "Succeeded") {
                        setOpenstackCredsValidated(true);
                        setValidatingOpenstackCreds(false);
                    } else if (status === "Failed") {
                        setOpenstackError(message || "Validation failed");
                        setValidatingOpenstackCreds(false);
                        // Clean up failed credentials
                        deleteOpenstackCredentials(credsId).catch(console.error);
                    } else {
                        setTimeout(pollValidation, 3000);
                    }
                } catch (error) {
                    console.error("Error polling validation status:", error);
                    setOpenstackError("Failed to validate credentials");
                    setValidatingOpenstackCreds(false);
                    // Clean up on error
                    deleteOpenstackCredentials(credsId).catch(console.error);
                }
            };

            pollValidation();
        } catch (error) {
            console.error("Error in validation process:", error);
            setOpenstackError("Failed to validate credentials");
            setValidatingOpenstackCreds(false);
        }
    }, []);

    const debouncedValidation = useCallback(
        debounce((creds) => validateOpenstackCreds(creds), 3000),
        [validateOpenstackCreds]
    );

    const handleOpenstackCredsChange = (values: any) => {
        setOpenstackCreds(values);
        setOpenstackCredsValidated(false);
        debouncedValidation(values);
    };

    const handleSubmit = async () => {
        if (!openstackCreds || !flavor || !nodeCount || !openstackCredsValidated) {
            setError('Please fill in all required fields and ensure credentials are validated');
            return;
        }

        try {
            setLoading(true);
            // TODO: API call to scale up nodes using openstackCredsId
            console.log('Scaling up nodes with:', {
                openstackCredsId,
                flavor,
                nodeCount,
                role: 'worker'
            });

            await new Promise(resolve => setTimeout(resolve, 1000));
            onClose();
        } catch (error) {
            console.error('Error scaling up nodes:', error);
            setError('Failed to scale up nodes. Please try again.');
        } finally {
            setLoading(false);
        }
    };

    // Cleanup debounce on unmount
    useEffect(() => {
        return () => {
            debouncedValidation.cancel();
        };
    }, [debouncedValidation]);

    return (
        <StyledDrawer
            anchor="right"
            open={open}
            onClose={handleClose}
        >
            <Header title="Scale Up Nodes" />
            <DrawerContent>
                <Box sx={{ display: "grid", gap: 4 }}>
                    {/* Step 1: OpenStack Credentials */}
                    <div>
                        <StepHeader
                            number="1"
                            label="OpenStack Credentials"
                            tooltip="Upload your OpenStack RC file to authenticate with the OpenStack platform where new nodes will be created."
                        />
                        <Box sx={{ ml: 6, mt: 2 }}>
                            <FormControl fullWidth error={!!openstackError} required>
                                <OpenstackRCFileUpload
                                    onChange={handleOpenstackCredsChange}
                                />
                                <Box sx={{ display: "flex", gap: 2, mt: 1 }}>
                                    {validatingOpenstackCreds && (
                                        <>
                                            <CircularProgress size={24} />
                                            <FormLabel sx={{ mb: 1 }}>
                                                Validating OpenStack Credentials...
                                            </FormLabel>
                                        </>
                                    )}
                                    {openstackCredsValidated && (
                                        <>
                                            <CheckIcon color="success" fontSize="small" />
                                            <FormLabel sx={{ mb: 1 }}>
                                                OpenStack Credentials Validated
                                            </FormLabel>
                                        </>
                                    )}
                                    {openstackError && (
                                        <FormLabel error sx={{ mb: 1 }}>
                                            {openstackError}
                                        </FormLabel>
                                    )}
                                </Box>
                            </FormControl>
                        </Box>
                    </div>

                    {/* Step 2: Node Template */}
                    <div>
                        <StepHeader
                            number="2"
                            label="Node Template"
                            tooltip="Configure the specification for the new nodes."
                        />
                        <Box sx={{ ml: 6, mt: 2, display: 'grid', gap: 3 }}>
                            <FormControl fullWidth>
                                <InputLabel>Instance Type</InputLabel>
                                <Select
                                    value={flavor}
                                    label="Instance Type"
                                    onChange={(e) => setFlavor(e.target.value)}
                                    required
                                >
                                    {FLAVORS.map((flavor) => (
                                        <MenuItem key={flavor.id} value={flavor.id}>
                                            {flavor.name}
                                        </MenuItem>
                                    ))}
                                </Select>
                            </FormControl>
                            {/* 
                            <FormControl fullWidth>
                                <InputLabel>Image</InputLabel>
                                <Select
                                    value={image}
                                    label="Image"
                                    onChange={(e) => setImage(e.target.value)}
                                    required
                                >
                                    {IMAGES.map((image) => (
                                        <MenuItem key={image.id} value={image.id}>
                                            {image.name}
                                        </MenuItem>
                                    ))}
                                </Select>
                            </FormControl>

                            <TextField
                                label="Role"
                                value="worker"
                                disabled
                                fullWidth
                            /> */}
                        </Box>
                    </div>

                    {/* Step 3: Node Count */}
                    <div>
                        <StepHeader
                            number="3"
                            label="Node Count"
                            tooltip="Specify how many new nodes to create based on the above node template."
                        />
                        <Box sx={{ ml: 6, mt: 2 }}>
                            <TextField
                                type="number"
                                label="Number of Nodes"
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
                disableSubmit={!openstackCreds || !flavor || loading || !openstackCredsValidated}
                submitting={loading}
            />
        </StyledDrawer>
    );
} 