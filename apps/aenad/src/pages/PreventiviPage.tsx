import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuotesList, useHubSpotRetry } from '../api/queries';
import { formatMoney } from '../utils/format';
import styles from './PreventiviPage.module.css';


const PAGE_SIZE = 50;

function formatDate(value: string | null) {
  if (!value) return '-';
  const date = new Date(value);
  if (!Number.isNaN(date.getTime())) {
    return date.toLocaleDateString('it-IT', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
    });
  }
  return value;
}

export function PreventiviPage() {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const { toast } = useToast();

  const quotesQuery = useQuotesList(page, PAGE_SIZE);
  const retryMutation = useHubSpotRetry();

  const items = quotesQuery.data?.items ?? [];
  const total = quotesQuery.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const fromRow = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const toRow = Math.min(total, page * PAGE_SIZE);

  const handleRetrySync = (e: React.MouseEvent, id: number) => {
    e.stopPropagation();
    retryMutation.mutate(id, {
      onSuccess: () => {
        toast('Sincronizzazione HubSpot riaccodata con successo.', 'success');
        void quotesQuery.refetch();
      },
      onError: (err) => {
        toast(`Errore durante il rinvio: ${err instanceof Error ? err.message : 'Errore sconosciuto'}`, 'error');
      },
    });
  };

  const getAuthoringStatusBadge = (status: string) => {
    switch (status) {
      case 'draft':
        return <span className={`${styles.badge} ${styles.badgeDraft}`}>Bozza</span>;
      case 'ready':
        return <span className={`${styles.badge} ${styles.badgeReady}`}>Pronto</span>;
      case 'archived':
        return <span className={`${styles.badge} ${styles.badgeArchived}`}>Archiviato</span>;
      default:
        return <span className={styles.badge}>{status}</span>;
    }
  };

  const getHubSpotSyncBadge = (status: string, id: number) => {
    switch (status) {
      case 'pending':
        return <span className={`${styles.badge} ${styles.badgeHubSpotPending}`}>In Coda</span>;
      case 'succeeded':
        return <span className={`${styles.badge} ${styles.badgeHubSpotSucceeded}`}>Sincronizzato</span>;
      case 'failed':
        return (
          <span className={`${styles.badge} ${styles.badgeHubSpotFailed}`}>
            Fallito
            <button
              className={styles.retryBtn}
              onClick={(e) => handleRetrySync(e, id)}
              disabled={retryMutation.isPending}
              title="Riprova sincronizzazione"
            >
              Retry
            </button>
          </span>
        );
      default:
        return null;
    }
  };

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Vendite</p>
          <h1>Preventivi</h1>
        </div>
        <Button onClick={() => navigate('/preventivi/nuovo')}>
          <Icon name="plus" size={16} /> Nuovo Preventivo
        </Button>
      </header>

      <section className={styles.resultsPanel} aria-label="Elenco preventivi">
        <div className={styles.resultsHeader}>
          <div>
            <h2>Preventivi V1</h2>
            <p>
              {quotesQuery.isLoading || quotesQuery.isFetching
                ? 'Caricamento preventivi...'
                : `${fromRow}-${toRow} di ${total}`}
            </p>
          </div>
        </div>

        {quotesQuery.isLoading ? (
          <div className={styles.loadingPanel}>
            <Skeleton rows={8} />
          </div>
        ) : quotesQuery.error ? (
          <div className={styles.emptyPanel}>
            <div className={styles.emptyIcon} style={{ color: 'var(--color-error)' }}>
              !
            </div>
            <div className={styles.emptyText}>
              <h2>Errore di caricamento</h2>
              <p>Impossibile caricare i preventivi dal server.</p>
            </div>
          </div>
        ) : items.length === 0 ? (
          <div className={styles.emptyPanel}>
            <div className={styles.emptyIcon}>€</div>
            <div className={styles.emptyText}>
              <h2>Nessun preventivo trovato</h2>
              <p>Clicca su "Nuovo Preventivo" per iniziare.</p>
            </div>
          </div>
        ) : (
          <>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Numero</th>
                    <th>Data Doc</th>
                    <th>Cliente</th>
                    <th>Descrizione</th>
                    <th>Stato</th>
                    <th>Sync HubSpot</th>
                    <th className={styles.numCol}>Imponibile</th>
                    <th className={styles.numCol}>Lordo</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((row, index) => (
                    <tr
                      key={row.id}
                      style={{ animationDelay: `${Math.min(index, 8) * 25}ms` }}
                      className={styles.clickableRow}
                      onClick={() => navigate(`/preventivi/${row.id}`)}
                    >
                      <td className={styles.monoCell}>{row.quote_number || `Draft #${row.id}`}</td>
                      <td>{formatDate(row.document_date)}</td>
                      <td>{row.customer_name || '-'}</td>
                      <td className={styles.descriptionCell}>{row.description || '-'}</td>
                      <td>{getAuthoringStatusBadge(row.authoring_status)}</td>
                      <td>{getHubSpotSyncBadge(row.hubspot_sync_status, row.id)}</td>
                      <td className={styles.numCol}>{formatMoney(row.total_net)}</td>
                      <td className={styles.numCol}>{formatMoney(row.total_gross)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className={styles.pagination}>
              <span>
                {fromRow}-{toRow} di {total}
              </span>
              <div className={styles.pageActions}>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={page <= 1}
                  onClick={() => setPage((current) => current - 1)}
                >
                  Precedente
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={page >= totalPages}
                  onClick={() => setPage((current) => current + 1)}
                >
                  Successiva
                </Button>
              </div>
            </div>
          </>
        )}
      </section>
    </main>
  );
}

