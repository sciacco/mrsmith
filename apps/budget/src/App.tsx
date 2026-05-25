import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Home', path: '/home' },
  { label: 'Gruppi', path: '/groups' },
  { label: 'Centri di costo', path: '/cost-centers' },
  { label: 'Voci di costo', path: '/budgets' },
  { label: 'Utenti', path: '/users' },
];

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES.budget);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Budget Management" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Budget Management" userName={user?.name ?? 'John Doe'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <TabNav items={navItems} />
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes />
      </AppShell.Content>
    </AppShell>
  );
}
