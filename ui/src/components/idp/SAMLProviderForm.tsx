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
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Chip,
} from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import { IdentityProvider, SAMLConfig } from '../../api/idp/types';

interface SAMLProviderFormProps {
  provider: IdentityProvider | null;
  onSave: (provider: IdentityProvider) => void;
  onCancel: () => void;
}

const SAMLProviderForm = ({ provider, onSave, onCancel }: SAMLProviderFormProps) => {
  const [formData, setFormData] = useState<Partial<IdentityProvider>>({
    type: 'saml',
    name: '',
    enabled: true,
    description: '',
    config: {
      ssoURL: '',
      caData: '',
      entityIssuer: '',
      redirectURI: '',
      usernameAttr: 'name',
      emailAttr: 'email',
      groupsAttr: 'groups',
      nameIDPolicyFormat: 'persistent',
    } as SAMLConfig,
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
    // Clear error for this field
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
        ...(prev.config as SAMLConfig),
        [field]: value,
      },
    }));
  };

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name?.trim()) {
      newErrors.name = 'Name is required';
    }

    const config = formData.config as SAMLConfig;
    if (!config.ssoURL?.trim()) {
      newErrors.ssoURL = 'SSO URL is required';
    }

    if (!config.entityIssuer?.trim()) {
      newErrors.entityIssuer = 'Entity Issuer is required';
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
      id: provider?.id || `saml-${Date.now()}`,
      ...formData,
    } as IdentityProvider);
  };

  const config = formData.config as SAMLConfig;

  const presetConfigs = [
    {
      name: 'Azure AD',
      template: {
        ssoURL: 'https://login.microsoftonline.com/{tenant-id}/saml2',
        entityIssuer: 'https://sts.windows.net/{tenant-id}/',
        usernameAttr: 'http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name',
        emailAttr: 'http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress',
        groupsAttr: 'http://schemas.microsoft.com/ws/2008/06/identity/claims/groups',
      },
    },
    {
      name: 'Okta',
      template: {
        ssoURL: 'https://{your-domain}.okta.com/app/{app-id}/sso/saml',
        entityIssuer: 'http://www.okta.com/{app-id}',
        usernameAttr: 'name',
        emailAttr: 'email',
        groupsAttr: 'groups',
      },
    },
    {
      name: 'OneLogin',
      template: {
        ssoURL: 'https://{subdomain}.onelogin.com/trust/saml2/http-post/sso/{app-id}',
        entityIssuer: 'https://app.onelogin.com/saml/metadata/{app-id}',
        usernameAttr: 'User.username',
        emailAttr: 'User.email',
        groupsAttr: 'memberOf',
      },
    },
  ];

  const applyPreset = (preset: any) => {
    setFormData((prev) => ({
      ...prev,
      config: {
        ...(prev.config as SAMLConfig),
        ...preset.template,
      },
    }));
  };

  return (
    <Box sx={{ pt: 2 }}>
      <Alert severity="info" sx={{ mb: 3 }}>
        Configure SAML 2.0 authentication with enterprise identity providers like Azure AD, Okta,
        or OneLogin.
      </Alert>

      <Box mb={2}>
        <Typography variant="subtitle2" gutterBottom>
          Quick Presets
        </Typography>
        <Box display="flex" gap={1}>
          {presetConfigs.map((preset) => (
            <Chip
              key={preset.name}
              label={preset.name}
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
        helperText={errors.name || 'e.g., "Azure AD Corporate Login"'}
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
        SAML Configuration
      </Typography>

      <TextField
        fullWidth
        label="SSO URL"
        value={config.ssoURL || ''}
        onChange={(e) => handleConfigChange('ssoURL', e.target.value)}
        error={!!errors.ssoURL}
        helperText={errors.ssoURL || 'SAML 2.0 SSO endpoint URL'}
        margin="normal"
        required
      />

      <TextField
        fullWidth
        label="Entity Issuer"
        value={config.entityIssuer || ''}
        onChange={(e) => handleConfigChange('entityIssuer', e.target.value)}
        error={!!errors.entityIssuer}
        helperText={errors.entityIssuer || 'SAML entity ID / issuer'}
        margin="normal"
        required
      />

      <TextField
        fullWidth
        label="Redirect URI"
        value={config.redirectURI || ''}
        onChange={(e) => handleConfigChange('redirectURI', e.target.value)}
        error={!!errors.redirectURI}
        helperText={errors.redirectURI || 'Callback URL (usually http://your-domain/dex/callback)'}
        margin="normal"
        required
      />

      <TextField
        fullWidth
        label="CA Certificate"
        value={config.caData || ''}
        onChange={(e) => handleConfigChange('caData', e.target.value)}
        helperText="Base64 encoded PEM certificate (optional for testing)"
        margin="normal"
        multiline
        rows={4}
        placeholder="-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAKL...
-----END CERTIFICATE-----"
      />

      <Accordion expanded={showAdvanced} onChange={() => setShowAdvanced(!showAdvanced)}>
        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
          <Typography>Advanced Settings</Typography>
        </AccordionSummary>
        <AccordionDetails>
          <Box>
            <TextField
              fullWidth
              label="Username Attribute"
              value={config.usernameAttr || 'name'}
              onChange={(e) => handleConfigChange('usernameAttr', e.target.value)}
              helperText="SAML attribute name for username"
              margin="normal"
            />

            <TextField
              fullWidth
              label="Email Attribute"
              value={config.emailAttr || 'email'}
              onChange={(e) => handleConfigChange('emailAttr', e.target.value)}
              helperText="SAML attribute name for email"
              margin="normal"
            />

            <TextField
              fullWidth
              label="Groups Attribute"
              value={config.groupsAttr || 'groups'}
              onChange={(e) => handleConfigChange('groupsAttr', e.target.value)}
              helperText="SAML attribute name for groups (for RBAC)"
              margin="normal"
            />

            <FormControl fullWidth margin="normal">
              <InputLabel>NameID Policy Format</InputLabel>
              <Select
                value={config.nameIDPolicyFormat || 'persistent'}
                onChange={(e) => handleConfigChange('nameIDPolicyFormat', e.target.value)}
                label="NameID Policy Format"
              >
                <MenuItem value="persistent">Persistent</MenuItem>
                <MenuItem value="transient">Transient</MenuItem>
                <MenuItem value="email">Email</MenuItem>
                <MenuItem value="unspecified">Unspecified</MenuItem>
              </Select>
            </FormControl>

            <FormControlLabel
              control={
                <Switch
                  checked={config.insecureSkipSignatureValidation || false}
                  onChange={(e) =>
                    handleConfigChange('insecureSkipSignatureValidation', e.target.checked)
                  }
                />
              }
              label="Skip signature validation (insecure, for testing only)"
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

export default SAMLProviderForm;
