import { useDeferredValue, useMemo, useState } from 'react';
import { SearchInput, Skeleton, Drawer } from '@mrsmith/ui';
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

type SortKey = 'tenant' | 'day' | 'users' | 'service_extensions';
type SortDirection = 'asc' | 'desc';

function sortRows(rows: TimooDailyStat[], key: SortKey, dir: SortDirection): TimooDailyStat[] {
  const m = dir === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => {
    switch (key) {
      case 'tenant':
        return a.tenant_name.localeCompare(b.tenant_name) * m;
      case 'day':
        return dayOnly(a.day).localeCompare(dayOnly(b.day)) * m;
      case 'users':
        return (a.users - b.users) * m;
      case 'service_extensions':
        return (a.service_extensions - b.service_extensions) * m;
      default:
        return 0;
    }
  });
}

export function AccountingTimooPage() {
  const { data, isLoading, error } = useTimooDailyStats();

  const [search, setSearch] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [sortKey, setSortKey] = useState<SortKey>('day');
  const [sortDir, setSortDir] = useState<SortDirection>('desc');
  const [selectedTenant, setSelectedTenant] = useState<{ id: number; name: string } | null>(null);

  const deferredSearch = useDeferredValue(search);

  // Set min and max date ranges from data
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

  // Filter rows based on search and date range
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

  // Sort rows
  const sorted = useMemo(
    () => sortRows(filtered, sortKey, sortDir),
    [filtered, sortKey, sortDir],
  );

  // Calculate high-level KPIs based on the currently filtered data
  const { activeTenantsCount, peakUsers, peakExtensions, averageUsers } = useMemo(() => {
    if (!filtered.length) {
      return { activeTenantsCount: 0, peakUsers: 0, peakExtensions: 0, averageUsers: 0 };
    }

    const uniqueTenants = new Set<number>();
    const dailyUserSums: Record<string, number> = {};
    const dailySeSums: Record<string, number> = {};

    for (const row of filtered) {
      uniqueTenants.add(row.tenant_id);
      
      const dayKey = dayOnly(row.day);
      dailyUserSums[dayKey] = (dailyUserSums[dayKey] || 0) + row.users;
      dailySeSums[dayKey] = (dailySeSums[dayKey] || 0) + row.service_extensions;
    }

    const userValues = Object.values(dailyUserSums);
    const peakU = userValues.length > 0 ? Math.max(...userValues) : 0;
    const peakSe = Object.keys(dailySeSums).length > 0 ? Math.max(...Object.values(dailySeSums)) : 0;
    
    const avgU = userValues.length > 0 
      ? Math.round(userValues.reduce((a, b) => a + b, 0) / userValues.length)
      : 0;

    return {
      activeTenantsCount: uniqueTenants.size,
      peakUsers: peakU,
      peakExtensions: peakSe,
      averageUsers: avgU,
    };
  }, [filtered]);

  // Selected tenant historical log history for Drawer
  const tenantHistory = useMemo(() => {
    if (!data || !selectedTenant) return [];
    return data
      .filter((row) => row.tenant_id === selectedTenant.id)
      .sort((a, b) => b.day.localeCompare(a.day)); // newest first
  }, [data, selectedTenant]);

  // Selected tenant peaks for Drawer
  const tenantPeaks = useMemo(() => {
    if (!tenantHistory.length) return { peakUsers: 0, peakSe: 0 };
    let maxU = 0;
    let maxSe = 0;
    for (const row of tenantHistory) {
      if (row.users > maxU) maxU = row.users;
      if (row.service_extensions > maxSe) maxSe = row.service_extensions;
    }
    return { peakUsers: maxU, peakSe: maxSe };
  }, [tenantHistory]);

  const updateSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir(key === 'tenant' || key === 'day' ? 'desc' : 'desc');
    }
  };

  const sortHeader = (key: SortKey, label: string) => {
    const active = sortKey === key;
    return (
      <button
        type="button"
        className={`${styles.sortButton} ${active ? styles.sortButtonActive : ''}`}
        onClick={(e) => {
          e.stopPropagation();
          updateSort(key);
        }}
      >
        {label}
        <span className={styles.sortGlyph} aria-hidden="true">
          {active ? (sortDir === 'asc' ? ' ▲' : ' ▼') : ' ↕'}
        </span>
      </button>
    );
  };

  const handleExport = () => {
    if (!data) return;
    downloadCsv(
      'accounting-timoo.csv',
      ['Tenant ID', 'Tenant', 'Giorno', 'Utenti', 'Service Extensions'],
      filtered.map((row) => [
        row.tenant_id,
        row.tenant_name,
        formatDay(row.day),
        row.users,
        row.service_extensions,
      ]),
    );
  };

  const isFilterActive = search.trim() !== '' || dateFrom !== '' || dateTo !== '';

  const handleResetFilters = () => {
    setSearch('');
    setDateFrom('');
    setDateTo('');
  };

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.pageTitle}>Accounting TIMOO</h1>
          <p className={styles.pageSubtitle}>
            Consumi giornalieri di interni e service extension suddivisi per singolo tenant.
          </p>
        </div>
      </div>

      {isLoading && <Skeleton rows={8} />}

      {error && (
        <div className={styles.stateBox}>
          <div className={styles.stateIcon}>
            <svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true" stroke="currentColor" strokeWidth="1.5">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v4m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <p className={styles.stateTitle}>Errore nel caricamento</p>
          <p className={styles.stateText}>Non è stato possibile caricare i dati di contabilità TIMOO. Per favore riprova.</p>
        </div>
      )}

      {data && (
        <>
          {/* KPI Dashboard Cards */}
          <div className={styles.kpiContainer}>
            <div className={styles.kpiCard}>
              <span className={styles.kpiLabel}>Tenant Attivi</span>
              <span className={styles.kpiValue}>{activeTenantsCount}</span>
            </div>
            
            <div className={styles.kpiCard}>
              <span className={styles.kpiLabel}>Picco Utenti</span>
              <span className={styles.kpiValue}>{peakUsers}</span>
            </div>

            <div className={styles.kpiCard}>
              <span className={styles.kpiLabel}>Picco Estensioni</span>
              <span className={styles.kpiValue}>{peakExtensions}</span>
            </div>

            <div className={styles.kpiCard}>
              <span className={styles.kpiLabel}>Media Utenti Giornalieri</span>
              <span className={styles.kpiValue}>{averageUsers}</span>
            </div>
          </div>

          {/* Table & Filtering tools */}
          <div className={styles.tableCard}>
            <div className={styles.tableTools}>
              <SearchInput
                value={search}
                onChange={setSearch}
                placeholder="Cerca per tenant..."
                className={styles.search}
              />
              
              <div className={styles.field}>
                <label htmlFor="dateFromInput">Data da</label>
                <input
                  id="dateFromInput"
                  type="date"
                  className={styles.dateInput}
                  value={dateFrom}
                  min={minDate}
                  max={dateTo || maxDate}
                  onChange={(e) => setDateFrom(e.target.value)}
                />
              </div>

              <div className={styles.field}>
                <label htmlFor="dateToInput">Data a</label>
                <input
                  id="dateToInput"
                  type="date"
                  className={styles.dateInput}
                  value={dateTo}
                  min={dateFrom || minDate}
                  max={maxDate}
                  onChange={(e) => setDateTo(e.target.value)}
                />
              </div>

              {isFilterActive && (
                <button type="button" className={styles.resetBtn} onClick={handleResetFilters}>
                  Ripristina filtri
                </button>
              )}

              <button
                type="button"
                className={styles.exportBtn}
                onClick={handleExport}
                disabled={filtered.length === 0}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                  <polyline points="7 10 12 15 17 10" />
                  <line x1="12" y1="15" x2="12" y2="3" />
                </svg>
                Esporta CSV
              </button>
            </div>

            {/* Table layout */}
            {filtered.length === 0 ? (
              <div className={styles.stateBox}>
                <div className={styles.stateIcon}>
                  <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="11" cy="11" r="8" />
                    <line x1="21" y1="21" x2="16.65" y2="16.65" />
                  </svg>
                </div>
                <p className={styles.stateTitle}>Nessun dato corrispondente</p>
                <p className={styles.stateText}>Nessun record di accounting trovato per i filtri selezionati.</p>
              </div>
            ) : (
              <div className={styles.tableScroll}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th className={styles.barCol} />
                      <th>{sortHeader('tenant', 'Tenant')}</th>
                      <th>{sortHeader('day', 'Giorno')}</th>
                      <th className={styles.numCol}>{sortHeader('users', 'Utenti')}</th>
                      <th className={styles.numCol}>{sortHeader('service_extensions', 'Service Extensions')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sorted.map((row, i) => {
                      const isSelected = selectedTenant?.id === row.tenant_id;
                      return (
                        <tr
                          key={`${row.tenant_id}-${row.day}`}
                          className={isSelected ? styles.rowSelected : undefined}
                          onClick={() => setSelectedTenant({ id: row.tenant_id, name: row.tenant_name })}
                          style={{ animationDelay: `${Math.min(i * 0.02, 0.6)}s` }}
                          aria-selected={isSelected}
                        >
                          <td className={styles.barCol}>
                            <div className={styles.accentBar} />
                          </td>
                          <td>
                            <div className={styles.primaryText}>{row.tenant_name}</div>
                            <div className={styles.secondaryText}>ID: {row.tenant_id}</div>
                          </td>
                          <td>{formatDay(row.day)}</td>
                          <td className={styles.numCol}>{row.users}</td>
                          <td className={styles.numCol}>{row.service_extensions}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      )}

      {/* Selected Tenant Timeline Side Drawer */}
      <Drawer
        open={selectedTenant !== null}
        onClose={() => setSelectedTenant(null)}
        title="Dettaglio Tenant TIMOO"
        subtitle={selectedTenant ? `${selectedTenant.name} (ID: ${selectedTenant.id})` : undefined}
        size="md"
      >
        {selectedTenant && (
          <div className={styles.drawerContent}>
            {/* Tenant overall stats in period */}
            <div className={styles.drawerSection}>
              <h3>Valori di Picco nel Periodo</h3>
              <div className={styles.drawerGrid}>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Massimo Utenti Attivi</span>
                  <span className={styles.fieldValue}>{tenantPeaks.peakUsers}</span>
                </div>
                <div className={styles.drawerField}>
                  <span className={styles.fieldLabel}>Massimo Service Extensions</span>
                  <span className={styles.fieldValue}>{tenantPeaks.peakSe}</span>
                </div>
              </div>
            </div>

            {/* Tenant timeline history */}
            <div className={styles.drawerSection}>
              <h3>Timeline Storica</h3>
              <div className={styles.drawerTableScroll}>
                <table className={styles.drawerTable}>
                  <thead>
                    <tr>
                      <th>Giorno</th>
                      <th>Utenti</th>
                      <th>Service Ext.</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tenantHistory.map((row) => (
                      <tr key={row.day}>
                        <td>{formatDay(row.day)}</td>
                        <td>{row.users}</td>
                        <td>{row.service_extensions}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}
