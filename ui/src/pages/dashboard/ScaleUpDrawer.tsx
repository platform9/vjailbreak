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
import { postOpenstackCredentials, deleteOpenstackCredentials } from "src/api/openstack-creds/openstackCreds";
import { debounce } from "src/utils";
import { OpenstackCreds } from "src/api/openstack-creds/model";
import { createNodes, getMasterNode } from "src/api/nodes/nodeMappings";
import { ArrowDropDownIcon } from "@mui/x-date-pickers/icons";
import { OpenstackFlavor } from "src/api/nodes/model";
import { NodeItem } from "src/api/nodes/model";

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
    const [openstackCreds, setOpenstackCreds] = useState<OpenstackCreds | null>(null);
    const [nodeCount, setNodeCount] = useState(1);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [validatingOpenstackCreds, setValidatingOpenstackCreds] = useState(false);
    const [openstackCredsValidated, setOpenstackCredsValidated] = useState(false);
    const [openstackError, setOpenstackError] = useState<string | null>(null);
    const [openstackCredsId, setOpenstackCredsId] = useState<string>("");
//    const [masterNodeImage, setMasterNodeImage] = useState<OpenstackImage | null>(null);
    // const [loadingImages, setLoadingImages] = useState(false);
    // const [imagesError, setImagesError] = useState<string | null>(null);
    const [flavors, setFlavors] = useState<Array<OpenstackFlavor>>([]);
    const [selectedFlavor, setSelectedFlavor] = useState('');
    const [loadingFlavors, setLoadingFlavors] = useState(false);
    const [flavorsError, setFlavorsError] = useState<string | null>(null);

    // Reset state when drawer closes
    const handleClose = () => {
        clearStates();
        onClose();
    };

    const clearStates = () => {
        setOpenstackCreds(null);
        setOpenstackCredsId("");
        setOpenstackCredsValidated(false);
        setOpenstackError(null);
        setValidatingOpenstackCreds(false);
        setNodeCount(1);
        setError(null);
    //    setMasterNodeImage(null);
        setSelectedFlavor('');
        setFlavors([]);
        // setLoadingImages(false);
        setLoadingFlavors(false);
        // setImagesError(null);
        setFlavorsError(null);
    }

    const validateOpenstackCreds = useCallback(async (creds: OpenstackCreds) => {
        setValidatingOpenstackCreds(true);
        setOpenstackError(null);

        const credsId = uuidv4();

        const handleError = (error: Error, message: string) => {
            console.error(message, error);
            setOpenstackError(message);
            setValidatingOpenstackCreds(false);
            if (credsId) {
                deleteOpenstackCredentials(credsId).catch(console.error);
            }
        };

        try {
            const credsJson = createOpenstackCredsJson({ name: credsId, ...creds });
            await postOpenstackCredentials(credsJson);
            setOpenstackCredsId(credsId);


            setOpenstackCredsValidated(true);
            setValidatingOpenstackCreds(false);
        } catch (error) {
            handleError(error as Error, "Failed to validate credentials");
            return;
        }
    }, []);

 /*   useEffect(() => {
        const fetchImages = async () => {
            if (openstackCredsId && openstackCreds) {
                setLoadingImages(true);
                try {
                    const imagesResponse = await getOpenstackImages(openstackCreds);

                    // Check if master node id exists
                    if (!masterNode?.spec.imageid) {
                        setImagesError('Master Agent id is missing');
                        return;
                    }

                    // Check if master node image exists in OpenStack images
                    const foundImage = imagesResponse.images.find(img => img.id === masterNode.spec.imageid);
                    if (!foundImage) {
                        setImagesError('Master Agent image is not matching with the PCD images, please re-upload the image in PCD.');
                        return;
                    }

                    setMasterNodeImage(foundImage);
                } catch (error) {
                    console.error('Failed to fetch images:', error);
                    setImagesError('Failed to fetch OpenStack images. Please try again.');
                } finally {
                    setLoadingImages(false);
                }
            }
        };
        fetchImages();
    }, [openstackCredsId, openstackCreds]); */


   /* useEffect(() => {
        const fetchFlavors = async () => {
            if (openstackCreds && masterNodeImage) {
                setLoadingFlavors(true);
                try {
                    const response = await getOpenstackFlavors(openstackCreds);
                    const requiredDiskGiB = Math.ceil(masterNodeImage.virtual_size / (1024 * 1024 * 1024));
                    const filteredFlavors = response.flavors.filter(flavor => flavor.disk >= requiredDiskGiB);

                    if (filteredFlavors.length === 0) {
                        setFlavorsError(`No flavors available with disk size >= ${requiredDiskGiB}GiB`);
                    }

                    setFlavors(filteredFlavors);
                } catch (error) {
                    console.error('Failed to fetch flavors:', error);
                    setFlavorsError('Failed to fetch OpenStack flavors');
                } finally {
                    setLoadingFlavors(false);
                }
            }
        };
        fetchFlavors();
    }, [masterNodeImage]);  */

    useEffect(() => {
        const fetchFlavours = async () => {
            if (openstackCredsId && openstackCreds) {
                setLoadingFlavors(true);
                try {
                    const response  = await getMasterNode();
                    const flavours = response?.spec.availableflavours;

                    if (!flavours) {
                        setFlavorsError('No flavors available');
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
    }, [openstackCredsId, openstackCreds]);

    const debouncedValidation = useCallback(
        debounce((creds) => validateOpenstackCreds(creds), 3000),
        [validateOpenstackCreds]
    );

    const handleOpenstackCredsChange = (values: unknown) => {
        clearStates();
        setOpenstackCreds(values as OpenstackCreds);
        debouncedValidation.cancel();

        debouncedValidation(values);
    };


    const handleSubmit = async () => {
        if (!masterNode?.spec.imageid || !selectedFlavor || !nodeCount || !openstackCredsId) {
            setError('Please fill in all required fields');
            return;
        }

        try {
            setLoading(true);
            await createNodes({
                imageId: masterNode.spec.imageid,
                openstackCreds: {
                    kind: "openstackcreds" as const,
                    name: openstackCredsId,
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
            <Header title="Scale Up Agents" />
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

                    {/* Step 2: Agent Template */}
                    <div>
                        <StepHeader
                            number="2"
                            label="Agent Template"
                            tooltip="Configure the specification for the new nodes."
                        />
                        <Box sx={{ ml: 6, mt: 2, display: 'grid', gap: 3 }}>
                            // Add a TextField saying Picking the image based on the first vjailbreak node

                            <FormControl fullWidth>
                                <TextField
                                    label="Master Agent Image"
                                    value={'Image selected from the first vjailbreak node'}
                                    disabled
                                    fullWidth
                                />
                            </FormControl>

                            <FormControl error={!!flavorsError} fullWidth>
                                <InputLabel>{loadingFlavors ? "Loading Flavors..." : "Flavor"}</InputLabel>
                                <Select
                                    value={selectedFlavor}
                                    label="Flavor"
                                    onChange={(e) => setSelectedFlavor(e.target.value)}
                                    required
                                    disabled={loadingFlavors}
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