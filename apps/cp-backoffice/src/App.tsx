import { useRoutes } from 'react-router-dom';
import {
  APP_ACCESS_ROLES,
  CP_BACKOFFICE_FULL_ACCESS_ROLES,
  CP_BACKOFFICE_BIOMETRIC_ACCESS_ROLES,
  getAppAccessState,
  hasAnyRole,
} from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav, type TabNavItem } from '@mrsmith/ui';
import { createRoutes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

interface AppRoutesProps {
  canAccessFullBackoffice: boolean;
  canAccessBiometrics: boolean;
}

function AppRoutes({ canAccessFullBackoffice, canAccessBiometrics }: AppRoutesProps) {
  const routes = createRoutes(canAccessFullBackoffice, canAccessBiometrics);
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES['cp-backoffice']);
  const canAccessFullBackoffice = hasAnyRole(user?.roles, CP_BACKOFFICE_FULL_ACCESS_ROLES);
  const canAccessBiometrics = hasAnyRole(user?.roles, CP_BACKOFFICE_BIOMETRIC_ACCESS_ROLES);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="CP Backoffice" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  const navItems: TabNavItem[] = [];
  if (canAccessFullBackoffice) {
    navItems.push(
      { label: 'Stato Aziende', path: '/stato-aziende' },
      { label: 'Sezioni CP', path: '/sezioni-cp' },
      { label: 'Gestione Utenti', path: '/gestione-utenti' },
    );
  }
  if (canAccessBiometrics) {
    navItems.push(
      { label: 'Accessi Biometrico', path: '/accessi-biometrico' },
    );
  }

  return (
    <AppShell appName="CP Backoffice" userName={user?.name ?? 'John Doe'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <TabNav items={navItems} />
      </AppShell.Nav>
      <AppShell.Content>
        <AppRoutes
          canAccessFullBackoffice={canAccessFullBackoffice}
          canAccessBiometrics={canAccessBiometrics}
        />
      </AppShell.Content>
    </AppShell>
  );
}
