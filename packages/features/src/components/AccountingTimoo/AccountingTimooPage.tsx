import { useDeferredValue, useMemo, useState } from 'react';
import { SearchInput, Skeleton } from '@mrsmith/ui';
import { useTimooDailyStats } from '../../hooks/useTimooDailyStats';
import type { TimooDailyStat } from '../../api/timooDailyStats';
import { downloadCsv } from '../../utils/csv';
import styles from './AccountingTimooPage.module.css';

const dayFormatter = new Intl.DateTimeFormat('it-IT', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
});

function formatDay(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return dayFormatter.format(date);
}

function dayOnly(iso: string): string {
  return iso.slice(0, 10);
}

type SortKey = 'tenant' | 'day';
type SortDirection = 'asc' | 'desc';

function sortRows(rows: TimooDailyStat[], key: SortKey, dir: SortDirection): TimooDailyStat[] {
  const m = dir === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => {
    if (key === 'tenant') return a.tenant_name.localeCompare(b.tenant_name) * m;
    return dayOnly(a.day).localeCompare(dayOnly(b.day)) * m;
  });
}

export function AccountingTimooPage() {
  const { data, isLoading, error } = useTimooDailyStats();

  const [search, setSearch] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [sortKey, setSortKey] = useState<SortKey>('day');
  const [sortDir, setSortDir] = useState<SortDirection>('desc');
  const deferredSearch = useDeferredValue(search);

  const [minDate, maxDate] = useMemo(() => {
    if (!data?.length) return ['', ''] as const;
    const first = data[0]!;
    let min = dayOnly(first.day);
    let max = min;
    for (const row of data) {
      const d = dayOnly(row.day);
      if (d < min) min = d;
      if (d > max) max = d;
    }
    return [min, max];
  }, [data]);

  const filtered = useMemo(() => {
    if (!data) return [];
    const needle = deferredSearch.trim().toLowerCase();
    return data.filter((row) => {
      if (needle && !row.tenant_name.toLowerCase().includes(needle)
          && !String(row.tenant_id).includes(needle)) return false;
      const d = dayOnly(row.day);
      if (dateFrom && d < dateFrom) return false;
      if (dateTo && d > dateTo) return false;
      return true;
    });
  }, [data, deferredSearch, dateFrom, dateTo]);

  const sorted = useMemo(
    () => sortRows(filtered, sortKey, sortDir),
    [filtered, sortKey, sortDir],
  );

  function updateSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir(key === 'day' ? 'desc' : 'asc');
    }
  }

  function sortHeader(key: SortKey, label: string) {
    const active = sortKey === key;
    return (
      <button
        type="button"
        className={`${styles.sortButton} ${active ? styles.sortButtonActive : ''}`}
        onClick={() => updateSort(key)}
      >
        {label}
        <span className={styles.sortGlyph} aria-hidden="true">
          {active ? (sortDir === 'asc' ? '▲' : '▼') : '↕'}
        </span>
      </button>
    );
  }

  function handleExport() {
    if (!data) return;
    downloadCsv(
      'accounting-timoo.csv',
      ['Tenant ID', 'Tenant', 'Giorno', 'Utenti', 'Service Extensions'],
      data.map((row) => [
        row.tenant_id,
        row.tenant_name,
        formatDay(row.day),
        row.users,
        row.service_extensions,
      ]),
    );
  }

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>Accounting TIMOO</h1>

      <div className={styles.toolbar}>
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder="Cerca tenant..."
        />
        <div className={styles.field}>
          <label>Data da</label>
          <input
            type="date"
            className={styles.dateInput}
            value={dateFrom}
            min={minDate}
            max={dateTo || maxDate}
            onChange={(e) => setDateFrom(e.target.value)}
          />
        </div>
        <div className={styles.field}>
          <label>Data a</label>
          <input
            type="date"
            className={styles.dateInput}
            value={dateTo}
            min={dateFrom || minDate}
            max={maxDate}
            onChange={(e) => setDateTo(e.target.value)}
          />
        </div>
        <button type="button" className={styles.exportBtn} onClick={handleExport} disabled={!data}>
          Esporta CSV
        </button>
      </div>

      {isLoading && <Skeleton rows={8} />}

      {error && (
        <p>Errore nel caricamento dei dati.</p>
      )}

      {data && (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>{sortHeader('tenant', 'Tenant')}</th>
                <th>{sortHeader('day', 'Giorno')}</th>
                <th className={styles.numCol}>Utenti</th>
                <th className={styles.numCol}>Service Extensions</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((row, i) => (
                <tr key={`${row.tenant_id}-${row.day}`} style={{ animationDelay: `${Math.min(i * 0.02, 0.6)}s` }}>
                  <td>{row.tenant_name} ({row.tenant_id})</td>
                  <td>{formatDay(row.day)}</td>
                  <td className={styles.numCol}>{row.users}</td>
                  <td className={styles.numCol}>{row.service_extensions}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
