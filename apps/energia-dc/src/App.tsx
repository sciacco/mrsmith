import { Suspense } from 'react';
import { useRoutes } from 'react-router-dom';
import { AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { ViewState } from './components/ViewState';
import styles from './App.module.css';

const navItems = [
  { label: 'Situazione rack', path: '/situazione-rack' },
  { label: 'Consumi kW', path: '/consumi-kw' },
  { label: 'Addebiti', path: '/addebiti' },
  { label: 'Senza variabile', path: '/senza-variabile' },
  { label: 'Consumi < 1 A', path: '/consumi-bassi' },
];

function Nav() {
  return (
    <div className={styles.navRow}>
      <div className={styles.navScroller}>
        <div className={styles.navInner}>
          <TabNav items={navItems} />
        </div>
      </div>
    </div>
  );
}

export function App() {
  const { user, authenticated, loading, logout, status } = useOptionalAuth();
  const element = useRoutes(routes);

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
        <AppShell.Nav>
          <Nav />
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
          <Nav />
        </AppShell.Nav>
        <AppShell.Content>
          <section className={styles.reauthCard}>
            <p className={styles.eyebrow}>Autenticazione</p>
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
        <Nav />
      </AppShell.Nav>
      <AppShell.Content>
        <Suspense
          fallback={
            <ViewState
              title="Caricamento in corso"
              message="La sezione richiesta si sta preparando."
            />
          }
        >
          {element}
        </Suspense>
      </AppShell.Content>
    </AppShell>
  );
}
