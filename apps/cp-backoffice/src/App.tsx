import { useRoutes } from 'react-router-dom';
import {
  APP_ACCESS_ROLES,
  CP_BACKOFFICE_FULL_ACCESS_ROLES,
  getAppAccessState,
  hasAnyRole,
} from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { createRoutes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const fullNavItems = [
  { label: 'Stato Aziende', path: '/stato-aziende' },
  { label: 'Gestione Utenti', path: '/gestione-utenti' },
  { label: 'Accessi Biometrico', path: '/accessi-biometrico' },
];

const biometricNavItems = [
  { label: 'Accessi Biometrico', path: '/accessi-biometrico' },
];

interface AppRoutesProps {
  canAccessFullBackoffice: boolean;
}

function AppRoutes({ canAccessFullBackoffice }: AppRoutesProps) {
  const routes = createRoutes(canAccessFullBackoffice);
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES['cp-backoffice']);
  const canAccessFullBackoffice = hasAnyRole(user?.roles, CP_BACKOFFICE_FULL_ACCESS_ROLES);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="CP Backoffice" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  const navItems = canAccessFullBackoffice ? fullNavItems : biometricNavItems;

  return (
    <AppShell appName="CP Backoffice" userName={user?.name ?? 'John Doe'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <TabNav items={navItems} />
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes canAccessFullBackoffice={canAccessFullBackoffice} />
      </AppShell.Content>
    </AppShell>
  );
}
