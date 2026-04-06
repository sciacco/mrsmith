import { useRoutes } from 'react-router-dom';
import { AppShell } from '@mrsmith/ui';
import { TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Home', path: '/home' },
  { label: 'Gruppi', path: '/groups' },
  { label: 'Centri di costo', path: '/cost-centers' },
  { label: 'Voci di costo', path: '/budgets' },
];

export function App() {
  const { user, loading, logout } = useOptionalAuth();
  const element = useRoutes(routes);

  if (loading) return null;

  return (
    <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
      <AppShell.Nav>
        <TabNav items={navItems} />
      </AppShell.Nav>
      <AppShell.Content>
        {element}
      </AppShell.Content>
    </AppShell>
  );
}
