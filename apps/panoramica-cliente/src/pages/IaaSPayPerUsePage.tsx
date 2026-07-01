import { useState, useEffect } from 'react';
import { SearchInput, useTableFilter } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend,
} from 'recharts';
import { useIaaSAccounts, useChargesSeries, useChargesByCategory } from '../api/queries';
import { useSortedData } from '../hooks/useSort';
import { useCsvExport } from '../hooks/useCsvExport';
import {
  useChargeDrill,
  PERIOD_OPTIONS,
  GRANULARITIES,
  GROUP_LABELS,
  bucketLabel,
  formatBucketTick,
  type PeriodPreset,
  type Granularity,
} from '../hooks/useChargeDrill';
import { SortableHeader } from '../components/shared/SortableHeader';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import type { IaaSAccount, ChargeSeriesPoint } from '../types';
import s from './shared.module.css';
import is from './IaaSPayPerUse.module.css';

const CATEGORY_COLORS: Record<string, string> = {
  'VM': '#635bff',
  'Storage': '#10b981',
  'Licenze Windows': '#f59e0b',
  'Altro': '#94a3b8',
};
const DEFAULT_CATEGORY_COLOR = '#c7d2fe';

const accountCsvCols: { key: keyof IaaSAccount; label: string }[] = [
  { key: 'intestazione', label: 'Intestazione' },
  { key: 'credito', label: 'Credito' },
  { key: 'abbreviazione', label: 'Abbreviazione' },
  { key: 'serialnumber', label: 'Serialnumber' },
  { key: 'data_attivazione', label: 'Data Attivazione' },
];

const seriesCsvCols: { key: keyof ChargeSeriesPoint; label: string }[] = [
  { key: 'bucket', label: 'Periodo' },
  { key: 'total_importo', label: 'Totale' },
];

function is503(e: unknown): boolean {
  return e instanceof ApiError && e.status === 503;
}

function euro(n: number): string {
  return n.toLocaleString('it-IT', { style: 'currency', currency: 'EUR' });
}

function pct(part: number, total: number): string {
  if (!total) return '0%';
  return `${(part / total * 100).toFixed(1)}%`;
}

const tooltipStyle = {
  background: 'var(--color-bg-elevated)',
  border: '1px solid var(--color-border)',
  borderRadius: 8,
  fontSize: '0.8125rem',
};

function KpiCard({ label, value, sub, color }: { label: string; value: string; sub?: string; color?: string }) {
  return (
    <div className={is.kpi}>
      {color && <span className={is.kpiDot} style={{ background: color }} aria-hidden="true" />}
      <div className={is.kpiBody}>
        <span className={is.kpiLabel}>{label}</span>
        <span className={is.kpiValue}>{value}</span>
        {sub && <span className={is.kpiSub}>{sub}</span>}
      </div>
    </div>
  );
}

