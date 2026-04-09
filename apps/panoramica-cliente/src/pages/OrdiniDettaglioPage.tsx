import { useState, useCallback } from 'react';
import { SingleSelect, MultiSelect, SearchInput, useTableFilter } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCustomersWithOrders, useOrderStatuses, useOrdersDetail } from '../api/queries';
import { useCsvExport } from '../hooks/useCsvExport';
import { ServiceUnavailable } from '../components/shared/ServiceUnavailable';
import { SlideOverPanel } from '../components/shared/SlideOverPanel';
import type { OrderDetailRow } from '../types';
import s from './shared.module.css';
import os from './OrdiniDettaglio.module.css';

const defaultStati = ['Evaso', 'Confermato'];

const csvColumns: { key: keyof OrderDetailRow; label: string }[] = [
  { key: 'stato_ordine', label: 'Stato Ordine' },
  { key: 'ordine', label: 'Ordine' },
  { key: 'descrizione_long', label: 'Descrizione' },
  { key: 'tipo_ordine', label: 'Tipo Ordine' },
  { key: 'commerciale', label: 'Commerciale' },
  { key: 'data_ordine', label: 'Data Ordine' },
  { key: 'quantita', label: 'Qta' },
  { key: 'mrc', label: 'MRC' },
  { key: 'stato_riga', label: 'Stato Riga' },
  { key: 'serialnumber', label: 'Serialnumber' },
  { key: 'codice_prodotto', label: 'Codice Prodotto' },
];

function statoBadge(stato: string) {
  const map: Record<string, string | undefined> = {
    'Evaso': s.badgeGreen, 'Confermato': s.badgeYellow, 'Cessato': s.badgeRed,
    'Bloccato': s.badgeRed, 'Attiva': s.badgeGreen, 'Cessata': s.badgeRed,
    'Da attivare': s.badgeYellow, 'Annullata': s.badgeRed, 'Cessazione richiesta': s.badgeYellow,
    'Bloccata': s.badgeRed,
  };
  return <span className={`${s.badge} ${map[stato] ?? s.badgeGray}`}>{stato}</span>;
}

type TabId = 'testata' | 'riga' | 'righe' | 'storico';

