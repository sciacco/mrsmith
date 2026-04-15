import { useRoutes } from 'react-router-dom';
import { AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { routes } from './routes';
import styles from './App.module.css';

const navGroups: TabGroup[] = [
  {
    label: 'Registro',
    items: [{ label: 'Fornitori', path: '/fornitori' }],
  },
];

export function App() {
  const { user, loading, logout, status } = useOptionalAuth();
  const element = useRoutes(routes);

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
        <AppShell.Nav>
          <div className={styles.navRow}>
            <TabNavGroup groups={navGroups} />
            <span className={styles.navMeta}>Provisioning registry</span>
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
          <span className={styles.navMeta}>Provisioning registry</span>
        </div>
      </AppShell.Nav>
      <AppShell.Content>{element}</AppShell.Content>
    </AppShell>
  );
}
