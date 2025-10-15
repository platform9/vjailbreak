export type IdentityProviderType = 'saml' | 'oidc' | 'local' | 'ldap';

export interface SAMLConfig {
  ssoURL: string;
  caData?: string;
  entityIssuer: string;
  redirectURI: string;
  usernameAttr: string;
  emailAttr: string;
  groupsAttr: string;
  nameIDPolicyFormat: string;
  insecureSkipSignatureValidation?: boolean;
}

export interface OIDCConfig {
  issuer: string;
  clientID: string;
  clientSecret: string;
  redirectURI: string;
  scopes: string[];
  getUserInfo?: boolean;
  usernameClaim?: string;
  groupsClaim?: string;
  insecureSkipEmailVerified?: boolean;
  insecureEnableGroups?: boolean;
  hostedDomains?: string[];
}

export interface LocalConfig {
  users: LocalUser[];
}

export interface LocalUser {
  email: string;
  username: string;
  password?: string; // Only for creating/updating
  hash?: string; // Bcrypt hash (stored)
  userID: string;
  groups?: string[];
  role?: 'super-admin' | 'vjailbreak-admin' | 'admin' | 'operator' | 'viewer';
}

export interface IdentityProvider {
  id: string;
  type: IdentityProviderType;
  name: string;
  description?: string;
  enabled: boolean;
  config: SAMLConfig | OIDCConfig | LocalConfig;
}

export interface TestConnectionResult {
  success: boolean;
  message: string;
  details?: any;
}

export interface DexConfiguration {
  issuer: string;
  storage: {
    type: string;
    config: any;
  };
  web: {
    http: string;
    allowedOrigins: string[];
  };
  enablePasswordDB: boolean;
  staticPasswords?: LocalUser[];
  connectors?: any[];
  staticClients: Array<{
    id: string;
    redirectURIs: string[];
    name: string;
    secret?: string;
    public?: boolean;
  }>;
  oauth2?: {
    skipApprovalScreen: boolean;
    responseTypes: string[];
  };
}
