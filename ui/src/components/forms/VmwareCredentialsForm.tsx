import {
    Box,
    FormControl,
    FormLabel,
    TextField,
    CircularProgress,
    Collapse,
    InputAdornment,
    IconButton,
} from "@mui/material"
import { Visibility, VisibilityOff } from "@mui/icons-material"
import CheckIcon from "@mui/icons-material/Check"
import { useState, useCallback, useEffect } from "react"
import CredentialSelector from "./CredentialSelector"
import { debounce } from "src/utils"
import { RefetchOptions } from "@tanstack/react-query"
import { QueryObserverResult } from "@tanstack/react-query"

export interface VmwareCredential {
    metadata: {
        name: string
        namespace?: string
    }
    spec: {
        // Support for both direct credentials and secretRef
        VCENTER_HOST?: string
        VCENTER_USERNAME?: string
        VCENTER_PASSWORD?: string
        secretRef?: {
            name: string
        }
    }
    status?: {
        vmwareValidationStatus?: string
        vmwareValidationMessage?: string
    }
}

export interface VmwareCredentialsFormProps {
    credentialsList?: VmwareCredential[]
    loadingCredentials?: boolean
    refetchCredentials?: (options?: RefetchOptions) => Promise<QueryObserverResult<VmwareCredential[], Error>>
    validatingCredentials?: boolean
    credentialsValidated?: boolean | null
    error?: string
    onChange: (values: Record<string, string | number | boolean>) => void
    onCredentialSelect?: (credId: string | null) => void
    selectedCredential?: string | null
}

