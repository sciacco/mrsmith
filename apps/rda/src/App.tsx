import { Suspense, useMemo } from 'react';
import { AppShell, TabNav } from '@mrsmith/ui';
import { useRoutes } from 'react-router-dom';
import { usePermissions } from './api/queries';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

function AccessState({ title, message }: { title: string; message: string }) {
  return (
    <section className="stateCard">
      <p className="eyebrow">Autenticazione</p>
      <h1>{title}</h1>
      <p>{message}</p>
    </section>
  );
}

export function App() {
  const { user, authenticated, loading, logout, status } = useOptionalAuth();
  const element = useRoutes(routes);
  const permissions = usePermissions(authenticated && !loading && status !== 'reauthenticating');
  const navItems = useMemo(() => {
    const flags = authenticated ? permissions.data : undefined;
    return [
      { label: 'Le mie RDA', path: '/rda' },
      ...(flags?.is_approver ? [{ label: 'I/II livello', path: '/rda/inbox/level1-2' }] : []),
      ...(flags?.is_afc
        ? [
            { label: 'Leasing', path: '/rda/inbox/leasing' },
            { label: 'Metodo pagamento', path: '/rda/inbox/payment-method' },
          ]
        : []),
      ...(flags?.is_approver_no_leasing ? [{ label: 'No leasing', path: '/rda/inbox/no-leasing' }] : []),
      ...(flags?.is_approver_extra_budget
        ? [{ label: 'Incremento budget', path: '/rda/inbox/budget-increment' }]
        : []),
    ];
  }, [authenticated, permissions.data]);

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell appName="RDA" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
        <AppShell.Nav>
          <div className="navRow"><TabNav items={navItems} /></div>
        </AppShell.Nav>
        <AppShell.Content>
          <AccessState title="Sessione in ripristino" message="Reindirizzamento in corso." />
        </AppShell.Content>
      </AppShell>
    );
  }

  if (!authenticated) {
    return (
      <AppShell appName="RDA" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
        <AppShell.Nav>
          <div className="navRow"><TabNav items={navItems} /></div>
        </AppShell.Nav>
        <AppShell.Content>
          <AccessState title="Accesso richiesto" message="Ricarica la pagina o riapri l'app dal portale." />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="RDA" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
      <AppShell.Nav>
        <div className="navRow"><TabNav items={navItems} /></div>
      </AppShell.Nav>
      <AppShell.Content>
        <Suspense fallback={<section className="stateCard"><h1>Caricamento in corso</h1></section>}>
          {element}
        </Suspense>
      </AppShell.Content>
    </AppShell>
  );
}
