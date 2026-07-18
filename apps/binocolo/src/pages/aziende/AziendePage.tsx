import { useQuery } from '@tanstack/react-query';
import { Button, Icon, SearchInput, Skeleton, StatusBadge, type StatusBadgeVariant } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type { MACompanySearchResponse, MACompanySearchRow } from '../../api/types';
import { dateTimeLabel, relativeDate } from '../ricerche/helpers';
import styles from './AziendePage.module.css';

const cardStateLabels: Record<string, string> = {
  da_contattare: 'Da contattare',
  contattata: 'Contattata',
  in_dialogo: 'In dialogo',
  approfondimento: 'Approfondimento',
  offerta: 'Offerta',
};

const outcomeLabels: Record<string, string> = {
  conclusa: 'Conclusa',
  no_go: 'No go',
  non_idonea: 'Non idonea',
  sfumata: 'Sfumata',
  rimandata: 'Rimandata',
};

function statusPresentation(status: MACompanySearchRow['status']): {
  label: string;
  detail?: string;
  variant: StatusBadgeVariant;
} {
  switch (status.kind) {
    case 'working':
      return {
        label: `In lavorazione · ${cardStateLabels[status.value ?? ''] ?? status.value ?? ''}`,
        detail: status.contextTitle,
        variant: 'accent',
      };
    case 'closed':
      return {
        label: `Lavorazione chiusa${status.value ? ` · ${outcomeLabels[status.value] ?? status.value}` : ''}`,
        detail: status.contextTitle,
        variant: 'neutral',
      };
    case 'excluded':
      return { label: 'Esclusa dall’analista', detail: status.reason, variant: 'danger' };
    case 'preferred':
      return { label: `Preferita (${status.value ?? '1'}★)`, variant: 'success' };
    case 'thesis':
      return { label: 'In tesi', variant: 'success' };
    case 'review':
      return { label: 'Da verificare', detail: status.reason, variant: 'warning' };
    case 'actionable':
      return { label: 'Da rivedere', detail: status.reason, variant: 'accent' };
    case 'suppressed':
      return { label: 'Soppressa', detail: status.reason, variant: 'danger' };
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
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const queryReady = query.trim().length === 0 || query.trim().length >= 2;

  useEffect(() => {
    const timeout = window.setTimeout(() => setDebouncedQuery(query.trim()), 300);
    return () => window.clearTimeout(timeout);
  }, [query]);

  const companies = useQuery({
    queryKey: ['ma-companies-search', debouncedQuery],
    queryFn: () =>
      api.get<MACompanySearchResponse>(
        `/binocolo/v1/ma/companies?query=${encodeURIComponent(debouncedQuery)}`,
      ),
    enabled: debouncedQuery.length === 0 || debouncedQuery.length >= 2,
  });

  const items = companies.data?.items ?? [];
  const searching = query.trim() !== debouncedQuery || companies.isFetching;
  const hasQuery = debouncedQuery.length > 0;

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <span className={styles.eyebrow}>Corpus analizzato</span>
        <h1>Aziende</h1>
        <p>
          Ritrova le società già emerse nelle ricerche o nelle iniziative, senza interrogare fonti esterne.
        </p>
      </header>

      <section className={styles.searchPanel} aria-labelledby="aziende-search-title">
        <div className={styles.searchCopy}>
          <h2 id="aziende-search-title">Cerca nel corpus</h2>
          <p>{searchHint(query)}</p>
        </div>
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder="Ragione sociale, P.IVA o codice fiscale"
          ariaLabel="Cerca un’azienda nel corpus analizzato"
          autoFocus
        />
        {!queryReady ? <p className={styles.queryHint}>Inserisci almeno 2 caratteri.</p> : null}
      </section>

      <section className={styles.results} aria-busy={searching} aria-live="polite">
        <div className={styles.resultsHeader}>
          <div>
            <h2>{hasQuery ? 'Risultati' : 'Viste di recente'}</h2>
            <p>
              {hasQuery
                ? `Corrispondenze interne per “${debouncedQuery}”`
                : 'Ultime società comparse in una ricerca o iniziativa'}
            </p>
          </div>
          {searching ? <span className={styles.refreshing}>Aggiornamento…</span> : null}
        </div>

        {companies.isLoading ? (
          <div className={styles.loading}><Skeleton rows={7} /></div>
        ) : companies.isError ? (
          <div className={`${styles.emptyState} ${styles.errorState}`} role="alert">
            <span className={styles.emptyIcon}><Icon name="triangle-alert" size={32} /></span>
            <h3>Ricerca non disponibile</h3>
            <p>Il corpus non può essere caricato in questo momento. Riprova.</p>
            <Button variant="secondary" onClick={() => void companies.refetch()} leftIcon={<Icon name="refresh-cw" size={16} />}>
              Riprova
            </Button>
          </div>
        ) : !queryReady ? (
          <div className={styles.pendingState}>Completa la ricerca per vedere le corrispondenze.</div>
        ) : items.length === 0 ? (
          <div className={styles.emptyState}>
            <span className={styles.emptyIcon}><Icon name="search" size={32} /></span>
            <h3>{hasQuery ? 'Nessuna azienda trovata' : 'Nessuna azienda nel corpus'}</h3>
            <p>
              {hasQuery
                ? 'Prova con una parte diversa della ragione sociale oppure verifica P.IVA e codice fiscale.'
                : 'Le società compariranno qui dopo la prima ricerca o lavorazione.'}
            </p>
            {hasQuery ? <Button variant="secondary" onClick={() => setQuery('')}>Azzera ricerca</Button> : null}
          </div>
        ) : (
          <div className={styles.list}>
            {items.map((company) => {
              const status = statusPresentation(company.status);
              const location = [company.town, company.province].filter(Boolean).join(' · ');
              return (
                <Link
                  className={styles.companyRow}
                  key={`${company.primaryCompanyKey}-${company.vatCode ?? company.taxCode ?? ''}`}
                  to={`/aziende/${encodeURIComponent(company.primaryCompanyKey)}`}
                >
                  <span className={styles.accentBar} aria-hidden="true" />
                  <span className={styles.companyMain}>
                    <span className={styles.companyHeading}>
                      <strong>{company.companyName || 'Azienda senza denominazione'}</strong>
                      <StatusBadge value={company.status.kind} label={status.label} variant={status.variant} />
                    </span>
                    <span className={styles.identityLine}>
                      {company.vatCode ? <span>P.IVA {company.vatCode}</span> : null}
                      {company.taxCode && company.taxCode !== company.vatCode ? <span>CF {company.taxCode}</span> : null}
                      {location ? <span>{location}</span> : null}
                      {company.domain ? <span className={styles.domain}>{company.domain}</span> : null}
                    </span>
                    {status.detail ? <span className={styles.statusDetail}>{status.detail}</span> : null}
                  </span>

                  <span className={styles.history}>
                    <span className={styles.lastContext}>
                      {company.lastContext ? (
                        <>
                          <span>{company.lastContext.type === 'ricerca' ? 'Ricerca' : 'Iniziativa'}</span>
                          <strong>{company.lastContext.title || 'Senza titolo'}</strong>
                        </>
                      ) : (
                        <strong>Registro azienda</strong>
                      )}
                    </span>
                    <time dateTime={company.lastSeenAt} title={dateTimeLabel(company.lastSeenAt)}>
                      {relativeDate(company.lastSeenAt)}
                    </time>
                  </span>

                  <span className={styles.meta}>
                    <span>{company.sessionCount} {company.sessionCount === 1 ? 'ricerca' : 'ricerche'}</span>
                    <span>{company.initiativeCount} {company.initiativeCount === 1 ? 'iniziativa' : 'iniziative'}</span>
                    {company.hasDeep ? <span className={styles.deepBadge}>Deep disponibile</span> : null}
                    {company.companyKeys.length > 1 ? <span>{company.companyKeys.length} schede</span> : null}
                  </span>

                  <Icon className={styles.openIcon} name="arrow-right" size={18} />
                </Link>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
