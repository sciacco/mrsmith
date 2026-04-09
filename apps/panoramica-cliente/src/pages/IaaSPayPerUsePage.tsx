import { useState, useEffect } from 'react';
import { SearchInput, useTableFilter } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend,
} from 'recharts';
import { useIaaSAccounts, useDailyCharges, useMonthlyCharges, useChargeBreakdown } from '../api/queries';
import { useSortedData } from '../hooks/useSort';
import { useCsvExport } from '../hooks/useCsvExport';
import { SortableHeader } from '../components/shared/SortableHeader';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import type { IaaSAccount, DailyCharge } from '../types';
import s from './shared.module.css';
import is from './IaaSPayPerUse.module.css';

const PIE_COLORS = ['#635bff', '#4338ca', '#6366f1', '#818cf8', '#a5b4fc', '#c7d2fe', '#e0e7ff', '#312e81', '#4f46e5', '#7c3aed'];

const accountCsvCols: { key: keyof IaaSAccount; label: string }[] = [
  { key: 'intestazione', label: 'Intestazione' },
  { key: 'credito', label: 'Credito' },
  { key: 'abbreviazione', label: 'Abbreviazione' },
  { key: 'serialnumber', label: 'Serialnumber' },
  { key: 'data_attivazione', label: 'Data Attivazione' },
];

const dailyCsvCols: { key: keyof DailyCharge; label: string }[] = [
  { key: 'giorno', label: 'Giorno' },
  { key: 'utCredit', label: 'utCredit' },
  { key: 'total_importo', label: 'Totale' },
];

type TabId = 'giornaliero' | 'mensile';

