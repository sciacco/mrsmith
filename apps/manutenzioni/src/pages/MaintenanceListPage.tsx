import { Button, Icon, MultiSelect, SearchInput, Skeleton, Tooltip } from '@mrsmith/ui';
import { useDeferredValue, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import type {
  MaintenanceListItem,
  MaintenanceRadarResponse,
  ReferenceItem,
  StatusCount,
} from '../api/types';
import { useMaintenanceRadar, useMaintenances, useReferenceData } from '../api/queries';
import { Pagination } from '../components/Pagination';
import { StatusPill, statusTone } from '../components/StatusPill';
import {
  STATUS_OPTIONS,
  errorMessage,
  formatDateTime,
  minutesLabel,
  noticesSummary,
  statusLabel,
} from '../lib/format';
import shared from './shared.module.css';
import styles from './MaintenanceListPage.module.css';

type WindowBucket = 'next7' | 'next45' | 'sixMonths' | 'unscheduled';

const STATUS_FILTER_OPTIONS = STATUS_OPTIONS.map((status) => ({
  value: status,
  label: statusLabel(status),
}));

function referenceLabel(items: ReferenceItem[] | undefined, value: string | null, fallback: string) {
  if (!value) return fallback;
  return items?.find((item) => String(item.id) === value)?.name_it ?? fallback;
}

function dateKey(value?: string | null) {
  if (!value) return null;
  const key = value.slice(0, 10);
  return /^\d{4}-\d{2}-\d{2}$/.test(key) ? key : null;
}

function windowBucket(item: MaintenanceListItem, radar: MaintenanceRadarResponse): WindowBucket {
  if (!item.current_window?.scheduled_start_at) return 'unscheduled';
  const start = dateKey(item.current_window.scheduled_start_at);
  if (!start) return 'unscheduled';
  if (start >= radar.today && start <= radar.next_7_days_to) return 'next7';
  if (start >= radar.next_45_days_from && start <= radar.next_45_days_to) return 'next45';
  return 'sixMonths';
}

function formatRadarDate(value: string) {
  const [year, month, day] = value.split('-').map(Number);
  if (!year || !month || !day) return value;
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'medium' }).format(
    new Date(year, month - 1, day),
  );
}

function displayCode(item: MaintenanceListItem) {
  return item.code || 'Codice da assegnare';
}

function windowLabel(item: MaintenanceListItem) {
  if (!item.current_window) return 'Da pianificare';
  const formatted = formatDateTime(item.current_window.scheduled_start_at);
  return formatted === '-' ? 'Da pianificare' : formatted;
}

function downtimeLabel(item: MaintenanceListItem) {
  if (!item.current_window) return 'Da pianificare';
  const label = minutesLabel(item.current_window.expected_downtime_minutes);
  return label === '-' ? 'Da definire' : label;
}

function noticeLabel(counts: StatusCount[]) {
  return counts.length > 0 ? noticesSummary(counts) : 'Nessuna comunicazione';
}

function selectOptions(items: ReferenceItem[] | undefined) {
  return items?.map((item) => (
    <option key={item.id} value={item.id}>
      {item.name_it}
    </option>
  ));
}

