import { AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { hasAnyRole } from '@mrsmith/auth-client';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { MANUTENZIONI_MANAGER_ROLES } from './lib/roles';
import styles from './App.module.css';

export function App() {
  const { user, authenticated, loading, logout, status } = useOptionalAuth();
  const canManage = hasAnyRole(user?.roles, MANUTENZIONI_MANAGER_ROLES);
  const navGroups: TabGroup[] = [
    {
      label: 'Registro',
      items: [
        { label: 'Manutenzioni', path: '/manutenzioni' },
        { label: 'Nuova manutenzione', path: '/manutenzioni/new' },
      ],
    },
    ...(canManage
      ? [
          {
            label: 'Gestione',
            items: [{ label: 'Configurazione', path: '/manutenzioni/configurazione' }],
          },
        ]
      : []),
  ];
  const element = useRoutes(routes);

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell appName="Manutenzioni" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
        <AppShell.Nav>
          <div className={styles.navRow}>
            <TabNavGroup groups={navGroups} />
          </div>
        </AppShell.Nav>
        <AppShell.Content>
          <section className={styles.noticeCard}>
            <p className={styles.eyebrow}>Autenticazione</p>
            <h1>Sessione in ripristino</h1>
            <p>Reindirizzamento in corso.</p>
          </section>
        </AppShell.Content>
      </AppShell>
    );
  }

  if (!authenticated) {
    return (
      <AppShell appName="Manutenzioni" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
        <AppShell.Nav>
          <div className={styles.navRow}>
            <TabNavGroup groups={navGroups} />
          </div>
        </AppShell.Nav>
        <AppShell.Content>
          <section className={styles.noticeCard}>
            <p className={styles.eyebrow}>Autenticazione</p>
            <h1>Accesso richiesto</h1>
            <p>Riapri l&apos;app dal portale o ricarica la pagina.</p>
          </section>
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Manutenzioni" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNavGroup groups={navGroups} />
        </div>
      </AppShell.Nav>
      <AppShell.Content>{element}</AppShell.Content>
    </AppShell>
  );
}
