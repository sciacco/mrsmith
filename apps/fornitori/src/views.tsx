import { ApiError } from '@mrsmith/api-client';
import { Button, Drawer, Icon, type IconName, Modal, MultiSelect, SearchInput, SingleSelect, Skeleton, StatusBadge, TabNav, ToggleSwitch, useToast, type StatusBadgeVariant } from '@mrsmith/ui';
import { useEffect, useId, useMemo, useState } from 'react';
import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import {
  useArticleCategories,
  useAlyanteSuppliers,
  useCategories,
  useCountries,
  useDashboard,
  useDocumentTypes,
  useFornitoriMutations,
  usePaymentMethods,
  useProvider,
  useProviderCategories,
  useProviderDocuments,
  useProviderSummary,
} from './api/queries';
import type { AlyanteSupplier, Category, CategoryDocumentType, Country, DashboardCategory, DocumentType, PaymentMethod, Provider, ProviderCategory, ProviderDocument, ProviderPayload, ProviderReference, ProviderSummary } from './api/types';
import { provinceSelectOptions } from './lib/provinces';
import {
  PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE,
  PROVIDER_REFERENCE_PHONE_PATTERN,
  QUALIFICATION_REFERENCE_TYPE,
  isValidOptionalProviderRefPhone,
  stateLabel,
} from './lib/reference';
import { hasProviderErp, providerStateSelectOptions } from './lib/providerState';
import { saveBlob } from './lib/download';
import { useHasRole } from './hooks/useHasRole';
import {
  PROVIDER_ATTENTION_LABELS,
  buildDashboardProviderAttention,
  buildDetailProviderAttention,
  buildPrioritySummary,
  missingProviderActivationFields,
  type ProviderAttention,
  type ProviderAttentionAction,
  type ProviderAttentionSeverity,
  type PrioritySummary,
} from './lib/providerAttention';
import {
  legacyProviderDetailPath,
  normalizeProviderSection,
  providerDetailPath,
  type ProviderDetailSection,
} from './lib/providerRoutes';
import {
  CategoryStateBadge,
  daysUntilExpiry,
  DocumentStateBadge,
  DocumentUrgencyBadge,
  ProviderStateBadge,
} from './lib/badges';

function errorTitle(error: unknown) {
  if (error instanceof ApiError) {
    if (error.status === 403) return 'Accesso non consentito';
    if (error.status === 503) return 'Servizio temporaneamente non disponibile';
  }
  return 'Dati non disponibili';
}

function apiErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error;
    return error.message;
  }
  return fallback;
}

function stateBlock(title: string, message: string, iconName?: IconName) {
  return (
    <div className="emptyState">
      {iconName ? (
        <span className="emptyIcon" aria-hidden="true">
          <Icon name={iconName} size={28} />
        </span>
      ) : null}
      <p className="emptyTitle">{title}</p>
      <p className="emptyText">{message}</p>
    </div>
  );
}

function value(value?: string | number | boolean | null) {
  if (value === null || value === undefined || value === '') return '-';
  return String(value);
}

function dateLabel(raw?: string | null) {
  if (!raw) return '-';
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return raw.slice(0, 10);
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'medium' }).format(parsed);
}

function dateInputValue(raw?: string | null) {
  if (!raw) return '';
  const value = raw.slice(0, 10);
  return /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : '';
}

function getProviderRefs(provider?: Provider): ProviderReference[] {
  if (!provider) return [];
  if (provider.refs?.length) return provider.refs;
  return provider.ref ? [provider.ref] : [];
}

function selectOptions(items: Category[] | DocumentType[] | undefined) {
  return items?.map((item) => ({ value: item.id, label: item.name })) ?? [];
}

const languageOptions = [
  { value: 'it', label: 'Italiano' },
  { value: 'en', label: 'Inglese' },
];

function countrySelectOptions(countries: Country[] | undefined, current?: string | null) {
  const options = countries?.map((item) => ({ value: item.code, label: item.name })) ?? [];
  if (current && !options.some((item) => item.value === current)) {
    return [{ value: current, label: current }, ...options];
  }
  return options;
}

function ensureSelectedOption(options: { value: string; label: string }[], selected?: string | null) {
  if (!selected || options.some((item) => item.value === selected)) return options;
  return [{ value: selected, label: stateLabel(selected) }, ...options];
}

function parseErpId(raw?: string | null) {
  const trimmed = raw?.trim() ?? '';
  if (!trimmed) return null;
  const parsed = Number(trimmed);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
}

function hasInvalidErpId(raw?: string | null) {
  const trimmed = raw?.trim() ?? '';
  return trimmed !== '' && parseErpId(trimmed) === null;
}

function providerPayload(form: HTMLFormElement): ProviderPayload {
  const data = new FormData(form);
  const erp = String(data.get('erp_id') ?? '').trim();
  const country = String(data.get('country') || 'IT');
  const payload: ProviderPayload = {
    company_name: String(data.get('company_name') ?? '').trim(),
    state: String(data.get('state') || 'DRAFT'),
    vat_number: String(data.get('vat_number') ?? '').trim() || undefined,
    cf: String(data.get('cf') ?? '').trim() || undefined,
    address: String(data.get('address') ?? '').trim() || undefined,
    city: String(data.get('city') ?? '').trim() || undefined,
    postal_code: String(data.get('postal_code') ?? '').trim() || undefined,
    province: String(data.get('province') ?? '').trim() || undefined,
    erp_id: parseErpId(erp),
    language: String(data.get('language') || 'it'),
    country,
    default_payment_method: String(data.get('default_payment_method') ?? '') || null,
    ref: {
      first_name: String(data.get('ref_first_name') ?? '').trim(),
      last_name: String(data.get('ref_last_name') ?? '').trim(),
      email: String(data.get('ref_email') ?? '').trim(),
      phone: String(data.get('ref_phone') ?? '').trim(),
      reference_type: QUALIFICATION_REFERENCE_TYPE,
    },
  };
  if (data.get('skip_qualification_validation') === 'on') {
    payload.skip_qualification_validation = true;
  }
  return payload;
}

function validateProvider(payload: ProviderPayload): string | null {
  if (!payload.company_name) return 'Inserisci la ragione sociale';
  if (payload.country === 'IT' && !payload.cf && !payload.vat_number) return 'Per i fornitori italiani inserisci CF o P.IVA';
  if (payload.country === 'IT' && (payload.postal_code?.length ?? 0) < 5) return 'Per i fornitori italiani inserisci un CAP valido';
  if (payload.country === 'IT' && !payload.province) return 'Per i fornitori italiani seleziona la provincia';
  if ((payload.state === 'ACTIVE' || payload.state === 'INACTIVE') && !payload.erp_id) return 'Inserisci il codice Alyante prima di cambiare stato';
  return null;
}

interface CategoryGroup {
  provider_id: number;
  company_name: string;
  categories: { id: number; name: string; state: string; critical: boolean }[];
}

const PROVIDER_ATTENTION_BADGE: Record<Exclude<ProviderAttentionSeverity, 'none'>, StatusBadgeVariant> = {
  blocking: 'accent',
  expired: 'danger',
  expiring: 'warning',
  completion: 'neutral',
};

function groupCategoriesByProvider(rows: DashboardCategory[]): CategoryGroup[] {
  const groups = new Map<number, CategoryGroup>();
  for (const row of rows) {
    const existing = groups.get(row.provider_id);
    const entry = {
      id: row.category_id,
      name: row.category_name ?? '—',
      state: row.state ?? 'NEW',
      critical: row.critical,
    };
    if (existing) existing.categories.push(entry);
    else groups.set(row.provider_id, { provider_id: row.provider_id, company_name: row.company_name ?? '—', categories: [entry] });
  }
  return Array.from(groups.values());
}

function anomalyCountLabel(count: number) {
  return count === 1 ? '1 anomalia' : `${count} anomalie`;
}

function matchesProviderAnomalyQuery(card: ProviderAttention, query: string) {
  const terms = query.toLocaleLowerCase('it').trim().split(/\s+/).filter(Boolean);
  if (terms.length === 0) return true;
  const haystack = [
    card.companyName,
    PROVIDER_ATTENTION_LABELS[card.severity],
    anomalyCountLabel(card.openCount),
    ...card.actions.map((action) => action.detail),
  ].join(' ').toLocaleLowerCase('it');
  return terms.every((term) => haystack.includes(term));
}

function ProviderMissingConsole({
  cards,
  onOpen,
}: {
  cards: ProviderAttention[];
  onOpen: (href: string) => void;
}) {
  const visibleCards = cards.slice(0, 8);

  return (
    <section className="providerMissingConsole" aria-labelledby="provider-missing-title">
      <header className="providerMissingHeader">
        <div>
          <h2 id="provider-missing-title">Fornitori che richiedono attenzione</h2>
        </div>
        <span className="providerMissingCount">{cards.length > 0 ? `${visibleCards.length} in evidenza su ${cards.length}` : 'Tutto in ordine'}</span>
      </header>

      {cards.length === 0 ? (
        <div className="priorityEmpty">
          <span className="priorityEmptyIcon" aria-hidden="true"><Icon name="check-circle" size={20} /></span>
          <div>
            <strong>Nessun fornitore con mancanze</strong>
            <span>Le code fornitori non evidenziano blocchi o completamenti aperti.</span>
          </div>
        </div>
      ) : (
        <div className="providerMissingGrid">
          {visibleCards.map((card) => (
            <ProviderMissingCard key={card.providerId} card={card} onOpen={onOpen} />
          ))}
        </div>
      )}
    </section>
  );
}

function ProviderMissingCard({
  card,
  onOpen,
}: {
  card: ProviderAttention;
  onOpen: (href: string) => void;
}) {
  const label = PROVIDER_ATTENTION_LABELS[card.severity];
  const variant = card.severity === 'none' ? 'success' : PROVIDER_ATTENTION_BADGE[card.severity];
  const details = card.actions.slice(0, 3).map((action) => action.detail);
  return (
    <button
      type="button"
      className="providerMissingCard"
      data-classification={card.severity}
      onClick={() => onOpen(card.href)}
    >
      <span className="providerMissingTopline">
        <StatusBadge value={label} label={label} variant={variant} />
        <span>{anomalyCountLabel(card.openCount)}</span>
      </span>
      <span className="providerMissingName">{card.companyName}</span>
      <span className="providerMissingDetails">
        {details.map((detail, index) => (
          <span key={`${detail}-${index}`}>{detail}</span>
        ))}
      </span>
      <span className="providerMissingAction">
        {card.actionLabel}
        <Icon name="chevron-right" size={16} />
      </span>
    </button>
  );
}

function ProviderAnomalyList({
  cards,
  onOpen,
}: {
  cards: ProviderAttention[];
  onOpen: (href: string) => void;
}) {
  const [query, setQuery] = useState('');
  const filteredCards = useMemo(
    () => cards.filter((card) => matchesProviderAnomalyQuery(card, query)),
    [cards, query],
  );
  const counter = query.trim()
    ? `${filteredCards.length} di ${cards.length} fornitori`
    : cards.length === 1 ? '1 fornitore' : `${cards.length} fornitori`;

  return (
    <section className="providerAnomalyPanel" aria-labelledby="provider-anomaly-title">
      <header className="providerAnomalyHeader">
        <div>
          <h2 id="provider-anomaly-title">Tutti i fornitori con anomalie</h2>
        </div>
        {cards.length > 0 ? (
          <SearchInput
            className="providerAnomalySearch"
            value={query}
            onChange={setQuery}
            placeholder="Cerca fornitore"
          />
        ) : null}
        <span className="providerAnomalyTotal">{counter}</span>
      </header>

      {cards.length === 0 ? (
        <div className="priorityEmpty providerAnomalyEmpty">
          <span className="priorityEmptyIcon" aria-hidden="true"><Icon name="check-circle" size={20} /></span>
          <div>
            <strong>Nessun fornitore con anomalie</strong>
            <span>Le code fornitori non evidenziano blocchi o completamenti aperti.</span>
          </div>
        </div>
      ) : filteredCards.length === 0 ? (
        <div className="priorityEmpty providerAnomalyEmpty">
          <span className="priorityEmptyIcon" aria-hidden="true"><Icon name="search" size={20} /></span>
          <div>
            <strong>Nessun fornitore trovato</strong>
            <span>Modifica la ricerca per tornare alla lista completa.</span>
          </div>
        </div>
      ) : (
        <div className="providerAnomalyRows">
          {filteredCards.map((card) => (
            <ProviderAnomalyRow key={card.providerId} card={card} onOpen={onOpen} />
          ))}
        </div>
      )}
    </section>
  );
}

