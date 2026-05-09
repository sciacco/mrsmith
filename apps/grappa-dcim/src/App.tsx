import { Suspense, type ReactNode } from 'react';
import { getAppAccessState } from '@mrsmith/auth-client';
import { AccessNotice, AppShell, TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { useRoutes } from 'react-router-dom';
import { isGrappaDCIMNotConfigured, useGrappaDCIMMeta } from './api/queries';
import { ServiceUnavailable } from './components/ServiceUnavailable';
import { ViewState } from './components/ViewState';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import { GRAPPA_DCIM_ACCESS_ROLES } from './lib/roles';
import styles from './App.module.css';

const navGroups: TabGroup[] = [
  {
    label: 'Infrastruttura',
    items: [
      { label: 'Edifici', path: '/edifici' },
      { label: 'Sale e MMR', path: '/sale-mmr' },
      { label: 'Rack', path: '/rack' },
      { label: 'Isole e posizioni', path: '/isole-posizioni' },
    ],
  },
  {
    label: 'Asset',
    items: [
      { label: 'Apparati', path: '/apparati' },
      { label: 'Server', path: '/server' },
      { label: 'Storage', path: '/storage' },
      { label: 'Telecamere', path: '/telecamere' },
    ],
  },
  {
    label: 'Connettivita',
    items: [
      { label: 'Plenum', path: '/plenum' },
      { label: 'Cavi e fibre', path: '/cavi-fibre' },
      { label: 'Cross connect', path: '/cross-connect' },
    ],
  },
  {
    label: 'Topologia',
    items: [{ label: 'Anelli fibra', path: '/anelli-fibra' }],
  },
];

function AppRoutes() {
  const element = useRoutes(routes);
  return <>{element}</>;
}

function MetaGate({ children }: { children: ReactNode }) {
  const meta = useGrappaDCIMMeta();

  if (meta.isLoading) {
    return (
      <ViewState
        title="Caricamento in corso"
        message="Il workspace DCIM si sta preparando."
      />
    );
  }

  if (isGrappaDCIMNotConfigured(meta.error)) {
    return <ServiceUnavailable />;
  }

  if (meta.error || !meta.data?.canRead) {
    return (
      <ViewState
        title="Workspace non disponibile"
        message="Non e stato possibile aprire Grappa DCIM in questo momento."
        tone="error"
      />
    );
  }

  return <>{children}</>;
}

export function App() {
  const auth = useOptionalAuth();
  const { user, logout } = auth;
  const accessState = getAppAccessState(auth, GRAPPA_DCIM_ACCESS_ROLES);

  if (accessState !== 'allowed') {
    return (
      <AppShell appName="Grappa DCIM" userName={user?.name} onLogout={logout}>
        <AppShell.Content>
          <AccessNotice state={accessState} />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Grappa DCIM" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNavGroup groups={navGroups} />
        </div>
      </AppShell.Nav>
      <AppShell.Content>
        <MetaGate>
          <Suspense
            fallback={
              <ViewState
                title="Caricamento in corso"
                message="La sezione richiesta si sta preparando."
              />
            }
          >
            <AppRoutes />
          </Suspense>
        </MetaGate>
      </AppShell.Content>
    </AppShell>
  );
}
