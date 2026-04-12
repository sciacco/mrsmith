import { useRoutes } from 'react-router-dom';
import { AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Proposte', path: '/quotes' },
];

export function App() {
  const { user, loading, logout, status } = useOptionalAuth();
  const element = useRoutes(routes);

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
        <AppShell.Nav>
          <TabNav items={navItems} />
        </AppShell.Nav>
        <AppShell.Content>
          <section style={{ padding: '2rem', textAlign: 'center' }}>
            <p>Sessione in ripristino</p>
          </section>
        </AppShell.Content>
      </AppShell>
    );
  }

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