function ProviderAnomalyRow({
  card,
  onOpen,
}: {
  card: ProviderAttention;
  onOpen: (href: string) => void;
}) {
  const label = PROVIDER_ATTENTION_LABELS[card.severity];
  const variant = card.severity === 'none' ? 'success' : PROVIDER_ATTENTION_BADGE[card.severity];
  const details = card.actions.slice(0, 3).map((action) => action.detail);
  return (
    <button
      type="button"
      className="providerAnomalyRow"
      onClick={() => onOpen(card.href)}
    >
      <span className="providerAnomalySeverity">
        <StatusBadge value={label} label={label} variant={variant} />
      </span>
      <span className="providerAnomalyName">{card.companyName}</span>
      <span className="providerAnomalyCount">{anomalyCountLabel(card.openCount)}</span>
      <span className="providerAnomalyDetails">{details.length > 0 ? details.join(' · ') : '-'}</span>
      <span className="providerAnomalyAction">
        {card.actionLabel}
        <Icon name="chevron-right" size={16} />
      </span>
    </button>
  );
}

function DashboardSummaryStrip({
  summary,
  providerCount,
}: {
  summary: PrioritySummary;
  providerCount: number;
}) {
  const providerLabel = providerCount === 1 ? '1 fornitore con anomalie' : `${providerCount} fornitori con anomalie`;
  const draftLabel = `${summary.drafts} in bozza`;
  const overdueLabel = summary.overdue === 1 ? '1 documento scaduto' : `${summary.overdue} documenti scaduti`;
  const expiringDocumentsLabel = summary.expiring === 1
    ? '1 documento in scadenza'
    : `${summary.expiring} documenti in scadenza`;
  const documentLabel = summary.overdue > 0
    ? `${overdueLabel}, ${summary.expiring} in scadenza`
    : expiringDocumentsLabel;
  const categoriesLabel = summary.openCategories === 1
    ? '1 categoria da completare'
    : `${summary.openCategories} categorie da completare`;

  return (
    <div
      className="dashboardSummary"
      aria-label={`Riepilogo anomalie dashboard: ${providerLabel}, ${draftLabel}, ${documentLabel}, ${categoriesLabel}`}
    >
      <span className="dashboardSummaryGroup">
        <span className="dashboardSummaryMetric">
          <strong>{providerCount}</strong>
          {providerCount === 1 ? ' fornitore con anomalie' : ' fornitori con anomalie'}
        </span>
        <span className="dashboardSummarySub">
          <strong>{summary.drafts}</strong>
          {' in bozza'}
        </span>
      </span>
      <span className="dashboardSummaryGroup">
        {summary.overdue > 0 ? (
          <>
            <span className="dashboardSummaryMetric dashboardSummaryMetric--danger">
              <strong>{summary.overdue}</strong>
              {summary.overdue === 1 ? ' documento scaduto' : ' documenti scaduti'}
            </span>
            <span className="dashboardSummarySub dashboardSummarySub--warning">
              <strong>{summary.expiring}</strong>
              {' in scadenza'}
            </span>
          </>
        ) : (
          <span className="dashboardSummaryMetric dashboardSummaryMetric--warning">
            <strong>{summary.expiring}</strong>
            {summary.expiring === 1 ? ' documento in scadenza' : ' documenti in scadenza'}
          </span>
        )}
      </span>
      <span className="dashboardSummaryGroup">
        <span className="dashboardSummaryMetric">
          <strong>{summary.openCategories}</strong>
          {summary.openCategories === 1 ? ' categoria da completare' : ' categorie da completare'}
        </span>
      </span>
    </div>
  );
}

export function DashboardPage() {
  const navigate = useNavigate();
  const [draftsOpen, setDraftsOpen] = useState(false);
  const [documentsOpen, setDocumentsOpen] = useState(false);
  const [categoriesOpen, setCategoriesOpen] = useState(false);
  const { drafts, documents, categories } = useDashboard();
  const groupedCategories = useMemo(() => groupCategoriesByProvider(categories.data ?? []), [categories.data]);
  const prioritySummary = useMemo(
    () => buildPrioritySummary(documents.data ?? [], categories.data ?? [], drafts.data ?? []),
    [categories.data, documents.data, drafts.data],
  );
  const providerPriorityCards = useMemo(
    () => buildDashboardProviderAttention({
      documents: documents.data ?? [],
      categories: categories.data ?? [],
      drafts: drafts.data ?? [],
    }),
    [categories.data, documents.data, drafts.data],
  );
  const loading = drafts.isLoading || documents.isLoading || categories.isLoading;
  const error = drafts.error ?? documents.error ?? categories.error;

  return (
    <main className="page">
      <header className="pageHeader dashboardHeader">
        <div>
          <h1>Dashboard</h1>
          {!loading && !error ? (
            <DashboardSummaryStrip summary={prioritySummary} providerCount={providerPriorityCards.length} />
          ) : (
            <p>Fornitori da qualificare, documenti in scadenza e categorie da gestire.</p>
          )}
        </div>
      </header>
      {loading ? <Skeleton rows={8} /> : error ? stateBlock(errorTitle(error), 'Le attività fornitori non possono essere caricate.', 'triangle-alert') : (
        <>
          <ProviderMissingConsole cards={providerPriorityCards} onOpen={(href) => navigate(href)} />
          <ProviderAnomalyList cards={providerPriorityCards} onOpen={(href) => navigate(href)} />
          <Panel
            title="Da qualificare"
            subtitle="Fornitori in stato bozza"
            count={drafts.data?.length ?? 0}
            collapsible
            open={draftsOpen}
            onToggle={() => setDraftsOpen((open) => !open)}
          >
            {(drafts.data ?? []).length === 0 ? stateBlock('Nessun fornitore in attesa', 'Tutti i fornitori arrivati da Mistra sono stati lavorati.', 'check-circle') : (
              <div className="tableScroll dashboardScroll">
                <table className="table">
                  <thead><tr><th>Ragione sociale</th><th>P.IVA</th><th>CF</th><th /></tr></thead>
                  <tbody>{(drafts.data ?? []).map((row) => (
                    <tr key={row.id} onClick={() => navigate(providerDetailPath(row.id, 'dati'))}>
                      <td>{value(row.company_name)}</td>
                      <td>{value(row.vat_number)}</td>
                      <td>{value(row.cf)}</td>
                      <td className="iconCell"><Icon name="chevron-right" size={16} /></td>
                    </tr>
                  ))}</tbody>
                </table>
              </div>
            )}
          </Panel>
          <Panel
            title="Documenti in scadenza"
            subtitle="Scadenza ≤30 giorni, fornitori in bozza o attivi"
            count={documents.data?.length ?? 0}
            collapsible
            open={documentsOpen}
            onToggle={() => setDocumentsOpen((open) => !open)}
          >
            {(documents.data ?? []).length === 0 ? stateBlock('Nessun documento in scadenza', 'Tutti i documenti dei fornitori attivi sono in regola.', 'check-circle') : (
              <div className="tableScroll dashboardScroll">
                <table className="table">
                  <thead><tr><th>Fornitore</th><th>Documento</th><th>Scadenza</th><th>Urgenza</th><th /></tr></thead>
                  <tbody>{(documents.data ?? []).map((row) => (
                    <tr key={row.id} onClick={() => navigate(providerDetailPath(row.provider_id, 'documenti', `document-${row.id}`))}>
                      <td>{value(row.company_name)}</td>
                      <td>{value(row.document_type)}</td>
                      <td>{dateLabel(row.expire_date)}</td>
                      <td><DocumentUrgencyBadge expireDate={row.expire_date} days={row.days_remaining} /></td>
                      <td className="iconCell"><Icon name="chevron-right" size={16} /></td>
                    </tr>
                  ))}</tbody>
                </table>
              </div>
            )}
          </Panel>
          <Panel
            title="Categorie aperte"
            subtitle="Categorie da qualificare per fornitori in bozza o attivi"
            count={groupedCategories.length}
            countSuffix="fornitori"
            collapsible
            open={categoriesOpen}
            onToggle={() => setCategoriesOpen((open) => !open)}
          >
            {groupedCategories.length === 0 ? stateBlock('Nessuna categoria aperta', 'Tutte le categorie assegnate sono qualificate.', 'check-circle') : (
              <div className="tableScroll dashboardScroll">
                <table className="table">
                  <thead><tr><th>Fornitore</th><th>Categorie aperte</th><th /></tr></thead>
                  <tbody>{groupedCategories.map((group) => {
                    const firstCategory = group.categories[0];
                    return (
                      <tr
                        key={group.provider_id}
                        onClick={() => navigate(providerDetailPath(
                          group.provider_id,
                          'qualifica',
                          firstCategory ? `category-${firstCategory.id}` : null,
                        ))}
                      >
                        <td>{group.company_name}</td>
                        <td>
                          <div className="categoryChips">
                            {group.categories.map((category) => (
                              <span
                                key={category.id}
                                className="categoryChip"
                                data-state={category.state}
                                data-critical={category.critical ? 'true' : 'false'}
                                title={category.critical ? 'Categoria critica' : undefined}
                              >
                                {category.name}
                              </span>
                            ))}
                          </div>
                        </td>
                        <td className="iconCell"><Icon name="chevron-right" size={16} /></td>
                      </tr>
                    );
                  })}</tbody>
                </table>
              </div>
            )}
          </Panel>
        </>
      )}
    </main>
  );
}

function Panel({
  title,
  subtitle,
  count,
  countSuffix,
  children,
  className,
  collapsible,
  open = true,
  onToggle,
  actions,
}: {
  title: string;
  subtitle?: string;
  count?: number;
  countSuffix?: string;
  children: React.ReactNode;
  className?: string;
  collapsible?: boolean;
  open?: boolean;
  onToggle?: () => void;
  actions?: React.ReactNode;
}) {
  const headerInner = (
    <>
      <div>
        <h2>
          {collapsible ? <Icon name={open ? 'chevron-down' : 'chevron-right'} size={16} /> : null}
          {title}
          {count !== undefined ? <span className="panelCount">{count}{countSuffix ? ` ${countSuffix}` : ''}</span> : null}
        </h2>
        {subtitle ? <span className="panelSubtitle">{subtitle}</span> : null}
      </div>
      {actions ? <div className="panelActions">{actions}</div> : null}
    </>
  );

  return (
    <section className={`panel ${className ?? ''}`}>
      {collapsible ? (
        <button
          type="button"
          className="panelHeader panelHeader--button"
          aria-expanded={open}
          onClick={onToggle}
        >
          {headerInner}
        </button>
      ) : (
        <header className="panelHeader">{headerInner}</header>
      )}
      {(!collapsible || open) ? children : null}
    </section>
  );
}

const USABLE_PROVIDER_STATES = new Set(['DRAFT', 'ACTIVE']);

function matchesProviderQuery(item: ProviderSummary, query: string) {
  if (!query) return true;
  const lower = query.toLowerCase();
  return (
    (item.company_name ?? '').toLowerCase().includes(lower) ||
    (item.vat_number ?? '').toLowerCase().includes(lower) ||
    (item.cf ?? '').toLowerCase().includes(lower) ||
    String(item.erp_id ?? '').includes(lower)
  );
}

function qualificationCopy(item: ProviderSummary) {
  if (item.total_count === 0) return '—/—';
  return `${item.qualified_count}/${item.total_count}`;
}

function qualificationVariant(item: ProviderSummary): 'success' | 'warning' | 'danger' | 'neutral' {
  if (item.total_count === 0) return 'neutral';
  if (item.qualified_count === item.total_count) return 'success';
  if (item.qualified_count === 0) return 'danger';
  return 'warning';
}

