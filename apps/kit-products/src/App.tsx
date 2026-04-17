import { useRoutes } from 'react-router-dom';
import { AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { SettingsMenu } from './components/SettingsMenu';
import { getRuntimeConfig } from './runtimeConfig';
import styles from './App.module.css';

export function App() {
  const { user, authenticated, loading, logout, status } = useOptionalAuth();
  const element = useRoutes(routes);
  const { arakEnabled } = getRuntimeConfig();
  const navItems = [
    { label: 'Kit', path: '/kit' },
    { label: 'Prodotti', path: '/products' },
    ...(arakEnabled
      ? [
          { label: 'Sconti Kit', path: '/discounts' },
          { label: 'Simulatore', path: '/simulator' },
        ]
      : []),
  ];

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
        <AppShell.Nav>
          <div className={styles.navRow}>
            <TabNav items={navItems} />
            <SettingsMenu />
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
            <TabNav items={navItems} />
            <SettingsMenu />
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
          <TabNav items={navItems} />
          <SettingsMenu />
        </div>
      </AppShell.Nav>
      <AppShell.Content>
        {element}
      </AppShell.Content>
    </AppShell>
  );
}