export function OrdiniDettaglioPage() {
  const [cliente, setCliente] = useState<number | null>(null);
  const [stati, setStati] = useState<string[]>(defaultStati);
  const [searchTriggered, setSearchTriggered] = useState(false);
  const [search, setSearch] = useState('');
  const [selectedRow, setSelectedRow] = useState<OrderDetailRow | null>(null);
  const [activeTab, setActiveTab] = useState<TabId>('testata');

  const customersQ = useCustomersWithOrders('b');
  const statusesQ = useOrderStatuses();
  const ordersQ = useOrdersDetail(
    searchTriggered ? cliente : null,
    searchTriggered ? stati : [],
  );

  const { filtered } = useTableFilter<OrderDetailRow>({
    data: ordersQ.data,
    searchQuery: search,
    searchFields: ['descrizione_long', 'nome_testata_ordine', 'serialnumber', 'codice_prodotto'],
  });

  const exportCsv = useCsvExport(csvColumns, 'ordini-dettaglio');

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

  // All rows for this order
  const orderRows = selectedRow
    ? filtered.filter(r => r.nome_testata_ordine === selectedRow.nome_testata_ordine)
    : [];

  // Parse storico string into timeline steps
  const storicoSteps = selectedRow?.storico
    ? selectedRow.storico.split('>').map((part: string) => part.trim()).filter(Boolean)
    : null;

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
                  <th>Ordine / Descrizione</th>
                  <th>Tipo ordine</th>
                  <th>Commerciale</th>
                  <th>Data ordine</th>
                  <th className={s.numCol}>Qta</th>
                  <th className={s.numCol}>MRC</th>
                  <th>Stato riga</th>
                  <th>Serialnumber</th>
                  <th>Cod. prodotto</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((row, i) => (
                  <tr
                    key={`${row.nome_testata_ordine}-${row.progressivo_riga}-${i}`}
                    className={selectedRow === row ? os.selected : undefined}
                    onClick={() => { setSelectedRow(row); setActiveTab('testata'); }}
                    style={{ animationDelay: `${Math.min(i * 10, 300)}ms`, cursor: 'pointer' }}
                  >
                    <td>{statoBadge(row.stato_ordine)}</td>
                    <td>
                      {row.ordine && <div style={{ fontWeight: 700, color: 'var(--color-text)' }}>{row.ordine}</div>}
                      <div>{row.descrizione_long ?? row.descrizione_prodotto ?? ''}</div>
                    </td>
                    <td>{row.tipo_ordine ?? ''}</td>
                    <td>{row.commerciale ?? ''}</td>
                    <td>{row.data_ordine?.slice(0, 10) ?? ''}</td>
                    <td className={s.numCol}>{row.quantita}</td>
                    <td className={s.numCol}>{row.mrc.toFixed(2)}</td>
                    <td>{statoBadge(row.stato_riga)}</td>
                    <td className={s.mono}>{row.serialnumber ?? ''}</td>
                    <td className={s.mono}>{row.codice_prodotto ?? ''}</td>
                  </tr>
                ))}
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
        width={600}
        title={selectedRow?.intestazione_ordine ?? selectedRow?.nome_testata_ordine ?? ''}
      >
        {selectedRow && (
          <div>
            <div className={os.tabs}>
              {(['testata', 'riga', 'righe', 'storico'] as const).map(tab => (
                <button
                  key={tab}
                  className={`${os.tab} ${activeTab === tab ? os.tabActive : ''}`}
                  onClick={() => setActiveTab(tab)}
                >
                  {{ testata: 'Testata', riga: 'Riga selezionata', righe: 'Tutte le righe', storico: 'Storico' }[tab]}
                </button>
              ))}
            </div>

            {activeTab === 'testata' && (
              <div className={os.panelContent}>
                <Section title="Anagrafica">
                  <DL>
                    <DI label="Ragione sociale">{selectedRow.ragione_sociale}</DI>
                    <DI label="Stato ordine">{statoBadge(selectedRow.stato_ordine)}</DI>
                    <DI label="Tipo ordine">{selectedRow.tipo_ordine ?? '-'}</DI>
                    <DI label="Commerciale">{selectedRow.commerciale ?? '-'}</DI>
                    <DI label="Data ordine">{selectedRow.data_ordine?.slice(0, 10) ?? '-'}</DI>
                    <DI label="Data documento">{selectedRow.data_documento?.slice(0, 10) ?? '-'}</DI>
                  </DL>
                </Section>
                <Section title="Condizioni">
                  <DL>
                    <DI label="Durata servizio">{selectedRow.durata_servizio ?? '-'}</DI>
                    <DI label="Tacito rinnovo">{selectedRow.tacito_rinnovo ?? '-'}</DI>
                    <DI label="Durata rinnovo">{selectedRow.durata_rinnovo ?? '-'}</DI>
                    <DI label="Metodo pagamento">{selectedRow.metodo_pagamento ?? '-'}</DI>
                    <DI label="Tempi rilascio">{selectedRow.tempi_rilascio ?? '-'}</DI>
                  </DL>
                </Section>
                <Section title="Referente Amm.">
                  <DL>
                    <DI label="Nome">{selectedRow.referente_amm_nome ?? '-'}</DI>
                    <DI label="Email">{selectedRow.referente_amm_mail ?? '-'}</DI>
                    <DI label="Tel">{selectedRow.referente_amm_tel ?? '-'}</DI>
                  </DL>
                </Section>
                <Section title="Referente Tech.">
                  <DL>
                    <DI label="Nome">{selectedRow.referente_tech_nome ?? '-'}</DI>
                    <DI label="Email">{selectedRow.referente_tech_mail ?? '-'}</DI>
                    <DI label="Tel">{selectedRow.referente_tech_tel ?? '-'}</DI>
                  </DL>
                </Section>
                <Section title="Sostituzioni">
                  <DL>
                    <DI label="Sost. ordine">{selectedRow.sost_ord ?? '-'}</DI>
                    <DI label="Sostituito da">{selectedRow.sostituito_da ?? '-'}</DI>
                  </DL>
                </Section>
                {selectedRow.note_legali && (
                  <Section title="Note legali">
                    <p style={{ whiteSpace: 'pre-wrap', fontSize: '0.8125rem' }}>{selectedRow.note_legali}</p>
                  </Section>
                )}
              </div>
            )}

            {activeTab === 'riga' && (
              <div className={os.panelContent}>
                <Section title="Prodotto">
                  <DL>
                    <DI label="Codice prodotto">{selectedRow.codice_prodotto ?? '-'}</DI>
                    <DI label="Codice kit">{selectedRow.codice_kit ?? '-'}</DI>
                    <DI label="Descrizione">{selectedRow.descrizione_prodotto ?? '-'}</DI>
                    <DI label="Desc. estesa">{selectedRow.descrizione_estesa ?? '-'}</DI>
                    <DI label="Famiglia">{selectedRow.famiglia ?? '-'}</DI>
                    <DI label="Sotto famiglia">{selectedRow.sotto_famiglia ?? '-'}</DI>
                    <DI label="Conto ricavo">{selectedRow.conto_ricavo ?? '-'}</DI>
                  </DL>
                </Section>
                <Section title="Importi">
                  <DL>
                    <DI label="Quantita">{selectedRow.quantita}</DI>
                    <DI label="Setup">{selectedRow.setup.toFixed(2)}</DI>
                    <DI label="Canone">{selectedRow.canone.toFixed(2)}</DI>
                    <DI label="MRC">{selectedRow.mrc.toFixed(2)}</DI>
                    <DI label="Costo cessazione">{selectedRow.costo_cessazione.toFixed(2)}</DI>
                    <DI label="Valuta">{selectedRow.valuta ?? '-'}</DI>
                  </DL>
                </Section>
                <Section title="Date">
                  <DL>
                    <DI label="Attivazione">{selectedRow.data_attivazione?.slice(0, 10) ?? '-'}</DI>
                    <DI label="Disdetta">{selectedRow.data_disdetta?.slice(0, 10) ?? '-'}</DI>
                    <DI label="Cessazione">{selectedRow.data_cessazione?.slice(0, 10) ?? '-'}</DI>
                    <DI label="Ultima fatt.">{selectedRow.data_ultima_fatt?.slice(0, 10) ?? '-'}</DI>
                    <DI label="Fine fatt.">{selectedRow.data_fine_fatt?.slice(0, 10) ?? '-'}</DI>
                    <DI label="Scadenza ordine">{selectedRow.data_scadenza_ordine?.slice(0, 10) ?? '-'}</DI>
                  </DL>
                </Section>
                <Section title="Stato">
                  <DL>
                    <DI label="Stato riga">{statoBadge(selectedRow.stato_riga)}</DI>
                    <DI label="Annullato">{selectedRow.annullato ? 'Si' : 'No'}</DI>
                    <DI label="Serialnumber">{selectedRow.serialnumber ?? '-'}</DI>
                  </DL>
                </Section>
              </div>
            )}

            {activeTab === 'righe' && (
              <div className={os.panelContent}>
                <div className={s.info}>{orderRows.length} righe nell&apos;ordine</div>
                <div className={s.tableWrap}>
                  <table className={s.table}>
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>Prodotto</th>
                        <th className={s.numCol}>MRC</th>
                        <th>Stato</th>
                        <th>Serial</th>
                      </tr>
                    </thead>
                    <tbody>
                      {orderRows.map((r, i) => (
                        <tr
                          key={i}
                          className={r === selectedRow ? os.selected : undefined}
                          style={{ cursor: 'pointer' }}
                          onClick={() => { setSelectedRow(r); setActiveTab('riga'); }}
                        >
                          <td>{r.progressivo_riga}</td>
                          <td>{r.descrizione_long ?? r.descrizione_prodotto ?? ''}</td>
                          <td className={s.numCol}>{r.mrc.toFixed(2)}</td>
                          <td>{statoBadge(r.stato_riga)}</td>
                          <td className={s.mono}>{r.serialnumber ?? ''}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {activeTab === 'storico' && (
              <div className={os.panelContent}>
                {storicoSteps && storicoSteps.length > 0 ? (
                  <div className={os.timeline}>
                    {storicoSteps.map((step, i) => (
                      <div key={i} className={os.timelineItem}>
                        <div className={os.timelineDot} />
                        {i < storicoSteps.length - 1 && <div className={os.timelineLine} />}
                        <span className={os.timelineLabel}>{step}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className={s.empty}>Nessuno storico disponibile per questo ordine.</div>
                )}
              </div>
            )}
          </div>
        )}
      </SlideOverPanel>
    </div>
  );
}

// ── Helpers ──

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className={os.section}>
      <h3>{title}</h3>
      {children}
    </div>
  );
}

function DL({ children }: { children: React.ReactNode }) {
  return <dl className={os.detailGrid}>{children}</dl>;
}

function DI({ label, children }: { label: string; children: React.ReactNode }) {
  return <><dt>{label}</dt><dd>{children}</dd></>;
}
