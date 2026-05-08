import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import styles from './App.module.css';

const navGroups: TabGroup[] = [
  {
    label: 'Catalogo',
    items: [{ label: 'Kit di vendita', path: '/kit' }],
  },
  {
    label: 'Prezzi',
    items: [
      { label: 'IaaS Prezzi risorse', path: '/iaas-prezzi' },
      { label: 'Timoo Prezzi Partner', path: '/timoo-prezzi' },
    ],
  },
  {
    label: 'Sconti',
    items: [
      { label: 'Gruppi sconto', path: '/gruppi-sconto' },
      { label: 'Sconti Energia', path: '/sconti-energia' },
    ],
  },
  {
    label: 'Crediti',
    items: [
      { label: 'Crediti Omaggio IaaS', path: '/iaas-crediti' },
      { label: 'Gestione crediti', path: '/gestione-crediti' },
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
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES['listini-e-sconti']);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Listini e Sconti" userName={user?.name} onLogout={logout}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Listini e Sconti" userName={user?.name ?? 'John Doe'} onLogout={logout}>
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
