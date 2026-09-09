import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { Button, Icon, MultiSelect, SearchInput, Skeleton, StatusBadge, VisuallyHidden, type StatusBadgeVariant } from '@mrsmith/ui';
import { formatCurrency, formatNumber } from '@mrsmith/format';
import { useEffect, useId, useState } from 'react';
import { Link } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type { MACompanySearchAreas, MACompanySearchResponse, MACompanySearchRow } from '../../api/types';
import { maTagQueryPolicy, useMATagCatalog, withMATagFilter } from '../../hooks/useMATags';
import { bucketLabel } from '../ricerche/helpers';
import { stateLabel, esitoLabel } from '../../lib/cardStates';
import { AziendeSearchForm, companySearchParams, emptyCompanySearch, territorySummary, type CompanySearchDraft } from './AziendeSearchForm';
import styles from './AziendePage.module.css';

type Sort = 'name' | 'turnover' | 'employees';
type Direction = 'asc' | 'desc';

function statusPresentation(status: MACompanySearchRow['status']): {
  label: string;
  detail?: string;
  variant: StatusBadgeVariant;
} {
  switch (status.kind) {
    case 'working':
      return {
        label: `In lavorazione · ${stateLabel(status.value ?? '')}`,
        detail: status.contextTitle,
        variant: 'accent',
      };
    case 'closed': {
      // v2: value = stato terminale (won/ko_nostro/ko_target), reason = esito libero.
      const terminal = stateLabel(status.value ?? '');
      const esito = esitoLabel(status.reason ?? '');
      return {
        label: esito ? `${terminal} · ${esito}` : terminal,
        detail: status.contextTitle,
        variant: status.value === 'won' ? 'success' : 'neutral',
      };
    }
    case 'excluded':
      return { label: 'Esclusa dall’analista', detail: status.reason, variant: 'danger' };
    case 'preferred':
      return { label: `Preferita (${status.value ?? '1'}★)`, variant: 'success' };
    case 'thesis':
      return { label: bucketLabel('principale'), variant: 'success' };
    case 'review':
      return { label: bucketLabel('da_verificare'), detail: status.reason, variant: 'warning' };
    case 'actionable':
      return { label: bucketLabel('azionabile'), detail: status.reason, variant: 'accent' };
    case 'suppressed':
      return { label: bucketLabel('soppresso'), detail: status.reason, variant: 'danger' };
    default:
      return { label: 'Solo anagrafica', variant: 'neutral' };
  }
}

function searchHint(query: string): string {
  const compact = query.trim().toUpperCase().replace(/[.\s]/g, '');
  if (/^(IT)?\d{11}$/.test(compact)) return 'Ricerca esatta per P.IVA';
  if (/^[A-Z0-9]{16}$/.test(compact)) return 'Ricerca esatta per codice fiscale';
  return 'Ricerca parziale per ragione sociale';
}