export function FornitoriRoute() {
  const [params] = useSearchParams();
  const redirect = legacyProviderDetailPath(params.get('id_provider'), params.get('tab'));
  if (redirect) return <Navigate to={redirect} replace />;
  return <FornitoriPage />;
}

export function FornitoriPage() {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [showArchive, setShowArchive] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const summary = useProviderSummary();

  const { active, archive } = useMemo(() => {
    const all = summary.data ?? [];
    const matched = all.filter((item) => matchesProviderQuery(item, query));
    return {
      active: matched.filter((item) => USABLE_PROVIDER_STATES.has((item.state ?? '').toUpperCase())),
      archive: matched.filter((item) => !USABLE_PROVIDER_STATES.has((item.state ?? '').toUpperCase())),
    };
  }, [summary.data, query]);

  function selectProvider(id: number) {
    navigate(providerDetailPath(id, 'dati'));
  }

  return (
    <main className="page fornitoriListPage">
      <header className="pageHeader">
        <div><h1>Fornitori</h1><p>Anagrafica, contatti, qualifica e documenti.</p></div>
        <Button leftIcon={<Icon name="plus" />} onClick={() => setCreateOpen(true)}>Nuovo fornitore</Button>
      </header>
      <section className="panel fornitoriListPanel">
        <div className="toolbar"><SearchInput value={query} onChange={setQuery} placeholder="Cerca per nome, P.IVA, CF, codice ERP" /></div>
        {summary.isLoading ? <Skeleton rows={8} /> : summary.error ? stateBlock(errorTitle(summary.error), 'Elenco fornitori non disponibile.', 'triangle-alert') : (
          <div className="listRows fornitoriSearchRows">
            {active.map((item) => (
              <ProviderRow key={item.id} item={item} onSelect={selectProvider} />
            ))}
            {active.length === 0 && !showArchive ? stateBlock('Nessun fornitore trovato', 'Modifica i criteri di ricerca.', 'search') : null}
            {archive.length > 0 ? (
              <button type="button" className="archiveToggle" onClick={() => setShowArchive((value) => !value)}>
                <Icon name={showArchive ? 'chevron-down' : 'chevron-right'} size={14} />
                <span>{showArchive ? 'Nascondi' : 'Mostra'} fornitori cessati e sospesi ({archive.length})</span>
              </button>
            ) : null}
            {showArchive ? archive.map((item) => (
              <ProviderRow key={item.id} item={item} onSelect={selectProvider} />
            )) : null}
          </div>
        )}
      </section>
      <ProviderCreateModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(id) => {
          setCreateOpen(false);
          navigate(providerDetailPath(id, 'dati'));
        }}
      />
    </main>
  );
}

function ProviderRow({ item, onSelect }: { item: ProviderSummary; onSelect: (id: number) => void }) {
  const qualVariant = qualificationVariant(item);
  return (
    <button className="listRow" onClick={() => onSelect(item.id)}>
      <span>
        <strong>{item.company_name ?? '—'}</strong>
        <small className="providerRowMeta">
          <ProviderStateBadge state={item.state} />
          <span className={`qualPill qualPill--${qualVariant}`} title="Categorie qualificate / totali">
            {qualificationCopy(item)}
          </span>
          {item.has_expiring_docs ? (
            <span className="docFlag" title="Documenti in scadenza nei prossimi 30 giorni">
              <Icon name="triangle-alert" size={12} /> Doc
            </span>
          ) : null}
          {item.erp_id !== null && item.erp_id !== undefined ? <span className="providerRowErp">ERP {item.erp_id}</span> : null}
        </small>
      </span>
      <Icon name="chevron-right" size={16} />
    </button>
  );
}

function refPayload(form: HTMLFormElement, type?: string): ProviderReference {
  const data = new FormData(form);
  const payload: ProviderReference = {
    phone: String(data.get('phone') ?? '').trim(),
  };
  const firstName = String(data.get('first_name') ?? '').trim();
  const lastName = String(data.get('last_name') ?? '').trim();
  const email = String(data.get('email') ?? '').trim();
  if (firstName) payload.first_name = firstName;
  if (lastName) payload.last_name = lastName;
  if (email) payload.email = email;
  if (type) payload.reference_type = type;
  return payload;
}

function paymentCodeOf(value: Provider['default_payment_method']): string {
  if (value && typeof value === 'object') return value.code ?? '';
  if (typeof value === 'string') return value;
  if (typeof value === 'number') return String(value);
  return '';
}

interface ProviderFormState {
  company_name: string;
  state: string;
  vat_number: string;
  cf: string;
  erp_id: string;
  language: string;
  country: string;
  province: string;
  city: string;
  postal_code: string;
  address: string;
  default_payment_method: string;
  skip_qualification_validation: boolean;
}

function providerFormState(provider: Provider): ProviderFormState {
  return {
    company_name: provider.company_name ?? '',
    state: (provider.state ?? 'DRAFT').toUpperCase(),
    vat_number: provider.vat_number ?? '',
    cf: provider.cf ?? '',
    erp_id: provider.erp_id == null ? '' : String(provider.erp_id),
    language: provider.language ?? 'it',
    country: provider.country ?? 'IT',
    province: provider.province ?? '',
    city: provider.city ?? '',
    postal_code: provider.postal_code ?? '',
    address: provider.address ?? '',
    default_payment_method: paymentCodeOf(provider.default_payment_method),
    skip_qualification_validation: Boolean(provider.skip_qualification_validation),
  };
}

function providerFormStatesEqual(a: ProviderFormState, b: ProviderFormState) {
  return (
    a.company_name === b.company_name &&
    a.state === b.state &&
    a.vat_number === b.vat_number &&
    a.cf === b.cf &&
    a.erp_id === b.erp_id &&
    a.language === b.language &&
    a.country === b.country &&
    a.province === b.province &&
    a.city === b.city &&
    a.postal_code === b.postal_code &&
    a.address === b.address &&
    a.default_payment_method === b.default_payment_method &&
    a.skip_qualification_validation === b.skip_qualification_validation
  );
}

function providerPayloadFromState(state: ProviderFormState): ProviderPayload {
  const erp = state.erp_id.trim();
  const payload: ProviderPayload = {
    company_name: state.company_name.trim(),
    state: state.state || 'DRAFT',
    vat_number: state.vat_number.trim() || undefined,
    cf: state.cf.trim() || undefined,
    address: state.address.trim() || undefined,
    city: state.city.trim() || undefined,
    postal_code: state.postal_code.trim() || undefined,
    province: state.province.trim() || undefined,
    erp_id: parseErpId(erp),
    language: state.language || 'it',
    country: state.country || 'IT',
    default_payment_method: state.default_payment_method || null,
  };
  if (state.skip_qualification_validation) payload.skip_qualification_validation = true;
  return payload;
}

function ProviderCreateModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (id: number) => void;
}) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const paymentMethods = usePaymentMethods();
  const countriesQuery = useCountries();
  const [country, setCountry] = useState('IT');
  const [paymentMethod, setPaymentMethod] = useState('');
  const [erpId, setErpId] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);

  function close() {
    setCountry('IT');
    setPaymentMethod('');
    setErpId('');
    setShowAdvanced(false);
    onClose();
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const rawErpId = String(new FormData(event.currentTarget).get('erp_id') ?? '');
    if (hasInvalidErpId(rawErpId)) {
      toast('Inserisci un codice Alyante numerico', 'warning');
      return;
    }
    const body = providerPayload(event.currentTarget);
    const validation = validateCreatePayload(body);
    if (validation) {
      toast(validation, 'warning');
      return;
    }
    try {
      const created = await mutations.createProvider.mutateAsync(body);
      toast('Fornitore creato in bozza');
      onCreated(created.id);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Creazione fornitore non riuscita';
      toast(message, 'error');
    }
  }

  return (
    <Modal open={open} onClose={close} title="Nuovo fornitore" size="md">
      <form className="modalForm createForm" onSubmit={(event) => void submit(event)}>
        <input type="hidden" name="state" value="DRAFT" />

        <fieldset className="formSection">
          <legend>Dati essenziali</legend>
          <div className="formSectionGrid">
            <Input name="company_name" label="Ragione sociale" required wide />
            <Input name="vat_number" label="P.IVA" />
            <Input name="cf" label="Codice fiscale" />
            <SearchableSelectField
              name="country"
              label="Paese"
              value={country}
              options={countrySelectOptions(countriesQuery.data, country)}
              disabled={countriesQuery.isLoading || countriesQuery.isError}
              onChange={(next) => setCountry(next || 'IT')}
              placeholder="Seleziona paese"
            />
            {country === 'IT' ? (
              <Select name="province" label="Provincia" options={provinceSelectOptions()} />
            ) : (
              <Input name="province" label="Provincia / Stato" />
            )}
            <Input name="city" label="Città" />
            <Input name="postal_code" label="CAP" />
            <Input name="address" label="Indirizzo" wide />
          </div>
        </fieldset>

        <fieldset className="formSection">
          <legend>Contatto qualifica</legend>
          <div className="formSectionGrid">
            <Input name="ref_first_name" label="Nome" />
            <Input name="ref_last_name" label="Cognome" />
            <Input name="ref_email" label="Email" type="email" required />
            <Input name="ref_phone" label="Telefono" />
          </div>
        </fieldset>

        <button
          type="button"
          className="advancedToggle"
          aria-expanded={showAdvanced}
          onClick={() => setShowAdvanced((value) => !value)}
        >
          <Icon name={showAdvanced ? 'chevron-down' : 'chevron-right'} size={14} />
          <span>Altri dettagli {showAdvanced ? '' : '(opzionale)'}</span>
        </button>
        {showAdvanced ? (
          <div className="formSectionGrid">
            <Select name="language" label="Lingua" defaultValue="it" options={languageOptions} />
            <AlyanteSupplierLookupField value={erpId} onChange={setErpId} />
            <PaymentMethodField value={paymentMethod} options={paymentMethods.data ?? []} disabled={paymentMethods.isLoading || paymentMethods.isError} onChange={setPaymentMethod} />
          </div>
        ) : null}

        <div className="modalActions">
          <Button variant="secondary" type="button" onClick={close}>Annulla</Button>
          <Button type="submit" loading={mutations.createProvider.isPending} leftIcon={<Icon name="plus" />}>Crea fornitore</Button>
        </div>
      </form>
    </Modal>
  );
}

function validateCreatePayload(payload: ProviderPayload): string | null {
  if (!payload.company_name) return 'Inserisci la ragione sociale';
  if (!payload.ref?.email) return 'Inserisci l\'email del contatto qualifica';
  if (payload.country === 'IT') {
    if (!payload.cf && !payload.vat_number) return 'Per i fornitori italiani inserisci CF o P.IVA';
    if ((payload.postal_code?.length ?? 0) < 5) return 'Inserisci un CAP italiano valido';
    if (!payload.province) return 'Seleziona la provincia';
  }
  return null;
}

const PROVIDER_DETAIL_TABS = [
  { key: 'dati', label: 'Dati' },
  { key: 'qualifica', label: 'Qualifica' },
  { key: 'documenti', label: 'Documenti' },
  { key: 'contatti', label: 'Contatti' },
];

function parseProviderId(raw?: string) {
  const id = Number(raw);
  return Number.isInteger(id) && id > 0 ? id : null;
}

