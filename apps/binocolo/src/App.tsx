import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Target M&A', path: '/target' },
  { label: 'Dossier azienda', path: '/azienda' },
  { label: 'Ricerca web', path: '/ricerca-web' },
  { label: 'Configurazione', path: '/config' },
  { label: 'Test', path: '/test' },
];

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES.binocolo);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Binocolo" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Binocolo" userName={user?.name ?? 'John Doe'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <TabNav items={navItems} />
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes />
      </AppShell.Content>
    </AppShell>
  );
}
