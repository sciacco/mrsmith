import { APP_ACCESS_ROLES, getAppAccessState, hasAnyRole } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { MANUTENZIONI_APPROVAL_ROLES } from './lib/roles';
import styles from './App.module.css';

function buildNavGroups(canConfigure: boolean): TabGroup[] {
  return [
    {
      label: 'Registro',
      items: [
        { label: 'Manutenzioni', path: '/manutenzioni' },
        { label: 'Nuova manutenzione', path: '/manutenzioni/new' },
      ],
    },
    ...(canConfigure
      ? [
          {
            label: 'Gestione',
            items: [{ label: 'Configurazione', path: '/manutenzioni/configurazione' }],
          },
        ]
      : []),
  ];
}

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES.manutenzioni);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Manutenzioni" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  const canConfigure = hasAnyRole(user?.roles, MANUTENZIONI_APPROVAL_ROLES);
  const navGroups = buildNavGroups(canConfigure);

  return (
    <AppShell appName="Manutenzioni" userName={user?.name ?? 'MrSmith'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNavGroup groups={navGroups} />
        </div>
      </AppShell.Nav>
      <AppShell.Content><AppRoutes /></AppShell.Content>
    </AppShell>
  );
}