export function AziendePage() {
  const api = useApiClient();
  const [mode, setMode] = useState<'simple' | 'advanced'>('simple');
  const [query, setQuery] = useState('');
  const [draft, setDraft] = useState(emptyCompanySearch);
  const [appliedDraft, setAppliedDraft] = useState<CompanySearchDraft | null>(null);
  const [request, setRequest] = useState({ filters: 'mode=simple&query=', page: 1, sort: 'name' as Sort, direction: 'asc' as Direction });
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const catalog = useMATagCatalog();
  const queryReady = query.trim().length === 0 || query.trim().length >= 2;

  useEffect(() => {
    if (mode !== 'simple' || !queryReady) return;
    const timeout = window.setTimeout(() => {
      setRequest((previous) => {
        // Il filtro tag vale in entrambe le modalità: riformulare la query
        // semplice conserva i tag dell'ultima ricerca eseguita.
        const tags = new URLSearchParams(previous.filters).getAll('tagId');
        const filters = withMATagFilter(new URLSearchParams({ mode: 'simple', query: query.trim() }).toString(), tags);
        return previous.filters === filters ? previous : { ...previous, filters, page: 1 };
      });
      setAppliedDraft(null);
    }, 300);
    return () => window.clearTimeout(timeout);
  }, [query, mode, queryReady]);

  const params = `${request.filters}&page=${request.page}&pageSize=25&sort=${request.sort}&direction=${request.direction}`;
  const companies = useQuery({
    queryKey: ['ma-companies-search', params],
    queryFn: () => api.get<MACompanySearchResponse>(`/binocolo/v1/ma/companies?${params}`),
    placeholderData: keepPreviousData,
    // L'elenco è toccato dai tag (filtro e rinomine globali): rilettura a
    // apertura vista e ritorno sulla finestra come dalle altre viste.
    ...maTagQueryPolicy,
  });
  const areas = useQuery({
    queryKey: ['ma-company-search-areas'],
    queryFn: () => api.get<MACompanySearchAreas>('/binocolo/v1/ma/companies/areas'),
    enabled: mode === 'advanced',
    staleTime: 5 * 60 * 1000,
  });

  // Le selezioni di tag eliminati dal catalogo (globalmente, anche da un altro
  // utente) cadono solo dopo una lettura RIUSCITA del catalogo: mai durante
  // caricamento o errore, né su dati in attesa di refetch. La caduta aggiorna
  // anche l'ultima ricerca eseguita (pagina 1): un tagId inesistente non deve
  // continuare a filtrare all'insaputa dell'utente.
  useEffect(() => {
    if (!catalog.isSuccess || catalog.isFetching) return;
    const available = new Set(catalog.data?.map((tag) => tag.id) ?? []);
    const next = selectedTags.filter((id) => available.has(id));
    if (next.length === selectedTags.length) return;
    setSelectedTags(next);
    setRequest((previous) => ({ ...previous, filters: withMATagFilter(previous.filters, next), page: 1 }));
  }, [catalog.data, catalog.isSuccess, catalog.isFetching, selectedTags]);

  const items = companies.data?.items ?? [];
  const total = companies.data?.total;
  const page = companies.data?.page ?? request.page;
  const pageCount = Math.max(1, Math.ceil((total ?? 0) / 25));
  const appliedQuery = new URLSearchParams(request.filters).get('query') ?? '';
  const pendingSimple = mode === 'simple' && (query.trim() !== appliedQuery || appliedDraft !== null);
  const searching = companies.isFetching || (pendingSimple && queryReady);
  // Il confronto ignora i tagId (applicati subito): l'avviso riguarda solo i
  // criteri avanzati compilati e non ancora eseguiti con «Cerca».
  const pendingAdvanced = mode === 'advanced' && withMATagFilter(request.filters, []) !== companySearchParams(draft);
  const hasFilters = (appliedDraft !== null ? companySearchParams(appliedDraft) !== 'mode=advanced' : appliedQuery !== '') || selectedTags.length > 0;

  // I tag si applicano subito all'ultima ricerca ESEGUITA (pagina 1); le bozze
  // avanzate non toccate restano subordinate a «Cerca».
  function applyTags(tags: string[]) {
    setSelectedTags(tags);
    setRequest((previous) => {
      const filters = withMATagFilter(previous.filters, tags);
      return previous.filters === filters && previous.page === 1 ? previous : { ...previous, filters, page: 1 };
    });
  }
  function applySearch() {
    setAppliedDraft(draft);
    setRequest((previous) => ({ ...previous, filters: withMATagFilter(companySearchParams(draft), selectedTags), page: 1 }));
  }
  function resetSearch() {
    const empty = emptyCompanySearch();
    setDraft(empty);
    setQuery('');
    setSelectedTags([]);
    setAppliedDraft(mode === 'advanced' ? empty : null);
    setRequest((previous) => ({ ...previous, filters: mode === 'advanced' ? companySearchParams(empty) : 'mode=simple&query=', page: 1 }));
  }
  function sortBy(sort: Sort) {
    setRequest((previous) => ({ ...previous, sort, direction: previous.sort === sort && previous.direction === 'asc' ? 'desc' : 'asc', page: 1 }));
  }
  function sortHeader(sort: Sort, label: string) {
    return <th className={sort === 'name' ? undefined : styles.numeric} aria-sort={request.sort === sort ? (request.direction === 'asc' ? 'ascending' : 'descending') : 'none'}>
      <button type="button" className={styles.sortButton} onClick={() => sortBy(sort)}>
        {label}<Icon name={request.sort === sort && request.direction === 'asc' ? 'chevron-up' : 'chevron-down'} size={14} />
      </button>
    </th>;
  }

  // Il controllo tag vive dentro la riga del campo che lo accompagna (ricerca
  // libera in modalità semplice, NDA in avanzata): stessa struttura campo
  // etichetta + controllo + nota, così le due colonne restano allineate.
  const tagFilterId = useId();
  const tagFilter = (
    <div className={styles.tagFilter} role="group" aria-labelledby={tagFilterId}>
      <span className={styles.fieldLabel} id={tagFilterId}>Tag aziendali</span>
      {catalog.isLoading ? <div className={styles.tagFilterSkeleton}><Skeleton rows={1} /></div>
        : catalog.isError ? <div className={styles.tagCatalogError} role="alert">Tag non disponibili. <Button type="button" variant="ghost" onClick={() => void catalog.refetch()}>Riprova</Button></div>
        : <MultiSelect options={(catalog.data ?? []).map((tag) => ({ value: tag.id, label: tag.name }))} selected={selectedTags} onChange={applyTags} placeholder="Filtra per tag aziendali" />}
      <div className={styles.tagFooter}>
        <p className={styles.hint}>Mostra solo le aziende che hanno tutti i tag selezionati.</p>
        {selectedTags.length > 0 && <Button type="button" variant="ghost" size="sm" onClick={() => applyTags([])}>Azzera tag</Button>}
      </div>
    </div>
  );

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1>Aziende</h1>
        <p>Cerca nel corpus interno per ritrovare una società o selezionare aziende con caratteristiche comuni.</p>
      </header>
      <section className={styles.searchPanel} aria-label="Ricerca aziende">
        <div className={styles.searchMode} role="group" aria-label="Modalità di ricerca">
          <Button variant={mode === 'simple' ? 'primary' : 'ghost'} aria-pressed={mode === 'simple'} onClick={() => setMode('simple')}>Ricerca semplice</Button>
          <Button variant={mode === 'advanced' ? 'primary' : 'ghost'} aria-pressed={mode === 'advanced'} onClick={() => setMode('advanced')}>Ricerca avanzata</Button>
        </div>
        {mode === 'simple' ? <div className={styles.simpleSearch}>
          <div className={styles.searchRow}>
            <div className={styles.searchField}>
              <span className={styles.fieldLabel}>Ricerca libera</span>
              <SearchInput value={query} onChange={setQuery} placeholder="Ragione sociale, P.IVA o codice fiscale" ariaLabel="Cerca un’azienda nel corpus analizzato" autoFocus />
              <p className={styles.hint}>{queryReady ? searchHint(query) : 'Inserisci almeno 2 caratteri.'}</p>
            </div>
            {tagFilter}
          </div>
        </div> : <AziendeSearchForm draft={draft} onChange={setDraft} onSearch={applySearch} onReset={() => setDraft(emptyCompanySearch())} areas={areas.data} areasLoading={areas.isLoading} areasError={areas.isError} onRetryAreas={() => void areas.refetch()} tagFilter={tagFilter} />}
      </section>

      <section className={styles.results} aria-busy={searching} aria-label="Risultati della ricerca">
        <div className={styles.resultsHeader}>
          <div>
            <h2>{total === undefined ? 'Totale corrispondenze: —' : `${formatNumber(total)} ${total === 1 ? 'azienda' : 'aziende'}`}</h2>
            <p>{appliedDraft ? `Ricerca avanzata · ${territorySummary(appliedDraft, areas.data?.items ?? [])}` : appliedQuery ? `Ricerca semplice · “${appliedQuery}”` : 'Tutte le aziende del corpus'}</p>
          </div>
          {searching && <span className={styles.hint} role="status">Aggiornamento…</span>}
        </div>
        {pendingAdvanced && <p className={styles.pendingNotice}>Premi «Cerca» per applicare i criteri impostati. La tabella mostra l’ultima ricerca eseguita.</p>}
        {mode === 'simple' && !queryReady && <p className={styles.pendingNotice}>Completa la ricerca per aggiornare le corrispondenze.</p>}
        <VisuallyHidden role="status" aria-live="polite" aria-atomic="true">{!searching && total !== undefined ? `${total} corrispondenze, pagina ${page} di ${pageCount}` : ''}</VisuallyHidden>
        {companies.isLoading ? <div className={styles.loading}><Skeleton rows={7} /></div>
          : companies.isError ? <div className={`${styles.emptyState} ${styles.errorState}`} role="alert">
            <span className={styles.emptyIcon}><Icon name="triangle-alert" size={32} /></span>
            <h3>Ricerca non disponibile</h3><p>Il corpus non può essere caricato in questo momento. Riprova.</p>
            <Button onClick={() => void companies.refetch()}>Riprova</Button>
          </div>
          : items.length === 0 ? <div className={styles.emptyState}>
            <span className={styles.emptyIcon}><Icon name="search" size={32} /></span>
            <h3>{hasFilters ? 'Nessuna azienda trovata' : 'Nessuna azienda nel corpus'}</h3>
            <p>{hasFilters ? 'Modifica i criteri oppure azzera la ricerca per consultare tutto il corpus.' : 'Le società compariranno qui dopo la prima ricerca o lavorazione.'}</p>
            {hasFilters && <Button onClick={resetSearch}>Azzera ricerca</Button>}
          </div>
          : <>
            <div className={styles.tableScroll}>
              <table className={styles.table}>
                <VisuallyHidden as="caption">Aziende corrispondenti alla ricerca</VisuallyHidden>
                <thead><tr>{sortHeader('name', 'Azienda')}<th>Località</th>{sortHeader('turnover', 'Fatturato')}{sortHeader('employees', 'Dipendenti')}<th>Stato</th></tr></thead>
                <tbody>{items.map((company) => {
                  const status = statusPresentation(company.status);
                  return <tr key={company.primaryCompanyKey}>
                    <td className={styles.companyCell}>
                      <span className={styles.accentBar} aria-hidden="true" />
                      <Link to={`/aziende/${encodeURIComponent(company.primaryCompanyKey)}`} className={styles.companyLink}>{company.companyName || 'Azienda senza denominazione'}</Link>
                      <span className={styles.cellDetail}>{company.vatCode ? `P.IVA ${company.vatCode}` : company.taxCode ? `CF ${company.taxCode}` : 'Identificativo fiscale non disponibile'}</span>
                    </td>
                    <td>{[company.town, company.province].filter(Boolean).join(' · ') || 'Non disponibile'}</td>
                    <td className={styles.numeric}>{formatCurrency(company.turnover, 'EUR', { format: { maximumFractionDigits: 0 } }) ?? '—'}<span className={styles.cellDetail}>{company.turnover == null ? 'Non disponibile' : company.turnoverYear ?? 'Anno non disponibile'}</span></td>
                    <td className={styles.numeric}>{formatNumber(company.employees) ?? '—'}<span className={styles.cellDetail}>{company.employees == null ? 'Non disponibile' : company.employeesYear ?? 'Anno non disponibile'}</span></td>
                    <td><StatusBadge value={company.status.kind} label={status.label} variant={status.variant} />{status.detail && <span className={styles.cellDetail}>{status.detail}</span>}</td>
                  </tr>;
                })}</tbody>
              </table>
            </div>
            <nav className={styles.pagination} aria-label="Pagine dei risultati">
              <p>Pagina {formatNumber(page)} di {formatNumber(pageCount)} · {formatNumber((page - 1) * 25 + 1)}–{formatNumber(Math.min(page * 25, total ?? 0))} di {formatNumber(total)}</p>
              <div className={styles.paginationActions}>
                <Button variant="secondary" disabled={page <= 1 || searching} onClick={() => setRequest((previous) => ({ ...previous, page: page - 1 }))} leftIcon={<Icon name="chevron-left" size={16} />}>Precedente</Button>
                <Button variant="secondary" disabled={page >= pageCount || searching} onClick={() => setRequest((previous) => ({ ...previous, page: page + 1 }))} rightIcon={<Icon name="chevron-right" size={16} />}>Successiva</Button>
              </div>
            </nav>
          </>}
      </section>
    </div>
  );
}
