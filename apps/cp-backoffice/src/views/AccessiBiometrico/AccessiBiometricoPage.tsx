import { useDeferredValue, useMemo, useState } from 'react';
import { Button, Modal, SearchInput, Skeleton, useToast } from '@mrsmith/ui';
import { useBiometricRequests } from '../../hooks/useBiometricRequests';
import { useSetBiometricCompleted } from '../../hooks/useSetBiometricCompleted';
import type { BiometricRequestRow } from '../../api/biometric';
import styles from './AccessiBiometricoPage.module.css';

type TabKey = 'pending' | 'done';

interface ConfirmTarget {
  row: BiometricRequestRow;
  nextCompleted: boolean;
}

function formatTimestamp(value: string | null): string {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  const day = String(d.getDate()).padStart(2, '0');
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const year = d.getFullYear();
  const hours = String(d.getHours()).padStart(2, '0');
  const minutes = String(d.getMinutes()).padStart(2, '0');
  return `${day}/${month}/${year} ${hours}:${minutes}`;
}

function filterRows(
  rows: ReadonlyArray<BiometricRequestRow>,
  query: string,
): ReadonlyArray<BiometricRequestRow> {
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) return rows;
  return rows.filter((r) =>
    [r.nome, r.cognome, r.email, r.azienda, r.tipo_richiesta].some((v) =>
      v.toLocaleLowerCase().includes(needle),
    ),
  );
}

