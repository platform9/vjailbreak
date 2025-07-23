import React, { useState } from "react"
import InputMask from "react-input-mask"
import {
    Box,
    Drawer,
    Typography,
    styled,
    Alert,
    TextField,
    Button,
    CircularProgress,
    IconButton,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    Paper,
} from "@mui/material"
import NetworkCheckIcon from "@mui/icons-material/NetworkCheck"
import CheckCircleIcon from "@mui/icons-material/CheckCircle"
import WarningIcon from "@mui/icons-material/Warning"
import WifiIcon from "@mui/icons-material/Wifi"
import AddIcon from "@mui/icons-material/Add"
import DeleteIcon from "@mui/icons-material/Delete"
import Header from "../forms/Header"

const StyledDrawer = styled(Drawer)(({ theme }) => ({
    "& .MuiDrawer-paper": {
        display: "grid",
        gridTemplateRows: "max-content 1fr max-content",
        width: "800px",
        maxWidth: "90vw",
        zIndex: theme.zIndex.modal,
    },
}))

const DrawerContent = styled("div")(({ theme }) => ({
    overflow: "auto",
    padding: theme.spacing(4, 6, 4, 4),
}))

const ConfigSection = styled(Box)(({ theme }) => ({
    marginBottom: theme.spacing(4),
    padding: theme.spacing(3),
    borderRadius: theme.spacing(1),
    border: `1px solid ${theme.palette.divider}`,
}))

const CustomFooter = styled("div")(({ theme }) => ({
    display: "flex",
    justifyContent: "flex-end",
    alignItems: "center",
    gap: theme.spacing(2),
    padding: theme.spacing(2),
    borderTop: `1px solid ${theme.palette.divider}`,
}))

interface NetworkConfigDrawerProps {
    open: boolean
    onClose: () => void
    onSave: (config: NetworkConfig) => void
}

interface HostEntry {
    id: string
    ip: string
    hostname: string
}

interface NetworkConfig {
    dnsServers?: string[]
    hostEntries: HostEntry[]
}

