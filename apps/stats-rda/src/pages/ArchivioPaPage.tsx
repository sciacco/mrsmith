import { useCallback, useMemo } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Icon, SearchInput, SingleSelect, Skeleton } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import {
  useFilters,
  useIssueDetail,
  useIssueList,
  type IssueListParams,
} from '../api/queries';
import type { AutocompleteResponse } from '../api/types';
import { AutocompleteFilter } from '../components/AutocompleteFilter';
import { IssueDetailPanel } from '../components/IssueDetailPanel';
import { formatEUR, nz, shortDate, statusBadgeClass } from '../lib/format';
import s from './ArchivioPaPage.module.css';

const DEFAULT_LIMIT = 50;
const SORT_OPTIONS = [
  { value: 'created', label: 'Data creazione' },
  { value: 'importo_totale', label: 'Importo totale' },
  { value: 'issue_key', label: 'Codice ordine' },
  { value: 'reporter_name', label: 'Richiedente' },
];

function apiErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status === 403) return 'Accesso non consentito.';
  if (error instanceof ApiError && error.status === 401) return 'Accesso richiesto.';
  if (error instanceof ApiError && error.status === 503) return 'Archivio PA non disponibile.';
  return 'La ricerca non è disponibile in questo momento.';
}

export function ArchivioPaPage() {
  const [params, setParams] = useSearchParams();
  const { issueKey } = useParams();
  const navigate = useNavigate();

  // Filters read from URL search params (deep-linkable).
  const q = params.get('q') ?? '';
  const budget = params.get('budget') ?? '';
  const stato = params.get('stato') ?? '';
  const tipo = params.get('tipo') ?? '';
  const fornitore = params.get('fornitore') ?? '';
  const richiedente = params.get('richiedente') ?? '';
  const valuta = params.get('valuta') ?? '';
  const from = params.get('from') ?? '';
  const to = params.get('to') ?? '';
  const sort = params.get('sort') ?? 'created';
  const dir = params.get('dir') ?? 'desc';
  const page = Math.max(1, parseInt(params.get('page') ?? '1', 10) || 1);

  const filtersQ = useFilters();

  const listParams: IssueListParams = useMemo(
    () => ({ q, budget, stato, tipo, fornitore, richiedente, valuta, from, to, sort, dir, page, limit: DEFAULT_LIMIT }),
    [q, budget, stato, tipo, fornitore, richiedente, valuta, from, to, sort, dir, page],
  );

  const hasAnyFilter = Boolean(q || budget || stato || tipo || fornitore || richiedente || valuta || from || to);
  const listQ = useIssueList(listParams, hasAnyFilter);
  const detailQ = useIssueDetail(issueKey ?? null);
  const api = useApiClient();

  const updateParam = useCallback(
    (key: string, value: string) => {
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        if (value) next.set(key, value);
        else next.delete(key);
        next.delete('page');
        return next;
      });
    },
    [setParams],
  );

  const clearFilters = useCallback(() => {
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      for (const k of ['q', 'budget', 'stato', 'tipo', 'fornitore', 'richiedente', 'valuta', 'from', 'to', 'page']) {
        next.delete(k);
      }
      return next;
    });
  }, [setParams]);

  function selectIssue(key: string) {
    const search = params.toString();
    navigate(`/archivio-pa/${key}${search ? `?${search}` : ''}`);
  }

  function goToPage(nextPage: number) {
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      if (nextPage > 1) next.set('page', String(nextPage));
      else next.delete('page');
      return next;
    });
  }

  // Filters data
  const filters = filtersQ.data;
  const budgetOptions = useMemo(
    () => (filters?.budget ?? []).map((o) => ({ value: o.value, label: `${o.value} (${o.count})` })),
    [filters],
  );
  const statoOptions = useMemo(
    () => (filters?.stati ?? []).map((o) => ({ value: o.value, label: `${o.value} (${o.count})` })),
    [filters],
  );
  const tipoOptions = useMemo(
    () => (filters?.tipi ?? []).map((o) => ({ value: o.value, label: `${o.value} (${o.count})` })),
    [filters],
  );
  const valutaOptions = useMemo(
    () => (filters?.valute ?? []).map((o) => ({ value: o.value, label: `${o.value} (${o.count})` })),
    [filters],
  );

  const loadFornitori = useCallback(
    async (query: string) => {
      try {
        const data = await api.get<AutocompleteResponse>(
          `/stats-rda/v1/pa/fornitori?q=${encodeURIComponent(query)}&limit=20`,
        );
        return data.items ?? [];
      } catch {
        return [];
      }
    },
    [api],
  );
  const loadRichiedenti = useCallback(
    async (query: string) => {
      try {
        const data = await api.get<AutocompleteResponse>(
          `/stats-rda/v1/pa/richiedenti?q=${encodeURIComponent(query)}&limit=20`,
        );
        return data.items ?? [];
      } catch {
        return [];
      }
    },
    [api],
  );

  const list = listQ.data;
  const items = list?.items ?? [];
  const total = list?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / DEFAULT_LIMIT));

  return (
    <main className="statsRdaPage">
      <header className="pageHeader">
        <div className="pageHeaderCopy">
          <span className="pageHeaderEyebrow">
            <Icon name="bar-chart-2" size={14} /> Archivio PA
          </span>
          <h1>Archivio ordini di acquisto</h1>
          <p>Cerca e consulta gli ordini dell'archivio storico (PA) per budget, richiedente, fornitore, stato, tipo o periodo.</p>
        </div>
      </header>

      <div className={s.layout}>
        {/* MASTER — filters + list */}
        <section className={`${s.master} surface`}>
          <div className={s.toolbarInner}>
            <div className={`${s.toolbarField} ${s.toolbarSearch}`}>
              <label>Testo</label>
              <SearchInput
                value={q}
                onChange={(v) => updateParam('q', v)}
                placeholder="Cerca per codice, numero ordine o oggetto…"
              />
            </div>
            <div className={s.toolbarField}>
              <label>Budget</label>
              <SingleSelect
                options={budgetOptions}
                selected={budget || null}
                onChange={(v) => updateParam('budget', v ?? '')}
                placeholder="Tutti i budget"
                allowClear
                clearLabel="Tutti i budget"
              />
            </div>
            <div className={s.toolbarField}>
              <label>Stato</label>
              <SingleSelect
                options={statoOptions}
                selected={stato || null}
                onChange={(v) => updateParam('stato', v ?? '')}
                placeholder="Tutti gli stati"
                allowClear
                clearLabel="Tutti gli stati"
              />
            </div>
            <div className={s.toolbarField}>
              <label>Tipo</label>
              <SingleSelect
                options={tipoOptions}
                selected={tipo || null}
                onChange={(v) => updateParam('tipo', v ?? '')}
                placeholder="Tutti i tipi"
                allowClear
                clearLabel="Tutti i tipi"
              />
            </div>
            <div className={s.toolbarField}>
              <label>Valuta</label>
              <SingleSelect
                options={valutaOptions}
                selected={valuta || null}
                onChange={(v) => updateParam('valuta', v ?? '')}
                placeholder="Tutte le valute"
                allowClear
                clearLabel="Tutte le valute"
              />
            </div>
            <div className={s.toolbarField}>
              <label htmlFor="from">Dal</label>
              <input
                id="from"
                className={s.dateInput}
                type="date"
                value={from}
                max={to || undefined}
                onChange={(e) => updateParam('from', e.target.value)}
              />
            </div>
            <div className={s.toolbarField}>
              <label htmlFor="to">Al</label>
              <input
                id="to"
                className={s.dateInput}
                type="date"
                value={to}
                min={from || undefined}
                onChange={(e) => updateParam('to', e.target.value)}
              />
            </div>
            <div className={s.toolbarField}>
              <AutocompleteFilter
                id="fornitore"
                label="Fornitore"
                placeholder="Cerca fornitore…"
                value={fornitore}
                onValueChange={(v) => updateParam('fornitore', v)}
                loadSuggestions={loadFornitori}
              />
            </div>
            <div className={s.toolbarField}>
              <AutocompleteFilter
                id="richiedente"
                label="Richiedente"
                placeholder="Cerca richiedente…"
                value={richiedente}
                onValueChange={(v) => updateParam('richiedente', v)}
                loadSuggestions={loadRichiedenti}
              />
            </div>
            <div className={s.toolbarField}>
              <label>Ordina per</label>
              <SingleSelect
                options={SORT_OPTIONS}
                selected={sort}
                onChange={(v) => updateParam('sort', v ?? 'created')}
                placeholder="Ordina per"
              />
            </div>
            <div className={s.toolbarField}>
              <label>Direzione</label>
              <SingleSelect
                options={[
                  { value: 'desc', label: 'Decrescente' },
                  { value: 'asc', label: 'Crescente' },
                ]}
                selected={dir}
                onChange={(v) => updateParam('dir', v ?? 'desc')}
                placeholder="Direzione"
              />
            </div>
            <div className={s.toolbarActions}>
              {hasAnyFilter && (
                <button type="button" className="filterReset" onClick={clearFilters}>
                  Cancella filtri
                </button>
              )}
            </div>
          </div>

          {!hasAnyFilter && (
            <div className="emptyState">
              <div className="emptyStateIcon"><Icon name="filter" size={32} /></div>
              <p className="emptyStateTitle">Imposta almeno un filtro</p>
              <p className="emptyStateDesc">Cerca per testo, budget, richiedente, fornitore, stato, tipo o periodo per consultare gli ordini.</p>
            </div>
          )}

          {hasAnyFilter && listQ.isLoading && (
            <div className="stateCard"><Skeleton rows={6} /></div>
          )}

          {hasAnyFilter && listQ.error && (
            <div className="stateBlock">
              <div>
                <p className="stateTitle">{apiErrorMessage(listQ.error)}</p>
                <p className="muted">Riprova più tardi.</p>
              </div>
            </div>
          )}

          {hasAnyFilter && !listQ.isLoading && !listQ.error && items.length === 0 && (
            <div className="emptyState">
              <div className="emptyStateIcon"><Icon name="file-text" size={32} /></div>
              <p className="emptyStateTitle">Nessun ordine trovato</p>
              <p className="emptyStateDesc">Modifica i filtri di ricerca e riprova.</p>
            </div>
          )}

          {hasAnyFilter && items.length > 0 && (
            <>
              <div className={s.resultMeta}>
                <span>
                  {total} {total === 1 ? 'ordine' : 'ordini'} · pagina {page} di {totalPages}
                </span>
              </div>
              <div className="tableWrap">
                <table className={s.table}>
                  <thead>
                    <tr>
                      <th>Ordine</th>
                      <th>Richiedente</th>
                      <th>Budget</th>
                      <th>Fornitore</th>
                      <th>Stato</th>
                      <th className="colNum">Importo</th>
                      <th>Creato</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((row, idx) => (
                      <tr
                        key={row.issue_key}
                        className={row.issue_key === issueKey ? s.selected : ''}
                        onClick={() => selectIssue(row.issue_key)}
                        style={{ animationDelay: `${Math.min(idx * 20, 300)}ms` }}
                      >
                        <td>
                          <div className="colKey">{row.issue_key}</div>
                          <div className={s.colSummary}>{row.summary}</div>
                          {row.numero_ordine && <div className="colMuted">{row.numero_ordine}</div>}
                        </td>
                        <td>{nz(row.reporter_name)}</td>
                        <td>{nz(row.budget_di_riferimento)}</td>
                        <td>{nz(row.fornitore_selezionato)}</td>
                        <td>
                          <span className={statusBadgeClass(row.status)}>{nz(row.status)}</span>
                        </td>
                        <td className="colNum">
                          {row.importo_totale != null ? formatEUR(row.importo_totale) : '—'}
                          {row.valuta && <div className="colMuted">{row.valuta}</div>}
                        </td>
                        <td>{shortDate(row.created)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {totalPages > 1 && (
                <div className={s.pagination}>
                  <span className={s.paginationInfo}>
                    {(page - 1) * DEFAULT_LIMIT + 1}–{Math.min(page * DEFAULT_LIMIT, total)} di {total}
                  </span>
                  <div className={s.paginationBtns}>
                    <button
                      type="button"
                      className={s.pagBtn}
                      onClick={() => goToPage(page - 1)}
                      disabled={page <= 1}
                    >
                      ← Precedente
                    </button>
                    <button
                      type="button"
                      className={s.pagBtn}
                      onClick={() => goToPage(page + 1)}
                      disabled={page >= totalPages}
                    >
                      Successiva →
                    </button>
                  </div>
                </div>
              )}
            </>
          )}
        </section>

        {/* DETAIL — 8-section card */}
        <aside className={`${s.detail} surface`}>
          {!issueKey && (
            <div className="emptyState">
              <div className="emptyStateIcon"><Icon name="file-text" size={32} /></div>
              <p className="emptyStateTitle">Seleziona un ordine</p>
              <p className="emptyStateDesc">Scegli un ordine dalla lista per visualizzarne la scheda completa.</p>
            </div>
          )}
          {issueKey && detailQ.isLoading && <div className="stateCard"><Skeleton rows={8} /></div>}
          {issueKey && detailQ.error && (
            <div className="stateBlock">
              <div>
                <p className="stateTitle">{apiErrorMessage(detailQ.error)}</p>
                <p className="muted">Riprova più tardi.</p>
              </div>
            </div>
          )}
          {issueKey && detailQ.data && <IssueDetailPanel data={detailQ.data} />}
        </aside>
      </div>
    </main>
  );
}