export function AccessiBiometricoPage() {
  const { data, isLoading, isError, refetch } = useBiometricRequests();
  const mutation = useSetBiometricCompleted();
  const { toast } = useToast();

  const [tab, setTab] = useState<TabKey>('pending');
  const [searchQuery, setSearchQuery] = useState('');
  const deferredSearch = useDeferredValue(searchQuery);
  const [confirmTarget, setConfirmTarget] = useState<ConfirmTarget | null>(null);

  const rows = data ?? [];
  const pendingRows = useMemo(
    () => rows.filter((r) => !r.stato_richiesta),
    [rows],
  );
  const doneRows = useMemo(
    () => rows.filter((r) => r.stato_richiesta),
    [rows],
  );

  const visibleRows = useMemo(() => {
    const base = tab === 'pending' ? pendingRows : doneRows;
    return filterRows(base, deferredSearch);
  }, [tab, pendingRows, doneRows, deferredSearch]);

  const hasSearch = deferredSearch.trim().length > 0;

  function handleConfirm() {
    if (!confirmTarget) return;
    const { row, nextCompleted } = confirmTarget;
    mutation.mutate(
      { id: row.id, completed: nextCompleted },
      {
        onSuccess: () => {
          setConfirmTarget(null);
          toast(
            nextCompleted ? 'Richiesta confermata' : 'Richiesta riaperta',
            'success',
          );
        },
        onError: () => {
          setConfirmTarget(null);
          toast(
            nextCompleted ? 'Conferma non riuscita' : 'Riapertura non riuscita',
            'error',
          );
        },
      },
    );
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>Accessi biometrici</h1>
        <p className={styles.subtitle}>
          Conferma le richieste di accesso approvate.
        </p>
      </div>

      <div className={styles.card}>
        {isLoading ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={6} />
          </div>
        ) : isError ? (
          <div className={styles.stateBox}>
            <div className={styles.stateIcon}>
              <svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M12 9v4M12 17h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>
            <p className={styles.stateTitle}>Impossibile caricare le richieste</p>
            <p className={styles.stateText}>Riprova tra qualche minuto.</p>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => refetch()}
              className={styles.stateCta}
            >
              Riprova
            </Button>
          </div>
        ) : (
          <>
            <div className={styles.toolbar}>
              <div className={styles.tabs} role="tablist" aria-label="Filtro richieste">
                <button
                  type="button"
                  role="tab"
                  aria-selected={tab === 'pending'}
                  className={`${styles.tab} ${tab === 'pending' ? styles.tabActive : ''}`}
                  onClick={() => setTab('pending')}
                >
                  Da confermare
                  {pendingRows.length > 0 && (
                    <span className={styles.tabCount}>{pendingRows.length}</span>
                  )}
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={tab === 'done'}
                  className={`${styles.tab} ${tab === 'done' ? styles.tabActive : ''}`}
                  onClick={() => setTab('done')}
                >
                  Confermate
                </button>
              </div>
              <SearchInput
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder="Cerca per nome, email, azienda..."
                className={styles.search}
              />
            </div>

            {visibleRows.length === 0 ? (
              <EmptyState tab={tab} hasSearch={hasSearch} query={deferredSearch} />
            ) : (
              <div className={styles.tableScroll}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Richiedente</th>
                      <th>Azienda</th>
                      <th>Tipo</th>
                      <th>{tab === 'pending' ? 'Richiesta' : 'Confermata'}</th>
                      <th className={styles.actionCol} aria-label="Azione" />
                    </tr>
                  </thead>
                  <tbody>
                    {visibleRows.map((row) => {
                      const isDone = row.stato_richiesta;
                      return (
                        <tr key={row.id}>
                          <td>
                            <div className={styles.personName}>
                              {row.nome} {row.cognome}
                            </div>
                            <div className={styles.personEmail}>{row.email}</div>
                          </td>
                          <td>{row.azienda}</td>
                          <td>{row.tipo_richiesta}</td>
                          <td className={styles.timeCell}>
                            {tab === 'pending' ? (
                              formatTimestamp(row.data_richiesta)
                            ) : (
                              <>
                                <div>{formatTimestamp(row.data_approvazione)}</div>
                                <div className={styles.timeSub}>
                                  Richiesta {formatTimestamp(row.data_richiesta)}
                                </div>
                              </>
                            )}
                          </td>
                          <td className={styles.actionCol}>
                            {isDone ? (
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() =>
                                  setConfirmTarget({ row, nextCompleted: false })
                                }
                              >
                                Riapri
                              </Button>
                            ) : (
                              <Button
                                size="sm"
                                variant="primary"
                                onClick={() =>
                                  setConfirmTarget({ row, nextCompleted: true })
                                }
                              >
                                Conferma
                              </Button>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>

      <ConfirmDialog
        target={confirmTarget}
        pending={mutation.isPending}
        onCancel={() => {
          if (!mutation.isPending) setConfirmTarget(null);
        }}
        onConfirm={handleConfirm}
      />
    </section>
  );
}

interface ConfirmDialogProps {
  target: ConfirmTarget | null;
  pending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

function ConfirmDialog({ target, pending, onCancel, onConfirm }: ConfirmDialogProps) {
  const isConfirm = target?.nextCompleted ?? true;
  const title = isConfirm ? 'Conferma richiesta' : 'Riapri richiesta';
  const body = target
    ? isConfirm
      ? `La richiesta di ${target.row.nome} ${target.row.cognome} per ${target.row.azienda} sarà contrassegnata come confermata.`
      : `La richiesta di ${target.row.nome} ${target.row.cognome} per ${target.row.azienda} tornerà tra quelle da confermare.`
    : '';

  return (
    <Modal open={target != null} onClose={onCancel} title={title} size="sm" dismissible={!pending}>
      <div className={styles.confirmBody}>
        <p className={styles.confirmText}>{body}</p>
      </div>
      <div className={styles.confirmActions}>
        <Button size="md" variant="secondary" onClick={onCancel} disabled={pending}>
          Annulla
        </Button>
        <Button
          size="md"
          variant={isConfirm ? 'primary' : 'secondary'}
          onClick={onConfirm}
          loading={pending}
        >
          {isConfirm ? 'Conferma' : 'Riapri'}
        </Button>
      </div>
    </Modal>
  );
}

interface EmptyStateProps {
  tab: TabKey;
  hasSearch: boolean;
  query: string;
}

function EmptyState({ tab, hasSearch, query }: EmptyStateProps) {
  if (hasSearch) {
    return (
      <div className={styles.stateBox}>
        <div className={styles.stateIcon}>
          <SearchIcon />
        </div>
        <p className={styles.stateTitle}>Nessun risultato</p>
        <p className={styles.stateText}>
          Nessuna richiesta corrisponde a «{query.trim()}».
        </p>
      </div>
    );
  }
  if (tab === 'pending') {
    return (
      <div className={styles.stateBox}>
        <div className={styles.stateIcon}>
          <CheckCircleIcon />
        </div>
        <p className={styles.stateTitle}>Nessuna richiesta in sospeso</p>
        <p className={styles.stateText}>Tutte le richieste sono state evase.</p>
      </div>
    );
  }
  return (
    <div className={styles.stateBox}>
      <div className={styles.stateIcon}>
        <InboxIcon />
      </div>
      <p className={styles.stateTitle}>Nessuna richiesta confermata</p>
      <p className={styles.stateText}>Le conferme appariranno qui.</p>
    </div>
  );
}

function CheckCircleIcon() {
  return (
    <svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="m8 12 3 3 5-6"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function InboxIcon() {
  return (
    <svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M4 13h4l2 3h4l2-3h4M5 13 7 5h10l2 8v5a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2v-5Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function SearchIcon() {
  return (
    <svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="1.5" />
      <path d="m20 20-3.5-3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
