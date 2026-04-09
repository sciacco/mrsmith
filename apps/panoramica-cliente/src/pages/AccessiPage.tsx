import { useState, useCallback } from 'react';
import { MultiSelect, SearchInput, useTableFilter } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCustomersWithAccessLines, useConnectionTypes, useAccessLines } from '../api/queries';
import { useSortedData } from '../hooks/useSort';
import { useCsvExport } from '../hooks/useCsvExport';
import { SortableHeader } from '../components/shared/SortableHeader';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import type { AccessLine } from '../types';
import s from './shared.module.css';

const csvColumns: { key: keyof AccessLine; label: string }[] = [
  { key: 'tipo_conn', label: 'Tipo Conn.' },
  { key: 'fornitore', label: 'Fornitore' },
  { key: 'provincia', label: 'Provincia' },
  { key: 'comune', label: 'Comune' },
  { key: 'tipo', label: 'Tipo' },
  { key: 'profilo_commerciale', label: 'Profilo' },
  { key: 'intestatario', label: 'Intestatario' },
  { key: 'ordine', label: 'Ordine' },
  { key: 'stato', label: 'Stato' },
  { key: 'serialnumber', label: 'Serialnumber' },
];

const defaultStati = ['Attiva'];

export function AccessiPage() {
  const [selectedClients, setSelectedClients] = useState<number[]>([]);
  const [selectedStati, setSelectedStati] = useState<string[]>(defaultStati);
  const [selectedTipi, setSelectedTipi] = useState<string[]>([]);
  const [searchTriggered, setSearchTriggered] = useState(false);
  const [search, setSearch] = useState('');

  const customersQ = useCustomersWithAccessLines();
  const connTypesQ = useConnectionTypes();

  const accessQ = useAccessLines(selectedClients, selectedStati, selectedTipi, searchTriggered);

  const { filtered } = useTableFilter<AccessLine>({
    data: accessQ.data,
    searchQuery: search,
    searchFields: ['tipo_conn', 'fornitore', 'comune', 'intestatario', 'serialnumber', 'ordine'],
  });

  const { sortedData, sort, toggle } = useSortedData(filtered, 'tipo_conn');
  const exportCsv = useCsvExport(csvColumns, 'accessi');

  const handleSearch = useCallback(() => {
    setSearchTriggered(true);
  }, []);

  if (customersQ.error && (customersQ.error as ApiError).status === 503) {
    return <ServiceUnavailable service="Mistra" />;
  }

  const clientOptions = (customersQ.data ?? []).map(c => ({
    value: c.id,
    label: c.intestazione,
  }));

  const statiOptions = ['Attiva', 'Cessata', 'In lavorazione', 'Sospesa'].map(s => ({ value: s, label: s }));

  const tipiOptions = (connTypesQ.data ?? []).map(t => ({ value: t, label: t }));

  return (
    <div className={s.page}>
      <div className={s.toolbar}>
        <div className={s.field} style={{ minWidth: 280 }}>
          <label>Clienti</label>
          <MultiSelect options={clientOptions} selected={selectedClients} onChange={setSelectedClients} placeholder="Seleziona clienti..." />
        </div>
        <div className={s.field} style={{ minWidth: 200 }}>
          <label>Stato</label>
          <MultiSelect options={statiOptions} selected={selectedStati} onChange={setSelectedStati} placeholder="Stati..." />
        </div>
        <div className={s.field} style={{ minWidth: 200 }}>
          <label>Tipo connessione</label>
          <MultiSelect options={tipiOptions} selected={selectedTipi} onChange={setSelectedTipi} placeholder="Tipi..." />
        </div>
        <button
          className={s.btnPrimary}
          onClick={handleSearch}
          disabled={selectedClients.length === 0 || selectedStati.length === 0 || selectedTipi.length === 0}
        >
          Cerca
        </button>
        <SearchInput value={search} onChange={setSearch} placeholder="Filtra risultati..." />
        {sortedData.length > 0 && (
          <button className={s.btnSecondary} onClick={() => exportCsv(sortedData)}>CSV</button>
        )}
      </div>

      {!searchTriggered && <div className={s.empty}>Seleziona clienti, stati e tipi, poi premi Cerca.</div>}
      {searchTriggered && accessQ.isLoading && <div className={s.loading}>Caricamento...</div>}
      {searchTriggered && accessQ.error && (accessQ.error as ApiError).status === 503 && <ServiceUnavailable service="Mistra" />}

      {searchTriggered && sortedData.length > 0 && (
        <>
          <div className={s.info}>{sortedData.length} linee</div>
          <div className={s.tableWrap}>
            <table className={s.table}>
              <thead>
                <tr>
                  <SortableHeader label="Tipo Conn." sortKey="tipo_conn" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Fornitore" sortKey="fornitore" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Provincia" sortKey="provincia" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Comune" sortKey="comune" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Tipo" sortKey="tipo" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Profilo" sortKey="profilo_commerciale" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Intestatario" sortKey="intestatario" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Ordine" sortKey="ordine" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Stato" sortKey="stato" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Serialnumber" sortKey="serialnumber" sort={sort} onToggle={toggle} />
                </tr>
              </thead>
              <tbody>
                {sortedData.map((row, i) => (
                  <tr key={`${row.id}-${i}`} style={{ animationDelay: `${Math.min(i * 15, 300)}ms` }}>
                    <td>{row.tipo_conn}</td>
                    <td>{row.fornitore ?? ''}</td>
                    <td>{row.provincia ?? ''}</td>
                    <td>{row.comune ?? ''}</td>
                    <td>{row.tipo ?? ''}</td>
                    <td>{row.profilo_commerciale ?? ''}</td>
                    <td>{row.intestatario ?? ''}</td>
                    <td className={s.mono}>{row.ordine ?? ''}</td>
                    <td>{row.stato}</td>
                    <td className={s.mono}>{row.serialnumber ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {searchTriggered && !accessQ.isLoading && sortedData.length === 0 && !accessQ.error && (
        <div className={s.empty}>Nessun accesso trovato.</div>
      )}
    </div>
  );
}
