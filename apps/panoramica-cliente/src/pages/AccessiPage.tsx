import { useState, useCallback } from 'react';
import { Button, Icon, MultiSelect, SearchInput, useTableFilter } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCustomersWithAccessLines, useConnectionTypes, useAccessLines } from '../api/queries';
import { useSortedData } from '../hooks/useSort';
import { localDateStamp, useExcelExport, type ExcelColumn } from '../hooks/useExcelExport';
import { SortableHeader } from '../components/shared/SortableHeader';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import type { AccessLine } from '../types';
import s from './shared.module.css';

const excelColumns: ExcelColumn<AccessLine>[] = [
  { key: 'tipo_conn', label: 'Tipo Conn.', width: 18 },
  { key: 'fornitore', label: 'Fornitore', width: 24 },
  { key: 'provincia', label: 'Provincia', width: 14 },
  { key: 'comune', label: 'Comune', width: 22 },
  { key: 'tipo', label: 'Tipo', width: 18 },
  { key: 'profilo_commerciale', label: 'Profilo', width: 28 },
  { key: 'intestatario', label: 'Intestatario', width: 32 },
  { key: 'ordine', label: 'Ordine', width: 20 },
  { key: 'stato', label: 'Stato', width: 18 },
  { key: 'serialnumber', label: 'Serialnumber', width: 24 },
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
  const selectedClientNames = (customersQ.data ?? [])
    .filter(customer => selectedClients.includes(customer.id))
    .map(customer => customer.intestazione);
  const clientContext = selectedClientNames.length === 1 ? selectedClientNames[0] : `${selectedClientNames.length}-clienti`;
  const { exportExcel, exporting, error: exportError } = useExcelExport(excelColumns, {
    filename: `accessi_${clientContext}_${localDateStamp()}`,
    sheetName: 'Accessi',
  });

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
        <Button
          size="sm"
          onClick={handleSearch}
          disabled={selectedClients.length === 0 || selectedStati.length === 0 || selectedTipi.length === 0}
        >
          Cerca
        </Button>
        <SearchInput value={search} onChange={setSearch} placeholder="Filtra risultati..." />
        {sortedData.length > 0 && (
          <Button variant="secondary" size="sm" leftIcon={<Icon name="download" size={16} />} loading={exporting} onClick={() => exportExcel(sortedData)}>
            {exporting ? 'Preparazione file Excel…' : 'Esporta Excel'}
          </Button>
        )}
      </div>

      {exportError && <div className={s.exportError} role="alert">Non è stato possibile generare il file Excel. Riprova.</div>}

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
