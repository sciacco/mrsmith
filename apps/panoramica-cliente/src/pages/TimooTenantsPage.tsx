import { useState } from 'react';
import { SingleSelect, SearchInput, useTableFilter } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useTimooTenants, usePbxStats } from '../api/queries';
import { useSortedData } from '../hooks/useSort';
import { useCsvExport } from '../hooks/useCsvExport';
import { SortableHeader } from '../components/shared/SortableHeader';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import type { PbxRow } from '../types';
import s from './shared.module.css';

const csvColumns: { key: keyof PbxRow; label: string }[] = [
  { key: 'pbx_name', label: 'PBX' },
  { key: 'pbx_id', label: 'PBX ID' },
  { key: 'users', label: 'Users' },
  { key: 'service_extensions', label: 'Service Ext.' },
  { key: 'totale', label: 'Totale' },
];

export function TimooTenantsPage() {
  const [tenantId, setTenantId] = useState<number | null>(null);
  const [search, setSearch] = useState('');

  const tenantsQ = useTimooTenants();
  const pbxQ = usePbxStats(tenantId);

  const { filtered } = useTableFilter<PbxRow>({
    data: pbxQ.data?.rows,
    searchQuery: search,
    searchFields: ['pbx_name'],
  });

  const { sortedData, sort, toggle } = useSortedData(filtered, 'pbx_name');
  const exportCsv = useCsvExport(csvColumns, 'timoo-pbx');

  if (tenantsQ.error && (tenantsQ.error as ApiError).status === 503) {
    return <ServiceUnavailable service="Anisetta" />;
  }

  const tenantOptions = (tenantsQ.data ?? []).map(t => ({
    value: t.as7_tenant_id,
    label: t.name,
  }));

  return (
    <div className={s.page}>
      <div className={s.toolbar}>
        <div className={s.field} style={{ minWidth: 280 }}>
          <label>Tenant</label>
          <SingleSelect options={tenantOptions} selected={tenantId} onChange={setTenantId} placeholder="Seleziona tenant..." />
        </div>
        <SearchInput value={search} onChange={setSearch} placeholder="Cerca PBX..." />
        {sortedData.length > 0 && (
          <button className={s.btnSecondary} onClick={() => exportCsv(sortedData)}>CSV</button>
        )}
      </div>

      {!tenantId && <div className={s.empty}>Seleziona un tenant per visualizzare le statistiche PBX.</div>}
      {tenantId && pbxQ.isLoading && <div className={s.loading}>Caricamento...</div>}
      {tenantId && pbxQ.error && (pbxQ.error as ApiError).status === 503 && <ServiceUnavailable service="Anisetta" />}

      {tenantId && pbxQ.data && (
        <>
          <div className={s.summaryRow}>
            <div className={s.summaryItem}>
              <span className={s.summaryLabel}>Totale Users</span>
              <span className={s.summaryValue}>{pbxQ.data.totalUsers}</span>
            </div>
            <div className={s.summaryItem}>
              <span className={s.summaryLabel}>Totale Service Ext.</span>
              <span className={s.summaryValue}>{pbxQ.data.totalSE}</span>
            </div>
            <div className={s.summaryItem}>
              <span className={s.summaryLabel}>Totale</span>
              <span className={s.summaryValue}>{pbxQ.data.totalUsers + pbxQ.data.totalSE}</span>
            </div>
          </div>

          {sortedData.length > 0 && (
            <>
              <div className={s.info}>{sortedData.length} PBX</div>
              <div className={s.tableWrap}>
                <table className={s.table}>
                  <thead>
                    <tr>
                      <SortableHeader label="PBX" sortKey="pbx_name" sort={sort} onToggle={toggle} />
                      <SortableHeader label="PBX ID" sortKey="pbx_id" sort={sort} onToggle={toggle} className={s.numCol} />
                      <SortableHeader label="Users" sortKey="users" sort={sort} onToggle={toggle} className={s.numCol} />
                      <SortableHeader label="Service Ext." sortKey="service_extensions" sort={sort} onToggle={toggle} className={s.numCol} />
                      <SortableHeader label="Totale" sortKey="totale" sort={sort} onToggle={toggle} className={s.numCol} />
                    </tr>
                  </thead>
                  <tbody>
                    {sortedData.map((row, i) => (
                      <tr key={row.pbx_id} style={{ animationDelay: `${Math.min(i * 15, 300)}ms` }}>
                        <td>{row.pbx_name}</td>
                        <td className={s.numCol}>{row.pbx_id}</td>
                        <td className={s.numCol}>{row.users}</td>
                        <td className={s.numCol}>{row.service_extensions}</td>
                        <td className={s.numCol}>{row.totale}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {sortedData.length === 0 && !pbxQ.isLoading && (
            <div className={s.empty}>Nessun PBX trovato per questo tenant.</div>
          )}
        </>
      )}
    </div>
  );
}
