import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import Keycloak from 'keycloak-js';

export type AuthStatus = 'loading' | 'authenticated' | 'reauthenticating' | 'unauthenticated';

type AuthUser = { name: string; email: string; roles: string[] };

export interface AuthContextValue {
  status: AuthStatus;
  authenticated: boolean;
  token: string | undefined;
  user: AuthUser | null;
  login: () => void;
  logout: () => void;
  loading: boolean;
  getAccessToken: (minValidity?: number) => Promise<string | undefined>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

interface AuthProviderProps {
  keycloakUrl: string;
  realm: string;
  clientId: string;
  children: ReactNode;
}

interface AuthState {
  status: AuthStatus;
  authenticated: boolean;
  token: string | undefined;
  user: AuthUser | null;
}

function sameUser(left: AuthUser | null, right: AuthUser | null): boolean {
  if (left === right) return true;
  if (!left || !right) return false;
  if (left.name !== right.name || left.email !== right.email) return false;
  if (left.roles.length !== right.roles.length) return false;
  return left.roles.every((role, index) => role === right.roles[index]);
}

function getTokenValiditySeconds(tokenParsed: Keycloak['tokenParsed']): number {
  const exp = tokenParsed?.exp;
  if (!exp) return 0;
  return exp - Math.ceil(Date.now() / 1000);
}

function getUser(tokenParsed: Keycloak['tokenParsed']): AuthUser | null {
  if (!tokenParsed) return null;
  return {
    name: tokenParsed.preferred_username ?? '',
    email: tokenParsed.email ?? '',
    roles: tokenParsed.realm_access?.roles ?? [],
  };
}

export function AuthProvider({ keycloakUrl, realm, clientId, children }: AuthProviderProps) {
  const [keycloak] = useState(
    () =>
      new Keycloak({
        url: keycloakUrl,
        realm,
        clientId,
      }),
  );
  const [authState, setAuthState] = useState<AuthState>({
    status: 'loading',
    authenticated: false,
    token: undefined,
    user: null,
  });
  const loginTriggeredRef = useRef(false);

  const syncAuthState = useCallback(
    (nextStatus?: AuthStatus) => {
      const authenticated = Boolean(keycloak.authenticated && keycloak.token);
      const user = authenticated ? getUser(keycloak.tokenParsed) : null;
      const status = nextStatus ?? (authenticated ? 'authenticated' : 'unauthenticated');
      setAuthState((current) => {
        if (
          current.status === status &&
          current.authenticated === authenticated &&
          current.token === keycloak.token &&
          sameUser(current.user, user)
        ) {
          return current;
        }
        return {
          status,
          authenticated,
          token: keycloak.token,
          user,
        };
      });
    },
    [keycloak],
  );

  const login = useCallback(() => {
    if (loginTriggeredRef.current) return;
    loginTriggeredRef.current = true;
    setAuthState((current) => ({
      ...current,
      status: 'reauthenticating',
      authenticated: false,
      token: undefined,
    }));
    void keycloak.login().catch(() => {
      loginTriggeredRef.current = false;
      syncAuthState('unauthenticated');
    });
  }, [keycloak, syncAuthState]);

  const logout = useCallback(() => {
    loginTriggeredRef.current = false;
    setAuthState({
      status: 'unauthenticated',
      authenticated: false,
      token: undefined,
      user: null,
    });
    void keycloak.logout();
  }, [keycloak]);

  const getAccessToken = useCallback(
    async (minValidity = 30) => {
      if (!keycloak.authenticated) return undefined;
      const token = keycloak.token;
      if (token && getTokenValiditySeconds(keycloak.tokenParsed) > minValidity) {
        return token;
      }
      try {
        await keycloak.updateToken(minValidity);
        loginTriggeredRef.current = false;
        syncAuthState();
        return keycloak.token;
      } catch {
        return undefined;
      }
    },
    [keycloak, syncAuthState],
  );

  useEffect(() => {
    keycloak.onAuthSuccess = () => {
      loginTriggeredRef.current = false;
      syncAuthState();
    };
    keycloak.onAuthRefreshSuccess = () => {
      loginTriggeredRef.current = false;
      syncAuthState();
    };
    keycloak.onAuthLogout = () => {
      loginTriggeredRef.current = false;
      syncAuthState('unauthenticated');
    };
    keycloak.onAuthRefreshError = () => {
      login();
    };
    keycloak.onTokenExpired = () => {
      void getAccessToken(0);
    };

    keycloak
      .init({ onLoad: 'login-required', checkLoginIframe: false })
      .then((auth) => {
        loginTriggeredRef.current = false;
        syncAuthState(auth ? 'authenticated' : 'unauthenticated');
      })
      .catch(() => {
        loginTriggeredRef.current = false;
        syncAuthState('unauthenticated');
      });

    const interval = setInterval(() => {
      if (!keycloak.authenticated) return;
      void getAccessToken(30);
    }, 30_000);

    return () => {
      clearInterval(interval);
      keycloak.onAuthSuccess = undefined;
      keycloak.onAuthRefreshSuccess = undefined;
      keycloak.onAuthLogout = undefined;
      keycloak.onAuthRefreshError = undefined;
      keycloak.onTokenExpired = undefined;
    };
  }, [getAccessToken, keycloak, login, syncAuthState]);

  const value = useMemo<AuthContextValue>(
    () => ({
      status: authState.status,
      authenticated: authState.authenticated,
      token: authState.token,
      user: authState.user,
      login,
      logout,
      loading: authState.status === 'loading',
      getAccessToken,
    }),
    [authState, getAccessToken, login, logout],
  );

  return (
    <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within <AuthProvider>');
  return ctx;
}
