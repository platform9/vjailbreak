import { useState, useEffect } from 'react';
import {
  Box,
  TextField,
  Button,
  FormControlLabel,
  Switch,
  Typography,
  Alert,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Chip,
} from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import { IdentityProvider, OIDCConfig } from '../../api/idp/types';

interface OIDCProviderFormProps {
  provider: IdentityProvider | null;
  onSave: (provider: IdentityProvider) => void;
  onCancel: () => void;
}

const OIDCProviderForm = ({ provider, onSave, onCancel }: OIDCProviderFormProps) => {
  const [formData, setFormData] = useState<Partial<IdentityProvider>>({
    type: 'oidc',
    name: '',
    enabled: true,
    description: '',
    config: {
      issuer: '',
      clientID: '',
      clientSecret: '',
      redirectURI: '',
      scopes: ['openid', 'profile', 'email', 'groups'],
      getUserInfo: true,
      insecureSkipEmailVerified: false,
    } as OIDCConfig,
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showAdvanced, setShowAdvanced] = useState(false);

  useEffect(() => {
    if (provider) {
      setFormData(provider);
    }
  }, [provider]);

  const handleChange = (field: string, value: any) => {
    setFormData((prev) => ({
      ...prev,
      [field]: value,
    }));
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  const handleConfigChange = (field: string, value: any) => {
    setFormData((prev) => ({
      ...prev,
      config: {
        ...(prev.config as OIDCConfig),
        [field]: value,
      },
    }));
  };

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name?.trim()) {
      newErrors.name = 'Name is required';
    }

    const config = formData.config as OIDCConfig;
    if (!config.issuer?.trim()) {
      newErrors.issuer = 'Issuer URL is required';
    }

    if (!config.clientID?.trim()) {
      newErrors.clientID = 'Client ID is required';
    }

    if (!config.clientSecret?.trim()) {
      newErrors.clientSecret = 'Client Secret is required';
    }

    if (!config.redirectURI?.trim()) {
      newErrors.redirectURI = 'Redirect URI is required';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = () => {
    if (!validate()) {
      return;
    }

    onSave({
      id: provider?.id || `oidc-${Date.now()}`,
      ...formData,
    } as IdentityProvider);
  };

  const config = formData.config as OIDCConfig;

  const presetConfigs = [
    {
      name: 'Google',
      icon: '🔍',
      template: {
        issuer: 'https://accounts.google.com',
        scopes: ['openid', 'profile', 'email'],
      },
    },
    {
      name: 'GitHub',
      icon: '🐙',
      template: {
        issuer: 'https://github.com',
        scopes: ['openid', 'profile', 'email', 'read:org'],
      },
    },
    {
      name: 'GitLab',
      icon: '🦊',
      template: {
        issuer: 'https://gitlab.com',
        scopes: ['openid', 'profile', 'email', 'read_user'],
      },
    },
    {
      name: 'Keycloak',
      icon: '🔑',
      template: {
        issuer: 'https://your-keycloak.com/auth/realms/your-realm',
        scopes: ['openid', 'profile', 'email', 'groups'],
      },
    },
    {
      name: 'Auth0',
      icon: '🔒',
      template: {
        issuer: 'https://your-tenant.auth0.com',
        scopes: ['openid', 'profile', 'email', 'groups'],
      },
    },
    {
      name: 'Okta',
      icon: '🟦',
      template: {
        issuer: 'https://your-domain.okta.com',
        scopes: ['openid', 'profile', 'email', 'groups'],
      },
    },
  ];

  const applyPreset = (preset: any) => {
    setFormData((prev) => ({
      ...prev,
      name: prev.name || preset.name,
      description: prev.description || `${preset.name} OIDC Integration`,
      config: {
        ...(prev.config as OIDCConfig),
        ...preset.template,
      },
    }));
  };

  const handleScopeChange = (scopes: string) => {
    const scopeArray = scopes.split(/[\s,]+/).filter((s) => s.trim());
    handleConfigChange('scopes', scopeArray);
  };

  return (
    <Box sx={{ pt: 2 }}>
      <Alert severity="info" sx={{ mb: 3 }}>
        Configure OpenID Connect (OIDC) / OAuth2 authentication with providers like Google,
        GitHub, GitLab, Keycloak, Auth0, or Okta.
      </Alert>

      <Box mb={2}>
        <Typography variant="subtitle2" gutterBottom>
          Quick Presets
        </Typography>
        <Box display="flex" gap={1} flexWrap="wrap">
          {presetConfigs.map((preset) => (
            <Chip
              key={preset.name}
              label={`${preset.icon} ${preset.name}`}
              onClick={() => applyPreset(preset)}
              clickable
              variant="outlined"
            />
          ))}
        </Box>
      </Box>

      <TextField
        fullWidth
        label="Provider Name"
        value={formData.name || ''}
        onChange={(e) => handleChange('name', e.target.value)}
        error={!!errors.name}
        helperText={errors.name || 'e.g., "Google Corporate Login"'}
        margin="normal"
        required
      />

      <TextField
        fullWidth
        label="Description"
        value={formData.description || ''}
        onChange={(e) => handleChange('description', e.target.value)}
        helperText="Optional description for this provider"
        margin="normal"
        multiline
        rows={2}
      />

      <FormControlLabel
        control={
          <Switch
            checked={formData.enabled || false}
            onChange={(e) => handleChange('enabled', e.target.checked)}
          />
        }
        label="Enable this provider"
      />

      <Typography variant="h6" sx={{ mt: 3, mb: 2 }}>
        OIDC Configuration
      </Typography>

      <TextField
        fullWidth
        label="Issuer URL"
        value={config.issuer || ''}
        onChange={(e) => handleConfigChange('issuer', e.target.value)}
        error={!!errors.issuer}
        helperText={errors.issuer || 'OIDC issuer endpoint (e.g., https://accounts.google.com)'}
        margin="normal"
        required
      />

      <TextField
        fullWidth
        label="Client ID"
        value={config.clientID || ''}
        onChange={(e) => handleConfigChange('clientID', e.target.value)}
        error={!!errors.clientID}
        helperText={errors.clientID || 'OAuth2 client ID from your provider'}
        margin="normal"
        required
      />

      <TextField
        fullWidth
        label="Client Secret"
        value={config.clientSecret || ''}
        onChange={(e) => handleConfigChange('clientSecret', e.target.value)}
        error={!!errors.clientSecret}
        helperText={errors.clientSecret || 'OAuth2 client secret (keep this secure!)'}
        margin="normal"
        type="password"
        required
      />

      <TextField
        fullWidth
        label="Redirect URI"
        value={config.redirectURI || ''}
        onChange={(e) => handleConfigChange('redirectURI', e.target.value)}
        error={!!errors.redirectURI}
        helperText={
          errors.redirectURI ||
          'Callback URL configured in your provider (e.g., http://your-domain/dex/callback)'
        }
        margin="normal"
        required
      />

      <TextField
        fullWidth
        label="Scopes"
        value={(config.scopes || []).join(' ')}
        onChange={(e) => handleScopeChange(e.target.value)}
        helperText="Space or comma-separated list of OAuth2 scopes"
        margin="normal"
        placeholder="openid profile email groups"
      />

      <Accordion expanded={showAdvanced} onChange={() => setShowAdvanced(!showAdvanced)}>
        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
          <Typography>Advanced Settings</Typography>
        </AccordionSummary>
        <AccordionDetails>
          <Box>
            <TextField
              fullWidth
              label="Username Claim"
              value={config.usernameClaim || ''}
              onChange={(e) => handleConfigChange('usernameClaim', e.target.value)}
              helperText="JWT claim to use as username (default: preferred_username or email)"
              margin="normal"
              placeholder="preferred_username"
            />

            <TextField
              fullWidth
              label="Groups Claim"
              value={config.groupsClaim || ''}
              onChange={(e) => handleConfigChange('groupsClaim', e.target.value)}
              helperText="JWT claim containing user groups (for RBAC)"
              margin="normal"
              placeholder="groups"
            />

            <FormControlLabel
              control={
                <Switch
                  checked={config.getUserInfo || false}
                  onChange={(e) => handleConfigChange('getUserInfo', e.target.checked)}
                />
              }
              label="Fetch user info from /userinfo endpoint"
            />

            <FormControlLabel
              control={
                <Switch
                  checked={config.insecureSkipEmailVerified || false}
                  onChange={(e) =>
                    handleConfigChange('insecureSkipEmailVerified', e.target.checked)
                  }
                />
              }
              label="Skip email verification check (insecure, for testing)"
            />

            <FormControlLabel
              control={
                <Switch
                  checked={config.insecureEnableGroups || false}
                  onChange={(e) => handleConfigChange('insecureEnableGroups', e.target.checked)}
                />
              }
              label="Enable groups without verification"
            />

            <TextField
              fullWidth
              label="Hosted Domains"
              value={(config.hostedDomains || []).join(', ')}
              onChange={(e) =>
                handleConfigChange(
                  'hostedDomains',
                  e.target.value.split(',').map((d) => d.trim())
                )
              }
              helperText="Restrict to specific email domains (comma-separated)"
              margin="normal"
              placeholder="company.com, example.org"
            />
          </Box>
        </AccordionDetails>
      </Accordion>

      <Box display="flex" gap={2} justifyContent="flex-end" mt={3}>
        <Button onClick={onCancel} variant="outlined">
          Cancel
        </Button>
        <Button onClick={handleSubmit} variant="contained" color="primary">
          {provider ? 'Update' : 'Create'} Provider
        </Button>
      </Box>
    </Box>
  );
};

export default OIDCProviderForm;
