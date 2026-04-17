import { useAuth, type AuthContextValue } from '@mrsmith/auth-client';

const noAuth: AuthContextValue = {
  status: 'unauthenticated',
  authenticated: false,
  token: undefined,
  user: null,
  login: () => {},
  logout: () => {},
  loading: false,
  getAccessToken: async () => undefined,
  forceRefreshToken: async () => undefined,
};

export function useOptionalAuth(): AuthContextValue {
  try {
    return useAuth();
  } catch {
    return noAuth;
  }
}
