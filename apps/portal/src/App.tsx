import { createApiClient, ApiError } from '@mrsmith/api-client';
import { useAuth } from '@mrsmith/auth-client';
import { useEffect, useMemo, useState } from 'react';
import { Portal } from './components/Portal';
import type { Category, PortalUser } from './types';

export function App() {
  const { authenticated, loading, token, logout } = useAuth();
  const api = useMemo(
    () =>
      createApiClient({
        baseUrl: '/api',
        getToken: () => token,
      }),
    [token],
  );

  const [user, setUser] = useState<PortalUser | null>(null);
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [bootstrapping, setBootstrapping] = useState(true);

  useEffect(() => {
    if (loading) return;
    if (!authenticated || !token) {
      setBootstrapping(false);
      setUser(null);
      setCategories([]);
      return;
    }

    let cancelled = false;
    setBootstrapping(true);
    setError(null);

    Promise.all([
      api.get<PortalUser>('/portal/me'),
      api.get<{ categories: Category[] }>('/portal/apps'),
    ])
      .then(([nextUser, appsResponse]) => {
        if (cancelled) return;
        setUser(nextUser);
        setCategories(appsResponse.categories);
        setBootstrapping(false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
          setError('Your session is authenticated but not authorized to load launcher data.');
        } else {
          setError('The portal could not load your identity and app entitlements.');
        }
        setBootstrapping(false);
      });

    return () => {
      cancelled = true;
    };
  }, [api, authenticated, loading, token]);

  if (loading || bootstrapping) {
    return (
      <Portal
        categories={[]}
        userName="Agent Session"
        statusTitle="AUTHENTICATING SESSION"
        statusMessage="Connecting to Keycloak and loading your launcher entitlements."
      />
    );
  }

  if (!authenticated) {
    return (
      <Portal
        categories={[]}
        userName="Unauthenticated"
        statusTitle="ACCESS DENIED"
        statusTone="error"
        statusMessage="The portal requires a valid Keycloak session. Check the frontend auth configuration and retry."
      />
    );
  }

  if (error) {
    return (
      <Portal
        categories={[]}
        userName={user?.name ?? 'Authenticated User'}
        statusTitle="ENTITLEMENT LOAD FAILED"
        statusTone="error"
        statusMessage={error}
        onLogout={logout}
      />
    );
  }

  if (categories.length === 0) {
    return (
      <Portal
        categories={[]}
        userName={user?.name ?? 'Authenticated User'}
        statusTitle="NO APPLICATIONS ASSIGNED"
        statusMessage="Your account is authenticated, but no launcher roles are currently mapped to this profile."
        onLogout={logout}
      />
    );
  }

  return <Portal categories={categories} userName={user?.name ?? 'Authenticated User'} onLogout={logout} />;
}