export function ProviderDetailPage() {
  const navigate = useNavigate();
  const { providerId: rawProviderId } = useParams();
  const providerId = parseProviderId(rawProviderId);
  const [params, setParams] = useSearchParams();
  const section = normalizeProviderSection(params.get('section'));
  const focus = params.get('focus');

  const provider = useProvider(providerId);
  const loadedProviderId = provider.data ? providerId : null;
  const providerCategories = useProviderCategories(loadedProviderId);
  const documents = useProviderDocuments(loadedProviderId);

  useEffect(() => {
    if (!focus) return;
    const handle = window.setTimeout(() => {
      const target = document.querySelector<HTMLElement>(`[data-focus-id="${focus}"]`);
      target?.scrollIntoView({ block: 'center', behavior: 'smooth' });
    }, 120);
    return () => window.clearTimeout(handle);
  }, [focus, section, providerCategories.data, documents.data]);

  function updateSection(nextSection: ProviderDetailSection, nextFocus?: string | null) {
    const next = new URLSearchParams(params);
    next.set('section', nextSection);
    if (nextFocus) next.set('focus', nextFocus);
    else next.delete('focus');
    setParams(next);
  }

  function openAttentionAction(action: ProviderAttentionAction) {
    updateSection(action.section, action.focus ?? null);
  }

  if (providerId === null) return <Navigate to="/fornitori" replace />;

  if (provider.isLoading) {
    return (
      <main className="page providerDetailPage">
        <section className="stateCard"><Skeleton rows={10} /></section>
      </main>
    );
  }

  if (provider.error || !provider.data) {
    return (
      <main className="page providerDetailPage">
        <div className="detailTopBar">
          <Button variant="secondary" leftIcon={<Icon name="arrow-left" />} onClick={() => navigate('/fornitori')}>Elenco fornitori</Button>
        </div>
        <section className="panel">{stateBlock(errorTitle(provider.error), 'Il dettaglio fornitore non può essere caricato.', 'triangle-alert')}</section>
      </main>
    );
  }

  const data = provider.data;
  const stateUpper = (data.state ?? '').toUpperCase();
  const isUsable = USABLE_PROVIDER_STATES.has(stateUpper);
  const isActive = stateUpper === 'ACTIVE';
  const isDraft = stateUpper === 'DRAFT';
  const fullReadonly = !isUsable;
  const attention = buildDetailProviderAttention({
    provider: data,
    providerCategories: providerCategories.data ?? [],
    providerDocuments: documents.data ?? [],
  });

  const dotIndicator = {
    dati: attention.counts.drafts > 0 ? 'warning' : null,
    qualifica: attention.counts.criticalCategories > 0 ? 'danger' : attention.counts.openCategories > 0 ? 'warning' : null,
    documenti: attention.counts.expiredDocuments > 0 ? 'danger' : attention.counts.expiringDocuments > 0 ? 'warning' : null,
    contatti: null,
  } as const;

  let content: React.ReactNode;
  if (section === 'qualifica') {
    content = (
      <QualificationSection
        providerId={providerId}
        readonly={fullReadonly}
        providerCategories={providerCategories.data ?? []}
        providerCategoriesLoading={providerCategories.isLoading}
        providerCategoriesError={providerCategories.error}
        documents={documents.data ?? []}
        documentsLoading={documents.isLoading}
        focus={focus}
      />
    );
  } else if (section === 'documenti') {
    content = (
      <DocumentsSection
        providerId={providerId}
        readonly={fullReadonly}
        documents={documents.data ?? []}
        loading={documents.isLoading}
        error={documents.error}
        focus={focus}
      />
    );
  } else if (section === 'contatti') {
    content = <ContactsSection provider={data} readonly={fullReadonly} />;
  } else {
    content = (
      <AnagraficaSection
        provider={data}
        fullReadonly={fullReadonly}
        anagraficaLocked={isActive}
        onDeleted={() => navigate('/fornitori')}
      />
    );
  }

  return (
    <main className="page providerDetailPage">
      <div className="detailTopBar">
        <Button variant="secondary" leftIcon={<Icon name="arrow-left" />} onClick={() => navigate('/fornitori')}>Elenco fornitori</Button>
      </div>
      <ProviderHeader provider={data} providerCategories={providerCategories.data ?? []} />
      {fullReadonly ? <StateBanner state={data.state} /> : null}
      {isDraft ? <CompletenessBanner provider={data} /> : null}
      <section className="providerDetailNav" aria-label="Sezioni fornitore">
        <TabNav
          items={PROVIDER_DETAIL_TABS}
          activeKey={section}
          onTabChange={(key) => updateSection(normalizeProviderSection(key))}
          dotIndicator={dotIndicator}
        />
      </section>
      <div className="providerDetailWorkspace">
        <div className="providerDetailMain">
          {content}
        </div>
        <ProviderAttentionRail attention={attention} onOpen={openAttentionAction} />
      </div>
    </main>
  );
}

function ProviderHeader({ provider, providerCategories }: { provider: Provider; providerCategories: ProviderCategory[] }) {
  const total = providerCategories.length;
  const qualified = providerCategories.filter((row) => (row.status ?? row.state)?.toUpperCase() === 'QUALIFIED').length;
  const percent = total > 0 ? Math.round((qualified / total) * 100) : 0;
  const isDraft = (provider.state ?? '').toUpperCase() === 'DRAFT';
  const missing = isDraft ? missingProviderActivationFields(provider) : [];
  const activationBlocked = missing.length > 0;

  return (
    <header className="providerHeader">
      <div className="providerHeaderTop">
        <h2>{provider.company_name ?? '—'}</h2>
        <ProviderStateBadge state={provider.state} />
        {isDraft ? (
          <span className={`activationBadge activationBadge--${activationBlocked ? 'blocked' : 'ready'}`}>
            {activationBlocked ? 'Non attivabile' : 'Pronto per attivazione'}
          </span>
        ) : null}
      </div>
      <p className="providerHeaderMeta">
        {[
          provider.vat_number ? `P.IVA ${provider.vat_number}` : null,
          provider.cf ? `CF ${provider.cf}` : null,
          provider.erp_id ? `ERP ${provider.erp_id}` : null,
        ].filter(Boolean).join(' · ') || 'Identificativi non disponibili'}
      </p>
      {isDraft ? (
        <p className={`providerHeaderHint providerHeaderHint--${activationBlocked ? 'blocked' : 'ready'}`}>
          {activationBlocked ? `Mancano: ${formatActivationFieldList(missing)}.` : 'Dati completi. Attivazione gestita dalla sync Mistra.'}
        </p>
      ) : null}
      {total > 0 ? (
        <div className="providerHeaderProgress" aria-label={`Categorie qualificate ${percent}%`}>
          <div className="progressLabel">Categorie qualificate · {qualified}/{total}</div>
          <div className="progressTrack">
            <div className="progressFill" style={{ width: `${percent}%` }} data-state={qualified === total ? 'complete' : 'partial'} />
          </div>
          <div className="progressPercent">{percent}%</div>
        </div>
      ) : null}
    </header>
  );
}

function StateBanner({ state }: { state?: string | null }) {
  const upper = (state ?? '').toUpperCase();
  if (upper === 'CEASED') {
    return (
      <div className="banner banner--neutral">
        <Icon name="lock" size={16} aria-hidden="true" />
        <span>Fornitore cessato. Pagina in sola consultazione.</span>
      </div>
    );
  }
  if (upper === 'INACTIVE') {
    return (
      <div className="banner banner--neutral">
        <Icon name="lock" size={16} aria-hidden="true" />
        <span>Fornitore sospeso. Modifiche disabilitate fino a riattivazione.</span>
      </div>
    );
  }
  return null;
}

function CompletenessBanner({ provider }: { provider: Provider }) {
  const missing = missingProviderActivationFields(provider);
  if (missing.length === 0) {
    return (
      <div className="banner banner--success">
        <Icon name="check-circle" size={16} aria-hidden="true" />
        <span>Dati obbligatori completi. Il fornitore sarà attivabile dalla sync Mistra.</span>
      </div>
    );
  }
  return (
    <div className="banner banner--warning">
      <Icon name="triangle-alert" size={16} aria-hidden="true" />
      <span>Attivazione bloccata: aggiungi {formatActivationFieldList(missing)}.</span>
    </div>
  );
}

function formatActivationFieldList(fields: string[]) {
  if (fields.length === 0) return 'i dati obbligatori';
  if (fields.length === 1) return fields[0];
  return `${fields.slice(0, -1).join(', ')} e ${fields[fields.length - 1]}`;
}

function ProviderAttentionRail({
  attention,
  onOpen,
}: {
  attention: ProviderAttention;
  onOpen: (action: ProviderAttentionAction) => void;
}) {
  return (
    <aside className="providerAttentionRail" aria-labelledby="provider-attention-title">
      <header className="providerAttentionHeader">
        <div>
          <h2 id="provider-attention-title">Azioni richieste</h2>
          <span>{attention.openCount === 1 ? '1 attività aperta' : `${attention.openCount} attività aperte`}</span>
        </div>
      </header>
      {attention.actions.length === 0 ? (
        <div className="priorityEmpty providerAttentionEmpty">
          <span className="priorityEmptyIcon" aria-hidden="true"><Icon name="check-circle" size={20} /></span>
          <div>
            <strong>Nessuna azione richiesta</strong>
            <span>Il fornitore non richiede interventi.</span>
          </div>
        </div>
      ) : (
        <div className="providerAttentionActions">
          {attention.actions.map((action) => {
            const severityLabel = action.severity === 'blocking' ? 'Blocca attivazione' : PROVIDER_ATTENTION_LABELS[action.severity];
            return (
              <button
                key={action.id}
                type="button"
                className="providerAttentionAction"
                onClick={() => onOpen(action)}
              >
                <span className="providerAttentionActionTopline">
                  <StatusBadge
                    value={severityLabel}
                    label={severityLabel}
                    variant={PROVIDER_ATTENTION_BADGE[action.severity]}
                  />
                  <span>{action.label}</span>
                </span>
                <strong>{action.detail}</strong>
                <span className="providerAttentionActionIcon"><Icon name="chevron-right" size={16} /></span>
              </button>
            );
          })}
        </div>
      )}
    </aside>
  );
}

function QualificationSection({
  providerId,
  readonly,
  providerCategories,
  providerCategoriesLoading,
  providerCategoriesError,
  documents,
  documentsLoading,
  focus,
}: {
  providerId: number;
  readonly: boolean;
  providerCategories: ProviderCategory[];
  providerCategoriesLoading: boolean;
  providerCategoriesError: unknown;
  documents: ProviderDocument[];
  documentsLoading: boolean;
  focus: string | null;
}) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const allCategories = useCategories();
  const [addOpen, setAddOpen] = useState(false);

  const categoryById = useMemo(() => {
    const map = new Map<number, Category>();
    for (const item of allCategories.data ?? []) map.set(item.id, item);
    return map;
  }, [allCategories.data]);

  const documentsByType = useMemo(() => {
    const map = new Map<number, ProviderDocument[]>();
    for (const doc of documents) {
      const typeId = doc.document_type?.id;
      if (typeId == null) continue;
      const list = map.get(typeId) ?? [];
      list.push(doc);
      map.set(typeId, list);
    }
    return map;
  }, [documents]);

  const rows = providerCategories;
  const totalCount = rows.length;
  const qualifiedCount = rows.filter((row) => (row.status ?? row.state)?.toUpperCase() === 'QUALIFIED').length;
  const availableCategories = (allCategories.data ?? []).filter(
    (cat) => !rows.some((row) => row.category?.id === cat.id),
  );

  return (
    <Panel
      title="Qualifica"
      subtitle={totalCount === 0 ? 'Nessuna categoria assegnata' : `${qualifiedCount} di ${totalCount} categorie qualificate`}
      actions={!readonly ? (
        <Button size="sm" leftIcon={<Icon name="plus" />} onClick={() => setAddOpen(true)} disabled={availableCategories.length === 0}>
          Aggiungi categoria
        </Button>
      ) : undefined}
    >
      {providerCategoriesError ? stateBlock(errorTitle(providerCategoriesError), 'Le categorie del fornitore non possono essere caricate.', 'triangle-alert') : providerCategoriesLoading || documentsLoading || allCategories.isLoading ? <Skeleton rows={4} /> : rows.length === 0 ? stateBlock('Nessuna categoria assegnata', 'Aggiungi una categoria di qualifica per iniziare.', 'box') : (
        <div className="qualificationList">
          {rows.map((row) => {
            const categoryId = row.category?.id;
            const fullCategory = categoryId != null ? categoryById.get(categoryId) : undefined;
            return (
              <CategoryCard
                key={categoryId ?? row.category?.name}
                providerCategory={row}
                category={fullCategory}
                documentsByType={documentsByType}
                providerId={providerId}
                readonly={readonly}
                focused={focus === `category-${categoryId}`}
              />
            );
          })}
        </div>
      )}
      <AddCategoryModal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        availableCategories={availableCategories}
        onSubmit={async (categoryIds, critical) => {
          await mutations.addProviderCategories.mutateAsync({ providerId, categoryIds, critical });
          toast('Categoria aggiunta');
          setAddOpen(false);
        }}
        pending={mutations.addProviderCategories.isPending}
      />
    </Panel>
  );
}

