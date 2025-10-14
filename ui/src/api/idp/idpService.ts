import axios from 'axios';
import { authService } from '../auth/authService';
import {
  IdentityProvider,
  TestConnectionResult,
  DexConfiguration,
  SAMLConfig,
  OIDCConfig,
  LocalConfig,
  LocalUser,
} from './types';

const API_BASE = import.meta.env.VITE_API_BASE || '/api/v1';

class IDPService {
  /**
   * Get authenticated axios client
   */
  private getClient() {
    return authService.createAuthenticatedClient();
  }

  /**
   * List all configured identity providers
   */
  async listProviders(): Promise<IdentityProvider[]> {
    try {
      const response = await this.getClient().get(`${API_BASE}/idp/providers`);
      return response.data.providers || [];
    } catch (error: any) {
      console.error('Failed to list providers:', error);
      throw new Error(error.response?.data?.error || 'Failed to list identity providers');
    }
  }

  /**
   * Get a specific identity provider by ID
   */
  async getProvider(id: string): Promise<IdentityProvider> {
    try {
      const response = await this.getClient().get(`${API_BASE}/idp/providers/${id}`);
      return response.data;
    } catch (error: any) {
      console.error(`Failed to get provider ${id}:`, error);
      throw new Error(error.response?.data?.error || 'Failed to get identity provider');
    }
  }

  /**
   * Create a new identity provider
   */
  async createProvider(provider: IdentityProvider): Promise<IdentityProvider> {
    try {
      const response = await this.getClient().post(`${API_BASE}/idp/providers`, provider);
      return response.data;
    } catch (error: any) {
      console.error('Failed to create provider:', error);
      throw new Error(error.response?.data?.error || 'Failed to create identity provider');
    }
  }

  /**
   * Update an existing identity provider
   */
  async updateProvider(id: string, provider: Partial<IdentityProvider>): Promise<IdentityProvider> {
    try {
      const response = await this.getClient().put(`${API_BASE}/idp/providers/${id}`, provider);
      return response.data;
    } catch (error: any) {
      console.error(`Failed to update provider ${id}:`, error);
      throw new Error(error.response?.data?.error || 'Failed to update identity provider');
    }
  }

  /**
   * Delete an identity provider
   */
  async deleteProvider(id: string): Promise<void> {
    try {
      await this.getClient().delete(`${API_BASE}/idp/providers/${id}`);
    } catch (error: any) {
      console.error(`Failed to delete provider ${id}:`, error);
      throw new Error(error.response?.data?.error || 'Failed to delete identity provider');
    }
  }

  /**
   * Test connection to an identity provider
   */
  async testProvider(id: string): Promise<TestConnectionResult> {
    try {
      const response = await this.getClient().post(`${API_BASE}/idp/providers/${id}/test`);
      return response.data;
    } catch (error: any) {
      console.error(`Failed to test provider ${id}:`, error);
      return {
        success: false,
        message: error.response?.data?.error || 'Connection test failed',
        details: error.response?.data,
      };
    }
  }

  /**
   * Get the full Dex configuration (for advanced users)
   */
  async getDexConfig(): Promise<DexConfiguration> {
    try {
      const response = await this.getClient().get(`${API_BASE}/idp/dex/config`);
      return response.data;
    } catch (error: any) {
      console.error('Failed to get Dex config:', error);
      throw new Error(error.response?.data?.error || 'Failed to get Dex configuration');
    }
  }

  /**
   * Update the full Dex configuration (for advanced users)
   */
  async updateDexConfig(config: DexConfiguration): Promise<void> {
    try {
      await this.getClient().put(`${API_BASE}/idp/dex/config`, config);
    } catch (error: any) {
      console.error('Failed to update Dex config:', error);
      throw new Error(error.response?.data?.error || 'Failed to update Dex configuration');
    }
  }

  /**
   * Restart Dex pod to apply configuration changes
   */
  async restartDex(): Promise<void> {
    try {
      await this.getClient().post(`${API_BASE}/idp/dex/restart`);
    } catch (error: any) {
      console.error('Failed to restart Dex:', error);
      throw new Error(error.response?.data?.error || 'Failed to restart Dex');
    }
  }

  /**
   * Add a local user (static password)
   */
  async addLocalUser(user: LocalUser): Promise<void> {
    try {
      await this.getClient().post(`${API_BASE}/idp/local/users`, user);
    } catch (error: any) {
      console.error('Failed to add local user:', error);
      throw new Error(error.response?.data?.error || 'Failed to add local user');
    }
  }

  /**
   * Update a local user
   */
  async updateLocalUser(email: string, user: Partial<LocalUser>): Promise<void> {
    try {
      await this.getClient().put(`${API_BASE}/idp/local/users/${encodeURIComponent(email)}`, user);
    } catch (error: any) {
      console.error(`Failed to update user ${email}:`, error);
      throw new Error(error.response?.data?.error || 'Failed to update local user');
    }
  }

  /**
   * Delete a local user
   */
  async deleteLocalUser(email: string): Promise<void> {
    try {
      await this.getClient().delete(`${API_BASE}/idp/local/users/${encodeURIComponent(email)}`);
    } catch (error: any) {
      console.error(`Failed to delete user ${email}:`, error);
      throw new Error(error.response?.data?.error || 'Failed to delete local user');
    }
  }

  /**
   * List all local users
   */
  async listLocalUsers(): Promise<LocalUser[]> {
    try {
      const response = await this.getClient().get(`${API_BASE}/idp/local/users`);
      return response.data.users || [];
    } catch (error: any) {
      console.error('Failed to list local users:', error);
      throw new Error(error.response?.data?.error || 'Failed to list local users');
    }
  }

  /**
   * Convert provider to Dex connector format
   */
  providerToConnector(provider: IdentityProvider): any {
    switch (provider.type) {
      case 'saml':
        const samlConfig = provider.config as SAMLConfig;
        return {
          type: 'saml',
          id: provider.id,
          name: provider.name,
          config: {
            ssoURL: samlConfig.ssoURL,
            ca: samlConfig.caData,
            redirectURI: samlConfig.redirectURI,
            entityIssuer: samlConfig.entityIssuer,
            usernameAttr: samlConfig.usernameAttr,
            emailAttr: samlConfig.emailAttr,
            groupsAttr: samlConfig.groupsAttr,
            nameIDPolicyFormat: samlConfig.nameIDPolicyFormat,
            insecureSkipSignatureValidation: samlConfig.insecureSkipSignatureValidation,
          },
        };

      case 'oidc':
        const oidcConfig = provider.config as OIDCConfig;
        return {
          type: 'oidc',
          id: provider.id,
          name: provider.name,
          config: {
            issuer: oidcConfig.issuer,
            clientID: oidcConfig.clientID,
            clientSecret: oidcConfig.clientSecret,
            redirectURI: oidcConfig.redirectURI,
            scopes: oidcConfig.scopes,
            getUserInfo: oidcConfig.getUserInfo,
            usernameClaim: oidcConfig.usernameClaim,
            groupsClaim: oidcConfig.groupsClaim,
            insecureSkipEmailVerified: oidcConfig.insecureSkipEmailVerified,
            insecureEnableGroups: oidcConfig.insecureEnableGroups,
            hostedDomains: oidcConfig.hostedDomains,
          },
        };

      default:
        return null;
    }
  }
}

export const idpService = new IDPService();
export default idpService;
