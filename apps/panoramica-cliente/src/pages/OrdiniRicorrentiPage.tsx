import { useState, useCallback } from 'react';
import { SingleSelect, MultiSelect, SearchInput, useTableFilter } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCustomersWithOrders, useOrderStatuses, useOrdersSummary } from '../api/queries';
import { useCsvExport } from '../hooks/useCsvExport';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import { SlideOverPanel } from '../components/shared/SlideOverPanel';
import type { OrderSummaryRow } from '../types';
import s from './shared.module.css';
import os from './OrdiniRicorrenti.module.css';

const defaultStati = ['Evaso', 'Confermato'];

const csvColumns: { key: keyof OrderSummaryRow; label: string }[] = [
  { key: 'stato_ordine', label: 'Stato Ordine' },
  { key: 'numero_ordine', label: 'N. Ordine' },
  { key: 'nome_testata_ordine', label: 'Ordine' },
  { key: 'descrizione_long', label: 'Descrizione' },
  { key: 'quantita', label: 'Qta' },
  { key: 'nrc', label: 'NRC' },
  { key: 'mrc', label: 'MRC' },
  { key: 'totale_mrc', label: 'Totale MRC' },
  { key: 'data_documento', label: 'Data Doc.' },
  { key: 'stato_riga', label: 'Stato Riga' },
  { key: 'serialnumber', label: 'Serialnumber' },
];

function statoBadge(stato: string) {
  const map: Record<string, string | undefined> = {
    'Evaso': s.badgeGreen,
    'Confermato': s.badgeYellow,
    'Cessato': s.badgeRed,
    'Bloccato': s.badgeRed,
  };
  return <span className={`${s.badge} ${map[stato] ?? s.badgeGray}`}>{stato}</span>;
}

