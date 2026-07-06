import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Archivio PA', path: '/archivio-pa' },
  { label: 'Riepilogo PA Jira', path: '/riepilogo-pa' },
  { label: 'RDA', path: '/rda' },
];

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES['stats-rda']);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Statistiche RDA" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Statistiche RDA" userName={user?.name ?? 'MrSmith'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <div className="navRow"><TabNav items={navItems} /></div>
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes />
      </AppShell.Content>
    </AppShell>
  );
}
