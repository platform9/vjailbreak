export interface AuthUser {
  email: string;
  name: string;
  groups: string[];
  sub: string;
}

export interface AuthToken {
  access_token: string;
  id_token: string;
  refresh_token?: string;
  token_type: string;
  expires_in: number;
}

export interface PasswordChangeRequest {
  username: string;
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

export interface AuthState {
  user: AuthUser | null;
  token: AuthToken | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  requirePasswordChange: boolean;
}

export interface DexUserInfo {
  email: string;
  email_verified: boolean;
  name: string;
  sub: string;
  groups?: string[];
}

export interface ServiceAccountToken {
  token: string;
  namespace: string;
  serviceAccount: string;
}
