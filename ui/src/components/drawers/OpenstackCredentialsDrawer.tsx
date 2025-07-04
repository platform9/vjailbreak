import { Box } from "@mui/material";
import { useState, useRef, useCallback } from "react";
import { StyledDrawer, DrawerContent } from "src/components/forms/StyledDrawer";
import Header from "src/components/forms/Header";
import Footer from "src/components/forms/Footer";
import { createOpenstackCredsWithSecretFlow, deleteOpenStackCredsWithSecretFlow } from "src/api/helpers";
import { useOpenstackCredentialsQuery } from "src/hooks/api/useOpenstackCredentialsQuery";
import { THREE_SECONDS } from "src/constants";
import { useInterval } from "src/hooks/useInterval";
import { getOpenstackCredentials } from "src/api/openstack-creds/openstackCreds";
import { TextField, FormControl, FormLabel, CircularProgress, Switch, FormControlLabel } from "@mui/material";
import { isValidName } from "src/utils";
import CheckIcon from "@mui/icons-material/Check";
import OpenstackRCFileUploader, { OpenstackRCFileUploaderRef } from "src/components/forms/OpenstackRCFileUpload";
import { useKeyboardSubmit } from "src/hooks/ui/useKeyboardSubmit";

interface OpenstackCredentialsDrawerProps {
    open: boolean;
    onClose: () => void;
}

