import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import styles from './App.module.css';

const navGroups: TabGroup[] = [
  {
    label: 'Ordini',
    items: [
      { label: 'Ordini Ricorrenti e Spot', path: '/ordini-dettaglio' },
      { label: 'Ordini Ricorrenti (OLD)', path: '/ordini-ricorrenti' },
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

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES['panoramica-cliente']);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Panoramica cliente" userName={user?.name} onLogout={logout}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Panoramica cliente" userName={user?.name ?? 'John Doe'} onLogout={logout}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNavGroup groups={navGroups} />
        </div>
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes />
      </AppShell.Content>
    </AppShell>
  );
}
