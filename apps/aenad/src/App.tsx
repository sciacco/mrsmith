import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Preventivi', path: '/preventivi' },
  { label: 'Archivio', path: '/archivio' },
];

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES.aenad);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Aenad" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Aenad" userName={user?.name ?? 'John Doe'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <TabNav items={navItems} />
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes />
      </AppShell.Content>
    </AppShell>
  );
}
