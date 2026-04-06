import { useAuth, type AuthContextValue } from '@mrsmith/auth-client';

const noAuth: AuthContextValue = {
  status: 'authenticated',
  authenticated: true,
  token: undefined,
  user: null,
  login: () => {},
  logout: () => {},
  loading: false,
  getAccessToken: async () => undefined,
};

export function useOptionalAuth(): AuthContextValue {
  try {
    return useAuth();
  } catch {
    return noAuth;
  }
}
