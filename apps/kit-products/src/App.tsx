import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { SettingsMenu } from './components/SettingsMenu';
import { getRuntimeConfig } from './runtimeConfig';
import styles from './App.module.css';

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES['kit-products']);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Kit e Prodotti" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

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

  return (
    <AppShell appName="Kit e Prodotti" userName={user?.name ?? 'John Doe'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNav items={navItems} />
          <SettingsMenu />
        </div>
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes />
      </AppShell.Content>
    </AppShell>
  );
}
