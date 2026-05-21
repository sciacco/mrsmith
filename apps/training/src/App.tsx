import { useRoutes } from 'react-router-dom';
import {
  APP_ACCESS_ROLES,
  TRAINING_PEOPLE_ADMIN_ROLES,
  getAppAccessState,
  hasAnyRole,
} from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav, type TabNavItem } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import styles from './App.module.css';

const navItems: TabNavItem[] = [
  { label: 'Piano', path: '/piano' },
  { label: 'Richieste', path: '/richieste' },
  { label: 'Catalogo', path: '/catalogo' },
  { label: 'Certificazioni', path: '/certificazioni' },
  { label: 'Report', path: '/report' },
];

function AppRoutes({ isPeopleAdmin }: { isPeopleAdmin: boolean }) {
  const element = useRoutes(routes(isPeopleAdmin));
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES.training);
  const isPeopleAdmin = hasAnyRole(user?.roles, TRAINING_PEOPLE_ADMIN_ROLES);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Formazione" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Formazione" userName={user?.name ?? 'Utente'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNav items={navItems} />
        </div>
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes isPeopleAdmin={isPeopleAdmin} />
      </AppShell.Content>
    </AppShell>
  );
}
