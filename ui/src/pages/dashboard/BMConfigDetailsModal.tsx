import { useState, useEffect } from 'react';
import {
    Box,
    Typography,
    Paper,
    styled,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Button,
    Grid,
    CircularProgress,
    Switch,
    Tooltip
} from '@mui/material';
import InfoIcon from '@mui/icons-material/Info';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneLight, oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { BMConfig } from '../../api/bmconfig/model';
import { getBMConfig } from '../../api/bmconfig/bmconfig';
import { getSecret } from '../../api/secrets/secrets';
import { useThemeContext } from 'src/theme/ThemeContext';

interface MaasConfigDetailsModalProps {
    open: boolean;
    onClose: () => void;
    configName: string;
    namespace?: string;
}

const StyledPaper = styled(Paper)({
    width: '100%',
    padding: '24px',
    boxSizing: 'border-box',
});

const CodeEditorContainer = styled(Box)(({ theme }) => ({
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: theme.shape.borderRadius,
    overflow: 'auto',
    position: 'relative',
    minHeight: '150px',
    maxHeight: '400px',
    backgroundColor: theme.palette.mode === 'dark'
        ? theme.palette.background.paper
        : theme.palette.common.white,
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
        backgroundColor: theme.palette.mode === 'dark'
            ? theme.palette.grey[700]
            : theme.palette.grey[300],
        borderRadius: '4px',
    },
    '&::-webkit-scrollbar-track': {
        backgroundColor: theme.palette.mode === 'dark'
            ? theme.palette.grey[900]
            : theme.palette.grey[100],
    }
}));

const Section = styled(Box)(({ theme }) => ({
    marginBottom: theme.spacing(3),
}));

const DetailItem = styled(Box)(({ theme }) => ({
    marginBottom: theme.spacing(2),
}));

export default function MaasConfigDetailsModal({
    open,
    onClose,
    configName,
    namespace = "migration-system"
}: MaasConfigDetailsModalProps) {
    const { mode } = useThemeContext();
    const [loading, setLoading] = useState(true);
    const [config, setConfig] = useState<BMConfig | null>(null);
    const [cloudInit, setCloudInit] = useState<string>('');
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        if (open && configName) {
            fetchConfigDetails();
        }
    }, [open, configName, namespace]);

    const fetchConfigDetails = async () => {
        setLoading(true);
        setError(null);
        try {
            const configData = await getBMConfig(configName, namespace);
            setConfig(configData);

            if (configData?.spec?.userDataSecretRef?.name) {
                try {
                    const secretData = await getSecret(
                        configData.spec.userDataSecretRef.name,
                        configData.spec.userDataSecretRef.namespace || namespace
                    );

                    if (secretData && secretData.data && secretData.data["user-data"]) {
                        setCloudInit(secretData.data["user-data"]);
                    }
                } catch (secretError) {
                    console.warn('Error fetching user-data secret:', secretError);
                }
            }
        } catch (error) {
            console.error('Error fetching MaasConfig details:', error);
            setError('Failed to load MaasConfig details');
        } finally {
            setLoading(false);
        }
    };

    return (
        <Dialog
            open={open}
            onClose={onClose}
            maxWidth="md"
            fullWidth
        >
            <DialogTitle>
                MAAS Configuration Details
            </DialogTitle>
            <DialogContent dividers>
                {loading ? (
                    <Box display="flex" justifyContent="center" alignItems="center" height="400px">
                        <CircularProgress />
                        <Typography variant="body1" sx={{ ml: 2 }}>
                            Loading configuration details...
                        </Typography>
                    </Box>
                ) : error ? (
                    <Typography color="error" align="center">
                        {error}
                    </Typography>
                ) : config ? (
                    <StyledPaper elevation={0}>
                        <Section>
                            <Grid container spacing={3}>
                                <Grid item xs={12} md={8}>
                                    <DetailItem>
                                        <Typography variant="subtitle2" color="text.secondary" sx={{ display: 'flex', alignItems: 'center' }}>
                                            MAAS URL
                                        </Typography>
                                        <Typography variant="body1">
                                            {config.spec.apiUrl || 'Not specified'}
                                        </Typography>
                                    </DetailItem>
                                </Grid>
                                <Grid item xs={12} md={4}>
                                    <DetailItem>
                                        <Typography variant="subtitle2" color="text.secondary">
                                            Insecure
                                        </Typography>
                                        <Switch
                                            checked={config.spec.insecure}
                                            disabled
                                            color="primary"
                                        />
                                    </DetailItem>
                                </Grid>
                                <Grid item xs={12}>
                                    <Box sx={{ display: 'flex', gap: 4 }}>
                                        <Box sx={{ flex: 1 }}>
                                            <Typography variant="subtitle2" color="text.secondary" sx={{ display: 'flex', alignItems: 'center' }}>
                                                OS
                                                <Tooltip title="Currently only Ubuntu Jammy (22.04) is supported for ESXi host deployments." arrow>
                                                    <InfoIcon fontSize="small" sx={{ ml: 1, opacity: 0.7 }} />
                                                </Tooltip>
                                            </Typography>
                                            <Typography variant="body1">
                                                {config.spec.os || 'Not specified'}
                                            </Typography>
                                        </Box>
                                        <Box sx={{ flex: 1 }}>
                                            <Typography variant="subtitle2" color="text.secondary">
                                                Provider Type
                                            </Typography>
                                            <Typography variant="body1">
                                                {config.spec.providerType || 'Not specified'}
                                            </Typography>
                                        </Box>
                                        <Box sx={{ flex: 1 }}>
                                            <Typography variant="subtitle2" color="text.secondary" gutterBottom>
                                                Validation Status
                                            </Typography>
                                            <Typography
                                                variant="body1"
                                                color={
                                                    config.status?.validationStatus === 'Succeeded'
                                                        ? 'success.main'
                                                        : 'error.main'
                                                }
                                            >
                                                {config.status?.validationStatus || 'Unknown'}
                                            </Typography>
                                            {config.status?.validationMessage && (
                                                <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                                                    {config.status.validationMessage}
                                                </Typography>
                                            )}
                                        </Box>
                                    </Box>
                                </Grid>
                            </Grid>
                        </Section>

                        {cloudInit && (
                            <Section>
                                <Typography variant="subtitle2" color="text.secondary" gutterBottom>
                                    Cloud-init (YAML)
                                </Typography>
                                <CodeEditorContainer>
                                    <SyntaxHighlighter
                                        language="yaml"
                                        style={mode === 'dark' ? oneDark : oneLight}
                                        showLineNumbers
                                        wrapLongLines
                                        customStyle={{
                                            margin: 0,
                                            maxHeight: '100%',
                                            backgroundColor: 'transparent'
                                        }}
                                    >
                                        {cloudInit}
                                    </SyntaxHighlighter>
                                </CodeEditorContainer>
                            </Section>
                        )}
                    </StyledPaper>
                ) : (
                    <Typography variant="body1" align="center">
                        No configuration found
                    </Typography>
                )}
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose} variant="contained">
                    Close
                </Button>
            </DialogActions>
        </Dialog>
    );
} 