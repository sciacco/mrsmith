import { useState } from 'react';
import { SingleSelect, SearchInput, useTableFilter } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCustomersWithInvoices, useInvoices } from '../api/queries';
import { useSortedData } from '../hooks/useSort';
import { useCsvExport } from '../hooks/useCsvExport';
import { SortableHeader } from '../components/shared/SortableHeader';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import type { InvoiceLine } from '../types';
import s from './shared.module.css';

const periodOptions = [
  { value: 0, label: 'Tutti' },
  { value: 6, label: 'Ultimi 6 mesi' },
  { value: 12, label: 'Ultimi 12 mesi' },
  { value: 24, label: 'Ultimi 24 mesi' },
  { value: 36, label: 'Ultimi 36 mesi' },
];

const csvColumns: { key: keyof InvoiceLine; label: string }[] = [
  { key: 'documento', label: 'Documento' },
  { key: 'descrizione_riga', label: 'Descrizione' },
  { key: 'qta', label: 'Qta' },
  { key: 'prezzo_unitario', label: 'Prezzo Unitario' },
  { key: 'prezzo_totale_netto', label: 'Totale Netto' },
  { key: 'codice_articolo', label: 'Codice Articolo' },
  { key: 'serialnumber', label: 'Serialnumber' },
  { key: 'condizione_pagamento', label: 'Condizione Pagamento' },
  { key: 'desc_conto_ricavo', label: 'Conto Ricavo' },
  { key: 'gruppo', label: 'Gruppo' },
  { key: 'sottogruppo', label: 'Sottogruppo' },
];

export function FatturePage() {
  const [cliente, setCliente] = useState<number | null>(null);
  const [mesi, setMesi] = useState<number>(0);
  const [search, setSearch] = useState('');

  const customersQ = useCustomersWithInvoices();
  const invoicesQ = useInvoices(cliente, mesi || null);

  const { filtered } = useTableFilter<InvoiceLine>({
    data: invoicesQ.data,
    searchQuery: search,
    searchFields: ['descrizione_riga', 'codice_articolo', 'serialnumber', 'documento'],
  });

  const { sortedData, sort, toggle } = useSortedData(filtered, 'rn');
  const exportCsv = useCsvExport(csvColumns, 'fatture');

  if (customersQ.error && (customersQ.error as ApiError).status === 503) {
    return <ServiceUnavailable service="Mistra" />;
  }

  const customerOptions = (customersQ.data ?? []).map(c => ({
    value: c.numero_azienda,
    label: c.ragione_sociale,
  }));

  return (
    <div className={s.page}>
      <div className={s.toolbar}>
        <div className={s.field} style={{ minWidth: 280 }}>
          <label>Cliente</label>
          <SingleSelect options={customerOptions} selected={cliente} onChange={setCliente} placeholder="Seleziona cliente..." />
        </div>
        <div className={s.field}>
          <label>Periodo</label>
          <select className={s.nativeSelect} value={mesi} onChange={e => setMesi(Number(e.target.value))}>
            {periodOptions.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <SearchInput value={search} onChange={setSearch} placeholder="Cerca fatture..." />
        {sortedData.length > 0 && (
          <button className={s.btnSecondary} onClick={() => exportCsv(sortedData)}>CSV</button>
        )}
      </div>

      {!cliente && <div className={s.empty}>Seleziona un cliente per visualizzare le fatture.</div>}
      {cliente && invoicesQ.isLoading && <div className={s.loading}>Caricamento...</div>}
      {cliente && invoicesQ.error && (invoicesQ.error as ApiError).status === 503 && <ServiceUnavailable service="Mistra" />}

      {cliente && sortedData.length > 0 && (
        <>
          <div className={s.info}>{sortedData.length} righe</div>
          <div className={s.tableWrap}>
            <table className={s.table}>
              <thead>
                <tr>
                  <SortableHeader label="Documento" sortKey="documento" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Descrizione" sortKey="descrizione_riga" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Qta" sortKey="qta" sort={sort} onToggle={toggle} className={s.numCol} />
                  <SortableHeader label="Prezzo Unit." sortKey="prezzo_unitario" sort={sort} onToggle={toggle} className={s.numCol} />
                  <SortableHeader label="Totale Netto" sortKey="prezzo_totale_netto" sort={sort} onToggle={toggle} className={s.numCol} />
                  <SortableHeader label="Cod. Articolo" sortKey="codice_articolo" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Serialnumber" sortKey="serialnumber" sort={sort} onToggle={toggle} />
                  <SortableHeader label="Conto Ricavo" sortKey="desc_conto_ricavo" sort={sort} onToggle={toggle} />
                </tr>
              </thead>
              <tbody>
                {sortedData.map((row, i) => (
                  <tr key={`${row.num_documento}-${row.progressivo_riga}-${i}`} style={{ animationDelay: `${Math.min(i * 15, 300)}ms` }}>
                    <td className={row.documento ? s.docHeader : undefined}>{row.documento ?? ''}</td>
                    <td>{row.descrizione_riga}</td>
                    <td className={s.numCol}>{row.qta}</td>
                    <td className={s.numCol}>{row.prezzo_unitario.toFixed(2)}</td>
                    <td className={s.numCol}>{row.prezzo_totale_netto.toFixed(2)}</td>
                    <td className={s.mono}>{row.codice_articolo ?? ''}</td>
                    <td className={s.mono}>{row.serialnumber ?? ''}</td>
                    <td>{row.desc_conto_ricavo ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {cliente && !invoicesQ.isLoading && sortedData.length === 0 && !invoicesQ.error && (
        <div className={s.empty}>Nessuna fattura trovata.</div>
      )}
    </div>
  );
}
