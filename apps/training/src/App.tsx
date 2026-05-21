import { useMemo } from 'react';
import { useLocation, useRoutes } from 'react-router-dom';
import {
  APP_ACCESS_ROLES,
  TRAINING_PEOPLE_ADMIN_ROLES,
  getAppAccessState,
  hasAnyRole,
} from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav, type TabNavItem } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { useTrainingWorkspace } from './api/queries';
import { TrainingHeader } from './components/TrainingHeader';
import styles from './App.module.css';

const navItems: TabNavItem[] = [
  { label: 'Overview', path: '/' },
  { label: 'Pipeline', path: '/pipeline' },
  { label: 'Persone', path: '/persone' },
  { label: 'Pianificazione', path: '/pianificazione' },
  { label: 'Piano', path: '/piano' },
  { label: 'Catalogo', path: '/catalogo' },
  { label: 'Certificazioni', path: '/certificazioni' },
];

const ROUTES_WITHOUT_GLOBAL_HEADER = new Set(['/pipeline']);

function AppRoutes({ isPeopleAdmin }: { isPeopleAdmin: boolean }) {
  const element = useRoutes(routes(isPeopleAdmin));
  return <>{element}</>;
}

function GlobalTrainingHeader({ isPeopleAdmin }: { isPeopleAdmin: boolean }) {
  const workspace = useTrainingWorkspace(isPeopleAdmin);
  const teamOptions = useMemo(
    () =>
      (workspace.data?.masterData?.teams ?? []).map((t) => ({
        value: t.code,
        label: t.name || t.code,
      })),
    [workspace.data],
  );
  return <TrainingHeader teamOptions={teamOptions} />;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const location = useLocation();
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

  const showGlobalHeader = isPeopleAdmin && !ROUTES_WITHOUT_GLOBAL_HEADER.has(location.pathname);

  return (
    <AppShell appName="Formazione" userName={user?.name ?? 'Utente'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNav items={navItems} />
        </div>
      </AppShell.Nav>
      <AppShell.Content>
        {showGlobalHeader && <GlobalTrainingHeader isPeopleAdmin={isPeopleAdmin} />}
        <AppRoutes isPeopleAdmin={isPeopleAdmin} />
      </AppShell.Content>
    </AppShell>
  );
}
