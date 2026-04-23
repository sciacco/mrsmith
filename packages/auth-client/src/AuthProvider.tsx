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
import { DEVADMIN_ROLE } from './roles';

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
  /**
   * Forces a token refresh regardless of current validity.
   * Returns the new token, or undefined if the refresh token is no longer valid
   * (in which case callers should surface the error or trigger `login()`).
   */
  forceRefreshToken: () => Promise<string | undefined>;
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

// Dev-only: when VITE_DEV_AUTH_BYPASS="true" the provider skips Keycloak
// entirely and injects a fake authenticated user. Roles come from
// VITE_DEV_FAKE_ROLES (comma-separated) or default to [DEVADMIN_ROLE] which
// makes hasAnyRole() return true for every app.
const DEV_BYPASS_ENABLED =
  (import.meta.env.VITE_DEV_AUTH_BYPASS ?? '').toString().toLowerCase() === 'true';

function parseRolesEnv(raw: string | undefined): string[] {
  if (!raw) return [];
  return raw
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean);
}

function buildDevUser(): AuthUser {
  const envRoles = parseRolesEnv(import.meta.env.VITE_DEV_FAKE_ROLES);
  return {
    name: (import.meta.env.VITE_DEV_FAKE_NAME as string | undefined) ?? 'Dev User',
    email: (import.meta.env.VITE_DEV_FAKE_EMAIL as string | undefined) ?? 'dev@local',
    roles: envRoles.length > 0 ? envRoles : [DEVADMIN_ROLE],
  };
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

export function AuthProvider(props: AuthProviderProps) {
  if (DEV_BYPASS_ENABLED) {
    return <DevBypassAuthProvider>{props.children}</DevBypassAuthProvider>;
  }
  return <KeycloakAuthProvider {...props} />;
}

function DevBypassAuthProvider({ children }: { children: ReactNode }) {
  const [user] = useState<AuthUser>(() => buildDevUser());
  const warnedRef = useRef(false);

  useEffect(() => {
    if (warnedRef.current) return;
    warnedRef.current = true;
    // eslint-disable-next-line no-console
    console.warn(
      '[auth-client] VITE_DEV_AUTH_BYPASS=true — Keycloak is bypassed. Fake user:',
      user,
    );
  }, [user]);

  const value = useMemo<AuthContextValue>(() => {
    const token = 'dev-token';
    const noop = () => {
      // eslint-disable-next-line no-console
      console.warn('[auth-client] login/logout invoked while dev bypass is active — ignored.');
    };
    return {
      status: 'authenticated',
      authenticated: true,
      token,
      user,
      login: noop,
      logout: noop,
      loading: false,
      getAccessToken: async () => token,
      forceRefreshToken: async () => token,
    };
  }, [user]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

function KeycloakAuthProvider({ keycloakUrl, realm, clientId, children }: AuthProviderProps) {
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
  const pendingRefreshRef = useRef<Promise<string | undefined> | null>(null);

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

  const refreshToken = useCallback(
    async ({
      minValidity,
      force = false,
      loginOnFailure = false,
    }: {
      minValidity: number;
      force?: boolean;
      loginOnFailure?: boolean;
    }): Promise<string | undefined> => {
      if (!keycloak.authenticated) return undefined;
      const currentToken = keycloak.token;
      if (!force && currentToken && getTokenValiditySeconds(keycloak.tokenParsed) > minValidity) {
        return currentToken;
      }
      if (pendingRefreshRef.current) {
        if (!loginOnFailure) return pendingRefreshRef.current;
        return pendingRefreshRef.current.then((token) => {
          if (token) return token;
          if (keycloak.authenticated && !loginTriggeredRef.current) {
            login();
          }
          return undefined;
        });
      }

      pendingRefreshRef.current = (async () => {
        try {
          await keycloak.updateToken(force ? -1 : minValidity);
          loginTriggeredRef.current = false;
          syncAuthState();
          return keycloak.token;
        } catch {
          if (loginOnFailure && keycloak.authenticated && !loginTriggeredRef.current) {
            login();
          }
          return undefined;
        } finally {
          pendingRefreshRef.current = null;
        }
      })();

      return pendingRefreshRef.current;
    },
    [keycloak, login, syncAuthState],
  );

  const getAccessToken = useCallback(
    (minValidity = 30) =>
      refreshToken({
        minValidity,
      }),
    [refreshToken],
  );

  const forceRefreshToken = useCallback(
    () =>
      refreshToken({
        minValidity: 0,
        force: true,
        loginOnFailure: true,
      }),
    [refreshToken],
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
      // Refresh token no longer valid — redirect to the Keycloak login page.
      // (Silent SSO via prompt=none was attempted but caused a redirect loop
      // when the refresh grant kept failing while SSO login succeeded.)
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
      if (loginTriggeredRef.current) return;
      void getAccessToken(30);
    }, 30_000);

    // Tabs are throttled while hidden and skipped entirely while the device is
    // asleep. Refresh on visibility return so the next user action doesn't 401.
    const onVisibilityChange = () => {
      if (document.visibilityState !== 'visible') return;
      if (!keycloak.authenticated) return;
      // Skip if a login redirect is already in flight to avoid refresh storms.
      if (loginTriggeredRef.current) return;
      void getAccessToken(60);
    };
    document.addEventListener('visibilitychange', onVisibilityChange);
    window.addEventListener('focus', onVisibilityChange);

    return () => {
      clearInterval(interval);
      document.removeEventListener('visibilitychange', onVisibilityChange);
      window.removeEventListener('focus', onVisibilityChange);
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
      forceRefreshToken,
    }),
    [authState, forceRefreshToken, getAccessToken, login, logout],
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
