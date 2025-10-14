import { useEffect, useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { Box, CircularProgress, Typography } from '@mui/material';
import { authService } from '../api/auth/authService';

interface AuthGuardProps {
  children: React.ReactNode;
  requiredRole?: 'admin' | 'operator' | 'viewer' | 'credential-manager';
}

const AuthGuard = ({ children, requiredRole }: AuthGuardProps) => {
  const location = useLocation();
  const [isAuthenticated, setIsAuthenticated] = useState<boolean | null>(null);
  const [requiresPasswordChange, setRequiresPasswordChange] = useState(false);

  useEffect(() => {
    checkAuthentication();
  }, [location.pathname]);

  const checkAuthentication = async () => {
    try {
      const authenticated = await authService.checkAuth();
      setIsAuthenticated(authenticated);

      if (authenticated) {
        // Check if password change is required
        const needsChange = authService.requiresPasswordChange();
        setRequiresPasswordChange(needsChange);

        // Check role if required - for now, allow all authenticated users
        // TODO: Implement proper RBAC with group checking once Dex groups are configured
        if (requiredRole) {
          // Temporarily allow all authenticated users
          // In production, implement proper role checking
          console.log(`Role required: ${requiredRole} - allowing for now`);
        }
      }
    } catch (error) {
      console.error('Authentication check failed:', error);
      setIsAuthenticated(false);
    }
  };

  // Still checking authentication
  if (isAuthenticated === null) {
    return (
      <Box
        display="flex"
        flexDirection="column"
        alignItems="center"
        justifyContent="center"
        minHeight="100vh"
      >
        <CircularProgress size={60} />
        <Typography variant="h6" sx={{ mt: 2 }}>
          Verifying authentication...
        </Typography>
      </Box>
    );
  }

  // Not authenticated - redirect to login
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // Authenticated but requires password change
  if (requiresPasswordChange && location.pathname !== '/change-password') {
    return <Navigate to="/change-password" replace />;
  }

  // Authenticated and authorized
  return <>{children}</>;
};

export default AuthGuard;