function CategoryCard({
  providerCategory,
  category,
  documentsByType,
  providerId,
  readonly,
  focused,
}: {
  providerCategory: ProviderCategory;
  category?: Category;
  documentsByType: Map<number, ProviderDocument[]>;
  providerId: number;
  readonly: boolean;
  focused: boolean;
}) {
  const state = (providerCategory.status ?? providerCategory.state ?? 'NEW').toUpperCase();
  const [open, setOpen] = useState(state !== 'QUALIFIED');
  const [uploadType, setUploadType] = useState<DocumentType | null>(null);
  const [replaceDocument, setReplaceDocument] = useState<ProviderDocument | null>(null);
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const docTypes = category?.document_types ?? [];
  const required = docTypes.filter((dt) => dt.required);
  const optional = docTypes.filter((dt) => !dt.required);
  const categoryId = providerCategory.category?.id ?? category?.id;

  useEffect(() => {
    if (focused) setOpen(true);
  }, [focused]);

  const requiredMissingNames = required
    .filter((dt) => !(documentsByType.get(dt.document_type.id) ?? []).some((d) => (d.state ?? '').toUpperCase() === 'OK'))
    .map((dt) => dt.document_type.name);

  const subtitle = required.length === 0
    ? 'Nessun documento richiesto'
    : requiredMissingNames.length === 0
      ? `${required.length} di ${required.length} documenti richiesti`
      : `${required.length - requiredMissingNames.length} di ${required.length} richiesti — manca: ${requiredMissingNames.join(', ')}`;

  async function download(id: number) {
    const blob = await mutations.downloadDocument(id);
    saveBlob(blob, `documento-${id}`);
  }

  return (
    <article
      className="qualificationCard"
      data-state={state}
      data-focus-id={categoryId != null ? `category-${categoryId}` : undefined}
      data-focus-highlight={focused ? 'true' : undefined}
    >
      <button type="button" className="qualificationCardHeader" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <div className="qualificationCardTitle">
          <Icon name={open ? 'chevron-down' : 'chevron-right'} size={16} />
          <strong>{providerCategory.category?.name ?? category?.name ?? '—'}</strong>
          <CategoryStateBadge state={state} />
          {providerCategory.critical ? <span className="criticalTag" title="Categoria critica">Critica</span> : null}
        </div>
        <span className="qualificationCardSubtitle">{subtitle}</span>
      </button>
      {open ? (
        <div className="qualificationCardBody">
          {required.length > 0 ? (
            <DocumentGroup
              title="Documenti richiesti"
              required
              types={required}
              documentsByType={documentsByType}
              readonly={readonly}
              onUpload={(t) => setUploadType(t)}
              onReplace={(document) => setReplaceDocument(document)}
              onDownload={(id) => void download(id)}
            />
          ) : null}
          {optional.length > 0 ? (
            <DocumentGroup
              title="Documenti opzionali"
              required={false}
              types={optional}
              documentsByType={documentsByType}
              readonly={readonly}
              onUpload={(t) => setUploadType(t)}
              onReplace={(document) => setReplaceDocument(document)}
              onDownload={(id) => void download(id)}
            />
          ) : null}
        </div>
      ) : null}
      <DocumentModal
        open={uploadType !== null}
        onClose={() => setUploadType(null)}
        providerId={providerId}
        prefillType={uploadType ?? undefined}
        onSaved={() => toast('Documento caricato')}
      />
      <DocumentModal
        open={replaceDocument !== null}
        onClose={() => setReplaceDocument(null)}
        providerId={providerId}
        replaceDocument={replaceDocument ?? undefined}
        onSaved={() => toast('Documento sostituito')}
      />
    </article>
  );
}

