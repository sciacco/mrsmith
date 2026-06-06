import { Button, Icon, SearchInput, SingleSelect, Skeleton } from '@mrsmith/ui';
import type { ReactNode } from 'react';
import { useMemo, useState } from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import { useRackFilterOptions, useRacks } from '../../api/queries';
import type { RackListItem } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import styles from './rackSearch.module.css';

const defaultStatusOptions = [
  { value: 'active', label: 'Solo attivi' },
  { value: 'all', label: 'Tutti' },
];

const powerFormatter = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 2 });

export function RackSearchPage() {
  const { rackId } = useParams();
  const navigate = useNavigate();
  const [q, setQ] = useState('');
  const [customerId, setCustomerId] = useState<number | null>(null);
  const [datacenterId, setDatacenterId] = useState<number | null>(null);
  const [status, setStatus] = useState('active');
  const filterOptions = useRackFilterOptions();
  const racks = useRacks({ q, status, customerId, datacenterId });

  const customerOptions = useMemo(
    () => (filterOptions.data?.customers ?? []).map((item) => ({ value: Number(item.id), label: item.label })),
    [filterOptions.data?.customers],
  );
  const datacenterOptions = useMemo(
    () => (filterOptions.data?.datacenters ?? []).map((item) => ({ value: item.id, label: item.label })),
    [filterOptions.data?.datacenters],
  );
  const statusOptions = useMemo(() => {
    const items = filterOptions.data?.statuses ?? [];
    if (items.length === 0) return defaultStatusOptions;
    return items.map((item) => ({ value: String(item.id), label: item.label }));
  }, [filterOptions.data?.statuses]);

  const rows = racks.data ?? [];
  const activeCount = rows.filter((item) => isActiveRack(item.status)).length;
  const customerCount = new Set(rows.map((item) => item.customerId ?? item.customerName).filter(Boolean)).size;
  const customerLabel = labelFor(customerOptions, customerId);
  const datacenterLabel = labelFor(datacenterOptions, datacenterId);
  const statusLabel = labelFor(statusOptions, status) ?? 'Solo attivi';
  const hasFilters = q.trim() !== '' || customerId !== null || datacenterId !== null || status !== 'active';

  if (rackId) {
    return <Navigate to={`/rack/${rackId}`} replace />;
  }

  function resetFilters() {
    setQ('');
    setCustomerId(null);
    setDatacenterId(null);
    setStatus('active');
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div className={styles.titleBlock}>
          <span className={styles.eyebrow}>Infrastruttura</span>
          <h1 className={styles.title}>Rack</h1>
        </div>
        <Button variant="secondary" onClick={() => navigate('/old-rack')} leftIcon={<Icon name="archive" size={16} />}>
          OLD RACK
        </Button>
      </header>

      <section className={styles.searchPanel}>
        <div className={styles.searchTop}>
          <div className={styles.searchWrap}>
            <SearchInput value={q} onChange={setQ} placeholder="Cerca rack, cliente, seriale o ordine" autoFocus />
          </div>
          <div className={styles.counterGrid} aria-live="polite">
            <Counter label="Risultati" value={rows.length} />
            <Counter label="Attivi" value={activeCount} />
            <Counter label="Clienti" value={customerCount} />
          </div>
        </div>

        <div className={styles.filterGrid}>
          <FilterField label="Cliente">
            <SingleSelect
              options={customerOptions}
              selected={customerId}
              onChange={setCustomerId}
              placeholder="Tutti i clienti"
              allowClear
              disabled={filterOptions.isLoading}
            />
          </FilterField>
          <FilterField label="Sala/MMR">
            <SingleSelect
              options={datacenterOptions}
              selected={datacenterId}
              onChange={setDatacenterId}
              placeholder="Tutte le sale"
              allowClear
              disabled={filterOptions.isLoading}
            />
          </FilterField>
          <FilterField label="Stato">
            <SingleSelect
              options={statusOptions}
              selected={status}
              onChange={(value) => setStatus(value ?? 'active')}
              searchable={false}
            />
          </FilterField>
        </div>

        <div className={styles.filterFooter}>
          <div className={styles.chipRow}>
            {q.trim() ? <FilterChip label={`Testo: ${q.trim()}`} onRemove={() => setQ('')} /> : null}
            {customerLabel ? <FilterChip label={`Cliente: ${customerLabel}`} onRemove={() => setCustomerId(null)} /> : null}
            {datacenterLabel ? <FilterChip label={`Sala/MMR: ${datacenterLabel}`} onRemove={() => setDatacenterId(null)} /> : null}
            {status !== 'active' ? <FilterChip label={`Stato: ${statusLabel}`} onRemove={() => setStatus('active')} /> : null}
            {!hasFilters ? <span className={styles.defaultChip}>Solo attivi</span> : null}
          </div>
          {hasFilters ? (
            <button type="button" className={styles.resetButton} onClick={resetFilters}>
              Reimposta
            </button>
          ) : null}
        </div>
      </section>

      <section className={styles.resultsShell}>
        {racks.isLoading ? (
          <div className={styles.loadingPanel}>
            <Skeleton rows={8} />
          </div>
        ) : racks.error ? (
          <ViewState title="Rack non disponibili" message="Non e stato possibile caricare la ricerca rack." tone="error" />
        ) : rows.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <Icon name="search" size={30} />
            </div>
            <h2>Nessun rack trovato</h2>
            <p>Modifica i filtri applicati.</p>
          </div>
        ) : (
          <RackResults rows={rows} onOpen={(id) => navigate(`/rack/${id}`)} />
        )}
      </section>
    </main>
  );
}

