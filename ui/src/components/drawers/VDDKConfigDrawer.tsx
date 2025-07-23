import {
    Box,
    Drawer,
    Typography,
    styled,
    Alert,
    List,
    ListItem,
    ListItemIcon,
    ListItemText,
    Button,
    LinearProgress,
    Card,
    CardContent,
    Stepper,
    Step,
    StepLabel,
    StepContent,
    IconButton,
} from "@mui/material"
import StorageIcon from "@mui/icons-material/Storage"
import CheckCircleIcon from "@mui/icons-material/CheckCircle"
import WarningIcon from "@mui/icons-material/Warning"
import ErrorIcon from "@mui/icons-material/Error"

import DownloadIcon from "@mui/icons-material/Download"
import FolderIcon from "@mui/icons-material/Folder"
import UploadFileIcon from "@mui/icons-material/UploadFile"
import CloudUploadIcon from "@mui/icons-material/CloudUpload"
import DeleteIcon from "@mui/icons-material/Delete"
import { useState, useEffect } from "react"
import Header from "../forms/Header"
import Footer from "../forms/Footer"
import ComputerIcon from "@mui/icons-material/Computer"
const StyledDrawer = styled(Drawer)(({ theme }) => ({
    "& .MuiDrawer-paper": {
        display: "grid",
        gridTemplateRows: "max-content 1fr max-content",
        width: "900px",
        maxWidth: "90vw",
        zIndex: theme.zIndex.modal,
    },
}))

const DrawerContent = styled("div")(({ theme }) => ({
    overflow: "auto",
    padding: theme.spacing(4, 6, 4, 4),
}))

const StatusCard = styled(Card)(({ theme }) => ({
    marginBottom: theme.spacing(2),
}))

const UploadArea = styled(Box)(({ theme }) => ({
    border: `2px dashed ${theme.palette.grey[300]}`,
    borderRadius: theme.spacing(1),
    padding: theme.spacing(4),
    textAlign: "center",
    cursor: "pointer",
    transition: "all 0.3s ease",
    "&:hover": {
        borderColor: theme.palette.primary.main,
        backgroundColor: theme.palette.action.hover,
    },
    "&.dragover": {
        borderColor: theme.palette.primary.main,
        backgroundColor: theme.palette.primary.light + "20",
    },
}))

const FileItem = styled(Box)(({ theme }) => ({
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    padding: theme.spacing(2),
    border: `1px solid ${theme.palette.grey[300]}`,
    borderRadius: theme.spacing(1),
    marginTop: theme.spacing(2),
}))

interface VDDKConfigDrawerProps {
    open: boolean
    onClose: () => void
    onComplete: () => void
}

interface VDDKStatus {
    isInstalled: boolean
    version?: string
    path?: string
    isValid: boolean
    error?: string
}

interface UploadedFile {
    file: File
    progress: number
    status: 'pending' | 'uploading' | 'completed' | 'error'
    error?: string
}

