import { createApiClient, ApiError } from '@mrsmith/api-client';
import { useAuth } from '@mrsmith/auth-client';
import { useEffect, useMemo, useState } from 'react';
import { Portal } from './components/Portal';
import type { Category, PortalUser } from './types';

const BOOTSTRAP_STATUS_DELAY_MS = 500;

function useDelayedFlag(active: boolean, delayMs: number): boolean {
  const [visible, setVisible] = useState(() => !active);

  useEffect(() => {
    if (!active) {
      setVisible(false);
      return undefined;
    }

    setVisible(false);
    const timeout = window.setTimeout(() => {
      setVisible(true);
    }, delayMs);

    return () => {
      window.clearTimeout(timeout);
    };
  }, [active, delayMs]);

  return visible;
}

export function App() {
  const { authenticated, loading, status, getAccessToken, forceRefreshToken, logout } = useAuth();
  const api = useMemo(
    () =>
      createApiClient({
        baseUrl: '/api',
        getToken: getAccessToken,
        forceRefreshToken,
      }),
    [forceRefreshToken, getAccessToken],
  );

  const [user, setUser] = useState<PortalUser | null>(null);
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [bootstrapping, setBootstrapping] = useState(true);
  const isTransientBootstrap = loading || bootstrapping || status === 'reauthenticating';
  const showBootstrapStatus = useDelayedFlag(isTransientBootstrap, BOOTSTRAP_STATUS_DELAY_MS);
  const notificationAuth = useMemo(
    () =>
      authenticated && !loading && status !== 'reauthenticating'
        ? { getAccessToken, forceRefreshToken }
        : undefined,
    [authenticated, forceRefreshToken, getAccessToken, loading, status],
  );

  useEffect(() => {
    if (loading) return;
    if (status === 'reauthenticating') return;
    if (!authenticated) {
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
        if (err instanceof ApiError && err.status === 403) {
          setError('Your session is authenticated but not authorized to load launcher data.');
        } else if (err instanceof ApiError && err.status === 401) {
          setError('Your session expired while loading launcher data. Redirecting to login.');
        } else {
          setError('The portal could not load your identity and app entitlements.');
        }
        setBootstrapping(false);
      });

    return () => {
      cancelled = true;
    };
  }, [api, authenticated, loading, status]);

  if (isTransientBootstrap) {
    if (!showBootstrapStatus) {
      return <Portal categories={[]} userName="Agent Session" />;
    }

    return (
      <Portal
        categories={[]}
        userName="Agent Session"
        statusTitle={status === 'reauthenticating' ? 'RESTORING SESSION' : 'AUTHENTICATING SESSION'}
        statusMessage={
          status === 'reauthenticating'
            ? 'Your session expired while idle. Redirecting to Keycloak to restore access.'
            : 'Connecting to Keycloak and loading your launcher entitlements.'
        }
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
        notifications={notificationAuth}
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
        notifications={notificationAuth}
      />
    );
  }

  return (
    <Portal
      categories={categories}
      userName={user?.name ?? 'Authenticated User'}
      onLogout={logout}
      notifications={notificationAuth}
    />
  );
}