export function MaintenanceListPage() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const reference = useReferenceData();
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [activeMaintenanceId, setActiveMaintenanceId] = useState<number | null>(null);

  const pageValue = Number(params.get('page') ?? '1');
  const page = Number.isFinite(pageValue) && pageValue > 0 ? pageValue : 1;
  const q = params.get('q') ?? '';
  const deferredQ = useDeferredValue(q);
  const selectedStatuses = useMemo(() => {
    const raw = params.get('status');
    return raw ? raw.split(',').filter(Boolean) : [];
  }, [params]);

  const query = useMaintenances({
    q: deferredQ || undefined,
    status: selectedStatuses,
    scheduled_from: params.get('scheduled_from') ?? undefined,
    scheduled_to: params.get('scheduled_to') ?? undefined,
    technical_domain_id: params.get('technical_domain_id') ?? undefined,
    maintenance_kind_id: params.get('maintenance_kind_id') ?? undefined,
    customer_scope_id: params.get('customer_scope_id') ?? undefined,
    site_id: params.get('site_id') ?? undefined,
    page,
    page_size: 20,
  });
  const radarQuery = useMaintenanceRadar({
    q: deferredQ || undefined,
    status: selectedStatuses,
    technical_domain_id: params.get('technical_domain_id') ?? undefined,
    maintenance_kind_id: params.get('maintenance_kind_id') ?? undefined,
    customer_scope_id: params.get('customer_scope_id') ?? undefined,
    site_id: params.get('site_id') ?? undefined,
  });

  const hasFilters =
    q !== '' ||
    selectedStatuses.length > 0 ||
    Boolean(params.get('scheduled_from')) ||
    Boolean(params.get('scheduled_to')) ||
    Boolean(params.get('technical_domain_id')) ||
    Boolean(params.get('maintenance_kind_id')) ||
    Boolean(params.get('customer_scope_id')) ||
    Boolean(params.get('site_id'));

  const secondaryFilterCount = [
    params.get('scheduled_from'),
    params.get('scheduled_to'),
    params.get('technical_domain_id'),
    params.get('maintenance_kind_id'),
    params.get('customer_scope_id'),
    params.get('site_id'),
  ].filter(Boolean).length;

  function updateParam(key: string, value: string) {
    setParams((current) => {
      const next = new URLSearchParams(current);
      if (value) next.set(key, value);
      else next.delete(key);
      next.set('page', '1');
      return next;
    });
  }

  function updateStatuses(nextStatuses: string[]) {
    setParams((current) => {
      const next = new URLSearchParams(current);
      if (nextStatuses.length > 0) next.set('status', nextStatuses.join(','));
      else next.delete('status');
      next.set('page', '1');
      return next;
    });
  }

  function updatePage(nextPage: number) {
    setParams((current) => {
      const next = new URLSearchParams(current);
      next.set('page', String(nextPage));
      return next;
    });
  }

  function clearFilters() {
    setParams({ page: '1' });
  }

  const activeFilterChips = [
    ...(q ? [{ key: 'q', label: `Ricerca: ${q}`, onRemove: () => updateParam('q', '') }] : []),
    ...selectedStatuses.map((status) => ({
      key: `status-${status}`,
      label: `Stato: ${statusLabel(status)}`,
      onRemove: () => updateStatuses(selectedStatuses.filter((item) => item !== status)),
    })),
    ...(params.get('scheduled_from')
      ? [
          {
            key: 'scheduled_from',
            label: `Dal: ${params.get('scheduled_from')}`,
            onRemove: () => updateParam('scheduled_from', ''),
          },
        ]
      : []),
    ...(params.get('scheduled_to')
      ? [
          {
            key: 'scheduled_to',
            label: `Al: ${params.get('scheduled_to')}`,
            onRemove: () => updateParam('scheduled_to', ''),
          },
        ]
      : []),
    ...(params.get('technical_domain_id')
      ? [
          {
            key: 'technical_domain_id',
            label: `Dominio: ${referenceLabel(
              reference.data?.technical_domains,
              params.get('technical_domain_id'),
              'Dominio selezionato',
            )}`,
            onRemove: () => updateParam('technical_domain_id', ''),
          },
        ]
      : []),
    ...(params.get('maintenance_kind_id')
      ? [
          {
            key: 'maintenance_kind_id',
            label: `Tipo: ${referenceLabel(
              reference.data?.maintenance_kinds,
              params.get('maintenance_kind_id'),
              'Tipo selezionato',
            )}`,
            onRemove: () => updateParam('maintenance_kind_id', ''),
          },
        ]
      : []),
    ...(params.get('customer_scope_id')
      ? [
          {
            key: 'customer_scope_id',
            label: `Ambito: ${referenceLabel(
              reference.data?.customer_scopes,
              params.get('customer_scope_id'),
              'Ambito selezionato',
            )}`,
            onRemove: () => updateParam('customer_scope_id', ''),
          },
        ]
      : []),
    ...(params.get('site_id')
      ? [
          {
            key: 'site_id',
            label: `Sito: ${referenceLabel(reference.data?.sites, params.get('site_id'), 'Sito selezionato')}`,
            onRemove: () => updateParam('site_id', ''),
          },
        ]
      : []),
  ];

  return (
    <section className={shared.page}>
      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>Registro Manutenzioni</h1>
        </div>
        <div className={shared.headerActions}>
          <Button
            variant="secondary"
            onClick={() => {
              void query.refetch();
              void radarQuery.refetch();
            }}
            loading={
              (query.isFetching && !query.isLoading) ||
              (radarQuery.isFetching && !radarQuery.isLoading)
            }
            leftIcon={<Icon name="loader" size={16} />}
          >
            Aggiorna
          </Button>
          <Button onClick={() => navigate('/manutenzioni/new')} leftIcon={<Icon name="plus" size={16} />}>
            Nuova manutenzione
          </Button>
        </div>
      </div>

      <div className={styles.filterShell}>
        <div className={styles.filterBar}>
          <div className={styles.searchControl}>
            <SearchInput
              value={q}
              onChange={(value) => updateParam('q', value)}
              placeholder="Cerca per codice, titolo, motivo, servizio o target..."
            />
          </div>
          <div className={styles.statusControl}>
            <MultiSelect<string>
              options={STATUS_FILTER_OPTIONS}
              selected={selectedStatuses}
              onChange={updateStatuses}
              placeholder="Tutti gli stati"
            />
          </div>
          <Button
            variant="secondary"
            onClick={() => setFiltersOpen((open) => !open)}
            leftIcon={<Icon name="filter" size={16} />}
            rightIcon={<Icon name={filtersOpen ? 'chevron-up' : 'chevron-down'} size={16} />}
            aria-expanded={filtersOpen}
            aria-controls="maintenance-secondary-filters"
          >
            {secondaryFilterCount > 0 ? `Altri filtri (${secondaryFilterCount})` : 'Altri filtri'}
          </Button>
          {hasFilters && (
            <Button variant="ghost" onClick={clearFilters}>
              Cancella filtri
            </Button>
          )}
        </div>

        <div id="maintenance-secondary-filters" className={styles.secondaryFilters} hidden={!filtersOpen}>
          <input
            className={shared.field}
            type="date"
            value={params.get('scheduled_from') ?? ''}
            onChange={(event) => updateParam('scheduled_from', event.target.value)}
            aria-label="Finestra dal"
          />
          <input
            className={shared.field}
            type="date"
            value={params.get('scheduled_to') ?? ''}
            onChange={(event) => updateParam('scheduled_to', event.target.value)}
            aria-label="Finestra al"
          />
          <select
            className={shared.select}
            value={params.get('technical_domain_id') ?? ''}
            onChange={(event) => updateParam('technical_domain_id', event.target.value)}
            aria-label="Dominio"
          >
            <option value="">Tutti i domini</option>
            {selectOptions(reference.data?.technical_domains)}
          </select>
          <select
            className={shared.select}
            value={params.get('maintenance_kind_id') ?? ''}
            onChange={(event) => updateParam('maintenance_kind_id', event.target.value)}
            aria-label="Tipo"
          >
            <option value="">Tutti i tipi</option>
            {selectOptions(reference.data?.maintenance_kinds)}
          </select>
          <select
            className={shared.select}
            value={params.get('customer_scope_id') ?? ''}
            onChange={(event) => updateParam('customer_scope_id', event.target.value)}
            aria-label="Ambito clienti"
          >
            <option value="">Tutti gli ambiti</option>
            {selectOptions(reference.data?.customer_scopes)}
          </select>
          <select
            className={shared.select}
            value={params.get('site_id') ?? ''}
            onChange={(event) => updateParam('site_id', event.target.value)}
            aria-label="Sito"
          >
            <option value="">Tutti i siti</option>
            {selectOptions(reference.data?.sites)}
          </select>
        </div>

        {activeFilterChips.length > 0 && (
          <div className={styles.filterChips} aria-label="Filtri attivi">
            {activeFilterChips.map((chip) => (
              <button key={chip.key} type="button" className={styles.filterChip} onClick={chip.onRemove}>
                <span>{chip.label}</span>
                <Icon name="x" size={13} />
              </button>
            ))}
          </div>
        )}
      </div>

      {radarQuery.isLoading ? (
        <div className={styles.radar}>
          <Skeleton rows={4} />
        </div>
      ) : radarQuery.error ? (
        <div className={styles.radar} role="status">
          <div className={styles.radarHeader}>
            <div>
              <h2>Finestre di manutenzione nei prossimi 6 mesi</h2>
              <p>{errorMessage(radarQuery.error, 'Radar non disponibile.')}</p>
            </div>
          </div>
        </div>
      ) : radarQuery.data ? (
        <MaintenanceWindowRadar
          radar={radarQuery.data}
          activeMaintenanceId={activeMaintenanceId}
          onActiveChange={setActiveMaintenanceId}
        />
      ) : null}

      {query.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={7} />
        </div>
      ) : query.error ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Registro non disponibile</h3>
          <p>{errorMessage(query.error, 'Impossibile caricare le manutenzioni.')}</p>
        </div>
      ) : !query.data || query.data.items.length === 0 ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIcon}>
            <Icon name="search" />
          </div>
          <h3>{hasFilters ? 'Nessuna manutenzione trovata' : 'Nessuna manutenzione'}</h3>
          <p>
            {hasFilters
              ? 'Non ci sono manutenzioni che corrispondono ai filtri selezionati.'
              : 'Crea una nuova manutenzione per iniziare il registro.'}
          </p>
        </div>
      ) : (
        <>
          <div className={styles.tableCard}>
            <div className={styles.tableScroll}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th className={styles.accentHeader} aria-label="Evidenza" />
                    <th>Manutenzione</th>
                    <th>Stato</th>
                    <th>Classificazione</th>
                    <th>Finestra</th>
                    <th>Impatto</th>
                    <th>Comunicazioni</th>
                    <th className={styles.actionsCell} aria-label="Azioni" />
                  </tr>
                </thead>
                <tbody>
                  {query.data.items.map((item) => (
                    <tr
                      key={item.maintenance_id}
                      className={styles.row}
                      data-active={activeMaintenanceId === item.maintenance_id}
                      tabIndex={0}
                      role="link"
                      aria-label={`Apri ${displayCode(item)} ${item.title_it}`}
                      onClick={() => navigate(`/manutenzioni/${item.maintenance_id}`)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          navigate(`/manutenzioni/${item.maintenance_id}`);
                        }
                      }}
                      onMouseEnter={() => setActiveMaintenanceId(item.maintenance_id)}
                      onMouseLeave={() => setActiveMaintenanceId(null)}
                      onFocus={() => setActiveMaintenanceId(item.maintenance_id)}
                      onBlur={() => setActiveMaintenanceId(null)}
                    >
                      <td className={styles.accentCell}>
                        <span className={styles.accentBar} data-tone={statusTone(item.status)} />
                      </td>
                      <td>
                        <div className={styles.maintenanceCell}>
                          <span className={styles.code}>{displayCode(item)}</span>
                          <strong>{item.title_it}</strong>
                          {item.title_en && <span className={styles.subtleLine}>{item.title_en}</span>}
                        </div>
                      </td>
                      <td>
                        <StatusPill tone={statusTone(item.status)}>{statusLabel(item.status)}</StatusPill>
                      </td>
                      <td>
                        <div className={styles.stackCell}>
                          <strong>{item.technical_domain.name_it}</strong>
                          <span>{item.maintenance_kind.name_it}</span>
                          <span className={!item.customer_scope ? styles.inlineHint : undefined}>
                            {item.customer_scope?.name_it ?? 'Ambito da definire'}
                          </span>
                        </div>
                      </td>
                      <td>
                        <div className={`${styles.stackCell} ${styles.numericCell}`}>
                          <strong className={!item.current_window ? styles.inlineHint : undefined}>
                            {windowLabel(item)}
                          </strong>
                          <span>{downtimeLabel(item)}</span>
                        </div>
                      </td>
                      <td>
                        <div className={styles.stackCell}>
                          <strong className={!item.primary_impact_label ? styles.inlineHint : undefined}>
                            {item.primary_impact_label ?? 'Impatto da definire'}
                          </strong>
                          <span className={!item.site ? styles.inlineHint : undefined}>
                            {item.site?.name_it ?? 'Sito non definito'}
                          </span>
                        </div>
                      </td>
                      <td>
                        <span className={!item.notice_statuses.length ? styles.inlineHint : undefined}>
                          {noticeLabel(item.notice_statuses)}
                        </span>
                      </td>
                      <td className={styles.actionsCell}>
                        <Button
                          size="sm"
                          variant="secondary"
                          rightIcon={<Icon name="chevron-right" size={14} />}
                          onClick={(event) => {
                            event.stopPropagation();
                            navigate(`/manutenzioni/${item.maintenance_id}`);
                          }}
                        >
                          Apri
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
          <Pagination
            page={query.data.page}
            pageSize={query.data.page_size}
            total={query.data.total}
            onPageChange={updatePage}
          />
        </>
      )}
    </section>
  );
}

