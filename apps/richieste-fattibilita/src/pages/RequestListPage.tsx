import { Button, Icon, MultiSelect, SearchInput, Skeleton, Tooltip } from '@mrsmith/ui';
import { Fragment, useDeferredValue, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useRichiesteSummary } from '../api/queries';
import { Pagination } from '../components/Pagination';
import { StatusPill, statusTone } from '../components/StatusPill';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import {
  DEFAULT_LIST_STATES,
  RICHIESTA_STATES,
  compactAddress,
  copyErrorMessage,
  formatCountsBreakdown,
  formatDate,
  isManager,
} from '../lib/format';
import shared from './shared.module.css';
import styles from './RequestListPage.module.css';

interface RequestListPageProps {
  mode: 'consultazione' | 'gestione';
}

const STATE_OPTIONS: { value: string; label: string }[] = RICHIESTA_STATES.map((value) => ({
  value,
  label: value.charAt(0).toUpperCase() + value.slice(1),
}));

export function RequestListPage({ mode }: RequestListPageProps) {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const { user } = useOptionalAuth();
  const canManage = isManager(user?.roles);
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const pageValue = Number(params.get('page') ?? '1');
  const page = Number.isFinite(pageValue) && pageValue > 0 ? pageValue : 1;
  const selectedStates = useMemo(() => {
    const raw = params.get('stato');
    if (raw === null) return DEFAULT_LIST_STATES;
    return raw.split(',').filter(Boolean);
  }, [params]);
  const qFilter = params.get('q') ?? '';
  const dataDaFilter = params.get('data_da') ?? '';
  const dataAFilter = params.get('data_a') ?? '';

  const deferredQ = useDeferredValue(qFilter);

  const summary = useRichiesteSummary({
    stato: selectedStates,
    q: deferredQ || undefined,
    data_da: dataDaFilter || undefined,
    data_a: dataAFilter || undefined,
    page,
    page_size: 20,
  });

  const statesMatchDefault =
    selectedStates.length === DEFAULT_LIST_STATES.length &&
    selectedStates.every((value, index) => value === DEFAULT_LIST_STATES[index]);
  const hasFilters =
    qFilter !== '' || dataDaFilter !== '' || dataAFilter !== '' || !statesMatchDefault;

  if (mode === 'gestione' && !canManage) {
    return (
      <section className={shared.forbiddenCard}>
        <div className={shared.emptyIconDanger}>
          <Icon name="lock" />
        </div>
        <h3>Accesso riservato</h3>
        <p className={shared.muted}>La gestione carrier è disponibile solo per il ruolo manager RDF.</p>
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
      const ordered = RICHIESTA_STATES.filter((value) => nextStates.includes(value));
      next.set('stato', ordered.join(','));
      next.set('page', '1');
      return next;
    });
  }

  function clearFilters() {
    setParams({
      stato: DEFAULT_LIST_STATES.join(','),
      page: '1',
    });
  }

  return (
    <section className={shared.page}>
      <div className={shared.pageHeader}>
        <div>
          <h1 className={shared.pageTitle}>
            {mode === 'gestione' ? 'Gestione RDF Carrier' : 'Consultazione RDF Carrier'}
          </h1>
          <p className={shared.pageSubtitle}>
            {mode === 'gestione'
              ? 'Monitora le richieste attive, apri il dettaglio e porta avanti le fattibilità dei carrier.'
              : 'Consulta lo stato delle richieste, apri il riepilogo completo e verifica l’avanzamento delle fattibilità.'}
          </p>
        </div>
        <div className={shared.headerActions}>
          <Button
            variant="secondary"
            onClick={() => summary.refetch()}
            loading={summary.isFetching && !summary.isLoading}
          >
            Aggiorna
          </Button>
          <Button onClick={() => navigate('/richieste/new')}>Nuova RDF</Button>
        </div>
      </div>

      <div className={styles.filterBar}>
        <div className={styles.filterSearch}>
          <SearchInput
            value={qFilter}
            onChange={(value) => updateParam('q', value)}
            placeholder="Cerca per deal, cliente, indirizzo o richiedente…"
          />
        </div>
        <div className={styles.stateSelect}>
          <MultiSelect<string>
            options={STATE_OPTIONS}
            selected={selectedStates}
            onChange={updateStates}
            placeholder="Tutti gli stati"
          />
        </div>
        <div className={styles.dateRange}>
          <input
            type="date"
            value={dataDaFilter}
            onChange={(event) => updateParam('data_da', event.target.value)}
            className={styles.dateInput}
            aria-label="Data richiesta dal"
          />
          <span className={styles.dateSep}>–</span>
          <input
            type="date"
            value={dataAFilter}
            onChange={(event) => updateParam('data_a', event.target.value)}
            className={styles.dateInput}
            aria-label="Data richiesta al"
          />
        </div>
        {hasFilters && (
          <button type="button" className={styles.clearLink} onClick={clearFilters}>
            Cancella filtri
          </button>
        )}
      </div>

      {summary.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={6} />
        </div>
      ) : summary.error ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Elenco non disponibile</h3>
          <p className={shared.muted}>
            {copyErrorMessage(summary.error, 'Impossibile caricare le richieste RDF.')}
          </p>
        </div>
      ) : !summary.data || summary.data.items.length === 0 ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIcon}>
            <Icon name="search" />
          </div>
          <h3>Nessuna richiesta trovata</h3>
          <p className={shared.muted}>Non ci sono richieste che corrispondono ai filtri selezionati.</p>
        </div>
      ) : (
        <>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={styles.colExpand} aria-label="Espandi dettaglio" />
                  <th>HP-ID</th>
                  <th>Cliente / Deal</th>
                  <th>Indirizzo</th>
                  <th>Richiesta</th>
                  <th>Stato</th>
                  <th className={styles.countCell}>RDF</th>
                  <th className={styles.actionsCell}>Azioni</th>
                </tr>
              </thead>
              <tbody>
                {summary.data.items.map((item) => {
                  const expanded = expandedId === item.id;
                  const completed = item.counts.completata;
                  const total = item.counts.totale;
                  return (
                    <Fragment key={item.id}>
                      <tr className={styles.row} data-expanded={expanded}>
                        <td>
                          <button
                            type="button"
                            className={styles.expandBtn}
                            onClick={() => setExpandedId(expanded ? null : item.id)}
                            aria-expanded={expanded}
                            aria-label={expanded ? 'Chiudi dettaglio' : 'Apri dettaglio'}
                          >
                            <Icon name={expanded ? 'chevron-up' : 'chevron-down'} size={16} />
                          </button>
                        </td>
                        <td>
                          <span className={styles.idLabel}>
                            {item.codice_deal || `RDF #${item.id}`}
                          </span>
                        </td>
                        <td>
                          <div className={styles.clienteCell}>
                            <span className={styles.clienteName}>
                              {item.company_name ?? 'Cliente non disponibile'}
                            </span>
                            {item.deal_name && (
                              <span className={styles.dealName} title={item.deal_name}>
                                {item.deal_name}
                              </span>
                            )}
                          </div>
                        </td>
                        <td>
                          <Tooltip content={item.indirizzo || '—'}>
                            <span className={styles.addrText}>
                              {compactAddress(item.indirizzo)}
                            </span>
                          </Tooltip>
                        </td>
                        <td>
                          <div className={styles.dateCell}>
                            <span>#{item.id}</span>
                            <span className={shared.small}>{formatDate(item.data_richiesta)}</span>
                          </div>
                        </td>
                        <td>
                          <StatusPill tone={statusTone(item.stato)} aria-label={`Stato ${item.stato}`}>
                            {item.stato}
                          </StatusPill>
                        </td>
                        <td className={styles.countCell}>
                          {total === 0 ? (
                            <span className={shared.small}>—</span>
                          ) : (
                            <Tooltip content={formatCountsBreakdown(item.counts)}>
                              <span className={styles.countBadge}>
                                {completed}/{total}
                              </span>
                            </Tooltip>
                          )}
                        </td>
                        <td className={styles.actionsCell}>
                          <Tooltip content="Visualizza RDF">
                            <button
                              type="button"
                              className={styles.iconAction}
                              onClick={() => navigate(`/richieste/${item.id}/view`)}
                              aria-label="Visualizza RDF"
                            >
                              <Icon name="eye" size={18} />
                            </button>
                          </Tooltip>
                          {canManage && (
                            <Tooltip content="Gestisci RDF">
                              <button
                                type="button"
                                className={`${styles.iconAction} ${styles.iconActionPrimary}`}
                                onClick={() => navigate(`/richieste/${item.id}`)}
                                aria-label="Gestisci RDF"
                              >
                                <Icon name="settings" size={18} />
                              </button>
                            </Tooltip>
                          )}
                        </td>
                      </tr>
                      {expanded && (
                        <tr className={styles.detailRow}>
                          <td colSpan={8}>
                            <div className={styles.detailGrid}>
                              <div>
                                <div className={styles.detailLabel}>Indirizzo completo</div>
                                <div>{item.indirizzo || '—'}</div>
                              </div>
                              <div>
                                <div className={styles.detailLabel}>Descrizione</div>
                                <div className={styles.detailText}>
                                  {item.descrizione || '—'}
                                </div>
                              </div>
                              {item.owner_email && (
                                <div>
                                  <div className={styles.detailLabel}>Owner deal</div>
                                  <div>{item.owner_email}</div>
                                </div>
                              )}
                              {item.created_by && (
                                <div>
                                  <div className={styles.detailLabel}>Richiesta da</div>
                                  <div>{item.created_by}</div>
                                </div>
                              )}
                              {total > 0 && (
                                <div>
                                  <div className={styles.detailLabel}>Dettaglio RDF</div>
                                  <div>{formatCountsBreakdown(item.counts)}</div>
                                </div>
                              )}
                            </div>
                          </td>
                        </tr>
                      )}
                    </Fragment>
                  );
                })}
              </tbody>
            </table>
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
