import { useRoutes } from 'react-router-dom';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { routes } from './routes';
import styles from './App.module.css';

const navGroups: TabGroup[] = [
  {
    label: 'Fornitori',
    items: [{ label: 'Fornitori', path: '/fornitori' }],
  },
];

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES['rdf-backend']);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="RDF Backend" userName={user?.name} onLogout={logout}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="RDF Backend" userName={user?.name ?? 'John Doe'} onLogout={logout}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNavGroup groups={navGroups} />
        </div>
      </AppShell.Nav>
      <AppShell.Content><AppRoutes /></AppShell.Content>
    </AppShell>
  );
}