export default function VmwareCredentialsForm({
    credentialsList = [],
    loadingCredentials = false,
    refetchCredentials,
    validatingCredentials = false,
    credentialsValidated = null,
    error,
    onChange,
    onCredentialSelect,
    selectedCredential = null,
}: VmwareCredentialsFormProps) {
    const [showPassword, setShowPassword] = useState(false)
    const [showForm, setShowForm] = useState(false)
    const [vmwareCreds, setVmwareCreds] = useState({
        vcenterHost: "",
        datacenter: "",
        username: "",
        password: "",
        credentialName: "",
    })
    const [credNameError, setCredNameError] = useState<string | null>(null)

    const credentialOptions = credentialsList.map(cred => ({
        label: cred.metadata.name,
        value: cred.metadata.name,
        metadata: cred.metadata,
        status: {
            validationStatus: cred.status?.vmwareValidationStatus,
            validationMessage: cred.status?.vmwareValidationMessage
        }
    }))

    const handleClickShowPassword = () => setShowPassword((show) => !show)

    const handleMouseDownPassword = (event: React.MouseEvent<HTMLButtonElement>) => {
        event.preventDefault()
    }

    const handleVmwareCredsChange = (value) => {
        setVmwareCreds({ ...vmwareCreds, ...value })

        if (value.credentialName) {
            setCredNameError(null)
        }
    }

    const debouncedOnChange = useCallback(
        debounce((creds) => {
            const mappedCreds = {
                ...creds,
                name: creds.credentialName,
            }
            onChange(mappedCreds)
        }, 1000 * 3),
        [onChange]
    )

    useEffect(() => {
        if (
            vmwareCreds.vcenterHost &&
            vmwareCreds.datacenter &&
            vmwareCreds.username &&
            vmwareCreds.password
        ) {
            if (!vmwareCreds.credentialName) {
                setCredNameError("Please provide a credential name")
                return
            }

            debouncedOnChange(vmwareCreds)
        }
        return () => {
            debouncedOnChange.cancel()
        }
    }, [vmwareCreds])

    useEffect(() => {
        if (credentialsValidated === true && showForm && refetchCredentials) {
            refetchCredentials().then(() => {
                setTimeout(() => {
                    const createdCredName = vmwareCreds.credentialName
                    const matchingCred = credentialsList.find(cred => cred.metadata.name === createdCredName)

                    if (matchingCred && onCredentialSelect) {
                        onCredentialSelect(matchingCred.metadata.name)
                        setShowForm(false)
                    } else if (refetchCredentials) {
                        refetchCredentials()
                    }
                }, 1000)
            })
        }
    }, [credentialsValidated, credentialsList, showForm, vmwareCreds.credentialName, onCredentialSelect, refetchCredentials])

    const toggleForm = () => {
        if (showForm) {
            setVmwareCreds({
                vcenterHost: "",
                datacenter: "",
                username: "",
                password: "",
                credentialName: "",
            })
            setCredNameError(null)
        }
        setShowForm(!showForm)
    }

    return (
        <div>
            <CredentialSelector
                placeholder="Select VMware credentials"
                options={credentialOptions}
                value={selectedCredential}
                onChange={onCredentialSelect || (() => { })}
                onAddNew={toggleForm}
                loading={loadingCredentials}
                emptyMessage="No VMware credentials found. Please create new ones."
            />

            <Collapse in={showForm}>
                <FormControl fullWidth error={!!error} required>
                    <Box sx={{ mt: 2 }}>
                        <TextField
                            id="credentialName"
                            label="Enter VMware Credential Name"
                            variant="outlined"
                            value={vmwareCreds.credentialName}
                            onChange={(e) =>
                                handleVmwareCredsChange({ credentialName: e.target.value })
                            }
                            error={!!error || !!credNameError}
                            helperText={credNameError}
                            required
                            size="small"
                            sx={{ mb: 2, width: "440px" }}
                        />
                        <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
                            <TextField
                                id="vcenterHost"
                                label="vCenter Server"
                                variant="outlined"
                                value={vmwareCreds.vcenterHost}
                                onChange={(e) =>
                                    handleVmwareCredsChange({ vcenterHost: e.target.value })
                                }
                                error={!!error}
                                required
                                size="small"
                            />
                            <TextField
                                id="datacenter"
                                label="Datacenter Name"
                                size="small"
                                variant="outlined"
                                value={vmwareCreds.datacenter}
                                onChange={(e) =>
                                    handleVmwareCredsChange({ datacenter: e.target.value })
                                }
                                error={!!error}
                                required
                            />
                        </Box>

                        <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2, mt: 2 }}>
                            <TextField
                                id="username"
                                label="Username"
                                variant="outlined"
                                value={vmwareCreds.username}
                                onChange={(e) =>
                                    handleVmwareCredsChange({ username: e.target.value })
                                }
                                error={!!error}
                                required
                                size="small"
                            />
                            <TextField
                                label="Password"
                                type={showPassword ? "text" : "password"}
                                variant="outlined"
                                size="small"
                                slotProps={{
                                    input: {
                                        endAdornment: (
                                            <InputAdornment position="end">
                                                <IconButton
                                                    onClick={handleClickShowPassword}
                                                    onMouseDown={handleMouseDownPassword}
                                                    edge="end"
                                                >
                                                    {showPassword ? <VisibilityOff /> : <Visibility />}
                                                </IconButton>
                                            </InputAdornment>
                                        ),
                                    },
                                }}
                                onChange={(e) =>
                                    handleVmwareCredsChange({ password: e.target.value })
                                }
                                fullWidth
                                required
                            />
                        </Box>

                        {/* VMware Validation Status */}
                        <Box sx={{ display: "flex", gap: 2, mt: 2, alignItems: "center" }}>
                            {validatingCredentials && (
                                <>
                                    <CircularProgress size={24} />
                                    <FormLabel>Validating & Creating VMware credentials...</FormLabel>
                                </>
                            )}
                            {credentialsValidated === true && (
                                <>
                                    <CheckIcon color="success" fontSize="small" />
                                    <FormLabel>VMware credentials created</FormLabel>
                                </>
                            )}
                            {error && (
                                <FormLabel sx={{ fontSize: "12px" }} color="error">{error}</FormLabel>
                            )}
                        </Box>
                    </Box>
                </FormControl>
            </Collapse>
        </div>
    )
} 