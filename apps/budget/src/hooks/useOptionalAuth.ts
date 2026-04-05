import { useAuth, type AuthContextValue } from '@mrsmith/auth-client';

const noAuth: AuthContextValue = {
  authenticated: true,
  token: undefined,
  user: null,
  login: () => {},
  logout: () => {},
  loading: false,
};

export function useOptionalAuth(): AuthContextValue {
  try {
    return useAuth();
  } catch {
    return noAuth;
  }
}
