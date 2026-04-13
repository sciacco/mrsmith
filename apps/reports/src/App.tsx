import { useRoutes } from 'react-router-dom';
import { AppShell } from '@mrsmith/ui';
import { TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import styles from './App.module.css';

const navGroups: TabGroup[] = [
  {
    label: 'Commerciale',
    items: [
      { label: 'Ordini', path: '/ordini' },
      { label: 'AOV', path: '/aov' },
    ],
  },
  {
    label: 'Rete',
    items: [
      { label: 'Accessi attivi', path: '/accessi-attivi' },
      { label: 'Attivazioni in corso', path: '/attivazioni-in-corso' },
    ],
  },
  {
    label: 'Contratti',
    items: [
      { label: 'Rinnovi in arrivo', path: '/rinnovi-in-arrivo' },
    ],
  },
  {
    label: 'Operativo',
    items: [
      { label: 'Anomalie MOR', path: '/anomalie-mor' },
      { label: 'Accounting TIMOO', path: '/accounting-timoo' },
    ],
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
      <AppShell.Content>
        {element}
      </AppShell.Content>
    </AppShell>
  );
}
