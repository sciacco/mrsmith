import { useRoutes } from 'react-router-dom';
import { AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import styles from './App.module.css';

const navGroups: TabGroup[] = [
  {
    label: 'Ordini',
    items: [
      { label: 'Ordini ricorrenti', path: '/ordini-ricorrenti' },
      { label: 'Ordini Ricorrenti e Spot', path: '/ordini-dettaglio' },
    ],
  },
  {
    label: 'Fatture',
    items: [{ label: 'Fatture', path: '/fatture' }],
  },
  {
    label: 'Servizi',
    items: [
      { label: 'Accessi', path: '/accessi' },
      { label: 'IaaS Pay Per Use', path: '/iaas-ppu' },
      { label: 'Timoo tenants', path: '/timoo' },
      { label: 'Licenze Windows', path: '/licenze-windows' },
    ],
  },
];

export function App() {
  const { user, authenticated, loading, logout, status } = useOptionalAuth();
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

  if (!authenticated) {
    return (
      <AppShell userName={user?.name ?? 'MrSmith'} onLogout={logout}>
        <AppShell.Nav>
          <div className={styles.navRow}>
            <TabNavGroup groups={navGroups} />
          </div>
        </AppShell.Nav>
        <AppShell.Content>
          <section className={styles.reauthCard}>
            <p className={styles.eyebrow}>Autenticazione</p>
            <h1>Accesso richiesto</h1>
            <p>La sessione Keycloak non è disponibile. Ricarica la pagina o riapri l&apos;app dal portale.</p>
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