export default function VDDKConfigDrawer({
    open,
    onClose,
    onComplete,
}: VDDKConfigDrawerProps) {
    const [vddkStatus, setVddkStatus] = useState<VDDKStatus>({
        isInstalled: false,
        isValid: false,
    })
    const [activeStep, setActiveStep] = useState(0)
    const [installing, setInstalling] = useState(false)
    const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([])
    const [dragOver, setDragOver] = useState(false)

    const steps = [
        {
            label: "Upload VDDK",
            description: "Upload VMware Virtual Disk Development Kit files",
        },
        {
            label: "Install & Configure",
            description: "Install VDDK and configure paths",
        },
        {
            label: "Verify Installation",
            description: "Validate VDDK installation and functionality",
        },
    ]

    useEffect(() => {
        if (open) {
            // Since VDDK is never pre-installed, start directly at upload step
            setActiveStep(0)
        }
    }, [open])



    const handleInstallVDDK = async () => {
        setInstalling(true)
        try {
            // Simulate installation process
            setActiveStep(1)
            await new Promise(resolve => setTimeout(resolve, 2000))

            setActiveStep(2)
            await new Promise(resolve => setTimeout(resolve, 1000))

            setVddkStatus({
                isInstalled: true,
                version: "8.0.3",
                path: "/home/ubuntu/vmware-vix-disklib-distrib",
                isValid: true,
            })
        } catch {
            setVddkStatus(prev => ({
                ...prev,
                error: "Installation failed",
            }))
        } finally {
            setInstalling(false)
        }
    }

    const handleComplete = () => {
        if (vddkStatus.isInstalled && vddkStatus.isValid) {
            onComplete()
            onClose()
        }
    }

    const getStatusIcon = () => {
        if (vddkStatus.error) return <ErrorIcon color="error" />
        if (vddkStatus.isInstalled && vddkStatus.isValid) return <CheckCircleIcon color="success" />
        return <WarningIcon color="warning" />
    }

    const getStatusText = () => {
        if (vddkStatus.error) return vddkStatus.error
        if (vddkStatus.isInstalled && vddkStatus.isValid) {
            return `VDDK ${vddkStatus.version} is installed and ready`
        }
        return "Ready to upload VDDK files"
    }

    const handleFileUpload = (files: FileList | null) => {
        if (!files) return

        Array.from(files).forEach((file) => {
            // Validate file type (basic validation)
            if (!file.name.toLowerCase().includes('vddk') && !file.name.toLowerCase().includes('vix')) {
                const newFile: UploadedFile = {
                    file,
                    progress: 0,
                    status: 'error',
                    error: 'Please upload a valid VDDK file',
                }
                setUploadedFiles(prev => [...prev, newFile])
                return
            }

            const newFile: UploadedFile = {
                file,
                progress: 0,
                status: 'pending',
            }
            setUploadedFiles(prev => [...prev, newFile])

            // Simulate upload process
            simulateUpload(newFile)
        })
    }

    const simulateUpload = async (uploadedFile: UploadedFile) => {
        const fileIndex = uploadedFiles.findIndex(f => f.file === uploadedFile.file)

        // Update status to uploading
        setUploadedFiles(prev =>
            prev.map((f, i) => i === fileIndex ? { ...f, status: 'uploading' } : f)
        )

        // Simulate upload progress
        for (let progress = 0; progress <= 100; progress += 10) {
            await new Promise(resolve => setTimeout(resolve, 200))
            setUploadedFiles(prev =>
                prev.map((f, i) => i === fileIndex ? { ...f, progress } : f)
            )
        }

        // Complete upload
        setUploadedFiles(prev =>
            prev.map((f, i) => i === fileIndex ? { ...f, status: 'completed', progress: 100 } : f)
        )

        // Update VDDK status if upload successful
        setVddkStatus({
            isInstalled: true,
            version: "8.0.3",
            path: "/home/ubuntu/vmware-vix-disklib-distrib",
            isValid: true,
        })
        setActiveStep(2)
    }

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault()
        setDragOver(false)
        handleFileUpload(e.dataTransfer.files)
    }

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault()
        setDragOver(true)
    }

    const handleDragLeave = (e: React.DragEvent) => {
        e.preventDefault()
        setDragOver(false)
    }

    const removeFile = (index: number) => {
        setUploadedFiles(prev => prev.filter((_, i) => i !== index))
    }

    return (
        <StyledDrawer anchor="right" open={open} onClose={onClose}>
            <Header
                title="VDDK Configuration"
                icon={<ComputerIcon />}
            />

            <DrawerContent>
                <Box sx={{ display: "grid", gap: 4 }}>
                    <Alert severity="info">
                        VMware Virtual Disk Development Kit (VDDK) is required for accessing VMware virtual machine disks
                        during migration. This component enables efficient data transfer from VMware environments.
                    </Alert>

                    {/* Status Card */}
                    <StatusCard>
                        <CardContent>
                            <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 2 }}>
                                {getStatusIcon()}
                                <Typography variant="h6">VDDK Status</Typography>
                            </Box>
                            <Typography variant="body2" color="text.secondary">
                                {getStatusText()}
                            </Typography>

                            {vddkStatus.isInstalled && (
                                <Box sx={{ mt: 2 }}>
                                    <Typography variant="body2">
                                        <strong>Version:</strong> {vddkStatus.version}
                                    </Typography>
                                    <Typography variant="body2">
                                        <strong>Path:</strong> {vddkStatus.path}
                                    </Typography>
                                </Box>
                            )}
                        </CardContent>
                    </StatusCard>

                    {/* Installation Steps */}
                    <Box>
                        <Typography variant="h6" gutterBottom>
                            Installation Steps
                        </Typography>

                        <Stepper activeStep={activeStep} orientation="vertical">
                            {steps.map((step, index) => (
                                <Step key={step.label}>
                                    <StepLabel>{step.label}</StepLabel>
                                    <StepContent>
                                        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                                            {step.description}
                                        </Typography>

                                        {index === 0 && (
                                            <Box>
                                                <Typography variant="subtitle2" gutterBottom>
                                                    Upload VDDK Files
                                                </Typography>
                                                <UploadArea
                                                    className={dragOver ? 'dragover' : ''}
                                                    onDrop={handleDrop}
                                                    onDragOver={handleDragOver}
                                                    onDragLeave={handleDragLeave}
                                                    onClick={() => document.getElementById('vddk-file-input')?.click()}
                                                >
                                                    <CloudUploadIcon sx={{ fontSize: 48, color: 'text.secondary', mb: 2 }} />
                                                    <Typography variant="h6" gutterBottom>
                                                        Drop VDDK files here or click to browse
                                                    </Typography>
                                                    <Typography variant="body2" color="text.secondary">
                                                        Supports .tar.gz, .zip, or extracted VDDK directories
                                                    </Typography>
                                                    <input
                                                        id="vddk-file-input"
                                                        type="file"
                                                        multiple
                                                        accept=".tar.gz,.zip,.tar"
                                                        style={{ display: 'none' }}
                                                        onChange={(e) => handleFileUpload(e.target.files)}
                                                    />
                                                </UploadArea>

                                                {/* Uploaded Files List */}
                                                {uploadedFiles.map((uploadedFile, fileIndex) => (
                                                    <FileItem key={fileIndex}>
                                                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flex: 1 }}>
                                                            <UploadFileIcon color="primary" />
                                                            <Box sx={{ flex: 1 }}>
                                                                <Typography variant="body2" noWrap>
                                                                    {uploadedFile.file.name}
                                                                </Typography>
                                                                <Typography variant="caption" color="text.secondary">
                                                                    {(uploadedFile.file.size / 1024 / 1024).toFixed(2)} MB
                                                                </Typography>
                                                                {uploadedFile.status === 'uploading' && (
                                                                    <LinearProgress
                                                                        variant="determinate"
                                                                        value={uploadedFile.progress}
                                                                        sx={{ mt: 1 }}
                                                                    />
                                                                )}
                                                                {uploadedFile.status === 'error' && (
                                                                    <Typography variant="caption" color="error">
                                                                        {uploadedFile.error}
                                                                    </Typography>
                                                                )}
                                                                {uploadedFile.status === 'completed' && (
                                                                    <Typography variant="caption" color="success.main">
                                                                        Upload completed
                                                                    </Typography>
                                                                )}
                                                            </Box>
                                                        </Box>
                                                        <IconButton
                                                            size="small"
                                                            onClick={() => removeFile(fileIndex)}
                                                            color="error"
                                                        >
                                                            <DeleteIcon />
                                                        </IconButton>
                                                    </FileItem>
                                                ))}

                                                <Box sx={{ mt: 2, display: "flex", gap: 2, flexWrap: "wrap" }}>
                                                    <Button
                                                        variant="outlined"
                                                        startIcon={<DownloadIcon />}
                                                        size="small"
                                                        onClick={() => window.open("https://developer.vmware.com/tools/vsphere-virtual-disk-development-kit", "_blank")}
                                                    >
                                                        Download VDDK
                                                    </Button>
                                                    <Button
                                                        variant="outlined"
                                                        startIcon={<FolderIcon />}
                                                        size="small"
                                                        onClick={() => window.open("/documentation/vddk-setup", "_blank")}
                                                    >
                                                        Setup Guide
                                                    </Button>
                                                </Box>
                                            </Box>
                                        )}

                                        {index === 1 && !vddkStatus.isInstalled && (
                                            <Button
                                                variant="contained"
                                                onClick={handleInstallVDDK}
                                                disabled={installing}
                                                startIcon={<StorageIcon />}
                                            >
                                                {installing ? "Installing..." : "Install VDDK"}
                                            </Button>
                                        )}
                                    </StepContent>
                                </Step>
                            ))}
                        </Stepper>
                    </Box>

                    {/* Installation Requirements */}
                    <Box>
                        <Typography variant="h6" gutterBottom>
                            Requirements
                        </Typography>

                        <List dense>
                            <ListItem>
                                <ListItemIcon>
                                    <CheckCircleIcon color="success" />
                                </ListItemIcon>
                                <ListItemText
                                    primary="VMware VDDK 8.0 or later"
                                    secondary="Required for optimal performance and compatibility"
                                />
                            </ListItem>

                            <ListItem>
                                <ListItemIcon>
                                    <CheckCircleIcon color="success" />
                                </ListItemIcon>
                                <ListItemText
                                    primary="Sufficient disk space (2GB minimum)"
                                    secondary="For VDDK libraries and temporary migration data"
                                />
                            </ListItem>

                            <ListItem>
                                <ListItemIcon>
                                    <CheckCircleIcon color="success" />
                                </ListItemIcon>
                                <ListItemText
                                    primary="Network connectivity to vCenter"
                                    secondary="Required for accessing VMware virtual machine disks"
                                />
                            </ListItem>
                        </List>
                    </Box>

                    {/* Troubleshooting */}
                    {vddkStatus.error && (
                        <Alert severity="error">
                            <Typography variant="body2" sx={{ mb: 1 }}>
                                <strong>Installation Error:</strong> {vddkStatus.error}
                            </Typography>
                            <Typography variant="body2">
                                Please check the installation guide or contact support for assistance.
                            </Typography>
                        </Alert>
                    )}
                </Box>
            </DrawerContent>

            <Footer
                onClose={onClose}
                onSubmit={handleComplete}
                submitButtonLabel="Complete Setup"
                disableSubmit={!vddkStatus.isInstalled || !vddkStatus.isValid || installing}
                submitting={installing}
            />
        </StyledDrawer >
    )
} 