function RackResults({ rows, onOpen }: { rows: RackListItem[]; onOpen: (id: number) => void }) {
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Rack</th>
            <th>Cliente</th>
            <th>Sala/MMR</th>
            <th>Formato</th>
            <th>Committed</th>
            <th>Socket</th>
            <th>Azioni</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((item, index) => (
            <tr key={item.id} style={{ animationDelay: `${Math.min(index, 10) * 35}ms` }}>
              <td>
                <button type="button" className={styles.rackButton} onClick={() => onOpen(item.id)}>
                  <span>{item.name}</span>
                  <Icon name="chevron-right" size={15} />
                </button>
                <div className={styles.rackPillRow}>
                  <StatusBadge status={item.status} />
                </div>
              </td>
              <td>
                <strong>{customerLabelForRow(item)}</strong>
                <div className={styles.rowMeta}>{serialOrderLabel(item)}</div>
              </td>
              <td>
                <strong>{item.datacenterName ?? '-'}</strong>
                <div className={styles.rowMeta}>{item.buildingName ?? 'Edificio non indicato'}</div>
              </td>
              <td>
                <span className={`${styles.formatBadge} ${formatBadgeTone(item.type, item.position)}`}>
                  {rackPositionLabel(item.type, item.position)}
                </span>
              </td>
              <td>
                <span className={styles.powerValue}>{powerLabel(item.committedPower)}</span>
              </td>
              <td>
                <span className={styles.socketBadge}>{item.socketCount}</span>
              </td>
              <td>
                <Button size="sm" variant="secondary" onClick={() => onOpen(item.id)} rightIcon={<Icon name="external-link" size={14} />}>
                  Apri
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function FilterField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className={styles.field}>
      <label>{label}</label>
      {children}
    </div>
  );
}

function FilterChip({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <button type="button" className={styles.filterChip} onClick={onRemove}>
      <span>{label}</span>
      <Icon name="x" size={13} />
    </button>
  );
}

function Counter({ label, value }: { label: string; value: number }) {
  return (
    <div className={styles.counter}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function StatusBadge({ status }: { status?: string }) {
  const text = status?.trim() || 'Non indicato';
  const blocked = ['cessato', 'cessata', 'spento', 'chiuso'].includes(text.toLowerCase());
  return <span className={blocked ? styles.statusDanger : styles.statusActive}>{text}</span>;
}

function labelFor<T extends string | number>(options: Array<{ value: T; label: string }>, value: T | null) {
  if (value === null) return null;
  return options.find((item) => item.value === value)?.label ?? null;
}

function rackPositionLabel(type?: string, position?: string) {
  if (type === 'Half' && position === 'A') return '1/2 alto';
  if (type === 'Half' && position === 'B') return '1/2 basso';
  if (type === 'Full') return 'Full';
  return [type, position].filter(Boolean).join(' ') || '-';
}

function formatBadgeTone(type?: string, position?: string) {
  const rackType = type?.trim().toLowerCase();
  const rackPosition = position?.trim().toUpperCase();
  if (rackType === 'full') return styles.formatFull;
  if (rackType === 'half' && rackPosition === 'A') return styles.formatHalfHigh;
  if (rackType === 'half' && rackPosition === 'B') return styles.formatHalfLow;
  return styles.formatUnknown;
}

function serialOrderLabel(item: RackListItem) {
  return [item.serialNumber, item.orderCode].filter(Boolean).join(' · ') || '-';
}

function powerLabel(value?: number) {
  return value === undefined || value === 0 ? '-' : `${powerFormatter.format(value)} kW`;
}

function isActiveRack(status?: string) {
  const value = status?.trim().toLowerCase();
  return !value || !['cessato', 'cessata', 'spento', 'chiuso'].includes(value);
}

function customerLabelForRow(item: RackListItem) {
  if (item.customerName && item.customerId) return `${item.customerName} (${item.customerId})`;
  if (item.customerName) return item.customerName;
  if (item.customerId) return `Cliente (${item.customerId})`;
  return '-';
}
