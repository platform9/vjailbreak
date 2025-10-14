import { useState } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Container,
  TextField,
  Typography,
  Alert,
  IconButton,
  InputAdornment,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
} from '@mui/material';
import { styled } from '@mui/material/styles';
import { Visibility, VisibilityOff, Check, Close } from '@mui/icons-material';
import { authService } from '../../api/auth/authService';
import { PasswordChangeRequest } from '../../api/auth/types';

const PasswordContainer = styled(Container)(({ theme }) => ({
  minHeight: '100vh',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  background: `linear-gradient(135deg, ${theme.palette.primary.dark} 0%, ${theme.palette.primary.main} 100%)`,
}));

const PasswordCard = styled(Card)(({ theme }) => ({
  maxWidth: 500,
  width: '100%',
  padding: theme.spacing(3),
  borderRadius: theme.spacing(2),
  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.1)',
}));

interface PasswordStrength {
  hasMinLength: boolean;
  hasUpperCase: boolean;
  hasLowerCase: boolean;
  hasNumber: boolean;
  hasSpecialChar: boolean;
}

const ChangePasswordPage = () => {
  const [formData, setFormData] = useState<PasswordChangeRequest>({
    username: authService.getCurrentUsername() || '',
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  });
  const [showPasswords, setShowPasswords] = useState({
    current: false,
    new: false,
    confirm: false,
  });
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const validatePassword = (password: string): PasswordStrength => {
    return {
      hasMinLength: password.length >= 12,
      hasUpperCase: /[A-Z]/.test(password),
      hasLowerCase: /[a-z]/.test(password),
      hasNumber: /\d/.test(password),
      hasSpecialChar: /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(password),
    };
  };

  const strength = validatePassword(formData.newPassword);
  const isPasswordValid = Object.values(strength).every(Boolean);
  const passwordsMatch = formData.newPassword === formData.confirmPassword;

  const handleChange = (field: keyof PasswordChangeRequest) => (
    event: React.ChangeEvent<HTMLInputElement>
  ) => {
    setFormData({ ...formData, [field]: event.target.value });
    setError(null);
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

    if (!isPasswordValid) {
      setError('Password does not meet requirements');
      return;
    }

    if (!passwordsMatch) {
      setError('Passwords do not match');
      return;
    }

    setIsSubmitting(true);

    try {
      await authService.changePassword(formData);
      setSuccess(true);
      
      // Redirect to dashboard after 2 seconds
      setTimeout(() => {
        window.location.href = '/dashboard/migrations';
      }, 2000);
    } catch (err: any) {
      setError(err.message || 'Failed to change password');
    } finally {
      setIsSubmitting(false);
    }
  };

  const togglePasswordVisibility = (field: 'current' | 'new' | 'confirm') => {
    setShowPasswords({ ...showPasswords, [field]: !showPasswords[field] });
  };

  const PasswordRequirement = ({ met, text }: { met: boolean; text: string }) => (
    <ListItem dense>
      <ListItemIcon sx={{ minWidth: 36 }}>
        {met ? <Check color="success" fontSize="small" /> : <Close color="error" fontSize="small" />}
      </ListItemIcon>
      <ListItemText 
        primary={text} 
        primaryTypographyProps={{ 
          variant: 'body2',
          color: met ? 'success.main' : 'text.secondary' 
        }} 
      />
    </ListItem>
  );

  if (success) {
    return (
      <PasswordContainer maxWidth={false}>
        <PasswordCard>
          <CardContent>
            <Alert severity="success" sx={{ mb: 2 }}>
              Password changed successfully! Redirecting to dashboard...
            </Alert>
          </CardContent>
        </PasswordCard>
      </PasswordContainer>
    );
  }

  return (
    <PasswordContainer maxWidth={false}>
      <PasswordCard>
        <CardContent>
          <Box textAlign="center" mb={4}>
            <Typography variant="h5" component="h1" gutterBottom fontWeight="bold">
              Change Password
            </Typography>
            <Typography variant="body2" color="text.secondary">
              You must change your password before continuing
            </Typography>
          </Box>

          {error && (
            <Alert severity="error" sx={{ mb: 3 }}>
              {error}
            </Alert>
          )}

          <form onSubmit={handleSubmit}>
            <TextField
              fullWidth
              label="Username"
              value={formData.username}
              disabled
              margin="normal"
              variant="outlined"
            />

            <TextField
              fullWidth
              label="Current Password"
              type={showPasswords.current ? 'text' : 'password'}
              value={formData.currentPassword}
              onChange={handleChange('currentPassword')}
              margin="normal"
              required
              InputProps={{
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      onClick={() => togglePasswordVisibility('current')}
                      edge="end"
                    >
                      {showPasswords.current ? <VisibilityOff /> : <Visibility />}
                    </IconButton>
                  </InputAdornment>
                ),
              }}
            />

            <TextField
              fullWidth
              label="New Password"
              type={showPasswords.new ? 'text' : 'password'}
              value={formData.newPassword}
              onChange={handleChange('newPassword')}
              margin="normal"
              required
              InputProps={{
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      onClick={() => togglePasswordVisibility('new')}
                      edge="end"
                    >
                      {showPasswords.new ? <VisibilityOff /> : <Visibility />}
                    </IconButton>
                  </InputAdornment>
                ),
              }}
            />

            <TextField
              fullWidth
              label="Confirm New Password"
              type={showPasswords.confirm ? 'text' : 'password'}
              value={formData.confirmPassword}
              onChange={handleChange('confirmPassword')}
              margin="normal"
              required
              error={formData.confirmPassword.length > 0 && !passwordsMatch}
              helperText={
                formData.confirmPassword.length > 0 && !passwordsMatch
                  ? 'Passwords do not match'
                  : ''
              }
              InputProps={{
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      onClick={() => togglePasswordVisibility('confirm')}
                      edge="end"
                    >
                      {showPasswords.confirm ? <VisibilityOff /> : <Visibility />}
                    </IconButton>
                  </InputAdornment>
                ),
              }}
            />

            <Box mt={3} mb={2}>
              <Typography variant="subtitle2" gutterBottom>
                Password Requirements:
              </Typography>
              <List dense>
                <PasswordRequirement met={strength.hasMinLength} text="At least 12 characters" />
                <PasswordRequirement met={strength.hasUpperCase} text="One uppercase letter" />
                <PasswordRequirement met={strength.hasLowerCase} text="One lowercase letter" />
                <PasswordRequirement met={strength.hasNumber} text="One number" />
                <PasswordRequirement met={strength.hasSpecialChar} text="One special character" />
              </List>
            </Box>

            <Button
              type="submit"
              variant="contained"
              fullWidth
              size="large"
              disabled={!isPasswordValid || !passwordsMatch || isSubmitting}
              sx={{
                py: 1.5,
                fontSize: '1rem',
                textTransform: 'none',
                borderRadius: 2,
              }}
            >
              {isSubmitting ? 'Changing Password...' : 'Change Password'}
            </Button>
          </form>
        </CardContent>
      </PasswordCard>
    </PasswordContainer>
  );
};

export default ChangePasswordPage;
