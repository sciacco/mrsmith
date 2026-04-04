import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import Keycloak from 'keycloak-js';

export interface AuthContextValue {
  authenticated: boolean;
  token: string | undefined;
  user: { name: string; email: string; roles: string[] } | null;
  login: () => void;
  logout: () => void;
  loading: boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

interface AuthProviderProps {
  keycloakUrl: string;
  realm: string;
  clientId: string;
  children: ReactNode;
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
  const [loading, setLoading] = useState(true);
  const [authenticated, setAuthenticated] = useState(false);

  useEffect(() => {
    keycloak
      .init({ onLoad: 'login-required', checkLoginIframe: false })
      .then((auth) => {
        setAuthenticated(auth);
        setLoading(false);
      })
      .catch(() => setLoading(false));

    // Token refresh
    const interval = setInterval(() => {
      keycloak.updateToken(30).catch(() => keycloak.login());
    }, 30_000);

    return () => clearInterval(interval);
  }, [keycloak]);

  const user = authenticated
    ? {
        name: keycloak.tokenParsed?.preferred_username ?? '',
        email: keycloak.tokenParsed?.email ?? '',
        roles: keycloak.tokenParsed?.realm_access?.roles ?? [],
      }
    : null;

  return (
    <AuthContext.Provider
      value={{
        authenticated,
        token: keycloak.token,
        user,
        login: () => keycloak.login(),
        logout: () => keycloak.logout(),
        loading,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within <AuthProvider>');
  return ctx;
}
