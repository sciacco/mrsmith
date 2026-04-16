import { Button, Icon, SearchInput, Skeleton, TableToolbar } from '@mrsmith/ui';
import { useDeferredValue, useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useRichiesteSummary } from '../api/queries';
import { Pagination } from '../components/Pagination';
import { StatusPill, statusTone } from '../components/StatusPill';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { DEFAULT_LIST_STATES, copyErrorMessage, formatCounts, formatDate, isManager, RICHIESTA_STATES } from '../lib/format';
import styles from './shared.module.css';

interface RequestListPageProps {
  mode: 'consultazione' | 'gestione';
}

export function RequestListPage({ mode }: RequestListPageProps) {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const { user } = useOptionalAuth();
  const canManage = isManager(user?.roles);

  const pageValue = Number(params.get('page') ?? '1');
  const page = Number.isFinite(pageValue) && pageValue > 0 ? pageValue : 1;
  const selectedStates = useMemo(
    () => (params.get('stato')?.split(',').filter(Boolean) ?? DEFAULT_LIST_STATES),
    [params],
  );
  const dealFilter = params.get('deal') ?? '';
  const richiedenteFilter = params.get('richiedente') ?? '';
  const clienteFilter = params.get('cliente') ?? '';

  const deferredDeal = useDeferredValue(dealFilter);
  const deferredRichiedente = useDeferredValue(richiedenteFilter);
  const deferredCliente = useDeferredValue(clienteFilter);

  const summary = useRichiesteSummary({
    stato: selectedStates,
    deal: deferredDeal || undefined,
    richiedente: mode === 'gestione' ? deferredRichiedente || undefined : undefined,
    cliente: mode === 'consultazione' ? deferredCliente || undefined : undefined,
    page,
    page_size: 12,
  });

  const hasFilters =
    dealFilter !== '' ||
    (mode === 'gestione' ? richiedenteFilter !== '' : clienteFilter !== '') ||
    selectedStates.length !== DEFAULT_LIST_STATES.length ||
    selectedStates.some((value, index) => value !== DEFAULT_LIST_STATES[index]);

  if (mode === 'gestione' && !canManage) {
    return (
      <section className={styles.forbiddenCard}>
        <div className={styles.emptyIconDanger}><Icon name="lock" /></div>
        <h3>Accesso riservato</h3>
        <p className={styles.muted}>La gestione carrier è disponibile solo per il ruolo manager RDF.</p>
      </section>
    );
  }

  function updateParam(key: string, value: string) {
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      if (value) next.set(key, value);
      else next.delete(key);
      next.set('page', '1');
      return next;
    });
  }

  function updatePage(nextPage: number) {
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      next.set('page', String(nextPage));
      return next;
    });
  }

  function updateStates(nextStates: string[]) {
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      next.set('stato', nextStates.join(','));
      next.set('page', '1');
      return next;
    });
  }

  function toggleState(state: string) {
    const current = new Set(selectedStates);
    if (current.has(state)) current.delete(state);
    else current.add(state);
    const nextStates = RICHIESTA_STATES.filter((value) => current.has(value));
    updateStates(nextStates.length ? nextStates : DEFAULT_LIST_STATES);
  }

  function clearFilters() {
    setParams({
      stato: DEFAULT_LIST_STATES.join(','),
      page: '1',
    });
  }

  const filterInputId = mode === 'gestione' ? 'filter-richiedente' : 'filter-cliente';
  const filterInputLabel = mode === 'gestione' ? 'Filtra per richiedente' : 'Filtra per cliente';

  return (
    <section className={styles.page}>
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>
            {mode === 'gestione' ? 'Gestione RDF Carrier' : 'Consultazione RDF Carrier'}
          </h1>
          <p className={styles.pageSubtitle}>
            {mode === 'gestione'
              ? 'Monitora le richieste attive, apri il dettaglio e porta avanti le fattibilità dei carrier.'
              : 'Consulta lo stato delle richieste, apri il riepilogo completo e verifica l’avanzamento delle fattibilità.'}
          </p>
        </div>
        <div className={styles.headerActions}>
          <Button variant="secondary" onClick={() => summary.refetch()} loading={summary.isFetching && !summary.isLoading}>
            Aggiorna
          </Button>
          <Button onClick={() => navigate('/richieste/new')}>Nuova RDF</Button>
        </div>
      </div>

      <div className={styles.filterStack}>
        <div
          className={styles.statusRow}
          role="group"
          aria-label="Filtra per stato richiesta"
        >
          {RICHIESTA_STATES.map((state) => {
            const active = selectedStates.includes(state);
            return (
              <button
                key={state}
                type="button"
                role="checkbox"
                aria-checked={active}
                className={`${styles.statusChip} ${active ? styles.statusChipActive : ''}`}
                onClick={() => toggleState(state)}
              >
                {state}
              </button>
            );
          })}
        </div>

        <TableToolbar
          activeFilterCount={hasFilters ? 1 : 0}
          filters={
            <div className={styles.toolbarFilters}>
              <label htmlFor={filterInputId} className={styles.sectionLabel} style={{ display: 'none' }}>
                {filterInputLabel}
              </label>
              {mode === 'gestione' ? (
                <input
                  id={filterInputId}
                  className={styles.inlineInput}
                  value={richiedenteFilter}
                  onChange={(event) => updateParam('richiedente', event.target.value)}
                  placeholder="Filtra per richiedente"
                  aria-label="Filtra per richiedente"
                />
              ) : (
                <input
                  id={filterInputId}
                  className={styles.inlineInput}
                  value={clienteFilter}
                  onChange={(event) => updateParam('cliente', event.target.value)}
                  placeholder="Filtra per cliente"
                  aria-label="Filtra per cliente"
                />
              )}
              {hasFilters && (
                <Button variant="ghost" onClick={clearFilters}>
                  Cancella filtri
                </Button>
              )}
            </div>
          }
        >
          <SearchInput
            value={dealFilter}
            onChange={(value) => updateParam('deal', value)}
            placeholder="Cerca per codice deal"
          />
        </TableToolbar>
      </div>

      {summary.isLoading ? (
        <div className={styles.panel}>
          <Skeleton rows={6} />
        </div>
      ) : summary.error ? (
        <div className={styles.emptyCard}>
          <div className={styles.emptyIconDanger}><Icon name="triangle-alert" /></div>
          <h3>Elenco non disponibile</h3>
          <p className={styles.muted}>{copyErrorMessage(summary.error, 'Impossibile caricare le richieste RDF.')}</p>
        </div>
      ) : !summary.data || summary.data.items.length === 0 ? (
        <div className={styles.emptyCard}>
          <div className={styles.emptyIcon}><Icon name="search" /></div>
          <h3>Nessuna richiesta trovata</h3>
          <p className={styles.muted}>Non ci sono richieste che corrispondono ai filtri selezionati.</p>
        </div>
      ) : (
        <>
          <div className={styles.cards}>
            {summary.data.items.map((item) => (
              <article key={item.id} className={styles.summaryCard}>
                <div className={styles.summaryTop}>
                  <div>
                    <div className={styles.summaryCode}>{item.codice_deal || `RDF #${item.id}`}</div>
                    <h2 className={styles.summaryHeading}>{item.company_name ?? 'Cliente non disponibile'}</h2>
                    <p className={styles.small}>{item.deal_name ?? 'Deal non disponibile'}</p>
                  </div>
                  <StatusPill tone={statusTone(item.stato)} aria-label={`Stato ${item.stato}`}>
                    {item.stato}
                  </StatusPill>
                </div>

                <div>
                  <p>{item.indirizzo}</p>
                  <p className={styles.muted}>{item.descrizione}</p>
                </div>

                <div className={styles.summaryBottom}>
                  <div>
                    <p className={styles.small}>Richiesta #{item.id} del {formatDate(item.data_richiesta)}</p>
                    <p className={styles.small}>{formatCounts(item.counts)}</p>
                  </div>
                  <div className={styles.actionsRow}>
                    <Button variant="secondary" onClick={() => navigate(`/richieste/${item.id}/view`)}>
                      Visualizza RDF
                    </Button>
                    {canManage && (
                      <Button onClick={() => navigate(`/richieste/${item.id}`)}>
                        Gestisci
                      </Button>
                    )}
                  </div>
                </div>
              </article>
            ))}
          </div>

          {summary.data.total > summary.data.page_size && (
            <Pagination
              page={summary.data.page}
              pageSize={summary.data.page_size}
              total={summary.data.total}
              label={summary.data.total === 1 ? 'richiesta' : 'richieste'}
              onPageChange={updatePage}
            />
          )}
        </>
      )}
    </section>
  );
}
