import { hasAnyRole } from '@mrsmith/auth-client';
import { AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import styles from './App.module.css';

const managerRoles = ['app_rdf_manager'];

export function App() {
  const { user, loading, logout, status } = useOptionalAuth();
  const canManage = hasAnyRole(user?.roles, managerRoles);
  const navGroups: TabGroup[] = [
    {
      label: 'Richieste',
      items: [
        { label: 'Consultazione RDF', path: '/richieste' },
        { label: 'Nuova RDF', path: '/richieste/new' },
        ...(canManage ? [{ label: 'Gestione RDF Carrier', path: '/richieste/gestione' }] : []),
      ],
    },
  ];
  const element = useRoutes(routes);

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
        <AppShell.Nav>
          <div className={styles.navRow}>
            <TabNavGroup groups={navGroups} />
          </div>
        </AppShell.Nav>
        <AppShell.Content>
          <section className={styles.reauthCard}>
            <p className={styles.eyebrow}>Autenticazione</p>
            <h1>Sessione in ripristino</h1>
            <p>La sessione e scaduta durante l&apos;inattivita. Reindirizzamento a Keycloak in corso.</p>
          </section>
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNavGroup groups={navGroups} />
        </div>
      </AppShell.Nav>
      <AppShell.Content>{element}</AppShell.Content>
    </AppShell>
  );
}
