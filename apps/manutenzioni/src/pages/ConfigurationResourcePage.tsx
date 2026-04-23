import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Button, Icon, Modal, SearchInput, Skeleton, ToggleSwitch, useToast } from '@mrsmith/ui';
import { useCallback, useDeferredValue, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import {
  useConfigCounts,
  useConfigList,
  useConfigMutations,
  useConfigUsage,
  useReferenceData,
} from '../api/queries';
import type { ReferenceItem } from '../api/types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { StatusPill } from '../components/StatusPill';
import { errorMessage } from '../lib/format';
import { capitalize, getResourceMeta, type ResourceMeta } from '../lib/resourceMeta';
import shared from './shared.module.css';

type ActiveFilter = 'active' | 'inactive' | 'all';

function parseActive(value: string | null): ActiveFilter {
  if (value === 'active' || value === 'inactive') return value;
  return 'all';
}

const clockFormatter = new Intl.DateTimeFormat('it-IT', {
  hour: '2-digit',
  minute: '2-digit',
});

function formatClockTime(timestamp: number): string {
  return clockFormatter.format(new Date(timestamp));
}

export function ConfigurationResourcePage() {
  const navigate = useNavigate();
  const params = useParams();
  const resource = params.resource ?? '';
  const meta = getResourceMeta(resource);
  const [searchParams, setSearchParams] = useSearchParams();
  const active = parseActive(searchParams.get('active'));
  const q = searchParams.get('q') ?? '';
  const deferredQ = useDeferredValue(q);
  const domainParam = searchParams.get('domain');
  const domainFilter = useMemo(() => {
    if (!domainParam) return null;
    const parsed = Number(domainParam);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
  }, [domainParam]);
  const [editing, setEditing] = useState<ReferenceItem | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [confirm, setConfirm] = useState<ReferenceItem | null>(null);
  const list = useConfigList(resource, active, deferredQ);
  const counts = useConfigCounts(resource);
  const reference = useReferenceData();
  const mutations = useConfigMutations(resource);
  const usage = useConfigUsage(resource, confirm?.is_active ? confirm.id : null);
  const toast = useToast();

  const filterActive = resource === 'service-taxonomy' && domainFilter !== null;
  const selectedDomain = useMemo(() => {
    if (!filterActive) return null;
    return reference.data?.technical_domains.find((d) => d.id === domainFilter) ?? null;
  }, [filterActive, domainFilter, reference.data?.technical_domains]);
  const displayItems = useMemo(() => {
    const items = list.data ?? [];
    if (!filterActive) return items;
    return items.filter((item) => item.technical_domain_id === domainFilter);
  }, [list.data, filterActive, domainFilter]);
  const serviceCountByDomain = useMemo(() => {
    if (resource !== 'technical-domains') return null;
    const services = reference.data?.service_taxonomy ?? [];
    const counts = new Map<number, number>();
    for (const service of services) {
      if (!service.is_active) continue;
      const domainId = service.technical_domain_id ?? 0;
      if (!domainId) continue;
      counts.set(domainId, (counts.get(domainId) ?? 0) + 1);
    }
    return counts;
  }, [resource, reference.data?.service_taxonomy]);

  const confirmActive = useMemo(() => {
    if (!confirm) return false;
    return confirm.is_active;
  }, [confirm]);

  function updateParam(key: string, value: string) {
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current);
        if (value) next.set(key, value);
        else next.delete(key);
        return next;
      },
      { replace: true },
    );
  }

  function setActive(value: ActiveFilter) {
    updateParam('active', value === 'all' ? '' : value);
  }

  function clearSearch() {
    updateParam('q', '');
  }

  if (!meta) {
    return (
      <section className={shared.emptyCard}>
        <div className={shared.emptyIconDanger}>
          <Icon name="triangle-alert" />
        </div>
        <h3>Configurazione non trovata</h3>
        <p>La pagina richiesta non è disponibile.</p>
      </section>
    );
  }

  async function toggleActive() {
    if (!confirm || !meta) return;
    try {
      if (confirm.is_active) await mutations.deactivate.mutateAsync(confirm.id);
      else await mutations.reactivate.mutateAsync(confirm.id);
      toast.toast(
        confirm.is_active
          ? `${capitalize(meta.singular)} disattivato.`
          : `${capitalize(meta.singular)} riattivato.`,
      );
      setConfirm(null);
    } catch (error) {
      toast.toast(errorMessage(error, 'Aggiornamento non riuscito.'), 'error');
    }
  }

  const totalActive = counts.data?.active ?? 0;
  const totalInactive = counts.data?.inactive ?? 0;
  const totalAll = totalActive + totalInactive;
  const isEmpty = !list.isLoading && !list.error && displayItems.length === 0;
  const hasSearch = q.trim().length > 0;
  const lastUpdated = list.dataUpdatedAt ? formatClockTime(list.dataUpdatedAt) : null;

  const supportsReorder = meta.fields.sort_order !== 'hidden';
  const canReorder = supportsReorder && !hasSearch && !filterActive;
  const sortableIds = useMemo(() => displayItems.map((item) => item.id), [displayItems]);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active: dragged, over } = event;
      if (!over || dragged.id === over.id) return;
      const items = list.data;
      if (!items) return;
      const oldIndex = items.findIndex((it) => it.id === Number(dragged.id));
      const newIndex = items.findIndex((it) => it.id === Number(over.id));
      if (oldIndex < 0 || newIndex < 0) return;
      const reordered = arrayMove(items, oldIndex, newIndex);
      const payload = reordered.map((it, index) => ({ id: it.id, sort_order: (index + 1) * 10 }));
      mutations.reorder.mutate(payload, {
        onError: (error) => toast.toast(errorMessage(error, 'Riordino non riuscito.'), 'error'),
      });
    },
    [list.data, mutations.reorder, toast],
  );

  const breadcrumbLabel = filterActive ? selectedDomain?.name_it ?? 'Dominio' : null;

  return (
    <section className={shared.page}>
      {filterActive ? (
        <nav className={shared.breadcrumb} aria-label="Percorso">
          <button
            type="button"
            className={shared.breadcrumbLink}
            onClick={() => navigate('/manutenzioni/configurazione/technical-domains')}
          >
            Domini tecnici
          </button>
          <Icon name="chevron-right" size={14} />
          <span className={shared.breadcrumbCurrent}>{breadcrumbLabel}</span>
          <Icon name="chevron-right" size={14} />
          <span className={shared.breadcrumbCurrent}>Servizi</span>
        </nav>
      ) : (
        <button
          type="button"
          className={shared.backLink}
          onClick={() => navigate('/manutenzioni/configurazione')}
        >
          <Icon name="chevron-left" size={16} />
          Torna alla configurazione
        </button>
      )}
      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>{meta.title}</h1>
          <p className={shared.pageSubtitle}>{meta.subtitle}</p>
        </div>
        <div className={shared.headerActions}>
          <Button
            variant="secondary"
            onClick={() => {
              list.refetch();
              counts.refetch();
            }}
            loading={list.isFetching && !list.isLoading}
            leftIcon={<Icon name="loader" size={16} />}
          >
            Aggiorna
          </Button>
          <Button
            onClick={() => {
              setEditing(null);
              setModalOpen(true);
            }}
            leftIcon={<Icon name="plus" size={16} />}
          >
            Nuovo {meta.singular}
          </Button>
        </div>
      </div>

      <div
        className={shared.filterBar}
        style={{
          gridTemplateColumns: filterActive
            ? 'minmax(240px, 1fr) auto auto'
            : 'minmax(240px, 1fr) auto',
        }}
      >
        <SearchInput
          value={q}
          onChange={(value) => updateParam('q', value)}
          placeholder="Cerca per codice, nome o descrizione..."
        />
        {filterActive ? (
          <button
            type="button"
            className={shared.filterChip}
            onClick={() => updateParam('domain', '')}
          >
            Dominio: {breadcrumbLabel}
            <Icon name="x" size={14} />
          </button>
        ) : null}
        <div className={shared.segmented}>
          {([
            ['active', 'Attivi'],
            ['inactive', 'Non attivi'],
            ['all', 'Tutti'],
          ] as Array<[ActiveFilter, string]>).map(([value, label]) => (
            <button
              key={value}
              type="button"
              className={`${shared.segment} ${active === value ? shared.segmentActive : ''}`}
              onClick={() => setActive(value)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {list.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={6} />
        </div>
      ) : list.error ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Elenco non disponibile</h3>
          <p>{errorMessage(list.error, 'Impossibile caricare i valori.')}</p>
        </div>
      ) : isEmpty ? (
        <EmptyState
          meta={meta}
          context={emptyContext({ totalAll, hasSearch, active, filterActive })}
          searchValue={q}
          domainLabel={breadcrumbLabel}
          onCreate={() => {
            setEditing(null);
            setModalOpen(true);
          }}
          onClearSearch={clearSearch}
        />
      ) : (
        <div className={shared.tableCard}>
          {supportsReorder && (hasSearch || filterActive) ? (
            <div className={shared.dragHint}>
              <Icon name="info" size={14} />
              {hasSearch
                ? 'Cancella la ricerca per riordinare le voci.'
                : 'Rimuovi il filtro dominio per riordinare le voci.'}
            </div>
          ) : null}
          <div className={shared.tableScroll}>
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
              <SortableContext items={sortableIds} strategy={verticalListSortingStrategy}>
                <table className={shared.table}>
                  <thead>
                    <tr>
                      {supportsReorder ? <th className={shared.dragHandleCell} aria-label="Riordina" /> : null}
                      <th>Codice</th>
                      <th>Nome</th>
                      {resource === 'technical-domains' && <th>Servizi</th>}
                      {resource === 'service-taxonomy' && !filterActive && <th>Dominio</th>}
                      {resource === 'sites' && <th>Città</th>}
                      <th>Stato</th>
                      <th className={shared.actionsCell}>Azioni</th>
                    </tr>
                  </thead>
                  <tbody>
                    {displayItems.map((item) => (
                      <SortableConfigRow
                        key={item.id}
                        item={item}
                        resource={resource}
                        supportsReorder={supportsReorder}
                        canReorder={canReorder}
                        hideDomainColumn={filterActive}
                        serviceCount={
                          resource === 'technical-domains'
                            ? serviceCountByDomain?.get(item.id) ?? 0
                            : null
                        }
                        onEdit={() => {
                          setEditing(item);
                          setModalOpen(true);
                        }}
                        onToggle={() => setConfirm(item)}
                      />
                    ))}
                  </tbody>
                </table>
              </SortableContext>
            </DndContext>
          </div>
          <div className={shared.tableFooter}>
            <span>
              {totalAll} {totalAll === 1 ? 'valore' : 'valori'} totali
              <span className={shared.dot}>·</span>
              {totalActive} attivi
              <span className={shared.dot}>·</span>
              {totalInactive} non attivi
            </span>
            {lastUpdated ? <span>Aggiornato alle {lastUpdated}</span> : null}
          </div>
        </div>
      )}

      <ConfigModal
        open={modalOpen}
        meta={meta}
        item={editing}
        technicalDomains={reference.data?.technical_domains ?? []}
        defaultTechnicalDomainId={filterActive ? domainFilter : null}
        onClose={() => setModalOpen(false)}
        onSaved={() => {
          setModalOpen(false);
          setEditing(null);
        }}
      />
      <ConfirmDialog
        open={confirm !== null}
        title={
          confirmActive
            ? `Disattiva ${meta.singular}`
            : `Riattiva ${meta.singular}`
        }
        message={
          confirmActive
            ? `${capitalize(meta.singular)} non sarà più proposto nei nuovi inserimenti. Puoi riattivarlo in qualsiasi momento dall'elenco "Non attivi".`
            : `${capitalize(meta.singular)} tornerà disponibile per le nuove manutenzioni.`
        }
        details={
          confirmActive ? (
            <UsageHint loading={usage.isLoading} error={Boolean(usage.error)} count={usage.data?.active_maintenances ?? null} />
          ) : undefined
        }
        confirmLabel={confirmActive ? 'Disattiva' : 'Riattiva'}
        variant={confirmActive ? 'danger' : 'primary'}
        busy={mutations.deactivate.isPending || mutations.reactivate.isPending}
        onClose={() => setConfirm(null)}
        onConfirm={toggleActive}
      />
    </section>
  );
}

function SortableConfigRow({
  item,
  resource,
  supportsReorder,
  canReorder,
  hideDomainColumn,
  serviceCount,
  onEdit,
  onToggle,
}: {
  item: ReferenceItem;
  resource: string;
  supportsReorder: boolean;
  canReorder: boolean;
  hideDomainColumn: boolean;
  serviceCount: number | null;
  onEdit: () => void;
  onToggle: () => void;
}) {
  const sortable = useSortable({ id: item.id, disabled: !canReorder });
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = sortable;
  const style = supportsReorder
    ? {
        transform: CSS.Transform.toString(transform),
        transition,
      }
    : undefined;

  return (
    <tr
      ref={supportsReorder ? setNodeRef : undefined}
      style={style}
      className={isDragging ? shared.rowDragging : undefined}
    >
      {supportsReorder ? (
        <td className={shared.dragHandleCell}>
          <button
            type="button"
            className={shared.dragHandle}
            aria-label={canReorder ? 'Trascina per riordinare' : 'Riordinamento non disponibile con filtri attivi'}
            title={canReorder ? 'Trascina per riordinare' : 'Cancella la ricerca per riordinare'}
            {...attributes}
            {...(canReorder ? listeners : {})}
          >
            <Icon name="grip-vertical" size={16} />
          </button>
        </td>
      ) : null}
      <td className={shared.mono}>{item.code}</td>
      <td>
        <div className={shared.rowTitle}>
          <strong>{item.name_it}</strong>
          {item.description && <span className={shared.small}>{item.description}</span>}
        </div>
      </td>
      {resource === 'technical-domains' && serviceCount !== null ? (
        <td>
          <Link
            to={`/manutenzioni/configurazione/service-taxonomy?domain=${item.id}`}
            className={shared.cellLink}
          >
            {serviceCount > 0 ? `${serviceCount}` : 'Aggiungi'}
            <Icon name="chevron-right" size={12} />
          </Link>
        </td>
      ) : null}
      {resource === 'service-taxonomy' && !hideDomainColumn && <td>{item.technical_domain_name ?? '-'}</td>}
      {resource === 'sites' && <td>{item.city ?? '-'}</td>}
      <td>
        <StatusPill tone={item.is_active ? 'success' : 'neutral'}>
          {item.is_active ? 'Attivo' : 'Non attivo'}
        </StatusPill>
      </td>
      <td className={shared.actionsCell}>
        <div className={shared.inlineActions}>
          <Button size="sm" variant="secondary" onClick={onEdit}>
            Modifica
          </Button>
          <Button size="sm" variant={item.is_active ? 'danger' : 'secondary'} onClick={onToggle}>
            {item.is_active ? 'Disattiva' : 'Riattiva'}
          </Button>
        </div>
      </td>
    </tr>
  );
}

function UsageHint({
  loading,
  error,
  count,
}: {
  loading: boolean;
  error: boolean;
  count: number | null;
}) {
  if (loading) {
    return <span>Verifica utilizzo in corso…</span>;
  }
  if (error || count === null) {
    return null;
  }
  if (count === 0) {
    return <span>Nessuna manutenzione attiva usa questo valore.</span>;
  }
  return (
    <span>
      Usato in {count} {count === 1 ? 'manutenzione corrente' : 'manutenzioni correnti'} — resteranno invariate.
    </span>
  );
}

type EmptyContext =
  | 'firstRun'
  | 'noSearchMatch'
  | 'inactiveTabEmpty'
  | 'activeTabEmpty'
  | 'domainScopedEmpty';

function emptyContext({
  totalAll,
  hasSearch,
  active,
  filterActive,
}: {
  totalAll: number;
  hasSearch: boolean;
  active: ActiveFilter;
  filterActive: boolean;
}): EmptyContext {
  if (hasSearch) return 'noSearchMatch';
  if (filterActive) return 'domainScopedEmpty';
  if (totalAll === 0) return 'firstRun';
  if (active === 'inactive') return 'inactiveTabEmpty';
  return 'activeTabEmpty';
}

function EmptyState({
  meta,
  context,
  searchValue,
  domainLabel,
  onCreate,
  onClearSearch,
}: {
  meta: ResourceMeta;
  context: EmptyContext;
  searchValue: string;
  domainLabel: string | null;
  onCreate: () => void;
  onClearSearch: () => void;
}) {
  if (context === 'firstRun') {
    return (
      <div className={shared.emptyCard}>
        <div className={shared.emptyIcon}>
          <Icon name="package" />
        </div>
        <h3>Nessun {meta.singular} configurato</h3>
        <p>Aggiungi il primo {meta.singular} per renderlo disponibile nelle manutenzioni.</p>
        <div style={{ marginTop: '1rem' }}>
          <Button onClick={onCreate} leftIcon={<Icon name="plus" size={16} />}>
            Nuovo {meta.singular}
          </Button>
        </div>
      </div>
    );
  }
  if (context === 'domainScopedEmpty') {
    return (
      <div className={shared.emptyCard}>
        <div className={shared.emptyIcon}>
          <Icon name="package" />
        </div>
        <h3>Nessun servizio in {domainLabel ?? 'questo dominio'}</h3>
        <p>Aggiungi il primo servizio associato al dominio selezionato.</p>
        <div style={{ marginTop: '1rem' }}>
          <Button onClick={onCreate} leftIcon={<Icon name="plus" size={16} />}>
            Aggiungi servizio
          </Button>
        </div>
      </div>
    );
  }
  if (context === 'noSearchMatch') {
    return (
      <div className={shared.emptyCard}>
        <div className={shared.emptyIcon}>
          <Icon name="search" />
        </div>
        <h3>Nessun {meta.singular} trovato</h3>
        <p>
          Nessun {meta.singular} corrisponde a &ldquo;{searchValue}&rdquo;.
        </p>
        <div style={{ marginTop: '1rem' }}>
          <Button variant="secondary" onClick={onClearSearch}>
            Cancella ricerca
          </Button>
        </div>
      </div>
    );
  }
  if (context === 'inactiveTabEmpty') {
    return (
      <div className={shared.emptyCard}>
        <div className={shared.emptyIcon}>
          <Icon name="check-circle" />
        </div>
        <h3>Nessun {meta.singular} disattivato</h3>
        <p>Tutti i {meta.plural} sono attualmente attivi.</p>
      </div>
    );
  }
  // activeTabEmpty — tutti disattivati, tab "Attivi" vuota
  return (
    <div className={shared.emptyCard}>
      <div className={shared.emptyIcon}>
        <Icon name="package" />
      </div>
      <h3>Nessun {meta.singular} attivo</h3>
      <p>Tutti i {meta.plural} configurati sono attualmente disattivi.</p>
    </div>
  );
}

type FormState = {
  code: string;
  name_it: string;
  name_en: string;
  description: string;
  city: string;
  country_code: string;
  technical_domain_id: string;
  is_active: boolean;
};

type FormErrors = Partial<Record<keyof FormState, string>>;

function ConfigModal({
  open,
  meta,
  item,
  technicalDomains,
  defaultTechnicalDomainId,
  onClose,
  onSaved,
}: {
  open: boolean;
  meta: ResourceMeta;
  item: ReferenceItem | null;
  technicalDomains: ReferenceItem[];
  defaultTechnicalDomainId: number | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const mutations = useConfigMutations(meta.key);
  const toast = useToast();
  const [form, setForm] = useState<FormState>(() => formFromItem(item, defaultTechnicalDomainId));
  const [errors, setErrors] = useState<FormErrors>({});
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (open) {
      setForm(formFromItem(item, defaultTechnicalDomainId));
      setErrors({});
      setSubmitted(false);
    }
  }, [item, open, defaultTechnicalDomainId]);

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
    if (submitted) {
      setErrors((prev) => {
        if (!prev[key]) return prev;
        const next = { ...prev };
        delete next[key];
        return next;
      });
    }
  }

  function validate(): FormErrors {
    const next: FormErrors = {};
    if (!item && !form.code.trim()) next.code = 'Il codice è obbligatorio.';
    if (!form.name_it.trim()) next.name_it = 'Il nome è obbligatorio.';
    if (meta.fields.technical_domain_id === 'required' && !form.technical_domain_id) {
      next.technical_domain_id = 'Seleziona un dominio tecnico.';
    }
    if (meta.fields.city === 'required' && !form.city.trim()) {
      next.city = 'La città è obbligatoria.';
    }
    if (meta.fields.country_code === 'required') {
      const code = form.country_code.trim();
      if (!code) next.country_code = 'Il codice paese è obbligatorio.';
      else if (code.length !== 2) next.country_code = 'Il codice paese deve essere di 2 caratteri (ISO-3166 alpha-2).';
    }
    return next;
  }

  async function save() {
    setSubmitted(true);
    const nextErrors = validate();
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;

    const body = {
      code: form.code.trim(),
      name_it: form.name_it.trim(),
      name_en: form.name_en.trim() || null,
      description: form.description.trim() || null,
      city: form.city.trim() || null,
      country_code: form.country_code.trim() || null,
      technical_domain_id: form.technical_domain_id ? Number(form.technical_domain_id) : undefined,
      is_active: form.is_active,
    };
    try {
      if (item) await mutations.update.mutateAsync({ id: item.id, body });
      else await mutations.create.mutateAsync(body);
      toast.toast(`${capitalize(meta.singular)} salvato.`);
      onSaved();
    } catch (error) {
      toast.toast(errorMessage(error, 'Salvataggio non riuscito.'), 'error');
    }
  }

  const showNameEn = meta.fields.name_en !== 'hidden';
  const showTechnicalDomain = meta.fields.technical_domain_id !== undefined;
  const showCity = meta.fields.city !== undefined;
  const showCountry = meta.fields.country_code !== undefined;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={item ? `Modifica ${meta.singular}` : `Nuovo ${meta.singular}`}
      size="lg"
    >
      <div className={shared.formGrid}>
        <label className={shared.label}>
          <span className={shared.labelText}>
            Codice<span className={shared.required}>*</span>
          </span>
          <input
            className={`${shared.field} ${errors.code ? shared.fieldInvalid : ''}`}
            value={form.code}
            disabled={Boolean(item)}
            autoFocus={!item}
            onChange={(event) => update('code', event.target.value)}
          />
          {errors.code ? (
            <span className={shared.fieldError}>{errors.code}</span>
          ) : item ? (
            <span className={shared.fieldHelper}>Il codice non è modificabile dopo la creazione.</span>
          ) : (
            <span className={shared.fieldHelper}>
              Identificativo univoco in minuscolo, es. <code>data_center_milano</code>.
            </span>
          )}
        </label>
        <label className={shared.label}>
          <span className={shared.labelText}>
            Nome (italiano)<span className={shared.required}>*</span>
          </span>
          <input
            className={`${shared.field} ${errors.name_it ? shared.fieldInvalid : ''}`}
            value={form.name_it}
            autoFocus={Boolean(item)}
            onChange={(event) => update('name_it', event.target.value)}
          />
          {errors.name_it ? <span className={shared.fieldError}>{errors.name_it}</span> : null}
        </label>
        {showNameEn ? (
          <label className={shared.label}>
            <span className={shared.labelText}>Nome (inglese)</span>
            <input
              className={shared.field}
              value={form.name_en}
              onChange={(event) => update('name_en', event.target.value)}
            />
          </label>
        ) : null}
        {showTechnicalDomain ? (
          <label className={shared.label}>
            <span className={shared.labelText}>
              Dominio<span className={shared.required}>*</span>
            </span>
            <select
              className={`${shared.select} ${errors.technical_domain_id ? shared.fieldInvalid : ''}`}
              value={form.technical_domain_id}
              onChange={(event) => update('technical_domain_id', event.target.value)}
            >
              <option value="">Seleziona dominio</option>
              {technicalDomains.map((domain) => (
                <option key={domain.id} value={domain.id}>
                  {domain.name_it}
                </option>
              ))}
            </select>
            {errors.technical_domain_id ? (
              <span className={shared.fieldError}>{errors.technical_domain_id}</span>
            ) : null}
          </label>
        ) : null}
        {showCity ? (
          <label className={shared.label}>
            <span className={shared.labelText}>
              Città<span className={shared.required}>*</span>
            </span>
            <input
              className={`${shared.field} ${errors.city ? shared.fieldInvalid : ''}`}
              value={form.city}
              onChange={(event) => update('city', event.target.value)}
            />
            {errors.city ? <span className={shared.fieldError}>{errors.city}</span> : null}
          </label>
        ) : null}
        {showCountry ? (
          <label className={shared.label}>
            <span className={shared.labelText}>
              Paese (ISO-2)<span className={shared.required}>*</span>
            </span>
            <input
              className={`${shared.field} ${errors.country_code ? shared.fieldInvalid : ''}`}
              value={form.country_code}
              onChange={(event) => update('country_code', event.target.value.toUpperCase())}
              maxLength={2}
              placeholder="IT"
            />
            {errors.country_code ? <span className={shared.fieldError}>{errors.country_code}</span> : null}
          </label>
        ) : null}
        <label className={`${shared.label} ${shared.formGridSpan}`}>
          <span className={shared.labelText}>Descrizione</span>
          <textarea
            className={shared.textarea}
            value={form.description}
            onChange={(event) => update('description', event.target.value)}
          />
        </label>
        {item ? (
          <div className={`${shared.label} ${shared.formGridSpan}`}>
            <span className={shared.labelText}>Stato</span>
            <div style={{ marginTop: '0.25rem' }}>
              <ToggleSwitch
                id={`config-active-${item.id}`}
                checked={form.is_active}
                onChange={(checked) => update('is_active', checked)}
                label={form.is_active ? 'Attivo' : 'Non attivo'}
              />
            </div>
            <span className={shared.fieldHelper}>
              Disattivare un {meta.singular} lo rimuove dai nuovi inserimenti ma non tocca le manutenzioni esistenti.
            </span>
          </div>
        ) : null}
      </div>
      <div className={shared.formActions} style={{ marginTop: '1rem' }}>
        <Button variant="secondary" onClick={onClose}>
          Annulla
        </Button>
        <Button onClick={save} loading={mutations.create.isPending || mutations.update.isPending}>
          Salva
        </Button>
      </div>
    </Modal>
  );
}

function formFromItem(
  item: ReferenceItem | null,
  defaultTechnicalDomainId: number | null = null,
): FormState {
  const defaultDomain =
    !item && defaultTechnicalDomainId ? String(defaultTechnicalDomainId) : '';
  return {
    code: item?.code ?? '',
    name_it: item?.name_it ?? '',
    name_en: item?.name_en ?? '',
    description: item?.description ?? '',
    city: item?.city ?? '',
    country_code: item?.country_code ?? '',
    technical_domain_id: item?.technical_domain_id
      ? String(item.technical_domain_id)
      : defaultDomain,
    is_active: item ? item.is_active : true,
  };
}