export default function OpenstackCredentialsDrawer({
    open,
    onClose,
}: OpenstackCredentialsDrawerProps) {
    const [validatingOpenstackCreds, setValidatingOpenstackCreds] = useState(false);
    const [openstackCredsValidated, setOpenstackCredsValidated] = useState<boolean | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [credNameError, setCredNameError] = useState<string | null>(null);
    const [submitting, setSubmitting] = useState(false);
    const [createdCredentialName, setCreatedCredentialName] = useState<string | null>(null);
    const [isPcd, setIsPcd] = useState(false);
    const rcFileUploaderRef = useRef<OpenstackRCFileUploaderRef>(null);

    // Fetch credentials list for the form
    const { refetch: refetchOpenstackCreds } = useOpenstackCredentialsQuery();

    // Reset state and clean up when the drawer is closed
    const closeDrawer = useCallback(() => {
        if (createdCredentialName) {
            console.log(`Cleaning up OpenStack credential on drawer close: ${createdCredentialName}`);
            try {
                deleteOpenStackCredsWithSecretFlow(createdCredentialName)
                    .then(() => console.log(`Cancelled credential ${createdCredentialName} deleted successfully`))
                    .catch(err => console.error(`Error deleting cancelled credential: ${createdCredentialName}`, err));
            } catch (err) {
                console.error(`Error initiating deletion of cancelled credential: ${createdCredentialName}`, err);
            }
        }

        // Reset state
        setCredentialName("");
        setRcFileValues(null);
        setCreatedCredentialName(null);
        setValidatingOpenstackCreds(false);
        setOpenstackCredsValidated(null);
        setError(null);
        setCredNameError(null);
        setSubmitting(false);
        setIsPcd(false);

        onClose();
    }, [createdCredentialName, onClose]);

    // Track form values
    const [credentialName, setCredentialName] = useState("");
    const [rcFileValues, setRcFileValues] = useState<Record<string, string> | null>(null);

    const isValidCredentialName = isValidName(credentialName);

    const handleCredentialNameChange = (event: React.ChangeEvent<HTMLInputElement>) => {
        setCredentialName(event.target.value);

        // Reset validation state when form values change
        setOpenstackCredsValidated(null);
        setError(null);

        // Validate credential name
        if (!isValidName(event.target.value)) {
            setCredNameError("Credential name must start with a letter or number, followed by letters, numbers or hyphens, with a maximum length of 253 characters");
        } else {
            setCredNameError(null);
        }
    };

    const handleRCFileChange = (values: unknown) => {
        setRcFileValues(values as Record<string, string>);
        setOpenstackCredsValidated(null);
        setError(null);
    };

    const handleSubmit = useCallback(async () => {
        if (!credentialName || !rcFileValues) {
            setError("Please provide a credential name and upload an RC file");
            return;
        }

        if (!isValidCredentialName) {
            setError("Please provide a valid credential name");
            return;
        }

        setSubmitting(true);
        setValidatingOpenstackCreds(true);

        try {
            // Use the new helper function that encapsulates the entire flow
            const response = await createOpenstackCredsWithSecretFlow(
                credentialName,
                {
                    OS_AUTH_URL: rcFileValues.OS_AUTH_URL,
                    OS_DOMAIN_NAME: rcFileValues.OS_DOMAIN_NAME,
                    OS_USERNAME: rcFileValues.OS_USERNAME,
                    OS_PASSWORD: rcFileValues.OS_PASSWORD,
                    OS_REGION_NAME: rcFileValues.OS_REGION_NAME,
                    OS_TENANT_NAME: rcFileValues.OS_TENANT_NAME,
                    OS_INSECURE: rcFileValues.OS_INSECURE?.toLowerCase() === "true"
                },
                isPcd
            );

            setCreatedCredentialName(response.metadata.name);

        } catch (error: unknown) {
            console.error("Error creating OpenStack credentials:", error);
            setOpenstackCredsValidated(false);
            setValidatingOpenstackCreds(false);

            // Handle different error types
            let errorMessage = "An unknown error occurred";
            
            if (error instanceof Error) {
                errorMessage = error.message;
            } else if (typeof error === 'object' && error !== null) {
                // Handle API error responses
                const apiError = error as any;
                if (apiError.response?.data?.message) {
                    errorMessage = apiError.response.data.message;
                } else if (apiError.message) {
                    errorMessage = apiError.message;
                }
            }

            // Clean up the error message for better UX
            errorMessage = errorMessage
                .replace(/^Error: /, '') // Remove leading 'Error: '
                .replace(/\n.*$/, '')     // Remove any newlines and anything after
                .trim();

            // Map common error patterns to user-friendly messages
            if (errorMessage.includes("credentials are valid but for a different OpenStack environment")) {
                errorMessage = "These credentials are valid but for a different OpenStack environment. Please use credentials for this environment.";
            } else if (errorMessage.includes("already exists")) {
                errorMessage = "A credential with this name already exists. Please use a different name.";
            } else if (errorMessage.includes("connection refused")) {
                errorMessage = "Could not connect to the OpenStack API. Please check the auth URL and ensure it's accessible.";
            } else if (errorMessage.includes("authentication failed")) {
                errorMessage = "Authentication failed. Please check your username and password.";
            } else if (!errorMessage.endsWith('.')) {
                // Ensure the error message ends with a period
                errorMessage += '.';
            }

            setError(errorMessage);
            setSubmitting(false);
        }
    }, [credentialName, rcFileValues, isValidCredentialName, submitting, isPcd]);

    // Use the custom hook for keyboard events
    useKeyboardSubmit({
        open,
        isSubmitDisabled: submitting || validatingOpenstackCreds || !isValidCredentialName || !rcFileValues,
        onSubmit: handleSubmit,
        onClose: closeDrawer
    });

    const handleValidationStatus = (status: string, message?: string) => {
        if (status === "Succeeded") {
            setOpenstackCredsValidated(true);
            setValidatingOpenstackCreds(false);
            // Close the drawer after a short delay to show success state
            setTimeout(() => {
                refetchOpenstackCreds();
                onClose();
            }, 1500);
        } else if (status === "Failed") {
            setOpenstackCredsValidated(false);
            setValidatingOpenstackCreds(false);
            
            // Format the error message to be more user-friendly
            let errorMessage = "Validation failed";
            if (message) {
                // Remove any technical details or stack traces
                errorMessage = message.split('\n')[0];
                
                // Map specific error messages to more user-friendly ones
                if (errorMessage.includes("credentials are valid but for a different OpenStack environment")) {
                    errorMessage = "These credentials are valid but for a different OpenStack environment. Please use credentials for this environment.";
                } else if (errorMessage.includes("authentication failed")) {
                    errorMessage = "Authentication failed. Please check your username and password.";
                } else if (errorMessage.includes("connection refused")) {
                    errorMessage = "Could not connect to the OpenStack API. Please check the auth URL and ensure it's accessible.";
                } else if (errorMessage.includes("domain not found")) {
                    errorMessage = "The specified domain was not found. Please check the domain name.";
                } else if (errorMessage.includes("project not found")) {
                    errorMessage = "The specified project/tenant was not found. Please check the project/tenant name.";
                }
            }
            
            setError(errorMessage);

            // Try to delete the failed credential to clean up
            if (createdCredentialName) {
                try {
                    deleteOpenStackCredsWithSecretFlow(createdCredentialName)
                        .then(() => console.log(`Failed credential ${createdCredentialName} deleted`))
                        .catch((deleteErr) => console.error(`Error deleting failed credential: ${createdCredentialName}`, deleteErr));
                } catch (deleteErr) {
                    console.error(`Error deleting failed credential: ${createdCredentialName}`, deleteErr);
                }
            }
        }
        setSubmitting(false);
    };

    // Define a clear polling condition similar to MigrationForm
    const shouldPollOpenstackCreds =
        !!createdCredentialName &&
        validatingOpenstackCreds;

    // Poll for credential status if we're waiting for validation
    useInterval(
        async () => {
            try {
                const response = await getOpenstackCredentials(createdCredentialName);
                if (response?.status?.openstackValidationStatus) {
                    handleValidationStatus(
                        response.status.openstackValidationStatus,
                        response.status.openstackValidationMessage
                    );
                }
            } catch (err) {
                console.error("Error validating OpenStack credentials", err);
                setError("Error validating OpenStack credentials");
                setValidatingOpenstackCreds(false);
                setSubmitting(false);
            }
        },
        THREE_SECONDS,
        shouldPollOpenstackCreds
    );

    return (
        <StyledDrawer
            anchor="right"
            open={open}
            onClose={closeDrawer}
        >
            <Header title="Add OpenStack Credentials" />
            <DrawerContent>
                <Box sx={{ display: "grid", gap: 3 }}>
                    <FormControl fullWidth error={!!error || !!credNameError} required>
                        <TextField
                            id="credentialName"
                            label="Enter OpenStack Credential Name"
                            variant="outlined"
                            value={credentialName}
                            onChange={handleCredentialNameChange}
                            error={!!error || !!credNameError}
                            helperText={
                                credNameError || (error && !credNameError ? error : "")
                            }
                            required
                            fullWidth
                            size="small"
                            sx={{ mb: 2 }}
                        />

                        <OpenstackRCFileUploader
                            ref={rcFileUploaderRef}
                            onChange={handleRCFileChange}
                            openstackCredsError={error || ""}
                            size="small"
                        />

                        <FormControlLabel
                            control={
                                <Switch
                                    checked={isPcd}
                                    onChange={(e) => setIsPcd(e.target.checked)}
                                    color="primary"
                                />
                            }
                            label="Is PCD credential"
                            sx={{ mt: 2 }}
                        />

                        {/* OpenStack Validation Status */}
                        <Box sx={{ display: "flex", gap: 2, mt: 2, alignItems: "center" }}>
                            {validatingOpenstackCreds && (
                                <>
                                    <CircularProgress size={24} />
                                    <FormLabel>
                                        Validating OpenStack credentials...
                                    </FormLabel>
                                </>
                            )}
                            {openstackCredsValidated === true && credentialName && (
                                <>
                                    <CheckIcon color="success" fontSize="small" />
                                    <FormLabel>OpenStack credentials created</FormLabel>
                                </>
                            )}
                            {error && !credNameError && (
                                <FormLabel sx={{ fontSize: "12px" }} color="error">
                                    {error}
                                </FormLabel>
                            )}
                        </Box>
                    </FormControl>
                </Box>
            </DrawerContent>
            <Footer
                submitButtonLabel={validatingOpenstackCreds ? "Validating..." : "Create Credentials"}
                onClose={closeDrawer}
                onSubmit={handleSubmit}
                disableSubmit={submitting || validatingOpenstackCreds || !isValidCredentialName || !rcFileValues}
                submitting={submitting}
            />
        </StyledDrawer>
    );
} 