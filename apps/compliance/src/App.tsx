import { useRoutes } from 'react-router-dom';
import { AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Blocchi', path: '/blocks' },
  { label: 'Rilasci', path: '/releases' },
  { label: 'Stato domini', path: '/domains' },
  { label: 'Riepilogo', path: '/history' },
  { label: 'Provenienze', path: '/origins' },
];

export function App() {
  const { user, authenticated, loading, logout, status } = useOptionalAuth();
  const element = useRoutes(routes);

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
        <AppShell.Nav>
          <TabNav items={navItems} />
        </AppShell.Nav>
        <AppShell.Content>
          <section>
            <h1>Sessione in ripristino</h1>
            <p>La sessione e scaduta durante l&apos;inattivita. Reindirizzamento a Keycloak in corso.</p>
          </section>
        </AppShell.Content>
      </AppShell>
    );
  }

  if (!authenticated) {
    return (
      <AppShell userName={user?.name ?? 'MrSmith'} onLogout={logout}>
        <AppShell.Nav>
          <TabNav items={navItems} />
        </AppShell.Nav>
        <AppShell.Content>
          <section>
            <h1>Accesso richiesto</h1>
            <p>La sessione Keycloak non e disponibile. Ricarica la pagina o riapri l&apos;app dal portale.</p>
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
