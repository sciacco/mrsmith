import { Suspense } from 'react';
import { useRoutes } from 'react-router-dom';
import { AppShell, TabNav } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';

const navItems = [
  { label: 'Dashboard', path: '/dashboard' },
  { label: 'Fornitori', path: '/fornitori' },
  { label: 'Impostazioni qualifica', path: '/impostazioni-qualifica' },
  { label: 'Pagamenti RDA', path: '/modalita-pagamenti-rda' },
  { label: 'Articoli-categorie', path: '/articoli-categorie' },
];

function Nav() {
  return (
    <div className="navRow">
      <TabNav items={navItems} />
    </div>
  );
}

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

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell appName="Fornitori" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
        <AppShell.Nav><Nav /></AppShell.Nav>
        <AppShell.Content>
          <AccessState title="Sessione in ripristino" message="Reindirizzamento in corso." />
        </AppShell.Content>
      </AppShell>
    );
  }

  if (!authenticated) {
    return (
      <AppShell appName="Fornitori" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
        <AppShell.Nav><Nav /></AppShell.Nav>
        <AppShell.Content>
          <AccessState title="Accesso richiesto" message="Ricarica la pagina o riapri l'app dal portale." />
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell appName="Fornitori" userName={user?.name ?? 'MrSmith'} onLogout={logout}>
      <AppShell.Nav><Nav /></AppShell.Nav>
      <AppShell.Content>
        <Suspense fallback={<section className="stateCard"><h1>Caricamento in corso</h1></section>}>
          {element}
        </Suspense>
      </AppShell.Content>
    </AppShell>
  );
}
