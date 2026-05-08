import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Blocchi', path: '/blocks' },
  { label: 'Rilasci', path: '/releases' },
  { label: 'Stato domini', path: '/domains' },
  { label: 'Riepilogo', path: '/history' },
  { label: 'Provenienze', path: '/origins' },
];

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES.compliance);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Compliance" userName={user?.name} onLogout={logout}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Compliance" userName={user?.name ?? 'John Doe'} onLogout={logout}>
      <AppShell.Nav>
        <TabNav items={navItems} />
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes />
      </AppShell.Content>
    </AppShell>
  );
}
