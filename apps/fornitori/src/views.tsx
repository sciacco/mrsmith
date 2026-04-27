import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, type IconName, Modal, MultiSelect, SearchInput, Skeleton, ToggleSwitch, useToast } from '@mrsmith/ui';
import { useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  useArticleCategories,
  useCategories,
  useDashboard,
  useDocumentTypes,
  useFornitoriMutations,
  usePaymentMethods,
  useProvider,
  useProviderCategories,
  useProviderDocuments,
  useProviderSummary,
} from './api/queries';
import type { Category, CategoryDocumentType, DashboardCategory, DocumentType, PaymentMethod, Provider, ProviderCategory, ProviderDocument, ProviderPayload, ProviderReference, ProviderSummary } from './api/types';
import { countries } from './lib/countries';
import { provinces } from './lib/provinces';
import { referenceTypeLabel, referenceTypes } from './lib/reference';
import { saveBlob } from './lib/download';
import { useHasRole } from './hooks/useHasRole';
import {
  CategoryStateBadge,
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

function getProviderRefs(provider?: Provider): ProviderReference[] {
  if (!provider) return [];
  if (provider.refs?.length) return provider.refs;
  return provider.ref ? [provider.ref] : [];
}

function qualificationRef(provider?: Provider) {
  return getProviderRefs(provider).find((ref) => ref.reference_type === 'QUALIFICATION_REF');
}

function selectOptions(items: Category[] | DocumentType[] | undefined) {
  return items?.map((item) => ({ value: item.id, label: item.name })) ?? [];
}

const languageOptions = [
  { value: 'it', label: 'Italiano' },
  { value: 'en', label: 'Inglese' },
];

const countryOptions = countries.map((item) => ({ value: item.code, label: item.name }));

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
    erp_id: erp ? Number(erp) : null,
    language: String(data.get('language') || 'it'),
    country,
    default_payment_method: String(data.get('default_payment_method') ?? '').trim() || null,
    ref: {
      first_name: String(data.get('ref_first_name') ?? '').trim(),
      last_name: String(data.get('ref_last_name') ?? '').trim(),
      email: String(data.get('ref_email') ?? '').trim(),
      phone: String(data.get('ref_phone') ?? '').trim(),
      reference_type: 'QUALIFICATION_REF',
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

export function DashboardPage() {
  const navigate = useNavigate();
  const { drafts, documents, categories } = useDashboard();
  const groupedCategories = useMemo(() => groupCategoriesByProvider(categories.data ?? []), [categories.data]);
  const loading = drafts.isLoading || documents.isLoading || categories.isLoading;
  const error = drafts.error ?? documents.error ?? categories.error;

  return (
    <main className="page">
      <header className="pageHeader">
        <div>
          <h1>Dashboard</h1>
          <p>Fornitori da qualificare, documenti in scadenza e categorie da gestire.</p>
        </div>
      </header>
      {loading ? <Skeleton rows={8} /> : error ? stateBlock(errorTitle(error), 'Le attività fornitori non possono essere caricate.', 'triangle-alert') : (
        <>
          <Panel title="Da qualificare" subtitle="Fornitori in stato bozza" count={drafts.data?.length ?? 0}>
            {(drafts.data ?? []).length === 0 ? stateBlock('Nessun fornitore in attesa', 'Tutti i fornitori arrivati da Mistra sono stati lavorati.', 'check-circle') : (
              <div className="tableScroll dashboardScroll">
                <table className="table">
                  <thead><tr><th>Ragione sociale</th><th>P.IVA</th><th>CF</th><th /></tr></thead>
                  <tbody>{(drafts.data ?? []).map((row) => (
                    <tr key={row.id} onClick={() => navigate(`/fornitori?id_provider=${row.id}`)}>
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
          <Panel title="Documenti in scadenza" subtitle="Scadenza ≤30 giorni, fornitori in bozza o attivi" count={documents.data?.length ?? 0}>
            {(documents.data ?? []).length === 0 ? stateBlock('Nessun documento in scadenza', 'Tutti i documenti dei fornitori attivi sono in regola.', 'check-circle') : (
              <div className="tableScroll dashboardScroll">
                <table className="table">
                  <thead><tr><th>Fornitore</th><th>Documento</th><th>Scadenza</th><th>Urgenza</th><th /></tr></thead>
                  <tbody>{(documents.data ?? []).map((row) => (
                    <tr key={row.id} onClick={() => navigate(`/fornitori?id_provider=${row.provider_id}&tab=Qualifica`)}>
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
          <Panel title="Categorie aperte" subtitle="Categorie da qualificare per fornitori in bozza o attivi" count={groupedCategories.length} countSuffix="fornitori">
            {groupedCategories.length === 0 ? stateBlock('Nessuna categoria aperta', 'Tutte le categorie assegnate sono qualificate.', 'check-circle') : (
              <div className="tableScroll dashboardScroll">
                <table className="table">
                  <thead><tr><th>Fornitore</th><th>Categorie aperte</th><th /></tr></thead>
                  <tbody>{groupedCategories.map((group) => (
                    <tr key={group.provider_id} onClick={() => navigate(`/fornitori?id_provider=${group.provider_id}&tab=Qualifica`)}>
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
                  ))}</tbody>
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

export function FornitoriPage() {
  const [params, setParams] = useSearchParams();
  const selectedId = Number(params.get('id_provider') ?? '') || null;
  const [query, setQuery] = useState('');
  const [showArchive, setShowArchive] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const summary = useProviderSummary();
  const summaryById = useMemo(() => {
    const map = new Map<number, ProviderSummary>();
    for (const item of summary.data ?? []) map.set(item.id, item);
    return map;
  }, [summary.data]);

  const { active, archive } = useMemo(() => {
    const all = summary.data ?? [];
    const matched = all.filter((item) => matchesProviderQuery(item, query));
    return {
      active: matched.filter((item) => USABLE_PROVIDER_STATES.has((item.state ?? '').toUpperCase())),
      archive: matched.filter((item) => !USABLE_PROVIDER_STATES.has((item.state ?? '').toUpperCase())),
    };
  }, [summary.data, query]);

  function selectProvider(id: number) {
    setParams({ id_provider: String(id) });
  }

  return (
    <main className="page">
      <header className="pageHeader">
        <div><h1>Fornitori</h1><p>Anagrafica, contatti, qualifica e documenti.</p></div>
        <Button leftIcon={<Icon name="plus" />} onClick={() => setCreateOpen(true)}>Nuovo fornitore</Button>
      </header>
      <div className="workspace">
        <section className="master panel">
          <div className="toolbar"><SearchInput value={query} onChange={setQuery} placeholder="Cerca per nome, P.IVA, CF, codice ERP" /></div>
          {summary.isLoading ? <Skeleton rows={8} /> : summary.error ? stateBlock(errorTitle(summary.error), 'Elenco fornitori non disponibile.', 'triangle-alert') : (
              <div className="listRows">
                {active.map((item) => (
                  <ProviderRow key={item.id} item={item} selected={item.id === selectedId} onSelect={selectProvider} />
                ))}
                {active.length === 0 && !showArchive ? stateBlock('Nessun fornitore trovato', 'Modifica i criteri di ricerca.', 'search') : null}
                {archive.length > 0 ? (
                  <button type="button" className="archiveToggle" onClick={() => setShowArchive((value) => !value)}>
                    <Icon name={showArchive ? 'chevron-down' : 'chevron-right'} size={14} />
                    <span>{showArchive ? 'Nascondi' : 'Mostra'} fornitori cessati e sospesi ({archive.length})</span>
                  </button>
                ) : null}
                {showArchive ? archive.map((item) => (
                  <ProviderRow key={item.id} item={item} selected={item.id === selectedId} onSelect={selectProvider} />
                )) : null}
            </div>
          )}
        </section>
        <section className="detail panel">
          {!selectedId
            ? stateBlock('Seleziona un fornitore', 'Scegli un fornitore dalla lista per vedere i dettagli.', 'user')
            : <ProviderPage providerId={selectedId} summary={summaryById.get(selectedId)} />}
        </section>
      </div>
      <ProviderCreateModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(id) => {
          setCreateOpen(false);
          setParams({ id_provider: String(id) });
        }}
      />
    </main>
  );
}

function ProviderRow({ item, selected, onSelect }: { item: ProviderSummary; selected: boolean; onSelect: (id: number) => void }) {
  const qualVariant = qualificationVariant(item);
  return (
    <button className={`listRow ${selected ? 'selected' : ''}`} onClick={() => onSelect(item.id)}>
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
  return {
    first_name: String(data.get('first_name') ?? ''),
    last_name: String(data.get('last_name') ?? ''),
    email: String(data.get('email') ?? ''),
    phone: String(data.get('phone') ?? ''),
    reference_type: type,
  };
}

function paymentCodeOf(value: Provider['default_payment_method']): string {
  if (value && typeof value === 'object') return value.code ?? '';
  if (typeof value === 'string') return value;
  return '';
}

function missingForActivation(provider: Provider): string[] {
  const missing: string[] = [];
  if (!provider.company_name?.trim()) missing.push('ragione sociale');
  if (!provider.address?.trim()) missing.push('indirizzo');
  if (!provider.city?.trim()) missing.push('città');
  if (!provider.postal_code?.trim()) missing.push('CAP');
  if (!provider.country?.trim()) missing.push('paese');
  if (!provider.erp_id || provider.erp_id <= 0) missing.push('codice ERP');
  if (!paymentCodeOf(provider.default_payment_method).trim()) missing.push('metodo di pagamento');
  if (getProviderRefs(provider).length === 0) missing.push('almeno un contatto');
  return missing;
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
  const [country, setCountry] = useState('IT');
  const [showAdvanced, setShowAdvanced] = useState(false);

  function close() {
    setCountry('IT');
    setShowAdvanced(false);
    onClose();
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
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
            <Select name="country" label="Paese" defaultValue="IT" options={countryOptions} onChange={(event) => setCountry(event.target.value)} />
            {country === 'IT' ? (
              <Select name="province" label="Provincia" options={['', ...provinces]} />
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
            <Input name="erp_id" label="Codice Alyante" type="number" />
            <PaymentMethodField defaultValue="" options={paymentMethods.data ?? []} disabled={false} />
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

function ProviderPage({ providerId, summary }: { providerId: number; summary?: ProviderSummary }) {
  const provider = useProvider(providerId);

  if (provider.isLoading) return <Skeleton rows={10} />;
  if (provider.error) return stateBlock(errorTitle(provider.error), 'Il dettaglio fornitore non può essere caricato.', 'triangle-alert');
  if (!provider.data) return null;

  const data = provider.data;
  const stateUpper = (data.state ?? '').toUpperCase();
  const isUsable = USABLE_PROVIDER_STATES.has(stateUpper);
  const isActive = stateUpper === 'ACTIVE';
  const isDraft = stateUpper === 'DRAFT';
  const fullReadonly = !isUsable;

  return (
    <div className="providerPage">
      <ProviderHeader provider={data} summary={summary} />
      {fullReadonly ? <StateBanner state={data.state} /> : null}
      {isDraft ? <CompletenessBanner provider={data} /> : null}
      <QualificationSection providerId={providerId} readonly={fullReadonly} />
      <ContactsSection provider={data} readonly={fullReadonly} />
      <AnagraficaSection provider={data} fullReadonly={fullReadonly} anagraficaLocked={isActive} />
    </div>
  );
}

function ProviderHeader({ provider, summary }: { provider: Provider; summary?: ProviderSummary }) {
  const total = summary?.total_count ?? 0;
  const qualified = summary?.qualified_count ?? 0;
  const percent = total > 0 ? Math.round((qualified / total) * 100) : 0;

  return (
    <header className="providerHeader">
      <div className="providerHeaderTop">
        <h2>{provider.company_name ?? '—'}</h2>
        <ProviderStateBadge state={provider.state} />
      </div>
      <p className="providerHeaderMeta">
        {[
          provider.vat_number ? `P.IVA ${provider.vat_number}` : null,
          provider.cf ? `CF ${provider.cf}` : null,
          provider.erp_id ? `ERP ${provider.erp_id}` : null,
        ].filter(Boolean).join(' · ') || 'Identificativi non disponibili'}
      </p>
      {total > 0 ? (
        <div className="providerHeaderProgress" aria-label={`Completezza qualifica ${percent}%`}>
          <div className="progressLabel">Qualifica · {qualified}/{total} categorie</div>
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
  const missing = missingForActivation(provider);
  if (missing.length === 0) {
    return (
      <div className="banner banner--success">
        <Icon name="check-circle" size={16} aria-hidden="true" />
        <span>Tutti i campi obbligatori sono compilati. Il fornitore è pronto per essere attivato.</span>
      </div>
    );
  }
  return (
    <div className="banner banner--info">
      <Icon name="info" size={16} aria-hidden="true" />
      <span>Per attivare il fornitore: {missing.join(', ')}.</span>
    </div>
  );
}

function QualificationSection({ providerId, readonly }: { providerId: number; readonly: boolean }) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const providerCategories = useProviderCategories(providerId);
  const allCategories = useCategories();
  const documents = useProviderDocuments(providerId);
  const [addOpen, setAddOpen] = useState(false);

  const categoryById = useMemo(() => {
    const map = new Map<number, Category>();
    for (const item of allCategories.data ?? []) map.set(item.id, item);
    return map;
  }, [allCategories.data]);

  const documentsByType = useMemo(() => {
    const map = new Map<number, ProviderDocument[]>();
    for (const doc of documents.data ?? []) {
      const typeId = doc.document_type?.id;
      if (typeId == null) continue;
      const list = map.get(typeId) ?? [];
      list.push(doc);
      map.set(typeId, list);
    }
    return map;
  }, [documents.data]);

  const rows = providerCategories.data ?? [];
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
      {providerCategories.isLoading ? <Skeleton rows={4} /> : rows.length === 0 ? stateBlock('Nessuna categoria assegnata', 'Aggiungi una categoria di qualifica per iniziare.', 'box') : (
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
}: {
  providerCategory: ProviderCategory;
  category?: Category;
  documentsByType: Map<number, ProviderDocument[]>;
  providerId: number;
  readonly: boolean;
}) {
  const state = (providerCategory.status ?? providerCategory.state ?? 'NEW').toUpperCase();
  const [open, setOpen] = useState(state !== 'QUALIFIED');
  const [uploadType, setUploadType] = useState<DocumentType | null>(null);
  const [editDocId, setEditDocId] = useState<number | null>(null);
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const docTypes = category?.document_types ?? [];
  const required = docTypes.filter((dt) => dt.required);
  const optional = docTypes.filter((dt) => !dt.required);

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
    <article className="qualificationCard" data-state={state}>
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
              onEdit={(id) => setEditDocId(id)}
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
              onEdit={(id) => setEditDocId(id)}
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
        open={editDocId !== null}
        onClose={() => setEditDocId(null)}
        providerId={providerId}
        documentId={editDocId ?? undefined}
        onSaved={() => toast('Documento aggiornato')}
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
  onEdit,
  onDownload,
}: {
  title: string;
  required: boolean;
  types: CategoryDocumentType[];
  documentsByType: Map<number, ProviderDocument[]>;
  readonly: boolean;
  onUpload: (type: DocumentType) => void;
  onEdit: (id: number) => void;
  onDownload: (id: number) => void;
}) {
  return (
    <div className="docGroup">
      <span className="docGroupTitle">{title}</span>
      <ul className="docTypeList">
        {types.map((entry) => {
          const docs = documentsByType.get(entry.document_type.id) ?? [];
          const hasUsable = docs.some((d) => (d.state ?? '').toUpperCase() === 'OK');
          const display = docs[0];
          return (
            <li key={entry.document_type.id} className="docTypeRow" data-required={required ? 'true' : 'false'} data-uploaded={display ? 'true' : 'false'}>
              <span className="docTypeIndicator" aria-hidden="true">{display ? <Icon name={hasUsable ? 'check-circle' : 'circle'} size={14} /> : <Icon name="circle" size={14} />}</span>
              <span className="docTypeName">{entry.document_type.name}</span>
              <span className="docTypeMeta">
                {display?.expire_date ? dateLabel(display.expire_date) : '—'}
              </span>
              <span className="docTypeBadges">
                <DocumentStateBadge state={display?.state} />
                <DocumentUrgencyBadge expireDate={display?.expire_date} />
              </span>
              <span className="docTypeActions">
                {display ? (
                  <>
                    <Button size="sm" variant="ghost" leftIcon={<Icon name="download" />} aria-label="Scarica documento" onClick={() => onDownload(display.id)} />
                    {!readonly ? <Button size="sm" variant="secondary" leftIcon={<Icon name="pencil" />} aria-label="Modifica documento" onClick={() => onEdit(display.id)} /> : null}
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

function ContactsSection({ provider, readonly }: { provider: Provider; readonly: boolean }) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const refs = getProviderRefs(provider).filter((ref) => ref.reference_type !== 'QUALIFICATION_REF');
  const [addOpen, setAddOpen] = useState(false);

  async function update(ref: ProviderReference, body: ProviderReference) {
    if (!ref.id) return;
    await mutations.updateReference.mutateAsync({ providerId: provider.id, refId: ref.id, body });
    toast('Contatto aggiornato');
  }

  async function add(body: ProviderReference) {
    await mutations.createReference.mutateAsync({ providerId: provider.id, body });
    toast('Contatto aggiunto');
    setAddOpen(false);
  }

  return (
    <Panel
      title="Contatti"
      subtitle={refs.length === 0 ? 'Nessun contatto registrato' : `${refs.length} contatto/i`}
      actions={!readonly ? (
        <Button size="sm" leftIcon={<Icon name="plus" />} onClick={() => setAddOpen(true)}>Aggiungi</Button>
      ) : undefined}
    >
      {refs.length === 0 && !addOpen ? stateBlock('Nessun contatto registrato', 'Aggiungi almeno un contatto amministrativo o tecnico.', 'user') : (
        <div className="contactsList">
          {refs.map((item) => (
            <ContactCard key={item.id} contact={item} readonly={readonly} onSave={(body) => void update(item, body)} pending={mutations.updateReference.isPending} />
          ))}
        </div>
      )}
      <ContactAddForm open={addOpen} onClose={() => setAddOpen(false)} onSave={(body) => void add(body)} pending={mutations.createReference.isPending} />
    </Panel>
  );
}

function ContactCard({
  contact,
  readonly,
  onSave,
  pending,
}: {
  contact: ProviderReference;
  readonly: boolean;
  onSave: (body: ProviderReference) => void;
  pending: boolean;
}) {
  const [editing, setEditing] = useState(false);

  if (!editing) {
    return (
      <article className="contactCard">
        <div className="contactCardMain">
          <span className="contactCardName">{[contact.first_name, contact.last_name].filter(Boolean).join(' ') || '—'}</span>
          <span className="contactCardType">{referenceTypeLabel(contact.reference_type)}</span>
        </div>
        <div className="contactCardMeta">
          <span>{contact.email || '—'}</span>
          <span>{contact.phone || '—'}</span>
        </div>
        {!readonly ? (
          <Button size="sm" variant="ghost" leftIcon={<Icon name="pencil" />} onClick={() => setEditing(true)}>Modifica</Button>
        ) : null}
      </article>
    );
  }

  return (
    <form
      className="contactForm"
      onSubmit={(event) => {
        event.preventDefault();
        const body = refPayload(event.currentTarget, contact.reference_type);
        onSave(body);
        setEditing(false);
      }}
    >
      <div className="contactFormGrid">
        <Input name="first_name" label="Nome" defaultValue={contact.first_name} />
        <Input name="last_name" label="Cognome" defaultValue={contact.last_name} />
        <Input name="email" label="Email" type="email" defaultValue={contact.email} />
        <Input name="phone" label="Telefono" defaultValue={contact.phone} />
      </div>
      <div className="formActions">
        <Button variant="secondary" type="button" onClick={() => setEditing(false)}>Annulla</Button>
        <Button type="submit" loading={pending} leftIcon={<Icon name="check" />}>Salva</Button>
      </div>
    </form>
  );
}

function ContactAddForm({
  open,
  onClose,
  onSave,
  pending,
}: {
  open: boolean;
  onClose: () => void;
  onSave: (body: ProviderReference) => void;
  pending: boolean;
}) {
  const [type, setType] = useState<string>('ADMINISTRATIVE_REF');
  if (!open) return null;
  return (
    <form
      className="contactForm contactForm--new"
      onSubmit={(event) => {
        event.preventDefault();
        const body = refPayload(event.currentTarget, type);
        onSave(body);
      }}
    >
      <div className="contactFormGrid">
        <label className="field">
          <span>Tipo contatto</span>
          <select value={type} onChange={(event) => setType(event.target.value)}>
            {referenceTypes.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select>
        </label>
        <Input name="first_name" label="Nome" />
        <Input name="last_name" label="Cognome" />
        <Input name="email" label="Email" type="email" />
        <Input name="phone" label="Telefono" />
      </div>
      <div className="formActions">
        <Button variant="secondary" type="button" onClick={onClose}>Annulla</Button>
        <Button type="submit" loading={pending} leftIcon={<Icon name="plus" />}>Aggiungi</Button>
      </div>
    </form>
  );
}

function AnagraficaSection({
  provider,
  fullReadonly,
  anagraficaLocked,
}: {
  provider: Provider;
  fullReadonly: boolean;
  anagraficaLocked: boolean;
}) {
  const { toast } = useToast();
  const skipRole = useHasRole('app_fornitori_skip_qualification');
  const mutations = useFornitoriMutations();
  const paymentMethods = usePaymentMethods();
  const [open, setOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const ref = qualificationRef(provider);
  const currentPaymentCode = paymentCodeOf(provider.default_payment_method);

  // Anagrafica master fields are locked when state=ACTIVE (cfr. trg_provider_state_guard).
  // Always editable on ACTIVE: payment method, qualification ref, skip flag.
  // Always editable on DRAFT.
  // Never editable on INACTIVE/CEASED (fullReadonly).
  const masterFieldsDisabled = fullReadonly || anagraficaLocked;
  const editableSidefieldsDisabled = fullReadonly;

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = providerPayload(event.currentTarget);
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
  }

  return (
    <Panel
      title="Anagrafica"
      subtitle={anagraficaLocked && !fullReadonly ? 'Anagrafica bloccata: provider attivo' : undefined}
      collapsible
      open={open}
      onToggle={() => setOpen((v) => !v)}
    >
      {open ? (
        <>
          <form className="formGrid" onSubmit={(event) => void submit(event)}>
            <input type="hidden" name="state" value={provider.state ?? 'DRAFT'} />
            <Input name="company_name" label="Ragione sociale" defaultValue={provider.company_name} disabled={masterFieldsDisabled} />
            <Input name="vat_number" label="P.IVA" defaultValue={provider.vat_number} disabled={masterFieldsDisabled} />
            <Input name="cf" label="CF" defaultValue={provider.cf} disabled={masterFieldsDisabled} />
            <Input name="erp_id" label="Codice Alyante" type="number" defaultValue={provider.erp_id ?? ''} disabled={masterFieldsDisabled} />
            <Select name="language" label="Lingua" defaultValue={provider.language ?? 'it'} options={languageOptions} disabled={masterFieldsDisabled} />
            <Select name="country" label="Paese" defaultValue={provider.country ?? 'IT'} options={countryOptions} disabled={masterFieldsDisabled} />
            <Select name="province" label="Provincia" defaultValue={provider.province ?? ''} options={['', ...provinces]} disabled={masterFieldsDisabled} />
            <Input name="city" label="Città" defaultValue={provider.city} disabled={masterFieldsDisabled} />
            <Input name="postal_code" label="CAP" defaultValue={provider.postal_code} disabled={masterFieldsDisabled} />
            <Input name="address" label="Indirizzo" defaultValue={provider.address} disabled={masterFieldsDisabled} wide />
            <PaymentMethodField defaultValue={currentPaymentCode} options={paymentMethods.data ?? []} disabled={editableSidefieldsDisabled} />
            <Input name="ref_first_name" label="Nome contatto qualifica" defaultValue={ref?.first_name} disabled={editableSidefieldsDisabled} />
            <Input name="ref_last_name" label="Cognome contatto qualifica" defaultValue={ref?.last_name} disabled={editableSidefieldsDisabled} />
            <Input name="ref_email" label="Email contatto qualifica" type="email" defaultValue={ref?.email} disabled={editableSidefieldsDisabled} />
            <Input name="ref_phone" label="Telefono contatto qualifica" defaultValue={ref?.phone} disabled={editableSidefieldsDisabled} />
            {skipRole ? (
              <label className="checkLine">
                <input name="skip_qualification_validation" type="checkbox" disabled={editableSidefieldsDisabled} defaultChecked={false} />
                Salta controllo qualifica
              </label>
            ) : null}
            {!fullReadonly ? (
              <div className="formActions">
                <Button type="submit" leftIcon={<Icon name="check" />} loading={mutations.updateProvider.isPending}>Salva anagrafica</Button>
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
        </>
      ) : null}
    </Panel>
  );
}

function PaymentMethodField({
  defaultValue,
  options,
  disabled,
}: {
  defaultValue: string;
  options: PaymentMethod[];
  disabled: boolean;
}) {
  const selectOptionsList: SelectOption[] = [
    { value: '', label: '—' },
    ...options.map((item) => ({ value: item.code, label: `${item.description} (${item.code.trim()})` })),
  ];
  return (
    <Select name="default_payment_method" label="Metodo di pagamento" defaultValue={defaultValue} options={selectOptionsList} disabled={disabled} />
  );
}

function DocumentModal({
  open,
  onClose,
  providerId,
  documentId,
  prefillType,
  onSaved,
}: {
  open: boolean;
  onClose: () => void;
  providerId: number;
  documentId?: number;
  prefillType?: DocumentType;
  onSaved: () => void;
}) {
  const { toast } = useToast();
  const mutations = useFornitoriMutations();
  const documentTypes = useDocumentTypes();
  const isEdit = documentId !== undefined && documentId !== null;
  const showTypeSelect = !isEdit && !prefillType;

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const file = data.get('file');
    if (!(file instanceof File) || file.size === 0) {
      toast('Seleziona un file da caricare', 'warning');
      return;
    }
    data.set('provider_id', String(providerId));
    if (prefillType) data.set('document_type_id', String(prefillType.id));
    if (isEdit) await mutations.updateDocument.mutateAsync({ id: documentId, body: data });
    else await mutations.uploadDocument.mutateAsync(data);
    onClose();
    onSaved();
  }

  return (
    <Modal open={open} onClose={onClose} title={isEdit ? 'Modifica documento' : `Nuovo documento${prefillType ? ' · ' + prefillType.name : ''}`} size="md">
      <form className="modalForm" onSubmit={(event) => void submit(event)}>
        {showTypeSelect ? (
          <Select
            name="document_type_id"
            label="Tipo documento"
            options={(documentTypes.data ?? []).map((item) => ({ value: String(item.id), label: item.name }))}
          />
        ) : null}
        <Input name="expire_date" label="Scadenza" type="date" />
        <label className="field"><span>File</span><input name="file" type="file" /></label>
        <div className="modalActions">
          <Button variant="secondary" type="button" onClick={onClose}>Annulla</Button>
          <Button type="submit" loading={mutations.uploadDocument.isPending || mutations.updateDocument.isPending}>Salva</Button>
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
