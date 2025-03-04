import {
    Box,
    FormControl,
    FormLabel,
    TextField,
    CircularProgress,
    Collapse,
} from "@mui/material"
import CheckIcon from "@mui/icons-material/Check"
import { useState, useCallback, useEffect, useRef } from "react"
import OpenstackRCFileUpload, { OpenstackRCFileUploaderRef } from "./OpenstackRCFileUpload"
import CredentialSelector from "./CredentialSelector"
import { debounce } from "src/utils"
import { QueryObserverResult } from "@tanstack/react-query"
import { RefetchOptions } from "@tanstack/react-query"

export interface OpenstackCredential {
    metadata: {
        name: string
        namespace?: string
    }
    spec: {
        // Support for both direct credentials and secretRef
        OS_AUTH_URL?: string
        OS_DOMAIN_NAME?: string
        OS_USERNAME?: string
        OS_PASSWORD?: string
        OS_REGION_NAME?: string
        OS_TENANT_NAME?: string
        secretRef?: {
            name: string
        }
    }
    status?: {
        openstackValidationStatus?: string
        openstackValidationMessage?: string
    }
}

export interface OpenstackCredentialsFormProps {
    credentialsList?: OpenstackCredential[]
    loadingCredentials?: boolean
    refetchCredentials?: (options?: RefetchOptions) => Promise<QueryObserverResult<OpenstackCredential[], Error>>
    validatingCredentials?: boolean
    credentialsValidated?: boolean | null
    error?: string
    onChange: (values: Record<string, string | number | boolean>) => void
    onCredentialSelect?: (credId: string | null) => void
    selectedCredential?: string | null
    showCredentialNameField?: boolean
    showCredentialSelector?: boolean
    fullWidth?: boolean
    size?: "small" | "medium"
}

export default function OpenstackCredentialsForm({
    credentialsList = [],
    loadingCredentials = false,
    refetchCredentials,
    validatingCredentials = false,
    credentialsValidated = null,
    error,
    size = "small",
    fullWidth = false,
    onChange,
    onCredentialSelect,
    selectedCredential = null,
    showCredentialNameField = true,
    showCredentialSelector = true,
}: OpenstackCredentialsFormProps) {
    const [showForm, setShowForm] = useState(!showCredentialSelector)
    const [openstackCreds, setOpenstackCreds] = useState({
        OS_AUTH_URL: "",
        OS_DOMAIN_NAME: "",
        OS_USERNAME: "",
        OS_PASSWORD: "",
        OS_REGION_NAME: "",
        OS_TENANT_NAME: "",
        credentialName: "",
    })

    // Add ref for the OpenStack RC file uploader
    const openstackRCFileUploaderRef = useRef<OpenstackRCFileUploaderRef>(null)

    // Format credentials for the selector
    const credentialOptions = credentialsList.map(cred => ({
        label: cred.metadata.name,
        value: cred.metadata.name,
        metadata: cred.metadata,
        status: {
            validationStatus: cred.status?.openstackValidationStatus,
            validationMessage: cred.status?.openstackValidationMessage
        }
    }))

    const handleOpenstackCredsChange = (values) => {
        const updatedCreds = { ...openstackCreds, ...values }
        setOpenstackCreds(updatedCreds)
    }

    const debouncedOnChange = useCallback(
        debounce((creds) => {
            onChange({
                ...creds,
                name: creds.credentialName,
            })
        }, 1000 * 3),
        [onChange]
    )

    useEffect(() => {
        if (
            openstackCreds.OS_AUTH_URL &&
            openstackCreds.OS_DOMAIN_NAME &&
            openstackCreds.OS_USERNAME &&
            openstackCreds.OS_PASSWORD &&
            openstackCreds.credentialName
        ) {
            debouncedOnChange(openstackCreds)
        }

        return () => {
            debouncedOnChange.cancel()
        }
    }, [openstackCreds])

    useEffect(() => {
        if (credentialsValidated === true && showForm && refetchCredentials) {
            refetchCredentials().then(() => {
                setTimeout(() => {
                    const createdCredName = openstackCreds.credentialName
                    const matchingCred = credentialsList.find(cred => cred.metadata.name === createdCredName)

                    if (matchingCred && onCredentialSelect) {
                        onCredentialSelect(matchingCred.metadata.name)
                        setShowForm(false)

                        // Reset the file uploader when credentials are validated
                        if (openstackRCFileUploaderRef.current) {
                            openstackRCFileUploaderRef.current.reset()
                        }
                    } else if (refetchCredentials) {
                        refetchCredentials()
                    }
                }, 1000)
            })
        }
    }, [credentialsValidated, credentialsList, showForm, openstackCreds.credentialName, onCredentialSelect, refetchCredentials])

    // Handle form visibility toggle
    const toggleForm = () => {
        if (showForm) {
            setOpenstackCreds({
                OS_AUTH_URL: "",
                OS_DOMAIN_NAME: "",
                OS_USERNAME: "",
                OS_PASSWORD: "",
                OS_REGION_NAME: "",
                OS_TENANT_NAME: "",
                credentialName: "",
            })

            // Reset the file uploader when hiding the form
            if (openstackRCFileUploaderRef.current) {
                openstackRCFileUploaderRef.current.reset()
            }
        }
        setShowForm(!showForm)
    }

    return (
        <div>
            {showCredentialSelector && (
                <CredentialSelector
                    placeholder="Select OpenStack credentials"
                    options={credentialOptions}
                    value={selectedCredential}
                    onChange={onCredentialSelect || (() => { })}
                    onAddNew={toggleForm}
                    size={size}
                    loading={loadingCredentials}
                    emptyMessage="No OpenStack credentials found. Please create new ones."
                />
            )}

            <Collapse in={showForm}>
                <FormControl fullWidth error={!!error} required>
                    {showCredentialNameField && (
                        <TextField
                            id="openstackCredentialName"
                            label="Enter OpenStack Credential Name"
                            variant="outlined"
                            size={size}
                            value={openstackCreds.credentialName}
                            onChange={(e) => {
                                handleOpenstackCredsChange({
                                    credentialName: e.target.value
                                })
                            }}
                            error={!!error}
                            required
                            sx={{ mt: 2, width: fullWidth ? "100%" : "440px" }}
                        />
                    )}

                    <OpenstackRCFileUpload
                        size={size}
                        ref={openstackRCFileUploaderRef}
                        openstackCredsError={error}
                        onChange={(values) => {
                            handleOpenstackCredsChange({
                                ...values as Record<string, string>,
                                credentialName: openstackCreds.credentialName // Preserve the credential name
                            })
                        }}
                    />

                    {/* OpenStack Validation Status */}
                    <Box sx={{ display: "flex", gap: 2, mt: 2, alignItems: "center" }}>
                        {validatingCredentials && (
                            <>
                                <CircularProgress size={24} />
                                <FormLabel>Validating & Creating OpenStack credentials...</FormLabel>
                            </>
                        )}
                        {credentialsValidated === true && (
                            <>
                                <CheckIcon color="success" fontSize="small" />
                                <FormLabel>OpenStack credentials created</FormLabel>
                            </>
                        )}
                        {error && (
                            <FormLabel sx={{ fontSize: "12px" }} color="error">{error}</FormLabel>
                        )}
                    </Box>
                </FormControl>
            </Collapse>
        </div>
    )
} 