export function IaaSPayPerUsePage() {
  const [selectedDomain, setSelectedDomain] = useState<string | null>(null);
  const [selectedDay, setSelectedDay] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabId>('giornaliero');
  const [accountSearch, setAccountSearch] = useState('');

  const accountsQ = useIaaSAccounts();
  const dailyQ = useDailyCharges(selectedDomain);
  const monthlyQ = useMonthlyCharges(selectedDomain);
  const breakdownQ = useChargeBreakdown(selectedDomain, selectedDay);

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

  const { sortedData: sortedAccounts, sort: accountSort, toggle: toggleAccountSort } = useSortedData(filteredAccounts, 'intestazione');
  const exportAccounts = useCsvExport(accountCsvCols, 'iaas-accounts');
  const exportDaily = useCsvExport(dailyCsvCols, 'iaas-daily');

  if (accountsQ.error && (accountsQ.error as ApiError).status === 503) {
    return <ServiceUnavailable service="Grappa" />;
  }

  const monthlyData = (monthlyQ.data ?? []).slice().reverse();

  return (
    <div className={s.page}>
      {/* Account table */}
      <div style={{ marginBottom: 'var(--space-6)' }}>
        <div className={s.toolbar}>
          <SearchInput value={accountSearch} onChange={setAccountSearch} placeholder="Cerca account..." />
          {sortedAccounts.length > 0 && (
            <button className={s.btnSecondary} onClick={() => exportAccounts(sortedAccounts)}>CSV</button>
          )}
        </div>

        {accountsQ.isLoading && <div className={s.loading}>Caricamento account...</div>}

        {sortedAccounts.length > 0 && (
          <div className={s.tableWrap} style={{ maxHeight: 300, overflow: 'auto' }}>
            <table className={s.table}>
              <thead>
                <tr>
                  <SortableHeader label="Intestazione" sortKey="intestazione" sort={accountSort} onToggle={toggleAccountSort} />
                  <SortableHeader label="Credito" sortKey="credito" sort={accountSort} onToggle={toggleAccountSort} className={s.numCol} />
                  <SortableHeader label="Abbreviazione" sortKey="abbreviazione" sort={accountSort} onToggle={toggleAccountSort} />
                  <SortableHeader label="Serialnumber" sortKey="serialnumber" sort={accountSort} onToggle={toggleAccountSort} />
                  <SortableHeader label="Data Attivazione" sortKey="data_attivazione" sort={accountSort} onToggle={toggleAccountSort} />
                </tr>
              </thead>
              <tbody>
                {sortedAccounts.map((acc, i) => (
                  <tr
                    key={acc.cloudstack_domain}
                    className={selectedDomain === acc.cloudstack_domain ? is.selectedRow : undefined}
                    onClick={() => { setSelectedDomain(acc.cloudstack_domain); setSelectedDay(null); }}
                    style={{ cursor: 'pointer', animationDelay: `${Math.min(i * 10, 300)}ms` }}
                  >
                    <td>{acc.intestazione}</td>
                    <td className={s.numCol}>{acc.credito.toFixed(2)}</td>
                    <td>{acc.abbreviazione ?? ''}</td>
                    <td className={s.mono}>{acc.serialnumber ?? ''}</td>
                    <td>{acc.data_attivazione?.slice(0, 10) ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Detail tabs */}
      {selectedDomain && (
        <>
          <div className={is.tabs}>
            <button className={`${is.tab} ${activeTab === 'giornaliero' ? is.tabActive : ''}`} onClick={() => setActiveTab('giornaliero')}>
              Giornaliero
            </button>
            <button className={`${is.tab} ${activeTab === 'mensile' ? is.tabActive : ''}`} onClick={() => setActiveTab('mensile')}>
              Mensile
            </button>
          </div>

          {activeTab === 'giornaliero' && (
            <div className={is.detailSection}>
              {dailyQ.isLoading && <div className={s.loading}>Caricamento...</div>}

              {(dailyQ.data ?? []).length > 0 && (
                <div style={{ display: 'flex', gap: 'var(--space-6)', flexWrap: 'wrap' }}>
                  <div style={{ flex: '1 1 400px' }}>
                    <div className={s.toolbar}>
                      <div className={s.info}>{dailyQ.data?.length} giorni</div>
                      <button className={s.btnSecondary} onClick={() => exportDaily(dailyQ.data ?? [])}>CSV</button>
                    </div>
                    <div className={s.tableWrap} style={{ maxHeight: 400, overflow: 'auto' }}>
                      <table className={s.table}>
                        <thead>
                          <tr>
                            <th>Giorno</th>
                            <th className={s.numCol}>utCredit</th>
                            <th className={s.numCol}>Totale</th>
                          </tr>
                        </thead>
                        <tbody>
                          {(dailyQ.data ?? []).map(d => (
                            <tr
                              key={d.giorno}
                              className={selectedDay === d.giorno ? is.selectedRow : undefined}
                              onClick={() => setSelectedDay(d.giorno)}
                              style={{ cursor: 'pointer' }}
                            >
                              <td>{d.giorno.slice(0, 10)}</td>
                              <td className={s.numCol}>{d.utCredit.toFixed(2)}</td>
                              <td className={s.numCol}>{d.total_importo.toFixed(2)}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>

                  {selectedDay && breakdownQ.data && breakdownQ.data.charges.length > 0 && (
                    <div style={{ flex: '0 0 320px' }}>
                      <h3 style={{ fontSize: '0.8125rem', fontWeight: 600, marginBottom: 'var(--space-2)' }}>
                        Dettaglio {selectedDay.slice(0, 10)}
                      </h3>
                      <ResponsiveContainer width="100%" height={280}>
                        <PieChart>
                          <Pie
                            data={breakdownQ.data.charges}
                            dataKey="amount"
                            nameKey="label"
                            cx="50%"
                            cy="50%"
                            outerRadius={100}
                            label={({ label, percent }: { label: string; percent: number }) => `${label} ${(percent * 100).toFixed(0)}%`}
                          >
                            {breakdownQ.data.charges.map((_, i) => (
                              <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />
                            ))}
                          </Pie>
                          <Tooltip formatter={(v: number) => v.toFixed(2)} />
                          <Legend />
                        </PieChart>
                      </ResponsiveContainer>
                      <div style={{ textAlign: 'center', fontSize: '0.875rem', fontWeight: 600, marginTop: 'var(--space-2)' }}>
                        Totale: {breakdownQ.data.total.toFixed(2)}
                      </div>
                    </div>
                  )}
                </div>
              )}

              {!dailyQ.isLoading && (dailyQ.data ?? []).length === 0 && (
                <div className={s.empty}>Nessun dato giornaliero.</div>
              )}
            </div>
          )}

          {activeTab === 'mensile' && (
            <div className={is.detailSection}>
              {monthlyQ.isLoading && <div className={s.loading}>Caricamento...</div>}

              {monthlyData.length > 0 && (
                <div style={{ width: '100%', height: 400 }}>
                  <ResponsiveContainer>
                    <BarChart data={monthlyData} margin={{ top: 10, right: 30, left: 10, bottom: 10 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                      <XAxis dataKey="mese" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} />
                      <Tooltip
                        formatter={(v: number) => v.toFixed(2)}
                        contentStyle={{
                          background: 'var(--color-bg-elevated)',
                          border: '1px solid var(--color-border)',
                          borderRadius: 8,
                          fontSize: '0.8125rem',
                        }}
                      />
                      <Bar dataKey="importo" name="Importo" fill="var(--color-accent)" radius={[4, 4, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              )}

              {!monthlyQ.isLoading && monthlyData.length === 0 && (
                <div className={s.empty}>Nessun dato mensile.</div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}