export function OrdiniRicorrentiPage() {
  const [cliente, setCliente] = useState<number | null>(null);
  const [stati, setStati] = useState<string[]>(defaultStati);
  const [searchTriggered, setSearchTriggered] = useState(false);
  const [search, setSearch] = useState('');
  const [selectedRow, setSelectedRow] = useState<OrderSummaryRow | null>(null);

  const customersQ = useCustomersWithOrders('a');
  const statusesQ = useOrderStatuses();
  const ordersQ = useOrdersSummary(
    searchTriggered ? cliente : null,
    searchTriggered ? stati : [],
  );

  const { filtered } = useTableFilter<OrderSummaryRow>({
    data: ordersQ.data,
    searchQuery: search,
    searchFields: ['descrizione_long', 'numero_ordine', 'nome_testata_ordine', 'serialnumber'],
  });

  const exportCsv = useCsvExport(csvColumns, 'ordini-ricorrenti');

  const handleSearch = useCallback(() => {
    setSearchTriggered(true);
  }, []);

  if (customersQ.error && (customersQ.error as ApiError).status === 503) {
    return <ServiceUnavailable service="Mistra" />;
  }

  const customerOptions = (customersQ.data ?? []).map(c => ({
    value: c.numero_azienda,
    label: c.ragione_sociale,
  }));

  const statiOptions = (statusesQ.data ?? []).map(st => ({ value: st, label: st }));

  // Group siblings by nome_testata_ordine for the panel
  const siblings = selectedRow
    ? filtered.filter(r => r.nome_testata_ordine === selectedRow.nome_testata_ordine)
    : [];

  return (
    <div className={s.page}>
      <div className={s.toolbar}>
        <div className={s.field} style={{ minWidth: 280 }}>
          <label>Cliente</label>
          <SingleSelect options={customerOptions} selected={cliente} onChange={v => { setCliente(v); setSearchTriggered(false); }} placeholder="Seleziona cliente..." />
        </div>
        <div className={s.field} style={{ minWidth: 200 }}>
          <label>Stati ordine</label>
          <MultiSelect options={statiOptions} selected={stati} onChange={setStati} placeholder="Stati..." />
        </div>
        <button className={s.btnPrimary} onClick={handleSearch} disabled={cliente === null || stati.length === 0}>Cerca</button>
        <SearchInput value={search} onChange={setSearch} placeholder="Filtra..." />
        {filtered.length > 0 && (
          <button className={s.btnSecondary} onClick={() => exportCsv(filtered)}>CSV</button>
        )}
      </div>

      {!searchTriggered && <div className={s.empty}>Seleziona un cliente e gli stati ordine, poi premi Cerca.</div>}
      {searchTriggered && ordersQ.isLoading && <div className={s.loading}>Caricamento...</div>}

      {searchTriggered && filtered.length > 0 && (
        <>
          <div className={s.info}>{filtered.length} righe</div>
          <div className={s.tableWrap}>
            <table className={s.table}>
              <thead>
                <tr>
                  <th>Stato ordine</th>
                  <th>N. Ordine</th>
                  <th>Ordine / Descrizione</th>
                  <th className={s.numCol}>Qta</th>
                  <th className={s.numCol}>NRC</th>
                  <th className={s.numCol}>MRC</th>
                  <th className={s.numCol}>Tot. MRC</th>
                  <th>Data doc.</th>
                  <th>Stato riga</th>
                  <th>Serialnumber</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((row, i) => {
                  const isGroupHead = row.rn === 1;
                  return (
                    <tr
                      key={`${row.nome_testata_ordine}-${row.rn}-${i}`}
                      className={`${isGroupHead ? os.groupHead : os.groupChild} ${selectedRow === row ? os.selected : ''}`}
                      onClick={() => setSelectedRow(row)}
                      style={{ animationDelay: `${Math.min(i * 10, 300)}ms`, cursor: 'pointer' }}
                    >
                      <td>{statoBadge(row.stato_ordine)}</td>
                      <td className={s.mono}>{row.numero_ordine}</td>
                      <td>
                        {isGroupHead && <div className={os.orderName}>{row.nome_testata_ordine}</div>}
                        <div className={isGroupHead ? undefined : os.indented}>{row.descrizione_long}</div>
                      </td>
                      <td className={s.numCol}>{row.quantita}</td>
                      <td className={s.numCol}>{row.nrc.toFixed(2)}</td>
                      <td className={s.numCol}>{row.mrc.toFixed(2)}</td>
                      <td className={s.numCol}>{row.totale_mrc.toFixed(2)}</td>
                      <td>{row.data_documento?.slice(0, 10) ?? ''}</td>
                      <td>{statoBadge(row.stato_riga)}</td>
                      <td className={s.mono}>{row.serialnumber ?? ''}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}

      {searchTriggered && !ordersQ.isLoading && filtered.length === 0 && !ordersQ.error && (
        <div className={s.empty}>Nessun ordine trovato.</div>
      )}

      <SlideOverPanel
        open={selectedRow !== null}
        onClose={() => setSelectedRow(null)}
        title={selectedRow ? `${selectedRow.nome_testata_ordine} - ${selectedRow.numero_ordine}` : ''}
      >
        {selectedRow && (
          <div className={os.panelContent}>
            <div className={os.panelSection}>
              <h3>Testata ordine</h3>
              <dl className={os.detailGrid}>
                <dt>Stato ordine</dt><dd>{statoBadge(selectedRow.stato_ordine)}</dd>
                <dt>Data documento</dt><dd>{selectedRow.data_documento?.slice(0, 10) ?? '-'}</dd>
                <dt>Metodo pagamento</dt><dd>{selectedRow.metodo_pagamento ?? '-'}</dd>
                <dt>Durata servizio</dt><dd>{selectedRow.durata_servizio ?? '-'}</dd>
                <dt>Durata rinnovo</dt><dd>{selectedRow.durata_rinnovo ?? '-'}</dd>
                <dt>Storico</dt><dd>{selectedRow.storico ?? '-'}</dd>
                <dt>Sost. ordine</dt><dd>{selectedRow.sost_ord ?? '-'}</dd>
                <dt>Sostituito da</dt><dd>{selectedRow.sostituito_da ?? '-'}</dd>
              </dl>
            </div>

            <div className={os.panelSection}>
              <h3>Riga selezionata</h3>
              <dl className={os.detailGrid}>
                <dt>Descrizione</dt><dd>{selectedRow.descrizione_long}</dd>
                <dt>Quantita</dt><dd>{selectedRow.quantita}</dd>
                <dt>NRC</dt><dd>{selectedRow.nrc.toFixed(2)}</dd>
                <dt>MRC</dt><dd>{selectedRow.mrc.toFixed(2)}</dd>
                <dt>Totale MRC</dt><dd>{selectedRow.totale_mrc.toFixed(2)}</dd>
                <dt>Stato riga</dt><dd>{statoBadge(selectedRow.stato_riga)}</dd>
                <dt>Serialnumber</dt><dd className={s.mono}>{selectedRow.serialnumber ?? '-'}</dd>
                <dt>Data attivazione</dt><dd>{selectedRow.data_attivazione?.slice(0, 10) ?? '-'}</dd>
                <dt>Data cessazione</dt><dd>{selectedRow.data_cessazione?.slice(0, 10) ?? '-'}</dd>
                <dt>Data ultima fatt.</dt><dd>{selectedRow.data_ultima_fatt?.slice(0, 10) ?? '-'}</dd>
              </dl>
            </div>

            {selectedRow.note_legali && (
              <div className={os.panelSection}>
                <h3>Note legali</h3>
                <p style={{ whiteSpace: 'pre-wrap', fontSize: '0.8125rem' }}>{selectedRow.note_legali}</p>
              </div>
            )}

            {siblings.length > 1 && (
              <div className={os.panelSection}>
                <h3>Altre righe dell&apos;ordine ({siblings.length - 1})</h3>
                <ul className={os.siblingList}>
                  {siblings.filter(r => r !== selectedRow).map((r, i) => (
                    <li key={i} className={os.siblingItem} onClick={() => setSelectedRow(r)}>
                      <span>{r.descrizione_long}</span>
                      <span className={s.mono}>{r.serialnumber ?? ''}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </SlideOverPanel>
    </div>
  );
}
