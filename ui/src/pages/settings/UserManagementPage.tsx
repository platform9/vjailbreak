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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import { idpService } from '../../api/idp/idpService';
import { LocalUser } from '../../api/idp/types';

const UserManagementPage = () => {
  const [users, setUsers] = useState<LocalUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [openDialog, setOpenDialog] = useState(false);
  const [currentUser, setCurrentUser] = useState<LocalUser | null>(null);
  const [formData, setFormData] = useState({
    email: '',
    username: '',
    password: '',
    role: 'operator' as 'super-admin' | 'vjailbreak-admin' | 'admin' | 'operator' | 'viewer',
  });

  useEffect(() => {
    loadUsers();
  }, []);

  const loadUsers = async () => {
    setLoading(true);
    setError(null);
    try {
      const usersList = await idpService.listLocalUsers();
      setUsers(usersList);
    } catch (err) {
      setError('Failed to load users. Please try again.');
      console.error('Error loading users:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleAddUser = () => {
    setCurrentUser(null);
    setFormData({
      email: '',
      username: '',
      password: '',
      role: 'operator',
    });
    setOpenDialog(true);
  };

  const handleEditUser = (user: LocalUser) => {
    setCurrentUser(user);
    setFormData({
      email: user.email,
      username: user.username,
      password: '',
      role: user.role || 'operator',
    });
    setOpenDialog(true);
  };

  const handleDeleteUser = async (email: string) => {
    if (!window.confirm(`Are you sure you want to delete user ${email}?`)) {
      return;
    }

    try {
      await idpService.deleteLocalUser(email);
      setSuccess(`User ${email} deleted successfully`);
      await loadUsers();
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      setError(`Failed to delete user: ${err}`);
      setTimeout(() => setError(null), 5000);
    }
  };

  const handleSaveUser = async () => {
    setError(null);
    try {
      if (currentUser) {
        // Update existing user
        await idpService.updateLocalUser(currentUser.email, {
          ...formData,
          userID: currentUser.userID,
        });
        setSuccess(`User ${formData.email} updated successfully`);
      } else {
        // Add new user
        await idpService.addLocalUser({
          ...formData,
          userID: `user-${Date.now()}`, // Generate unique ID
        });
        setSuccess(`User ${formData.email} created successfully`);
      }
      
      setOpenDialog(false);
      await loadUsers();
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      setError(`Failed to save user: ${err}`);
      setTimeout(() => setError(null), 5000);
    }
  };

  const getRoleColor = (role?: string) => {
    switch (role) {
      case 'super-admin':
        return 'error';
      case 'vjailbreak-admin':
        return 'warning';
      case 'admin':
        return 'primary';
      case 'operator':
        return 'info';
      case 'viewer':
        return 'default';
      default:
        return 'default';
    }
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box sx={{ p: 3 }}>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Box>
          <Typography variant="h4" gutterBottom>
            User Management
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Manage local users and their roles. Only accessible to vjailbreak-admin users.
          </Typography>
        </Box>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={handleAddUser}
        >
          Add User
        </Button>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>
          {success}
        </Alert>
      )}

      <Card>
        <CardContent>
          <TableContainer component={Paper} variant="outlined">
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Email</TableCell>
                  <TableCell>Username</TableCell>
                  <TableCell>User ID</TableCell>
                  <TableCell>Role</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {users.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} align="center">
                      <Typography color="text.secondary">
                        No users found. Add your first user to get started.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  users.map((user) => (
                    <TableRow key={user.email}>
                      <TableCell>{user.email}</TableCell>
                      <TableCell>{user.username}</TableCell>
                      <TableCell>
                        <Typography variant="caption" color="text.secondary">
                          {user.userID}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          label={user.role || 'operator'}
                          color={getRoleColor(user.role)}
                          size="small"
                        />
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          onClick={() => handleEditUser(user)}
                          color="primary"
                        >
                          <EditIcon fontSize="small" />
                        </IconButton>
                        <IconButton
                          size="small"
                          onClick={() => handleDeleteUser(user.email)}
                          color="error"
                          disabled={user.role === 'super-admin'}
                        >
                          <DeleteIcon fontSize="small" />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </CardContent>
      </Card>

      {/* Add/Edit User Dialog */}
      <Dialog open={openDialog} onClose={() => setOpenDialog(false)} maxWidth="sm" fullWidth>
        <DialogTitle>
          {currentUser ? 'Edit User' : 'Add New User'}
        </DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              label="Email"
              type="email"
              value={formData.email}
              onChange={(e) => setFormData({ ...formData, email: e.target.value })}
              fullWidth
              required
              disabled={!!currentUser}
            />
            
            <TextField
              label="Username"
              value={formData.username}
              onChange={(e) => setFormData({ ...formData, username: e.target.value })}
              fullWidth
              required
            />
            
            <TextField
              label={currentUser ? 'New Password (leave blank to keep current)' : 'Password'}
              type="password"
              value={formData.password}
              onChange={(e) => setFormData({ ...formData, password: e.target.value })}
              fullWidth
              required={!currentUser}
            />
            
            <FormControl fullWidth>
              <InputLabel>Role</InputLabel>
              <Select
                value={formData.role}
                label="Role"
                onChange={(e) => setFormData({ ...formData, role: e.target.value as any })}
              >
                <MenuItem value="super-admin">Super Admin (Full Access)</MenuItem>
                <MenuItem value="vjailbreak-admin">vJailbreak Admin (User Management)</MenuItem>
                <MenuItem value="admin">Admin</MenuItem>
                <MenuItem value="operator">Operator</MenuItem>
                <MenuItem value="viewer">Viewer (Read-only)</MenuItem>
              </Select>
            </FormControl>
            
            <Box sx={{ mt: 2, display: 'flex', gap: 2, justifyContent: 'flex-end' }}>
              <Button onClick={() => setOpenDialog(false)}>
                Cancel
              </Button>
              <Button
                variant="contained"
                onClick={handleSaveUser}
                disabled={!formData.email || !formData.username || (!currentUser && !formData.password)}
              >
                {currentUser ? 'Update' : 'Create'}
              </Button>
            </Box>
          </Box>
        </DialogContent>
      </Dialog>
    </Box>
  );
};

export default UserManagementPage;
