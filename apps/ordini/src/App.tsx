import { Suspense } from 'react';
import { APP_ACCESS_ROLES, getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNav } from '@mrsmith/ui';
import { useRoutes } from 'react-router-dom';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [{ label: 'Tutti gli ordini', path: '/ordini' }];

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, APP_ACCESS_ROLES.ordini);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Ordini" userName={user?.name} onLogout={logout} support={auth}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Ordini" userName={user?.name ?? 'MrSmith'} onLogout={logout} support={auth}>
      <AppShell.Nav>
        <div className="navRow"><TabNav items={navItems} /></div>
      </AppShell.Nav>
      <AppShell.Content>
        <Suspense fallback={<section><h1>Caricamento in corso</h1></section>}>
          <AppRoutes />
        </Suspense>
      </AppShell.Content>
    </AppShell>
  );
}