interface MaintenanceWindowRadarProps {
  radar: MaintenanceRadarResponse;
  activeMaintenanceId: number | null;
  onActiveChange: (id: number | null) => void;
}

function MaintenanceWindowRadar({
  radar,
  activeMaintenanceId,
  onActiveChange,
}: MaintenanceWindowRadarProps) {
  const navigate = useNavigate();
  const buckets = useMemo(
    () => [
      { key: 'next7' as const, label: 'Prossimi 7 giorni' },
      {
        key: 'next45' as const,
        label: `Pianificate dal ${formatRadarDate(radar.next_45_days_from)} al ${formatRadarDate(
          radar.next_45_days_to,
        )}`,
      },
      {
        key: 'sixMonths' as const,
        label: `Pianificate fino al ${formatRadarDate(radar.six_months_to)}`,
      },
      { key: 'unscheduled' as const, label: 'Da pianificare' },
    ],
    [radar.next_45_days_from, radar.next_45_days_to, radar.six_months_to],
  );
  const grouped = useMemo(() => {
    const groupedBuckets: Record<WindowBucket, MaintenanceListItem[]> = {
      next7: [],
      next45: [],
      sixMonths: [],
      unscheduled: [],
    };
    radar.items.forEach((item) => groupedBuckets[windowBucket(item, radar)].push(item));
    return groupedBuckets;
  }, [radar]);

  return (
    <section className={styles.radar} aria-label="Finestre di manutenzione">
      <div className={styles.radarHeader}>
        <div>
          <h2>Finestre di manutenzione nei prossimi 6 mesi</h2>
        </div>
      </div>

      <div className={styles.radarGrid}>
        {buckets.map((bucket) => {
          const bucketItems = grouped[bucket.key];
          return (
            <div key={bucket.key} className={styles.radarLane} data-bucket={bucket.key}>
              <div className={styles.radarLaneLabel}>
                <span>{bucket.label}</span>
                <strong>{bucketItems.length}</strong>
              </div>
              <div className={styles.radarTrack}>
                {bucketItems.length === 0 ? (
                  <span className={styles.radarEmpty}>Nessuna</span>
                ) : (
                  <div className={styles.radarRail}>
                    {bucketItems.map((item) => {
                      const active = activeMaintenanceId === item.maintenance_id;
                      const tooltip = `${displayCode(item)} - ${item.title_it} - ${windowLabel(item)}`;
                      return (
                        <Tooltip key={item.maintenance_id} content={tooltip} showDelay={180}>
                          <button
                            type="button"
                            className={styles.radarMarker}
                            data-tone={statusTone(item.status)}
                            data-active={active}
                            aria-label={tooltip}
                            onClick={() => navigate(`/manutenzioni/${item.maintenance_id}`)}
                            onMouseEnter={() => onActiveChange(item.maintenance_id)}
                            onMouseLeave={() => onActiveChange(null)}
                            onFocus={() => onActiveChange(item.maintenance_id)}
                            onBlur={() => onActiveChange(null)}
                          />
                        </Tooltip>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
