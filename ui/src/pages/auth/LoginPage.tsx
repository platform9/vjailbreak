import { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Container,
  Typography,
  CircularProgress,
  Alert,
} from '@mui/material';
import { styled } from '@mui/material/styles';
import { authService } from '../../api/auth/authService';

const LoginContainer = styled(Container)(({ theme }) => ({
  minHeight: '100vh',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  background: `linear-gradient(135deg, ${theme.palette.primary.dark} 0%, ${theme.palette.primary.main} 100%)`,
}));

const LoginCard = styled(Card)(({ theme }) => ({
  maxWidth: 450,
  width: '100%',
  padding: theme.spacing(3),
  borderRadius: theme.spacing(2),
  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.1)',
}));

const Logo = styled('img')({
  width: 200,
  marginBottom: 24,
  display: 'block',
  marginLeft: 'auto',
  marginRight: 'auto',
});

const LoginPage = () => {
  const [isChecking, setIsChecking] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    checkAuthStatus();
  }, []);

  const checkAuthStatus = async () => {
    try {
      const isAuthenticated = await authService.checkAuth();
      if (isAuthenticated) {
        // User is already authenticated, redirect to dashboard
        const returnUrl = new URLSearchParams(window.location.search).get('rd') || '/dashboard/migrations';
        window.location.href = returnUrl;
      } else {
        setIsChecking(false);
      }
    } catch (err) {
      console.error('Auth check failed:', err);
      setIsChecking(false);
    }
  };

  const handleLogin = () => {
    try {
      const returnUrl = new URLSearchParams(window.location.search).get('rd') || '/dashboard/migrations';
      authService.initiateLogin(returnUrl);
    } catch (err: any) {
      setError(err.message || 'Failed to initiate login');
    }
  };

  if (isChecking) {
    return (
      <LoginContainer maxWidth={false}>
        <Box textAlign="center">
          <CircularProgress size={60} />
          <Typography variant="h6" color="white" sx={{ mt: 2 }}>
            Checking authentication...
          </Typography>
        </Box>
      </LoginContainer>
    );
  }

  return (
    <LoginContainer maxWidth={false}>
      <LoginCard>
        <CardContent>
          <Box textAlign="center" mb={4}>
            <Typography variant="h4" component="h1" gutterBottom fontWeight="bold">
              vJailbreak
            </Typography>
            <Typography variant="subtitle1" color="text.secondary">
              VMware to OpenStack Migration Platform
            </Typography>
          </Box>

          {error && (
            <Alert severity="error" sx={{ mb: 3 }}>
              {error}
            </Alert>
          )}

          <Box mb={3}>
            <Typography variant="body2" color="text.secondary" textAlign="center">
              Sign in to access the migration platform
            </Typography>
          </Box>

          <Button
            variant="contained"
            fullWidth
            size="large"
            onClick={handleLogin}
            sx={{
              py: 1.5,
              fontSize: '1rem',
              textTransform: 'none',
              borderRadius: 2,
            }}
          >
            Sign In with vJailbreak
          </Button>

          <Box mt={4} pt={3} borderTop="1px solid rgba(0,0,0,0.1)">
            <Typography variant="caption" color="text.secondary" textAlign="center" display="block">
              Default credentials: admin@vjailbreak.local / admin
            </Typography>
            <Typography variant="caption" color="error" textAlign="center" display="block" mt={1}>
              You will be required to change the password on first login
            </Typography>
          </Box>
        </CardContent>
      </LoginCard>
    </LoginContainer>
  );
};

export default LoginPage;
