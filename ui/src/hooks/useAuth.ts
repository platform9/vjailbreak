import { useState, useEffect } from 'react';
import { authService } from '../api/auth/authService';
import { AuthUser } from '../api/auth/types';

export const useAuth = () => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    checkAuth();
  }, []);

  const checkAuth = async () => {
    try {
      const authenticated = await authService.checkAuth();
      setIsAuthenticated(authenticated);

      if (authenticated) {
        const userInfo = await authService.getUserInfo();
        if (userInfo) {
          setUser({
            email: userInfo.email,
            name: userInfo.name,
            sub: userInfo.sub,
            groups: userInfo.groups || [],
          });
        }
      }
    } catch (error) {
      console.error('Auth check failed:', error);
      setIsAuthenticated(false);
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  const logout = async () => {
    await authService.logout();
    setIsAuthenticated(false);
    setUser(null);
  };

  const hasRole = (role: string): boolean => {
    return user?.groups?.includes(`vjailbreak-${role}s`) || false;
  };

  const isAdmin = (): boolean => {
    return hasRole('admin');
  };

  const canManageCredentials = (): boolean => {
    return isAdmin() || hasRole('credential-manager');
  };

  const canCreateMigrations = (): boolean => {
    return isAdmin() || hasRole('operator');
  };

  return {
    user,
    isAuthenticated,
    isLoading,
    logout,
    hasRole,
    isAdmin,
    canManageCredentials,
    canCreateMigrations,
  };
};
