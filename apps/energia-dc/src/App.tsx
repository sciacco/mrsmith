import { Suspense } from 'react';
import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
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

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES['energia-dc']);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Energia in DC" userName={user?.name} onLogout={logout}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Energia in DC" userName={user?.name ?? 'John Doe'} onLogout={logout}>
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
          <AppRoutes />
        </Suspense>
      </AppShell.Content>
    </AppShell>
  );
}
