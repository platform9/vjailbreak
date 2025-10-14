import { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  IconButton,
  Chip,
  Alert,
  CircularProgress,
  Grid,
  Tab,
  Tabs,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import { idpService } from '../../api/idp/idpService';
import { IdentityProvider, IdentityProviderType } from '../../api/idp/types';
import SAMLProviderForm from '../../components/idp/SAMLProviderForm';
import OIDCProviderForm from '../../components/idp/OIDCProviderForm';
import LocalProviderForm from '../../components/idp/LocalProviderForm';

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

const TabPanel = ({ children, value, index }: TabPanelProps) => {
  return (
    <div hidden={value !== index}>
      {value === index && <Box sx={{ pt: 3 }}>{children}</Box>}
    </div>
  );
};

const IdentityProvidersPage = () => {
  const [providers, setProviders] = useState<IdentityProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [openDialog, setOpenDialog] = useState(false);
  const [currentProvider, setCurrentProvider] = useState<IdentityProvider | null>(null);
  const [providerType, setProviderType] = useState<IdentityProviderType>('saml');
  const [tabValue, setTabValue] = useState(0);

  useEffect(() => {
    loadProviders();
  }, []);

  const loadProviders = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await idpService.listProviders();
      setProviders(data);
    } catch (err: any) {
      setError(err.message || 'Failed to load identity providers');
    } finally {
      setLoading(false);
    }
  };

  const handleOpenDialog = (type: IdentityProviderType, provider?: IdentityProvider) => {
    setProviderType(type);
    setCurrentProvider(provider || null);
    setOpenDialog(true);
  };

  const handleCloseDialog = () => {
    setOpenDialog(false);
    setCurrentProvider(null);
    setError(null);
  };

  const handleSave = async (provider: IdentityProvider) => {
    try {
      setError(null);
      if (currentProvider) {
        await idpService.updateProvider(currentProvider.id, provider);
        setSuccess('Identity provider updated successfully');
      } else {
        await idpService.createProvider(provider);
        setSuccess('Identity provider created successfully');
      }
      handleCloseDialog();
      await loadProviders();
      setTimeout(() => setSuccess(null), 5000);
    } catch (err: any) {
      setError(err.message || 'Failed to save identity provider');
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this identity provider?')) {
      return;
    }

    try {
      setError(null);
      await idpService.deleteProvider(id);
      setSuccess('Identity provider deleted successfully');
      await loadProviders();
      setTimeout(() => setSuccess(null), 5000);
    } catch (err: any) {
      setError(err.message || 'Failed to delete identity provider');
    }
  };

  const handleTest = async (id: string) => {
    try {
      setError(null);
      const result = await idpService.testProvider(id);
      if (result.success) {
        setSuccess(`Connection test successful: ${result.message}`);
      } else {
        setError(`Connection test failed: ${result.message}`);
      }
      setTimeout(() => {
        setSuccess(null);
        setError(null);
      }, 5000);
    } catch (err: any) {
      setError(err.message || 'Failed to test connection');
    }
  };

  const getProviderIcon = (type: IdentityProviderType) => {
    switch (type) {
      case 'saml':
        return '🔐';
      case 'oidc':
        return '🌐';
      case 'local':
        return '👤';
      default:
        return '❓';
    }
  };

  const getProviderTypeLabel = (type: IdentityProviderType) => {
    switch (type) {
      case 'saml':
        return 'SAML 2.0';
      case 'oidc':
        return 'OIDC';
      case 'local':
        return 'Local';
      default:
        return type;
    }
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="60vh">
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box sx={{ p: 3 }}>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Box>
          <Typography variant="h4" gutterBottom>
            Identity Providers
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Configure external identity providers for authentication (SAML, OIDC)
          </Typography>
        </Box>
        <Box>
          <Tabs value={tabValue} onChange={(_, val) => setTabValue(val)}>
            <Tab label="All Providers" />
            <Tab label="Add Provider" />
          </Tabs>
        </Box>
      </Box>

      {error && (
        <Alert severity="error" onClose={() => setError(null)} sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {success && (
        <Alert severity="success" onClose={() => setSuccess(null)} sx={{ mb: 2 }}>
          {success}
        </Alert>
      )}

      <TabPanel value={tabValue} index={0}>
        {providers.length === 0 ? (
          <Card>
            <CardContent>
              <Box textAlign="center" py={4}>
                <Typography variant="h6" color="text.secondary" gutterBottom>
                  No Identity Providers Configured
                </Typography>
                <Typography variant="body2" color="text.secondary" mb={2}>
                  Add an identity provider to enable external authentication
                </Typography>
                <Button
                  variant="contained"
                  startIcon={<AddIcon />}
                  onClick={() => setTabValue(1)}
                >
                  Add Identity Provider
                </Button>
              </Box>
            </CardContent>
          </Card>
        ) : (
          <Grid container spacing={2}>
            {providers.map((provider) => (
              <Grid item xs={12} md={6} lg={4} key={provider.id}>
                <Card>
                  <CardContent>
                    <Box display="flex" justifyContent="space-between" alignItems="start" mb={2}>
                      <Box display="flex" alignItems="center" gap={1}>
                        <Typography variant="h3">{getProviderIcon(provider.type)}</Typography>
                        <Box>
                          <Typography variant="h6">{provider.name}</Typography>
                          <Chip
                            label={getProviderTypeLabel(provider.type)}
                            size="small"
                            color="primary"
                            sx={{ mt: 0.5 }}
                          />
                        </Box>
                      </Box>
                      <Box>
                        {provider.enabled ? (
                          <CheckCircleIcon color="success" />
                        ) : (
                          <ErrorIcon color="disabled" />
                        )}
                      </Box>
                    </Box>

                    <Typography variant="body2" color="text.secondary" mb={2}>
                      {provider.description || 'No description'}
                    </Typography>

                    <Box display="flex" gap={1} flexWrap="wrap">
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<EditIcon />}
                        onClick={() => handleOpenDialog(provider.type, provider)}
                      >
                        Edit
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        onClick={() => handleTest(provider.id)}
                      >
                        Test
                      </Button>
                      <IconButton
                        size="small"
                        color="error"
                        onClick={() => handleDelete(provider.id)}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </Box>
                  </CardContent>
                </Card>
              </Grid>
            ))}
          </Grid>
        )}
      </TabPanel>

      <TabPanel value={tabValue} index={1}>
        <Grid container spacing={3}>
          <Grid item xs={12} md={4}>
            <Card
              sx={{ cursor: 'pointer', '&:hover': { boxShadow: 3 } }}
              onClick={() => handleOpenDialog('saml')}
            >
              <CardContent>
                <Box textAlign="center" py={2}>
                  <Typography variant="h2" mb={1}>
                    🔐
                  </Typography>
                  <Typography variant="h6" gutterBottom>
                    SAML 2.0
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Azure AD, Okta, OneLogin, ADFS
                  </Typography>
                </Box>
              </CardContent>
            </Card>
          </Grid>

          <Grid item xs={12} md={4}>
            <Card
              sx={{ cursor: 'pointer', '&:hover': { boxShadow: 3 } }}
              onClick={() => handleOpenDialog('oidc')}
            >
              <CardContent>
                <Box textAlign="center" py={2}>
                  <Typography variant="h2" mb={1}>
                    🌐
                  </Typography>
                  <Typography variant="h6" gutterBottom>
                    OIDC / OAuth2
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Google, GitHub, GitLab, Keycloak
                  </Typography>
                </Box>
              </CardContent>
            </Card>
          </Grid>

          <Grid item xs={12} md={4}>
            <Card
              sx={{ cursor: 'pointer', '&:hover': { boxShadow: 3 } }}
              onClick={() => handleOpenDialog('local')}
            >
              <CardContent>
                <Box textAlign="center" py={2}>
                  <Typography variant="h2" mb={1}>
                    👤
                  </Typography>
                  <Typography variant="h6" gutterBottom>
                    Local Users
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Static username/password authentication
                  </Typography>
                </Box>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      </TabPanel>

      <Dialog open={openDialog} onClose={handleCloseDialog} maxWidth="md" fullWidth>
        <DialogTitle>
          {currentProvider ? 'Edit' : 'Add'} Identity Provider
          {providerType && ` - ${getProviderTypeLabel(providerType)}`}
        </DialogTitle>
        <DialogContent>
          {providerType === 'saml' && (
            <SAMLProviderForm
              provider={currentProvider}
              onSave={handleSave}
              onCancel={handleCloseDialog}
            />
          )}
          {providerType === 'oidc' && (
            <OIDCProviderForm
              provider={currentProvider}
              onSave={handleSave}
              onCancel={handleCloseDialog}
            />
          )}
          {providerType === 'local' && (
            <LocalProviderForm
              provider={currentProvider}
              onSave={handleSave}
              onCancel={handleCloseDialog}
            />
          )}
        </DialogContent>
      </Dialog>
    </Box>
  );
};

export default IdentityProvidersPage;
