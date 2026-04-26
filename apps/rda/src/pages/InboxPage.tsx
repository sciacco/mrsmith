import { ApiError } from '@mrsmith/api-client';
import { Skeleton } from '@mrsmith/ui';
import { Navigate, useParams } from 'react-router-dom';
import { useInbox } from '../api/queries';
import { PoListTable } from '../components/PoListTable';
import { useHasRole } from '../hooks/useHasRole';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { inboxConfig, isInboxKind } from '../lib/inbox';

function inboxError(error: unknown): string {
  if (error instanceof ApiError && error.status === 403) return 'Accesso riservato';
  return 'La lista non e disponibile in questo momento.';
}

export function InboxPage() {
  const { kind } = useParams();
  const valid = isInboxKind(kind);
  const config = valid ? inboxConfig[kind] : inboxConfig['level1-2'];
  const hasRole = useHasRole(valid ? config.role : '');
  const inbox = useInbox(kind);
  const { user } = useOptionalAuth();

  if (!valid) return <Navigate to="/rda" replace />;

  if (!hasRole) {
    return (
      <main className="rdaPage">
        <section className="stateCard">
          <p className="eyebrow">Autorizzazione</p>
          <h1>Accesso riservato</h1>
          <p>Questa lista e disponibile solo agli utenti abilitati.</p>
        </section>
      </main>
    );
  }

  return (
    <main className="rdaPage">
      <header className="pageHeader">
        <div>
          <h1>{config.title}</h1>
          <p>Richieste in attesa di gestione.</p>
        </div>
      </header>

      <section className="surface">
        <div className="surfaceHeader"><h2>{config.title}</h2></div>
        {inbox.isLoading ? (
          <div className="stateCard"><Skeleton rows={8} /></div>
        ) : inbox.error ? (
          <div className="stateBlock">
            <div>
              <p className="stateTitle">{inboxError(inbox.error)}</p>
              <p className="muted">Riprova piu tardi.</p>
            </div>
          </div>
        ) : (
          <PoListTable rows={inbox.data ?? []} mode="inbox" currentEmail={user?.email} />
        )}
      </section>
    </main>
  );
}