export default function NetworkConfigDrawer({
    open,
    onClose,
    onSave,
}: NetworkConfigDrawerProps) {
    const [config, setConfig] = useState<NetworkConfig>({
        dnsServers: ["8.8.8.8", "8.8.4.4"],
        hostEntries: [],
    })

    const [newEntry, setNewEntry] = useState({ ip: '', hostname: '' })

    const [saving, setSaving] = useState(false)
    const [testing, setTesting] = useState(false)
    const [testResult, setTestResult] = useState<{
        status: 'success' | 'error' | null
        message: string
    }>({ status: null, message: '' })

    const handleSave = async () => {
        setSaving(true)
        try {
            await new Promise(resolve => setTimeout(resolve, 1000)) // Simulate API call
            onSave(config)
            onClose()
        } catch (error) {
            console.error("Failed to save network configuration:", error)
        } finally {
            setSaving(false)
        }
    }

    const handleTestConnection = async () => {
        setTesting(true)
        setTestResult({ status: null, message: '' })

        try {
            // Simulate network connectivity test
            await new Promise(resolve => setTimeout(resolve, 2000))

            // Simulate test results based on configuration
            const success = Math.random() > 0.3 // 70% success rate for demo

            if (success) {
                setTestResult({
                    status: 'success',
                    message: 'Network connectivity test passed! All endpoints are reachable.'
                })
            } else {
                setTestResult({
                    status: 'error',
                    message: 'Network connectivity test failed. Please check DNS settings and network mappings.'
                })
            }
        } catch {
            setTestResult({
                status: 'error',
                message: 'Failed to test network connectivity. Please try again.'
            })
        } finally {
            setTesting(false)
        }
    }

    const handleClose = () => {
        if (!saving && !testing) {
            onClose()
        }
    }

    return (
        <StyledDrawer anchor="right" open={open} onClose={handleClose}>
            <Header
                title="Network Configuration"
                icon={<NetworkCheckIcon />}
            />

            <DrawerContent>
                <Box sx={{ display: "grid", gap: 4 }}>
                    <Alert severity="info" sx={{ mb: 2 }}>
                        Configure DNS settings and host entries that will be applied during VM migration.
                    </Alert>

                    {/* DNS Configuration */}
                    <ConfigSection>
                        <Typography variant="h6" gutterBottom>
                            DNS Configuration
                        </Typography>

                        <TextField
                            label="DNS Servers (comma-separated)"
                            value={config.dnsServers?.join(", ") || ""}
                            onChange={(e) => setConfig(prev => ({
                                ...prev,
                                dnsServers: e.target.value.split(",").map(s => s.trim()).filter(Boolean)
                            }))}
                            placeholder="8.8.8.8, 8.8.4.4"
                            size="small"
                            fullWidth
                            helperText="Primary and secondary DNS servers for name resolution"
                        />
                    </ConfigSection>

                    {/* Host Entries Configuration */}
                    <ConfigSection>
                        <Typography variant="h6" gutterBottom>
                            Host Entries (/etc/hosts)
                        </Typography>

                        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                            Add custom IP to hostname mappings that will be applied to migrated VMs
                        </Typography>

                        {/* Add New Entry Form */}
                        <Box sx={{ display: "flex", gap: 2, mb: 3, alignItems: "flex-end" }}>
                            <Box sx={{ flex: 1 }}>
                                <Typography variant="body2" sx={{ mb: 0.5, fontSize: '0.75rem', color: 'text.secondary' }}>
                                    IP Address
                                </Typography>
                                <InputMask
                                    mask="999.999.999.999"
                                    value={newEntry.ip}
                                    onChange={(e) => setNewEntry(prev => ({ ...prev, ip: e.target.value }))}
                                    maskChar="_"
                                    alwaysShowMask={false}
                                >
                                    {(inputProps: React.InputHTMLAttributes<HTMLInputElement>) => (
                                        <TextField
                                            value={inputProps.value}
                                            onChange={inputProps.onChange}
                                            onBlur={inputProps.onBlur}
                                            onFocus={inputProps.onFocus}
                                            placeholder="192.168.1.100"
                                            size="small"
                                            fullWidth
                                            variant="outlined"
                                        />
                                    )}
                                </InputMask>
                            </Box>
                            <TextField
                                label="Hostname"
                                value={newEntry.hostname}
                                onChange={(e) => setNewEntry(prev => ({ ...prev, hostname: e.target.value }))}
                                placeholder="myserver.example.com"
                                size="small"
                                sx={{ flex: 1 }}
                            />
                            <Button
                                variant="outlined"
                                startIcon={<AddIcon />}
                                onClick={() => {
                                    if (newEntry.ip && newEntry.hostname) {
                                        setConfig(prev => ({
                                            ...prev,
                                            hostEntries: [...prev.hostEntries, {
                                                id: Date.now().toString(),
                                                ip: newEntry.ip,
                                                hostname: newEntry.hostname
                                            }]
                                        }))
                                        setNewEntry({ ip: '', hostname: '' })
                                    }
                                }}
                                disabled={!newEntry.ip || !newEntry.hostname}
                                size="small"
                            >
                                Add
                            </Button>
                        </Box>

                        {/* Host Entries Table */}
                        {config.hostEntries.length > 0 && (
                            <TableContainer component={Paper} variant="outlined">
                                <Table size="small">
                                    <TableHead>
                                        <TableRow>
                                            <TableCell>IP Address</TableCell>
                                            <TableCell>Hostname</TableCell>
                                            <TableCell width={60}>Actions</TableCell>
                                        </TableRow>
                                    </TableHead>
                                    <TableBody>
                                        {config.hostEntries.map((entry) => (
                                            <TableRow key={entry.id}>
                                                <TableCell>{entry.ip}</TableCell>
                                                <TableCell>{entry.hostname}</TableCell>
                                                <TableCell>
                                                    <IconButton
                                                        size="small"
                                                        onClick={() => {
                                                            setConfig(prev => ({
                                                                ...prev,
                                                                hostEntries: prev.hostEntries.filter(e => e.id !== entry.id)
                                                            }))
                                                        }}
                                                        color="error"
                                                    >
                                                        <DeleteIcon fontSize="small" />
                                                    </IconButton>
                                                </TableCell>
                                            </TableRow>
                                        ))}
                                    </TableBody>
                                </Table>
                            </TableContainer>
                        )}

                        {config.hostEntries.length === 0 && (
                            <Typography variant="body2" color="text.secondary" sx={{ textAlign: "center", py: 2 }}>
                                No host entries configured. Add entries above to create custom hostname mappings.
                            </Typography>
                        )}
                    </ConfigSection>

                    {/* Connection Test Results */}
                    {testResult.status && (
                        <Alert
                            severity={testResult.status}
                            sx={{ mb: 2 }}
                            icon={testResult.status === 'success' ? <CheckCircleIcon /> : <WarningIcon />}
                        >
                            <Typography variant="body2">
                                <strong>Connection Test Result:</strong> {testResult.message}
                            </Typography>
                        </Alert>
                    )}
                </Box>
            </DrawerContent>

            <CustomFooter>
                <Button
                    variant="outlined"
                    color="secondary"
                    onClick={handleClose}
                    disabled={saving || testing}
                >
                    Cancel
                </Button>

                <Button
                    variant="outlined"
                    startIcon={testing ? <CircularProgress size={16} /> : <WifiIcon />}
                    onClick={handleTestConnection}
                    disabled={saving || testing}
                    color={testResult.status === 'success' ? 'success' : 'primary'}
                >
                    {testing ? 'Testing...' : 'Test Connection'}
                </Button>

                <Button
                    variant="contained"
                    onClick={handleSave}
                    disabled={saving || testing || testResult.status !== 'success'}
                    startIcon={saving ? <CircularProgress size={16} /> : null}
                >
                    {saving ? 'Saving...' : 'Save Configuration'}
                </Button>
            </CustomFooter>
        </StyledDrawer>
    )
} 