function DocumentGroup({
  title,
  required,
  types,
  documentsByType,
  readonly,
  onUpload,
  onReplace,
  onDownload,
}: {
  title: string;
  required: boolean;
  types: CategoryDocumentType[];
  documentsByType: Map<number, ProviderDocument[]>;
  readonly: boolean;
  onUpload: (type: DocumentType) => void;
  onReplace: (document: ProviderDocument) => void;
  onDownload: (id: number) => void;
}) {
  return (
    <div className="docGroup">
      <span className="docGroupTitle">{title}</span>
      <ul className="docTypeList">
        {types.map((entry) => {
          const docs = documentsByType.get(entry.document_type.id) ?? [];
          const display = docs.find((d) => (d.state ?? '').toUpperCase() === 'OK') ?? docs[0];
          const stateUpper = (display?.state ?? '').toUpperCase();
          const isValid = stateUpper === 'OK';
          const expiryDays = daysUntilExpiry(display?.expire_date);
          const isExpired = expiryDays !== null && expiryDays < 0;
          const isCurrent = isValid && !isExpired;
          const hasBlockingIssue = Boolean(display && !isCurrent);
          const hasStateBadge = Boolean(stateUpper && stateUpper !== 'OK');
          const hasUrgencyBadge = expiryDays !== null && expiryDays <= 30;
          const status = display ? (hasBlockingIssue ? 'issue' : 'valid') : required ? 'missing-required' : 'missing-optional';
          const indicatorIcon: IconName = display ? (hasBlockingIssue ? 'file-warning' : 'file-check') : 'file-plus';
          return (
            <li
              key={entry.document_type.id}
              className="docTypeRow"
              data-required={required ? 'true' : 'false'}
              data-uploaded={display ? 'true' : 'false'}
              data-doc-status={status}
            >
              <span className="docTypeIndicator" aria-hidden="true"><Icon name={indicatorIcon} size={16} /></span>
              <span className="docTypeName">{entry.document_type.name}</span>
              <span className="docTypeMeta">
                {display?.expire_date ? dateLabel(display.expire_date) : '—'}
              </span>
              <span className="docTypeBadges">
                {display ? (
                  <>
                    <DocumentStateBadge state={display.state} />
                    <DocumentUrgencyBadge expireDate={display.expire_date} />
                    {isCurrent ? <StatusBadge value="valid" label="Valido" variant="success" dot={false} /> : null}
                    {!isCurrent && !hasStateBadge && !hasUrgencyBadge ? <StatusBadge value="uploaded" label="Caricato" variant="neutral" dot={false} /> : null}
                  </>
                ) : (
                  <StatusBadge
                    value={required ? 'missing' : 'optional'}
                    label={required ? 'Mancante' : 'Facoltativo'}
                    variant={required ? 'warning' : 'neutral'}
                    dot={false}
                  />
                )}
              </span>
              <span className="docTypeActions">
                {display ? (
                  <>
                    <Button size="sm" variant="ghost" leftIcon={<Icon name="download" />} aria-label="Scarica documento" onClick={() => onDownload(display.id)} />
                    {!readonly ? (
                      <Button
                        size="sm"
                        variant="secondary"
                        leftIcon={<Icon name="file-up" />}
                        aria-label="Sostituisci documento"
                        title="Sostituisci documento"
                        onClick={() => onReplace(display)}
                      />
                    ) : null}
                  </>
                ) : !readonly ? (
                  <Button size="sm" variant="primary" leftIcon={<Icon name="plus" />} onClick={() => onUpload(entry.document_type)}>Carica</Button>
                ) : <span className="muted">—</span>}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function DocumentsSection({
  providerId,
  readonly,
  documents,
  loading,
  error,
  focus,
}: {
  providerId: number;
  readonly: boolean;
  documents: ProviderDocument[];
  loading: boolean;
  error: unknown;
  focus: string | null;
}) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const [uploadOpen, setUploadOpen] = useState(false);
  const [replaceDocument, setReplaceDocument] = useState<ProviderDocument | null>(null);

  async function download(id: number) {
    try {
      const blob = await mutations.downloadDocument(id);
      saveBlob(blob, `documento-${id}`);
    } catch {
      toast('Download non riuscito', 'error');
    }
  }

  return (
    <Panel
      title="Documenti"
      subtitle={documents.length === 0 ? 'Nessun documento caricato' : `${documents.length} documenti`}
      actions={!readonly ? (
        <Button size="sm" leftIcon={<Icon name="plus" />} onClick={() => setUploadOpen(true)}>
          Carica documento
        </Button>
      ) : undefined}
    >
      {loading ? <Skeleton rows={5} /> : error ? stateBlock(errorTitle(error), 'I documenti del fornitore non possono essere caricati.', 'triangle-alert') : documents.length === 0 ? stateBlock('Nessun documento caricato', 'Carica un documento per collegarlo al fornitore.', 'file-text') : (
        <div className="tableScroll">
          <table className="table documentsTable">
            <thead>
              <tr>
                <th>Documento</th>
                <th>Scadenza</th>
                <th>Stato</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {documents.map((document) => {
                const focused = focus === `document-${document.id}`;
                const stateUpper = (document.state ?? '').toUpperCase();
                const expiryDays = daysUntilExpiry(document.expire_date);
                const hasStateBadge = Boolean(stateUpper && stateUpper !== 'OK');
                const hasUrgencyBadge = expiryDays !== null && expiryDays <= 30;
                return (
                  <tr
                    key={document.id}
                    data-focus-id={`document-${document.id}`}
                    data-focus-highlight={focused ? 'true' : undefined}
                  >
                    <td data-label="Documento">{document.document_type?.name ?? 'Documento'}</td>
                    <td data-label="Scadenza">{dateLabel(document.expire_date)}</td>
                    <td data-label="Stato">
                      <span className="docTypeBadges">
                        <DocumentStateBadge state={document.state} />
                        <DocumentUrgencyBadge expireDate={document.expire_date} />
                        {!hasStateBadge && !hasUrgencyBadge ? <span className="muted">Valido</span> : null}
                      </span>
                    </td>
                    <td data-label="Azioni">
                      <span className="docTypeActions">
                        <Button size="sm" variant="ghost" leftIcon={<Icon name="download" />} aria-label="Scarica documento" onClick={() => void download(document.id)} />
                        {!readonly ? (
                          <Button
                            size="sm"
                            variant="secondary"
                            leftIcon={<Icon name="file-up" />}
                            aria-label="Sostituisci documento"
                            title="Sostituisci documento"
                            onClick={() => setReplaceDocument(document)}
                          />
                        ) : null}
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      <DocumentModal
        open={uploadOpen}
        onClose={() => setUploadOpen(false)}
        providerId={providerId}
        onSaved={() => toast('Documento caricato')}
      />
      <DocumentModal
        open={replaceDocument !== null}
        onClose={() => setReplaceDocument(null)}
        providerId={providerId}
        replaceDocument={replaceDocument ?? undefined}
        onSaved={() => toast('Documento sostituito')}
      />
    </Panel>
  );
}

function AddCategoryModal({
  open,
  onClose,
  availableCategories,
  onSubmit,
  pending,
}: {
  open: boolean;
  onClose: () => void;
  availableCategories: Category[];
  onSubmit: (categoryIds: number[], critical: boolean) => void | Promise<void>;
  pending: boolean;
}) {
  const [selected, setSelected] = useState<number[]>([]);
  const [critical, setCritical] = useState(false);

  function close() {
    setSelected([]);
    setCritical(false);
    onClose();
  }

  return (
    <Modal open={open} onClose={close} title="Aggiungi categoria di qualifica" size="md">
      <div className="modalForm">
        <label className="field">
          <span>Categoria</span>
          <MultiSelect options={selectOptions(availableCategories)} selected={selected} onChange={setSelected} placeholder="Seleziona una o più categorie" />
        </label>
        <ToggleSwitch id="add-category-critical" checked={critical} onChange={setCritical} label="Categoria critica" />
        <div className="modalActions">
          <Button variant="secondary" onClick={close} type="button">Annulla</Button>
          <Button onClick={() => void onSubmit(selected, critical)} loading={pending} disabled={selected.length === 0}>Aggiungi</Button>
        </div>
      </div>
    </Modal>
  );
}

interface ContactRoleConfig {
  key: string;
  label: string;
  labelLower: string;
  icon: IconName;
  multiple: boolean;
}

const CONTACT_ROLES: ContactRoleConfig[] = [
  { key: QUALIFICATION_REFERENCE_TYPE, label: 'Qualifica', labelLower: 'qualifica', icon: 'check-circle', multiple: false },
  { key: 'ADMINISTRATIVE_REF', label: 'Amministrativo', labelLower: 'amministrativo', icon: 'mail', multiple: true },
  { key: 'TECHNICAL_REF', label: 'Tecnico', labelLower: 'tecnico', icon: 'settings', multiple: true },
  { key: 'OTHER_REF', label: 'Altro', labelLower: 'altro', icon: 'user', multiple: true },
];

interface ContactDrawerState {
  role: ContactRoleConfig;
  contact: ProviderReference | null;
  providerName: string;
}

function ContactsSection({ provider, readonly }: { provider: Provider; readonly: boolean }) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const [drawer, setDrawer] = useState<ContactDrawerState | null>(null);

  const refs = getProviderRefs(provider);
  const grouped = useMemo(() => {
    const map = new Map<string, ProviderReference[]>();
    for (const role of CONTACT_ROLES) map.set(role.key, []);
    for (const ref of refs) {
      const list = map.get(ref.reference_type ?? '');
      if (list) list.push(ref);
    }
    for (const list of map.values()) {
      list.sort((a, b) => (a.id ?? 0) - (b.id ?? 0));
    }
    return map;
  }, [refs]);

  async function handleSave(body: ProviderReference) {
    if (!drawer) return;
    const { role, contact } = drawer;
    try {
      if (role.key === QUALIFICATION_REFERENCE_TYPE) {
        // QUALIFICATION_REF is owned by Mistra and is created/updated through
        // PUT /provider/{id} with body.ref. The /reference endpoint refuses it.
        const { reference_type: _ignored, ...refBody } = body;
        await mutations.updateProvider.mutateAsync({
          id: provider.id,
          body: { ref: refBody },
        });
      } else if (contact?.id) {
        await mutations.updateReference.mutateAsync({ providerId: provider.id, refId: contact.id, body });
      } else {
        await mutations.createReference.mutateAsync({ providerId: provider.id, body });
      }
      toast(contact ? 'Contatto aggiornato' : 'Contatto aggiunto');
      setDrawer(null);
    } catch (err) {
      toast(apiErrorMessage(err, contact ? 'Salvataggio contatto non riuscito' : 'Creazione contatto non riuscita'), 'error');
    }
  }

  const savePending = mutations.createReference.isPending || mutations.updateReference.isPending || mutations.updateProvider.isPending;

  return (
    <Panel
      title="Contatti"
      subtitle="Riferimenti per qualifica, amministrazione e supporto tecnico"
    >
      <div className="contactsRoleGrid">
        {CONTACT_ROLES.map((role) => {
          const items = grouped.get(role.key) ?? [];
          const canAdd = !readonly && (role.multiple || items.length === 0);
          return (
            <ContactRoleSection
              key={role.key}
              role={role}
              items={items}
              readonly={readonly}
              canAdd={canAdd}
              onAdd={() => setDrawer({ role, contact: null, providerName: provider.company_name ?? '' })}
              onEdit={(contact) => setDrawer({ role, contact, providerName: provider.company_name ?? '' })}
            />
          );
        })}
      </div>
      <ContactDrawer
        state={drawer}
        onClose={() => setDrawer(null)}
        onSave={handleSave}
        pending={savePending}
      />
    </Panel>
  );
}

function ContactRoleSection({
  role,
  items,
  readonly,
  canAdd,
  onAdd,
  onEdit,
}: {
  role: ContactRoleConfig;
  items: ProviderReference[];
  readonly: boolean;
  canAdd: boolean;
  onAdd: () => void;
  onEdit: (contact: ProviderReference) => void;
}) {
  const countLabel = role.multiple
    ? items.length === 0 ? 'Nessuno' : `${items.length} contatt${items.length === 1 ? 'o' : 'i'}`
    : items.length === 0 ? 'Da compilare' : 'Compilato';

  return (
    <section className="contactRole" aria-label={role.label}>
      <header className="contactRoleHeader">
        <span className="contactRoleIcon" aria-hidden="true">
          <Icon name={role.icon} size={16} />
        </span>
        <span className="contactRoleTitle">{role.label}</span>
        <span className="contactRoleCount" data-empty={items.length === 0 ? 'true' : 'false'}>{countLabel}</span>
        {canAdd ? (
          <button
            type="button"
            className="contactRoleAdd"
            onClick={onAdd}
            aria-label={`Aggiungi contatto ${role.labelLower}`}
            title={`Aggiungi contatto ${role.labelLower}`}
          >
            <Icon name="plus" size={14} />
          </button>
        ) : null}
      </header>

      {items.length === 0 ? (
        <div className="contactRoleEmpty">
          <span>Nessun contatto {role.labelLower}</span>
          {canAdd ? (
            <button type="button" className="contactRoleEmptyAdd" onClick={onAdd}>
              <Icon name="plus" size={12} />
              <span>Aggiungi</span>
            </button>
          ) : null}
        </div>
      ) : (
        <ul className="contactRoleList">
          {items.map((item, index) => (
            <li key={item.id ?? `${role.key}-${index}`} className="contactRoleRow">
              <div className="contactRoleRowMain">
                <span className="contactRoleName">
                  {[item.first_name, item.last_name].filter(Boolean).join(' ') || '—'}
                </span>
                <span className="contactRoleMeta">
                  <span>{item.email || '—'}</span>
                  <span aria-hidden="true">·</span>
                  <span>{item.phone || '—'}</span>
                </span>
              </div>
              {!readonly ? (
                <button
                  type="button"
                  className="contactRoleEdit"
                  onClick={() => onEdit(item)}
                  aria-label={`Modifica contatto ${role.labelLower}`}
                  title={`Modifica contatto ${role.labelLower}`}
                >
                  <Icon name="pencil" size={14} />
                </button>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function contactDrawerTitle(state: ContactDrawerState) {
  const { role, contact } = state;
  if (role.key === QUALIFICATION_REFERENCE_TYPE) return 'Contatto qualifica';
  return `${contact ? 'Modifica' : 'Nuovo'} contatto ${role.labelLower}`;
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const isLikelyEmail = (v: string) => EMAIL_RE.test(v.trim());

function ContactField({
  name,
  label,
  type = 'text',
  defaultValue,
  helper,
  optional,
  icon,
  validate,
  invalid,
  invalidMessage,
  inputMode,
  pattern,
  title,
  onValueChange,
}: {
  name: string;
  label: string;
  type?: 'text' | 'email' | 'tel';
  defaultValue?: string;
  helper?: string;
  optional?: boolean;
  icon?: IconName;
  validate?: (value: string) => boolean;
  invalid?: boolean;
  invalidMessage?: string;
  inputMode?: 'tel';
  pattern?: string;
  title?: string;
  onValueChange?: (value: string) => void;
}) {
  const [value, setValue] = useState(defaultValue ?? '');
  const [touched, setTouched] = useState(false);
  const valid = validate ? validate(value) : false;
  const hasValue = value.trim().length > 0;
  const showInvalid = Boolean(invalid) || Boolean(validate && invalidMessage && touched && hasValue && !valid);
  const showCheck = Boolean(validate) && touched && hasValue && valid && !showInvalid;
  const helperID = `contactField-${name}-helper`;
  const helperText = showInvalid && invalidMessage ? invalidMessage : helper;

  return (
    <div className="contactField">
      <label className="contactFieldLabel" htmlFor={`contactField-${name}`}>
        <span>{label}</span>
        {optional ? <span className="contactFieldOptional"> · opzionale</span> : null}
      </label>
      <div className="contactFieldInputWrap" data-has-icon={icon ? 'true' : 'false'}>
        {icon ? (
          <span className="contactFieldIcon" aria-hidden="true">
            <Icon name={icon} size={14} />
          </span>
        ) : null}
        <input
          id={`contactField-${name}`}
          name={name}
          type={type}
          defaultValue={defaultValue}
          inputMode={inputMode}
          pattern={pattern}
          title={title}
          aria-invalid={showInvalid ? 'true' : undefined}
          aria-describedby={helperText ? helperID : undefined}
          onInput={(event) => {
            const next = event.currentTarget.value;
            setValue(next);
            onValueChange?.(next);
          }}
          onBlur={() => setTouched(true)}
          autoComplete="off"
        />
        {showCheck ? (
          <span className="contactFieldCheck" aria-hidden="true">
            <Icon name="check" size={14} />
          </span>
        ) : null}
      </div>
      {helperText ? (
        <p id={helperID} className={`contactFieldHelper${showInvalid ? ' contactFieldHelper--error' : ''}`}>
          {helperText}
        </p>
      ) : null}
    </div>
  );
}

function ContactDrawer({
  state,
  onClose,
  onSave,
  pending,
}: {
  state: ContactDrawerState | null;
  onClose: () => void;
  onSave: (body: ProviderReference) => Promise<void>;
  pending: boolean;
}) {
  const { toast } = useToast();
  const formId = useId();
  const open = state !== null;
  const isEdit = Boolean(state?.contact);
  const contact = state?.contact ?? null;
  const role = state?.role ?? null;
  const [phoneInvalid, setPhoneInvalid] = useState(false);

  useEffect(() => {
    setPhoneInvalid(false);
  }, [contact?.id, role?.key]);

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!role) return;
    const emailInput = event.currentTarget.elements.namedItem('email') as HTMLInputElement | null;
    if (!emailInput) return;
    const phoneInput = event.currentTarget.elements.namedItem('phone') as HTMLInputElement | null;
    const email = emailInput.value.trim();
    if (!email || !emailInput.validity.valid) {
      toast('Inserisci un indirizzo email valido per il contatto.', 'warning');
      return;
    }
    if (phoneInput && !isValidOptionalProviderRefPhone(phoneInput.value)) {
      setPhoneInvalid(true);
      phoneInput.focus();
      toast(PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE, 'warning');
      return;
    }
    setPhoneInvalid(false);
    const body = refPayload(event.currentTarget, role.key);
    void onSave(body);
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size="sm"
      side="right"
      title={state ? contactDrawerTitle(state) : ''}
      subtitle={state?.providerName || undefined}
      footer={
        <div className="contactDrawerFooter">
          <Button variant="secondary" type="button" onClick={onClose} disabled={pending}>Annulla</Button>
          <Button
            type="submit"
            form={formId}
            loading={pending}
            leftIcon={<Icon name={isEdit ? 'check' : 'plus'} />}
          >
            {isEdit ? 'Salva' : 'Aggiungi'}
          </Button>
        </div>
      }
    >
      {state ? (
        <form
          id={formId}
          className="contactDrawerForm"
          // Force the form to remount when switching contact/role so default values reset.
          key={`${state.role.key}-${contact?.id ?? 'new'}`}
          onSubmit={submit}
          noValidate
        >
          <ContactField name="first_name" label="Nome" defaultValue={contact?.first_name ?? ''} />
          <ContactField name="last_name" label="Cognome" defaultValue={contact?.last_name ?? ''} />
          <ContactField
            name="email"
            label="Email"
            type="email"
            icon="mail"
            validate={isLikelyEmail}
            defaultValue={contact?.email ?? ''}
          />
          <ContactField
            name="phone"
            label="Telefono"
            type="tel"
            icon="phone"
            optional
            validate={isValidOptionalProviderRefPhone}
            invalid={phoneInvalid}
            invalidMessage={PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE}
            inputMode="tel"
            pattern={PROVIDER_REFERENCE_PHONE_PATTERN}
            title={PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE}
            onValueChange={(value) => {
              if (phoneInvalid && isValidOptionalProviderRefPhone(value)) setPhoneInvalid(false);
            }}
            defaultValue={contact?.phone ?? ''}
          />
        </form>
      ) : null}
    </Drawer>
  );
}

function AnagraficaSection({
  provider,
  fullReadonly,
  anagraficaLocked,
  onDeleted,
}: {
  provider: Provider;
  fullReadonly: boolean;
  anagraficaLocked: boolean;
  onDeleted: () => void;
}) {
  const { toast } = useToast();
  const skipRole = useHasRole('app_fornitori_skip_qualification');
  const mutations = useFornitoriMutations();
  const paymentMethods = usePaymentMethods();
  const countriesQuery = useCountries();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const baseline = useMemo(() => providerFormState(provider), [provider]);
  const [formState, setFormState] = useState<ProviderFormState>(baseline);

  useEffect(() => {
    setFormState(baseline);
  }, [baseline]);

  const dirty = !providerFormStatesEqual(formState, baseline);
  const qualificationRef = (provider.refs ?? []).find((r) => r.reference_type === QUALIFICATION_REFERENCE_TYPE);

  useEffect(() => {
    if (!dirty) return;
    function onBeforeUnload(event: BeforeUnloadEvent) {
      event.preventDefault();
      event.returnValue = '';
    }
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  // Anagrafica master fields are locked when state=ACTIVE (cfr. trg_provider_state_guard).
  // Always editable on ACTIVE: payment method and skip flag.
  // Always editable on DRAFT.
  // Never editable on INACTIVE/CEASED (fullReadonly).
  const masterFieldsDisabled = fullReadonly || anagraficaLocked;
  const editableSidefieldsDisabled = fullReadonly;
  const stateOptions = ensureSelectedOption(
    providerStateSelectOptions(provider.state, formState.erp_id),
    formState.state,
  );
  const countryOptions = countrySelectOptions(countriesQuery.data, formState.country);

  function updateField<K extends keyof ProviderFormState>(key: K, currentValue: ProviderFormState[K]) {
    setFormState((current) => ({ ...current, [key]: currentValue }));
  }

  function updateErpId(nextValue: string) {
    setFormState((current) => ({
      ...current,
      erp_id: nextValue,
      state: (provider.state ?? '').toUpperCase() === 'DRAFT' && !hasProviderErp(nextValue) ? 'DRAFT' : current.state,
    }));
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (hasInvalidErpId(formState.erp_id)) {
      toast('Inserisci un codice Alyante numerico', 'warning');
      return;
    }
    const body = providerPayloadFromState(formState);
    const validation = validateProvider(body);
    if (validation) {
      toast(validation, 'warning');
      return;
    }
    await mutations.updateProvider.mutateAsync({ id: provider.id, body });
    toast('Aggiornamento completato');
  }

  async function remove() {
    await mutations.deleteProvider.mutateAsync(provider.id);
    setConfirmDelete(false);
    toast('Fornitore eliminato');
    onDeleted();
  }

  return (
    <Panel
      title="Anagrafica"
      subtitle={anagraficaLocked && !fullReadonly ? 'Anagrafica bloccata: fornitore attivo' : undefined}
    >
      <form className="providerDataForm" onSubmit={(event) => void submit(event)}>
        <div className="formSection">
          <div className="formSectionGrid">
            <Input name="company_name" label="Ragione sociale" value={formState.company_name} disabled={masterFieldsDisabled} onChange={(event) => updateField('company_name', event.target.value)} wide />
            <Input name="vat_number" label="P.IVA" value={formState.vat_number} disabled={masterFieldsDisabled} onChange={(event) => updateField('vat_number', event.target.value)} />
            <Input name="cf" label="CF" value={formState.cf} disabled={masterFieldsDisabled} onChange={(event) => updateField('cf', event.target.value)} />
            <AlyanteSupplierLookupField value={formState.erp_id} disabled={masterFieldsDisabled} onChange={updateErpId} />
            <SearchableSelectField
              label="Stato"
              value={formState.state}
              options={stateOptions}
              disabled={fullReadonly}
              onChange={(next) => updateField('state', next || formState.state)}
              placeholder="Seleziona stato"
            />
            <Select name="language" label="Lingua" value={formState.language} options={languageOptions} disabled={masterFieldsDisabled} onChange={(event) => updateField('language', event.target.value)} />
            <PaymentMethodField
              value={formState.default_payment_method}
              defaultValue={formState.default_payment_method}
              options={paymentMethods.data ?? []}
              disabled={editableSidefieldsDisabled || paymentMethods.isLoading || paymentMethods.isError}
              onChange={(next) => updateField('default_payment_method', next)}
            />
          </div>
        </div>

        <div className="formSection">
          <div className="formSectionGrid">
            <SearchableSelectField
              label="Paese"
              value={formState.country}
              options={countryOptions}
              disabled={masterFieldsDisabled || countriesQuery.isLoading || countriesQuery.isError}
              onChange={(next) => updateField('country', next || 'IT')}
              placeholder="Seleziona paese"
            />
            {formState.country === 'IT' ? (
              <Select name="province" label="Provincia" value={formState.province} options={provinceSelectOptions(formState.province)} disabled={masterFieldsDisabled} onChange={(event) => updateField('province', event.target.value)} />
            ) : (
              <Input name="province" label="Provincia / Stato" value={formState.province} disabled={masterFieldsDisabled} onChange={(event) => updateField('province', event.target.value)} />
            )}
            <Input name="city" label="Città" value={formState.city} disabled={masterFieldsDisabled} onChange={(event) => updateField('city', event.target.value)} />
            <Input name="postal_code" label="CAP" value={formState.postal_code} disabled={masterFieldsDisabled} onChange={(event) => updateField('postal_code', event.target.value)} />
            <Input name="address" label="Indirizzo" value={formState.address} disabled={masterFieldsDisabled} onChange={(event) => updateField('address', event.target.value)} wide />
            {qualificationRef ? (
              <p className="wideField addressContact">
                {'Contatto: '}
                {[
                  [qualificationRef.first_name, qualificationRef.last_name].filter(Boolean).join(' '),
                  qualificationRef.email,
                  qualificationRef.phone,
                ].filter(Boolean).join(' · ')}
              </p>
            ) : null}
          </div>
        </div>

        {skipRole ? (
          <div className="formSection">
            <div className="formSectionGrid">
              <div className="wideField">
                <ToggleSwitch
                  id={`skip-qualification-${provider.id}`}
                  checked={formState.skip_qualification_validation}
                  disabled={editableSidefieldsDisabled}
                  onChange={(checked) => updateField('skip_qualification_validation', checked)}
                  label="Salta controllo qualifica"
                />
              </div>
            </div>
          </div>
        ) : null}

        {!fullReadonly ? (
          <div className="formActions formActionsWithState">
            {dirty ? <span className="dirtyState">Modifiche non salvate</span> : null}
            <Button type="submit" leftIcon={<Icon name="check" />} loading={mutations.updateProvider.isPending} disabled={!dirty}>Salva anagrafica</Button>
            {!anagraficaLocked ? (
              <Button variant="danger" type="button" leftIcon={<Icon name="trash" />} onClick={() => setConfirmDelete(true)}>Elimina</Button>
            ) : null}
          </div>
        ) : null}
      </form>
      <Modal open={confirmDelete} onClose={() => setConfirmDelete(false)} title="Elimina fornitore" size="sm">
        <p className="modalText">Confermi l'eliminazione del fornitore? L'operazione non è reversibile.</p>
        <div className="modalActions">
          <Button variant="secondary" onClick={() => setConfirmDelete(false)}>Annulla</Button>
          <Button variant="danger" onClick={() => void remove()} loading={mutations.deleteProvider.isPending}>Elimina</Button>
        </div>
      </Modal>
    </Panel>
  );
}

function useDebouncedString(value: string, delayMs: number) {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const handle = window.setTimeout(() => setDebounced(value), delayMs);
    return () => window.clearTimeout(handle);
  }, [delayMs, value]);

  return debounced;
}

function AlyanteSupplierLookupField({
  value,
  disabled = false,
  onChange,
  name = 'erp_id',
  label = 'Codice Alyante',
}: {
  value: string;
  disabled?: boolean;
  onChange: (value: string) => void;
  name?: string;
  label?: string;
}) {
  const inputId = useId();
  const listId = `${inputId}-results`;
  const [open, setOpen] = useState(false);
  const debouncedSearch = useDebouncedString(value, 250);
  const currentSearch = value.trim();
  const settledSearch = debouncedSearch.trim();
  const results = useAlyanteSuppliers(open && !disabled ? settledSearch : '');
  const suppliers = results.data ?? [];
  const canSearch = currentSearch.length >= 3;
  const isSettled = currentSearch === settledSearch;
  const showMenu = open && !disabled && canSearch;

  function selectSupplier(supplier: AlyanteSupplier) {
    onChange(supplier.code);
    setOpen(false);
  }

  return (
    <div className="field alyanteLookupField">
      <label className="fieldLabel" htmlFor={inputId}>{label}</label>
      <div className="alyanteLookup">
        <input
          id={inputId}
          name={name}
          type="text"
          value={value}
          disabled={disabled}
          autoComplete="off"
          role="combobox"
          aria-expanded={showMenu}
          aria-controls={showMenu ? listId : undefined}
          aria-autocomplete="list"
          placeholder="Codice o ragione sociale"
          onFocus={() => setOpen(true)}
          onBlur={() => window.setTimeout(() => setOpen(false), 120)}
          onChange={(event) => {
            onChange(event.target.value);
            setOpen(true);
          }}
        />
        {showMenu ? (
          <div id={listId} className="alyanteLookupMenu" role="listbox">
            {!isSettled || results.isFetching ? (
              <div className="alyanteLookupState">Ricerca in corso</div>
            ) : results.isError ? (
              <div className="alyanteLookupState">Lookup Alyante non disponibile</div>
            ) : suppliers.length === 0 ? (
              <div className="alyanteLookupState">Nessun risultato</div>
            ) : (
              suppliers.map((supplier) => (
                <button
                  key={`${supplier.code}-${supplier.company_name}`}
                  type="button"
                  className="alyanteLookupOption"
                  role="option"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => selectSupplier(supplier)}
                >
                  <span className="alyanteLookupCode">{supplier.code}</span>
                  <span className="alyanteLookupName">{supplier.company_name || '-'}</span>
                </button>
              ))
            )}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function PaymentMethodField({
  value: selectedValue,
  defaultValue = '',
  options,
  disabled,
  onChange,
}: {
  value?: string;
  defaultValue?: string;
  options: PaymentMethod[];
  disabled: boolean;
  onChange?: (value: string) => void;
}) {
  const [internalValue, setInternalValue] = useState(defaultValue);
  const controlled = selectedValue !== undefined;
  const current = controlled ? selectedValue ?? '' : internalValue;

  useEffect(() => {
    if (!controlled) setInternalValue(defaultValue);
  }, [controlled, defaultValue]);

  const hasCurrent = current === '' || options.some((item) => item.code === current);
  const selectOptionsList = [
    ...(hasCurrent ? [] : [{ value: current, label: current }]),
    ...options.map((item) => ({ value: item.code, label: `${item.description} (${item.code.trim()})` })),
  ];

  function handleChange(next: string | null) {
    const value = next ?? '';
    if (!controlled) setInternalValue(value);
    onChange?.(value);
  }

  return (
    <SearchableSelectField
      name="default_payment_method"
      label="Metodo di pagamento"
      value={selectedValue}
      fallbackValue={current}
      options={selectOptionsList}
      disabled={disabled}
      onChange={handleChange}
      allowClear
      clearLabel="—"
      placeholder="—"
    />
  );
}

function SearchableSelectField({
  label,
  name,
  value,
  fallbackValue,
  options,
  disabled,
  onChange,
  placeholder,
  allowClear,
  clearLabel,
  wide,
}: {
  label: string;
  name?: string;
  value?: string;
  fallbackValue?: string;
  options: { value: string; label: string }[];
  disabled?: boolean;
  onChange: (value: string | null) => void;
  placeholder?: string;
  allowClear?: boolean;
  clearLabel?: string;
  wide?: boolean;
}) {
  const selectedValue = value ?? fallbackValue ?? '';
  return (
    <label className={`field ${wide ? 'wideField' : ''}`}>
      <span>{label}</span>
      <SingleSelect<string>
        options={options}
        selected={selectedValue ? selectedValue : null}
        onChange={onChange}
        placeholder={placeholder}
        allowClear={allowClear}
        clearLabel={clearLabel}
        disabled={disabled}
      />
      {name ? <input type="hidden" name={name} value={selectedValue} /> : null}
    </label>
  );
}

function DocumentModal({
  open,
  onClose,
  providerId,
  replaceDocument,
  prefillType,
  onSaved,
}: {
  open: boolean;
  onClose: () => void;
  providerId: number;
  replaceDocument?: ProviderDocument;
  prefillType?: DocumentType;
  onSaved: () => void;
}) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const documentTypes = useDocumentTypes();
  const isReplace = replaceDocument !== undefined;
  const showTypeSelect = !isReplace && !prefillType;
  const documentName = replaceDocument?.document_type?.name ?? prefillType?.name ?? 'Documento';
  const title = isReplace ? `Sostituisci documento · ${documentName}` : `Nuovo documento${prefillType ? ' · ' + prefillType.name : ''}`;
  const replaceStateUpper = (replaceDocument?.state ?? '').toUpperCase();
  const replaceExpiryDays = daysUntilExpiry(replaceDocument?.expire_date);
  const replaceHasStateBadge = Boolean(replaceStateUpper && replaceStateUpper !== 'OK');
  const replaceHasUrgencyBadge = replaceExpiryDays !== null && replaceExpiryDays <= 30;
  const pending = mutations.uploadDocument.isPending || mutations.updateDocument.isPending;

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const file = data.get('file');
    if (!(file instanceof File) || file.size === 0) {
      toast(isReplace ? 'Seleziona il nuovo file del documento' : 'Seleziona un file da caricare', 'warning');
      return;
    }
    if (isReplace) {
      await mutations.updateDocument.mutateAsync({ id: replaceDocument.id, body: data });
    } else {
      data.set('provider_id', String(providerId));
      if (prefillType) data.set('document_type_id', String(prefillType.id));
      await mutations.uploadDocument.mutateAsync(data);
    }
    onClose();
    onSaved();
  }

  return (
    <Modal open={open} onClose={onClose} title={title} size="md">
      <form className="modalForm" onSubmit={(event) => void submit(event)}>
        {isReplace ? (
          <section className="currentDocumentSummary" aria-label="Documento attuale">
            <div className="currentDocumentSummaryHeader">
              <span className="fieldLabel">Documento attuale</span>
              <span className="docTypeBadges">
                <DocumentStateBadge state={replaceDocument.state} />
                <DocumentUrgencyBadge expireDate={replaceDocument.expire_date} />
                {!replaceHasStateBadge && !replaceHasUrgencyBadge ? <StatusBadge value="valid" label="Valido" variant="success" dot={false} /> : null}
              </span>
            </div>
            <div className="currentDocumentSummaryGrid">
              <div>
                <span>Tipo</span>
                <strong>{documentName}</strong>
              </div>
              <div>
                <span>Scadenza attuale</span>
                <strong>{dateLabel(replaceDocument.expire_date)}</strong>
              </div>
            </div>
          </section>
        ) : null}
        {showTypeSelect ? (
          <Select
            name="document_type_id"
            label="Tipo documento"
            options={(documentTypes.data ?? []).map((item) => ({ value: String(item.id), label: item.name }))}
            required
          />
        ) : null}
        <Input
          name="expire_date"
          label={isReplace ? 'Nuova scadenza' : 'Scadenza'}
          type="date"
          defaultValue={isReplace ? dateInputValue(replaceDocument.expire_date) : undefined}
          required
        />
        <label className="field">
          <span>{isReplace ? 'Nuovo file' : 'File'}</span>
          <input name="file" type="file" />
          {isReplace ? <small className="fieldHint">La sostituzione aggiorna file e scadenza del documento corrente.</small> : null}
        </label>
        <div className="modalActions">
          <Button variant="secondary" type="button" onClick={onClose}>Annulla</Button>
          <Button type="submit" loading={pending}>{isReplace ? 'Sostituisci' : 'Carica'}</Button>
        </div>
      </form>
    </Modal>
  );
}

export function QualificationSettingsPage() {
  const readonly = useHasRole('app_fornitori_readonly');
  const { toast } = useToast();
  const categories = useCategories();
  const documentTypes = useDocumentTypes();
  const mutations = useFornitoriMutations();
  const [required, setRequired] = useState<number[]>([]);
  const [optional, setOptional] = useState<number[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(null);
  const [selectedDocType, setSelectedDocType] = useState<DocumentType | null>(null);

  async function saveCategory(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const overlap = required.find((id) => optional.includes(id));
    if (overlap) {
      toast('Lo stesso tipo documento non puo essere obbligatorio e facoltativo', 'warning');
      return;
    }
    const name = String(new FormData(event.currentTarget).get('name') ?? '').trim();
    const body = { name, required_document_type_ids: required, optional_document_type_ids: optional };
    if (selectedCategory) await mutations.updateCategory.mutateAsync({ id: selectedCategory.id, body });
    else await mutations.createCategory.mutateAsync(body);
    setSelectedCategory(null);
    setRequired([]);
    setOptional([]);
    toast('Aggiornamento completato');
  }

  async function saveDocType(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = String(new FormData(event.currentTarget).get('name') ?? '').trim();
    if (selectedDocType) await mutations.updateDocumentType.mutateAsync({ id: selectedDocType.id, name });
    else await mutations.createDocumentType.mutateAsync({ name });
    setSelectedDocType(null);
    toast('Aggiornamento completato');
  }

  return (
    <main className="page">
      <header className="pageHeader"><div><h1>Impostazioni qualifica</h1><p>Categorie e tipi documento.</p></div></header>
      <div className="twoCols">
        <Panel title="Categorie">
          <div className="tableScroll adminScroll"><table className="table"><tbody>{(categories.data ?? []).map((item) => <tr key={item.id} onClick={() => setSelectedCategory(item)}><td>{item.name}</td><td className="rightText"><Icon name="chevron-right" size={16} /></td></tr>)}</tbody></table></div>
          <form className="modalForm" onSubmit={(event) => void saveCategory(event)}>
            <Input name="name" label="Categoria" defaultValue={selectedCategory?.name ?? ''} />
            <span className="fieldLabel">Documenti obbligatori</span><MultiSelect options={selectOptions(documentTypes.data)} selected={required} onChange={setRequired} />
            <span className="fieldLabel">Documenti facoltativi</span><MultiSelect options={selectOptions(documentTypes.data)} selected={optional} onChange={setOptional} />
            <div className="formActions">
              <Button type="submit" disabled={readonly}>Salva</Button>
              {selectedCategory ? <Button variant="danger" disabled={readonly} onClick={() => void mutations.deleteCategory.mutateAsync(selectedCategory.id)}>Elimina</Button> : null}
            </div>
          </form>
        </Panel>
        <Panel title="Tipi documento">
          <div className="tableScroll adminScroll"><table className="table"><tbody>{(documentTypes.data ?? []).map((item) => <tr key={item.id} onClick={() => setSelectedDocType(item)}><td>{item.name}</td><td className="rightText"><Icon name="chevron-right" size={16} /></td></tr>)}</tbody></table></div>
          <form className="modalForm" onSubmit={(event) => void saveDocType(event)}>
            <Input name="name" label="Tipo documento" defaultValue={selectedDocType?.name ?? ''} />
            <div className="formActions">
              <Button type="submit" disabled={readonly}>Salva</Button>
              {selectedDocType ? <Button variant="danger" disabled={readonly} onClick={() => void mutations.deleteDocumentType.mutateAsync(selectedDocType.id)}>Elimina</Button> : null}
            </div>
          </form>
        </Panel>
      </div>
    </main>
  );
}

export function PaymentMethodsPage() {
  const readonly = useHasRole('app_fornitori_readonly');
  const { toast } = useToast();
  const methods = usePaymentMethods();
  const mutations = useFornitoriMutations();

  async function toggle(code: string, checked: boolean) {
    await mutations.setPaymentRda.mutateAsync({ code, rda_available: checked });
    toast('Aggiornamento completato');
  }

  return (
    <main className="page">
      <header className="pageHeader"><div><h1>Pagamenti RDA</h1><p>Disponibilita dei metodi di pagamento per RDA.</p></div></header>
      <section className="panel">
        {methods.isLoading ? <Skeleton rows={8} /> : methods.error ? stateBlock(errorTitle(methods.error), 'I metodi di pagamento non possono essere caricati.') : (
          <div className="tableScroll"><table className="table">
            <thead><tr><th>Codice</th><th>Descrizione</th><th>Disponibile RDA</th></tr></thead>
            <tbody>{(methods.data ?? []).map((item) => (
              <tr key={item.code}><td>{item.code}</td><td>{item.description}</td><td><ToggleSwitch id={`rda-${item.code}`} checked={Boolean(item.rda_available)} disabled={readonly} onChange={(checked) => void toggle(item.code, checked)} /></td></tr>
            ))}</tbody>
          </table></div>
        )}
      </section>
    </main>
  );
}

export function ArticleCategoriesPage() {
  const readonly = useHasRole('app_fornitori_readonly');
  const { toast } = useToast();
  const articles = useArticleCategories();
  const categories = useCategories();
  const mutations = useFornitoriMutations();
  const [selected, setSelected] = useState<string | null>(null);
  const selectedItem = articles.data?.find((item) => item.article_code === selected);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedItem) return;
    const categoryId = Number(new FormData(event.currentTarget).get('category_id'));
    await mutations.setArticleCategory.mutateAsync({ articleCode: selectedItem.article_code, categoryId });
    toast('Aggiornamento completato');
  }

  return (
    <main className="page">
      <header className="pageHeader"><div><h1>Articoli-categorie</h1><p>Associazione tra articoli e categorie di qualifica.</p></div></header>
      <div className="workspace">
        <section className="master panel">
          {articles.isLoading ? <Skeleton rows={8} /> : articles.error ? stateBlock(errorTitle(articles.error), 'Gli articoli non possono essere caricati.') : (
            <div className="tableScroll"><table className="table">
              <thead><tr><th>Articolo</th><th>Categoria</th></tr></thead>
              <tbody>{(articles.data ?? []).map((item) => <tr key={item.article_code} className={selected === item.article_code ? 'selectedRow' : ''} onClick={() => setSelected(item.article_code)}><td>{item.article_code}<small>{item.description}</small></td><td>{item.category_name}</td></tr>)}</tbody>
            </table></div>
          )}
        </section>
        <section className="detail panel">
          {!selectedItem ? stateBlock('Seleziona un articolo', 'La categoria potra essere modificata dopo la selezione.') : (
            <form className="modalForm" onSubmit={(event) => void submit(event)}>
              <h2>{selectedItem.article_code}</h2>
              <p className="muted">{selectedItem.description}</p>
              <label className="field"><span>Categoria</span><select name="category_id" defaultValue={selectedItem.category_id}>{(categories.data ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
              <Button type="submit" disabled={readonly}>Salva</Button>
            </form>
          )}
        </section>
      </div>
    </main>
  );
}

function Input({ label, wide, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { label: string; wide?: boolean }) {
  return <label className={`field ${wide ? 'wideField' : ''}`}><span>{label}</span><input {...props} /></label>;
}

type SelectOption = string | { value: string; label: string };

function Select({ label, options, ...props }: React.SelectHTMLAttributes<HTMLSelectElement> & { label: string; options: SelectOption[] }) {
  return (
    <label className="field">
      <span>{label}</span>
      <select {...props}>
        {options.map((item) => {
          const value = typeof item === 'string' ? item : item.value;
          const optionLabel = typeof item === 'string' ? item || '-' : item.label;
          return <option key={value} value={value}>{optionLabel}</option>;
        })}
      </select>
    </label>
  );
}