export function IaaSPayPerUsePage() {
  const [selectedDomain, setSelectedDomain] = useState<string | null>(null);
  const [accountSearch, setAccountSearch] = useState('');

  const accountsQ = useIaaSAccounts();
  const drill = useChargeDrill('12m');

  const { from, to, group } = drill.effective;
  const seriesQ = useChargesSeries(selectedDomain, from, to, group);
  const categoryQ = useChargesByCategory(selectedDomain, from, to);

  // Auto-select first account
  useEffect(() => {
    if (accountsQ.data && accountsQ.data.length > 0 && !selectedDomain) {
      setSelectedDomain(accountsQ.data[0]?.cloudstack_domain ?? null);
    }
  }, [accountsQ.data, selectedDomain]);

  const { filtered: filteredAccounts } = useTableFilter<IaaSAccount>({
    data: accountsQ.data,
    searchQuery: accountSearch,
    searchFields: ['intestazione', 'abbreviazione', 'serialnumber'],
  });

  const { sortedData: sortedAccounts } = useSortedData(filteredAccounts, 'intestazione');
  const exportAccounts = useCsvExport(accountCsvCols, 'iaas-accounts');
  const exportSeries = useCsvExport(seriesCsvCols, 'iaas-charges');

  const selectedAccount = accountsQ.data?.find(a => a.cloudstack_domain === selectedDomain) ?? null;
  const categories = categoryQ.data?.categories ?? [];
  const categoryTotal = categoryQ.data?.total ?? 0;
  const series = seriesQ.data ?? [];

  const { sortedData: sortedSeries, sort: seriesSort, toggle: toggleSeriesSort } = useSortedData(series, 'bucket', 'desc');

  const handleBarClick = (d: { payload?: ChargeSeriesPoint; bucket?: string }) => {
    const bucket = d?.payload?.bucket ?? d?.bucket;
    if (bucket) drill.drillInto(bucket);
  };

  if (accountsQ.error && is503(accountsQ.error)) {
    return <ServiceUnavailable service="Grappa" />;
  }
  if (accountsQ.error) {
    return <div className={s.empty}>Errore durante il caricamento degli account IaaS.</div>;
  }

  return (
    <div className={s.page}>
      <div className={is.layout}>
        {/* ── Master: account list ── */}
        <aside className={is.master}>
          <div className={is.masterHead}>
            <span className={is.masterTitle}>Account</span>
            {sortedAccounts.length > 0 && (
              <button className={s.btnSecondary} onClick={() => exportAccounts(sortedAccounts)}>CSV</button>
            )}
          </div>
          <div className={is.masterSearch}>
            <SearchInput value={accountSearch} onChange={setAccountSearch} placeholder="Cerca account..." />
          </div>

          <div className={is.accountList}>
            {accountsQ.isLoading && <div className={s.loading}>Caricamento account...</div>}
            {sortedAccounts.map(acc => {
              const active = selectedDomain === acc.cloudstack_domain;
              return (
                <button
                  key={acc.cloudstack_domain}
                  className={`${is.accountItem} ${active ? is.accountItemActive : ''}`}
                  onClick={() => { setSelectedDomain(acc.cloudstack_domain); drill.reset(); }}
                >
                  <span className={is.accountBar} />
                  <span className={is.accountMain}>
                    <span className={is.accountName}>{acc.intestazione}</span>
                    <span className={is.accountSub}>
                      <span>Credito {euro(acc.credito)}</span>
                      {acc.abbreviazione && <span className={is.accountAbbr}>{acc.abbreviazione}</span>}
                    </span>
                  </span>
                </button>
              );
            })}
            {!accountsQ.isLoading && sortedAccounts.length === 0 && (
              <div className={is.accountEmpty}>Nessun account.</div>
            )}
          </div>
        </aside>

        {/* ── Detail ── */}
        <section className={is.detail}>
          {!selectedAccount && <div className={s.empty}>Seleziona un account per visualizzare i consumi.</div>}

          {selectedAccount && (
            <>
              <header className={is.detailHead}>
                <h2 className={is.detailTitle}>{selectedAccount.intestazione}</h2>
                <div className={is.detailMeta}>
                  {selectedAccount.abbreviazione && <span>{selectedAccount.abbreviazione}</span>}
                  {selectedAccount.serialnumber && <span className={s.mono}>{selectedAccount.serialnumber}</span>}
                  {selectedAccount.data_attivazione && (
                    <span>Attivazione {selectedAccount.data_attivazione.slice(0, 10)}</span>
                  )}
                </div>
              </header>

              {/* Controls: period + granularity + breadcrumb */}
              <div className={is.controls}>
                <div className={s.field}>
                  <label>Periodo</label>
                  <select
                    className={s.nativeSelect}
                    value={drill.period}
                    onChange={e => drill.setPeriod(e.target.value as PeriodPreset)}
                    disabled={!drill.atBase}
                  >
                    {PERIOD_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                  </select>
                </div>
                <div className={s.field}>
                  <label>Aggregazione</label>
                  <select
                    className={s.nativeSelect}
                    value={drill.granularity}
                    onChange={e => drill.setGranularity(e.target.value as Granularity)}
                    disabled={!drill.atBase}
                  >
                    {GRANULARITIES.map(g => <option key={g} value={g}>{GROUP_LABELS[g]}</option>)}
                  </select>
                </div>

                {drill.breadcrumb.length > 1 && (
                  <nav className={is.breadcrumb} aria-label="Drill-down">
                    {drill.breadcrumb.map((item, i) => (
                      <span key={i} className={is.crumbWrap}>
                        {i > 0 && <span className={is.crumbSep} aria-hidden="true">›</span>}
                        <button className={is.crumb} onClick={() => drill.rollUpTo(item.index)}>{item.label}</button>
                      </span>
                    ))}
                  </nav>
                )}
              </div>

              {/* KPI row */}
              {categoryQ.error && !is503(categoryQ.error) && (
                <div className={s.empty}>Errore nel caricamento del riepilogo.</div>
              )}
              {categoryQ.error && is503(categoryQ.error) && <ServiceUnavailable service="Grappa" />}
              {!categoryQ.error && (
                <div className={is.kpiRow}>
                  <KpiCard label="Totale periodo" value={categoryQ.isLoading ? '…' : euro(categoryTotal)} />
                  {!categoryQ.isLoading && categories.map(c => (
                    <KpiCard
                      key={c.category}
                      label={c.category}
                      value={euro(c.amount)}
                      sub={pct(c.amount, categoryTotal)}
                      color={CATEGORY_COLORS[c.category] ?? DEFAULT_CATEGORY_COLOR}
                    />
                  ))}
                </div>
              )}

              {/* Charts */}
              <div className={is.charts}>
                <div className={is.chartCard}>
                  <div className={is.chartHead}>
                    <span>Consumo {GROUP_LABELS[drill.currentGroup].toLowerCase()}</span>
                    {drill.canDrill && <span className={is.chartHint}>clicca una barra per il dettaglio</span>}
                  </div>

                  {seriesQ.error && !is503(seriesQ.error) && <div className={s.empty}>Errore nel caricamento della serie.</div>}
                  {seriesQ.error && is503(seriesQ.error) && <ServiceUnavailable service="Grappa" />}
                  {seriesQ.isLoading && <div className={s.loading}>Caricamento...</div>}

                  {!seriesQ.error && !seriesQ.isLoading && series.length > 0 && (
                    <div className={is.chartBox}>
                      <ResponsiveContainer>
                        <BarChart data={series} margin={{ top: 10, right: 20, left: 0, bottom: 10 }}>
                          <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" vertical={false} />
                          <XAxis
                            dataKey="bucket"
                            tick={{ fontSize: 11 }}
                            tickFormatter={(b: string) => formatBucketTick(b, drill.currentGroup)}
                            minTickGap={16}
                          />
                          <YAxis tick={{ fontSize: 11 }} width={48} />
                          <Tooltip formatter={(v: number) => euro(v)} contentStyle={tooltipStyle} />
                          <Bar
                            dataKey="total_importo"
                            name="Importo"
                            fill="var(--color-accent)"
                            radius={[4, 4, 0, 0]}
                            cursor={drill.canDrill ? 'pointer' : 'default'}
                            onClick={drill.canDrill ? handleBarClick : undefined}
                          />
                        </BarChart>
                      </ResponsiveContainer>
                    </div>
                  )}

                  {!seriesQ.error && !seriesQ.isLoading && series.length === 0 && (
                    <div className={s.empty}>Nessun dato per il periodo selezionato.</div>
                  )}
                </div>

                <div className={is.pieCard}>
                  <div className={is.chartHead}><span>Composizione</span></div>
                  {categoryQ.isLoading && <div className={s.loading}>Caricamento...</div>}
                  {!categoryQ.isLoading && categories.length > 0 ? (
                    <div className={is.chartBox}>
                      <ResponsiveContainer>
                        <PieChart>
                          <Pie
                            data={categories}
                            dataKey="amount"
                            nameKey="category"
                            cx="50%"
                            cy="50%"
                            outerRadius={90}
                            innerRadius={45}
                            paddingAngle={2}
                            label={({ category, percent }: { category: string; percent: number }) =>
                              `${category} ${(percent * 100).toFixed(0)}%`
                            }
                          >
                            {categories.map(c => (
                              <Cell key={c.category} fill={CATEGORY_COLORS[c.category] ?? DEFAULT_CATEGORY_COLOR} />
                            ))}
                          </Pie>
                          <Tooltip formatter={(v: number) => euro(v)} contentStyle={tooltipStyle} />
                          <Legend wrapperStyle={{ fontSize: '0.75rem' }} />
                        </PieChart>
                      </ResponsiveContainer>
                    </div>
                  ) : (
                    !categoryQ.isLoading && <div className={s.empty}>Nessun dato di composizione.</div>
                  )}
                </div>
              </div>

              {/* Detail table */}
              <div className={is.tableSection}>
                <div className={s.toolbar}>
                  <div className={s.info}>{sortedSeries.length} periodi</div>
                  {sortedSeries.length > 0 && (
                    <button className={s.btnSecondary} onClick={() => exportSeries(sortedSeries)}>CSV</button>
                  )}
                </div>
                <div className={s.tableWrap}>
                  <table className={s.table}>
                    <thead>
                      <tr>
                        <SortableHeader label="Periodo" sortKey="bucket" sort={seriesSort} onToggle={toggleSeriesSort} />
                        <SortableHeader label="Totale" sortKey="total_importo" sort={seriesSort} onToggle={toggleSeriesSort} className={s.numCol} />
                      </tr>
                    </thead>
                    <tbody>
                      {sortedSeries.map((p, i) => (
                        <tr key={p.bucket} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                          <td>{bucketLabel(p.bucket, drill.currentGroup)}</td>
                          <td className={s.numCol}>{euro(p.total_importo)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </>
          )}
        </section>
      </div>
    </div>
  );
}
