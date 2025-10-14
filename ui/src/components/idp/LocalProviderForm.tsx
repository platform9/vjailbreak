import { useState, useEffect } from 'react';
import {
  Box,
  TextField,
  Button,
  FormControlLabel,
  Switch,
  Typography,
  Alert,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Chip,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import { IdentityProvider, LocalConfig, LocalUser } from '../../api/idp/types';

interface LocalProviderFormProps {
  provider: IdentityProvider | null;
  onSave: (provider: IdentityProvider) => void;
  onCancel: () => void;
}

const LocalProviderForm = ({ provider, onSave, onCancel }: LocalProviderFormProps) => {
  const [formData, setFormData] = useState<Partial<IdentityProvider>>({
    type: 'local',
    name: 'Local Users',
    enabled: true,
    description: 'Static username/password authentication',
    config: {
      users: [],
    } as LocalConfig,
  });

  const [openUserDialog, setOpenUserDialog] = useState(false);
  const [currentUser, setCurrentUser] = useState<LocalUser | null>(null);
  const [userForm, setUserForm] = useState<Partial<LocalUser>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});

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
  };

  const handleOpenUserDialog = (user?: LocalUser) => {
    if (user) {
      setCurrentUser(user);
      setUserForm({ ...user, password: '' }); // Don't show existing password
    } else {
      setCurrentUser(null);
      setUserForm({
        email: '',
        username: '',
        password: '',
        userID: `user-${Date.now()}`,
        groups: [],
      });
    }
    setOpenUserDialog(true);
    setErrors({});
  };

  const handleCloseUserDialog = () => {
    setOpenUserDialog(false);
    setCurrentUser(null);
    setUserForm({});
    setErrors({});
  };

  const validateUser = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!userForm.email?.trim()) {
      newErrors.email = 'Email is required';
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(userForm.email)) {
      newErrors.email = 'Invalid email format';
    }

    if (!userForm.username?.trim()) {
      newErrors.username = 'Username is required';
    }

    if (!currentUser && !userForm.password?.trim()) {
      newErrors.password = 'Password is required for new users';
    }

    if (userForm.password && userForm.password.length < 8) {
      newErrors.password = 'Password must be at least 8 characters';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSaveUser = () => {
    if (!validateUser()) {
      return;
    }

    const config = formData.config as LocalConfig;
    const users = config.users || [];

    if (currentUser) {
      // Update existing user
      const index = users.findIndex((u) => u.email === currentUser.email);
      if (index >= 0) {
        users[index] = { ...users[index], ...userForm } as LocalUser;
      }
    } else {
      // Add new user
      users.push(userForm as LocalUser);
    }

    setFormData((prev) => ({
      ...prev,
      config: {
        users,
      } as LocalConfig,
    }));

    handleCloseUserDialog();
  };

  const handleDeleteUser = (email: string) => {
    if (!window.confirm(`Delete user ${email}?`)) {
      return;
    }

    const config = formData.config as LocalConfig;
    const users = (config.users || []).filter((u) => u.email !== email);

    setFormData((prev) => ({
      ...prev,
      config: {
        users,
      } as LocalConfig,
    }));
  };

  const handleSubmit = () => {
    onSave({
      id: provider?.id || 'local',
      ...formData,
    } as IdentityProvider);
  };

  const config = formData.config as LocalConfig;
  const users = config.users || [];

  return (
    <Box sx={{ pt: 2 }}>
      <Alert severity="info" sx={{ mb: 3 }}>
        Manage local users with static passwords. For production, consider using an external
        identity provider (SAML or OIDC) for better security and management.
      </Alert>

      <TextField
        fullWidth
        label="Provider Name"
        value={formData.name || ''}
        onChange={(e) => handleChange('name', e.target.value)}
        margin="normal"
        disabled
      />

      <TextField
        fullWidth
        label="Description"
        value={formData.description || ''}
        onChange={(e) => handleChange('description', e.target.value)}
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
        label="Enable local authentication"
      />

      <Box mt={3} mb={2} display="flex" justifyContent="space-between" alignItems="center">
        <Typography variant="h6">Local Users</Typography>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={() => handleOpenUserDialog()}
          size="small"
        >
          Add User
        </Button>
      </Box>

      {users.length === 0 ? (
        <Alert severity="warning">
          No local users configured. Add at least one user to enable local authentication.
        </Alert>
      ) : (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Email</TableCell>
                <TableCell>Username</TableCell>
                <TableCell>Groups</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {users.map((user) => (
                <TableRow key={user.email}>
                  <TableCell>{user.email}</TableCell>
                  <TableCell>{user.username}</TableCell>
                  <TableCell>
                    {user.groups && user.groups.length > 0 ? (
                      <Box display="flex" gap={0.5}>
                        {user.groups.map((group) => (
                          <Chip key={group} label={group} size="small" />
                        ))}
                      </Box>
                    ) : (
                      <Typography variant="body2" color="text.secondary">
                        No groups
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell align="right">
                    <IconButton size="small" onClick={() => handleOpenUserDialog(user)}>
                      <EditIcon fontSize="small" />
                    </IconButton>
                    <IconButton
                      size="small"
                      color="error"
                      onClick={() => handleDeleteUser(user.email)}
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

      <Box display="flex" gap={2} justifyContent="flex-end" mt={3}>
        <Button onClick={onCancel} variant="outlined">
          Cancel
        </Button>
        <Button onClick={handleSubmit} variant="contained" color="primary">
          {provider ? 'Update' : 'Create'} Provider
        </Button>
      </Box>

      {/* User Dialog */}
      <Dialog open={openUserDialog} onClose={handleCloseUserDialog} maxWidth="sm" fullWidth>
        <DialogTitle>{currentUser ? 'Edit User' : 'Add New User'}</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            label="Email"
            value={userForm.email || ''}
            onChange={(e) => setUserForm({ ...userForm, email: e.target.value })}
            error={!!errors.email}
            helperText={errors.email || 'Used for login'}
            margin="normal"
            required
            disabled={!!currentUser} // Can't change email of existing user
          />

          <TextField
            fullWidth
            label="Username"
            value={userForm.username || ''}
            onChange={(e) => setUserForm({ ...userForm, username: e.target.value })}
            error={!!errors.username}
            helperText={errors.username || 'Display name'}
            margin="normal"
            required
          />

          <TextField
            fullWidth
            label="Password"
            value={userForm.password || ''}
            onChange={(e) => setUserForm({ ...userForm, password: e.target.value })}
            error={!!errors.password}
            helperText={
              errors.password ||
              (currentUser
                ? 'Leave blank to keep existing password'
                : 'Minimum 8 characters required')
            }
            margin="normal"
            type="password"
            required={!currentUser}
          />

          <TextField
            fullWidth
            label="Groups"
            value={(userForm.groups || []).join(', ')}
            onChange={(e) =>
              setUserForm({
                ...userForm,
                groups: e.target.value.split(',').map((g) => g.trim()).filter((g) => g),
              })
            }
            helperText="Comma-separated list (e.g., vjailbreak-admins, vjailbreak-operators)"
            margin="normal"
            placeholder="vjailbreak-admins, vjailbreak-operators"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseUserDialog}>Cancel</Button>
          <Button onClick={handleSaveUser} variant="contained">
            {currentUser ? 'Update' : 'Add'} User
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default LocalProviderForm;
