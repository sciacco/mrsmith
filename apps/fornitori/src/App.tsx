import { Suspense } from 'react';
import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { SettingsMenu } from './components/SettingsMenu';

const primaryNavItems = [
  { label: 'Dashboard', path: '/dashboard' },
  { label: 'Fornitori', path: '/fornitori' },
];

function Nav() {
  return (
    <div className="navRow">
      <div className="navScroller">
        <TabNav items={primaryNavItems} />
      </div>
      <SettingsMenu />
    </div>
  );
}

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES.fornitori);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Fornitori" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Fornitori" userName={user?.name ?? 'MrSmith'} onLogout={logout} support={auth}>
      <AppShell.Nav><Nav /></AppShell.Nav>
      <AppShell.Content>
        <Suspense fallback={<section className="stateCard"><h1>Caricamento in corso</h1></section>}>
          <AppRoutes />
        </Suspense>
      </AppShell.Content>
    </AppShell>
  );
}
