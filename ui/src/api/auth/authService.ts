import axios from 'axios';
import { AuthToken, DexUserInfo, PasswordChangeRequest, ServiceAccountToken } from './types';

const DEX_ISSUER = import.meta.env.VITE_DEX_ISSUER || 'http://localhost:5556/dex';
const OAUTH2_PROXY_URL = import.meta.env.VITE_OAUTH2_PROXY_URL || '/oauth2';

class AuthService {
  private readonly DEX_API_BASE = DEX_ISSUER;
  private readonly OAUTH2_BASE = OAUTH2_PROXY_URL;

  /**
   * Check if user is authenticated via OAuth2 Proxy
   */
  async checkAuth(): Promise<boolean> {
    try {
      const response = await axios.get(`${this.OAUTH2_BASE}/auth`, {
        withCredentials: true,
      });
      return response.status === 202 || response.status === 200;
    } catch (error) {
      return false;
    }
  }

  /**
   * Get user info from OAuth2 Proxy headers
   */
  async getUserInfo(): Promise<DexUserInfo | null> {
    try {
      const response = await axios.get(`${this.OAUTH2_BASE}/userinfo`, {
        withCredentials: true,
      });
      return response.data;
    } catch (error) {
      console.error('Failed to get user info:', error);
      return null;
    }
  }

  /**
   * Initiate login by redirecting to OAuth2 Proxy
   */
  initiateLogin(returnUrl?: string): void {
    const redirectUrl = returnUrl || window.location.pathname;
    window.location.href = `${this.OAUTH2_BASE}/start?rd=${encodeURIComponent(redirectUrl)}`;
  }

  /**
   * Logout from OAuth2 Proxy and Dex
   */
  async logout(): Promise<void> {
    try {
      // Clear local storage
      this.clearTokens();
      
      // Build full HTTPS URL for redirect to avoid HTTP redirect issues
      const protocol = window.location.protocol; // https:
      const host = window.location.host; // 10.9.2.145 or hostname
      const loginUrl = `${protocol}//${host}/login`;
      
      // Redirect to OAuth2 Proxy sign out with full URL
      window.location.href = `${this.OAUTH2_BASE}/sign_out?rd=${encodeURIComponent(loginUrl)}`;
    } catch (error) {
      console.error('Logout failed:', error);
      throw error;
    }
  }

  /**
   * Get access token from headers (set by OAuth2 Proxy)
   */
  getAccessToken(): string | null {
    return localStorage.getItem('access_token');
  }

  /**
   * Store tokens in local storage
   */
  storeTokens(token: AuthToken): void {
    localStorage.setItem('access_token', token.access_token);
    if (token.id_token) {
      localStorage.setItem('id_token', token.id_token);
    }
    if (token.refresh_token) {
      localStorage.setItem('refresh_token', token.refresh_token);
    }
  }

  /**
   * Clear tokens from local storage
   */
  clearTokens(): void {
    localStorage.removeItem('access_token');
    localStorage.removeItem('id_token');
    localStorage.removeItem('refresh_token');
  }

  /**
   * Change user password via Dex local connector API
   */
  async changePassword(request: PasswordChangeRequest): Promise<void> {
    try {
      // Dex local connector password change endpoint
      const response = await axios.post(
        `${this.DEX_API_BASE}/local/password`,
        {
          username: request.username,
          password: request.currentPassword,
          new_password: request.newPassword,
        },
        {
          headers: {
            'Content-Type': 'application/json',
          },
        }
      );

      if (response.status !== 200 && response.status !== 204) {
        throw new Error('Password change failed');
      }

      // Mark that password has been changed
      localStorage.setItem('password_changed', 'true');
    } catch (error: any) {
      console.error('Password change failed:', error);
      throw new Error(error.response?.data?.error || 'Failed to change password');
    }
  }

  /**
   * Check if this is first login (password needs to be changed)
   */
  requiresPasswordChange(): boolean {
    const passwordChanged = localStorage.getItem('password_changed');
    const username = this.getCurrentUsername();
    
    // If username is admin@vjailbreak.local and password hasn't been changed, require change
    return username === 'admin@vjailbreak.local' && passwordChanged !== 'true';
  }

  /**
   * Get current username from token
   */
  getCurrentUsername(): string | null {
    try {
      const idToken = localStorage.getItem('id_token');
      if (!idToken) return null;

      // Decode JWT token (simple base64 decode - not validating signature here)
      const payload = JSON.parse(atob(idToken.split('.')[1]));
      return payload.email || payload.preferred_username || null;
    } catch (error) {
      console.error('Failed to decode token:', error);
      return null;
    }
  }

  /**
   * Get user groups from token
   */
  getUserGroups(): string[] {
    try {
      const idToken = localStorage.getItem('id_token');
      if (!idToken) return [];

      const payload = JSON.parse(atob(idToken.split('.')[1]));
      return payload.groups || [];
    } catch (error) {
      console.error('Failed to get user groups:', error);
      return [];
    }
  }

  /**
   * Check if user has specific role
   */
  hasRole(role: string): boolean {
    const groups = this.getUserGroups();
    return groups.includes(`vjailbreak-${role}s`);
  }

  /**
   * Check if user is admin
   */
  isAdmin(): boolean {
    return this.hasRole('admin');
  }

  /**
   * Check if user can manage credentials
   */
  canManageCredentials(): boolean {
    return this.isAdmin() || this.hasRole('credential-manager');
  }

  /**
   * Check if user can create migrations
   */
  canCreateMigrations(): boolean {
    return this.isAdmin() || this.hasRole('operator');
  }

  /**
   * Get service account token for API calls
   */
  async getServiceAccountToken(): Promise<ServiceAccountToken | null> {
    try {
      // The token is mounted in the pod at this location
      const response = await axios.get('/var/run/secrets/kubernetes.io/serviceaccount/token');
      const namespace = await axios.get('/var/run/secrets/kubernetes.io/serviceaccount/namespace');
      
      return {
        token: response.data,
        namespace: namespace.data,
        serviceAccount: 'vjailbreak-ui-sa',
      };
    } catch (error) {
      console.error('Failed to get service account token:', error);
      return null;
    }
  }

  /**
   * Create authenticated axios instance with both user token and SA token
   */
  createAuthenticatedClient() {
    const accessToken = this.getAccessToken();
    
    return axios.create({
      headers: {
        'Authorization': `Bearer ${accessToken}`,
        'X-Auth-Request-Access-Token': accessToken || '',
      },
      withCredentials: true,
    });
  }
}

export const authService = new AuthService();
export default authService;
