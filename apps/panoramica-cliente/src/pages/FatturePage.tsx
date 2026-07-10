import { useEffect, useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, SearchInput, SingleSelect, Skeleton } from '@mrsmith/ui';
import { downloadInvoicesExcel, useCustomersWithInvoices, useInvoices, type InvoiceQueryParams } from '../api/queries';
import { useApiClient } from '../api/client';
import type { InvoiceDocument } from '../types';
import s from './FatturePage.module.css';

const PAGE_SIZE = 250;
const periodOptions = [
  { value: 0, label: 'Tutti' },
  { value: 6, label: 'Ultimi 6 mesi' },
  { value: 12, label: 'Ultimi 12 mesi' },
  { value: 24, label: 'Ultimi 24 mesi' },
  { value: 36, label: 'Ultimi 36 mesi' },
];

const money = new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR' });
const number = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 2 });
const date = new Intl.DateTimeFormat('it-IT');

function formatDate(value: string | null) {
  if (!value) return '—';
  const parsed = new Date(`${value}T00:00:00`);
  return Number.isNaN(parsed.getTime()) ? value : date.format(parsed);
}

function formatMoney(value: number | null) {
  return value === null ? '—' : money.format(value);
}

function InvoiceCard({ document }: { document: InvoiceDocument }) {
  const [expanded, setExpanded] = useState(false);
  const panelId = `invoice-${document.id.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
  return (
    <article className={s.documentCard}>
      <button className={s.documentHeader} type="button" onClick={() => setExpanded(v => !v)} aria-expanded={expanded} aria-controls={panelId}>
        <span className={s.documentIdentity}>
          <span className={document.segno < 0 ? s.creditBadge : s.invoiceBadge}>{document.doc || 'Documento'}</span>
          <strong>{document.num_documento}</strong>
          <span>{formatDate(document.data_documento)}</span>
        </span>
        <span className={s.documentSummary}>
          <span>{document.line_count} {document.line_count === 1 ? 'riga' : 'righe'}</span>
          <strong>{formatMoney(document.totale_netto)}</strong>
          <Icon name={expanded ? 'chevron-up' : 'chevron-down'} size={18} aria-hidden="true" />
        </span>
      </button>
      {expanded && (
        <div id={panelId} className={s.lineTableWrap}>
          <table className={s.lineTable}>
            <caption className={s.srOnly}>Righe del documento {document.doc} {document.num_documento}</caption>
            <thead><tr><th>Descrizione</th><th>Articolo</th><th>Serial number</th><th className={s.numeric}>Quantità</th><th className={s.numeric}>Prezzo unitario</th><th className={s.numeric}>Totale riga</th><th>Conto ricavo</th></tr></thead>
            <tbody>{document.lines.map(line => (
              <tr key={line.progressivo_riga}>
                <td><span className={s.mobileLabel}>Descrizione</span>{line.descrizione_riga || '—'}</td>
                <td className={s.mono}><span className={s.mobileLabel}>Articolo</span>{line.codice_articolo || '—'}</td>
                <td className={s.mono}><span className={s.mobileLabel}>Serial number</span>{line.serialnumber || '—'}</td>
                <td className={s.numeric}><span className={s.mobileLabel}>Quantità</span>{line.qta === null ? '—' : number.format(line.qta)}</td>
                <td className={s.numeric}><span className={s.mobileLabel}>Prezzo unitario</span>{formatMoney(line.prezzo_unitario)}</td>
                <td className={s.numeric}><span className={s.mobileLabel}>Totale riga</span>{formatMoney(line.prezzo_totale_netto)}</td>
                <td><span className={s.mobileLabel}>Conto ricavo</span>{line.desc_conto_ricavo || '—'}</td>
              </tr>
            ))}</tbody>
          </table>
        </div>
      )}
    </article>
  );
}

export function FatturePage() {
  const api = useApiClient();
  const [cliente, setCliente] = useState<number | null>(null);
  const [mesi, setMesi] = useState(0);
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [sort, setSort] = useState<InvoiceQueryParams['sort']>('data_documento');
  const [dir, setDir] = useState<InvoiceQueryParams['dir']>('desc');
  const [page, setPage] = useState(1);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState(false);

  useEffect(() => { const id = window.setTimeout(() => setDebouncedSearch(search), 300); return () => window.clearTimeout(id); }, [search]);
  useEffect(() => setPage(1), [cliente, mesi, debouncedSearch, sort, dir]);

  const customersQ = useCustomersWithInvoices();
  const params = useMemo<InvoiceQueryParams>(() => ({ cliente, mesi: mesi || null, q: debouncedSearch, sort, dir, page, pageSize: PAGE_SIZE }), [cliente, mesi, debouncedSearch, sort, dir, page]);
  const invoicesQ = useInvoices(params);
  const totalPages = Math.max(1, Math.ceil((invoicesQ.data?.total_documents ?? 0) / PAGE_SIZE));
  const customerOptions = (customersQ.data ?? []).map(c => ({ value: c.numero_azienda, label: c.ragione_sociale }));

  async function exportExcel() {
    setExporting(true); setExportError(false);
    try { await downloadInvoicesExcel(api, params); } catch { setExportError(true); } finally { setExporting(false); }
  }

  if (customersQ.error && (customersQ.error as ApiError).status === 503) {
    return <div className={s.state}><h2>Servizio non disponibile</h2><p>Le fatture non sono disponibili. Riprova più tardi.</p></div>;
  }

  return (
    <div className={s.page}>
      <header className={s.pageHeader}><div><h1>Fatture</h1><p>Consulta i documenti e le relative righe di dettaglio.</p></div></header>
      <section className={s.filters} aria-label="Filtri fatture">
        <div className={s.customerField}><label>Cliente</label><SingleSelect options={customerOptions} selected={cliente} onChange={setCliente} placeholder="Seleziona cliente…" /></div>
        <div className={s.field}><label htmlFor="invoice-period">Periodo</label><select id="invoice-period" value={mesi} onChange={e => setMesi(Number(e.target.value))}>{periodOptions.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}</select></div>
        <div className={s.searchField}><label>Ricerca</label><SearchInput value={search} onChange={setSearch} placeholder="Numero, articolo, serial number…" /></div>
        <div className={s.field}><label htmlFor="invoice-sort">Ordina per</label><select id="invoice-sort" value={`${sort}:${dir}`} onChange={e => { const [nextSort, nextDir] = e.target.value.split(':') as [InvoiceQueryParams['sort'], InvoiceQueryParams['dir']]; setSort(nextSort); setDir(nextDir); }}><option value="data_documento:desc">Data, più recenti</option><option value="data_documento:asc">Data, meno recenti</option><option value="documento:asc">Numero documento</option><option value="totale_netto:desc">Importo, più alto</option><option value="totale_netto:asc">Importo, più basso</option></select></div>
        <Button variant="secondary" leftIcon={<Icon name="download" size={16} />} onClick={exportExcel} loading={exporting} disabled={!cliente || !invoicesQ.data?.total_documents}>Esporta Excel</Button>
      </section>

      {exportError && <div className={s.error} role="alert">Esportazione non riuscita. Verifica la connessione e riprova.</div>}
      {!cliente && <div className={s.state}><h2>Seleziona un cliente</h2><p>Scegli un cliente per visualizzare le fatture disponibili.</p></div>}
      {cliente && invoicesQ.isLoading && <div className={s.loading} aria-label="Caricamento fatture"><Skeleton rows={6} /></div>}
      {cliente && invoicesQ.error && <div className={s.state} role="alert"><h2>Impossibile caricare le fatture</h2><p>Verifica la connessione e riprova.</p><Button variant="secondary" onClick={() => invoicesQ.refetch()}>Riprova</Button></div>}
      {cliente && invoicesQ.data && !invoicesQ.error && invoicesQ.data.items.length === 0 && <div className={s.state}><h2>{debouncedSearch ? 'Nessuna corrispondenza' : 'Nessuna fattura disponibile'}</h2><p>{debouncedSearch ? 'Modifica o cancella i termini di ricerca.' : 'Non risultano documenti per il cliente e il periodo selezionati.'}</p>{debouncedSearch && <Button variant="secondary" onClick={() => setSearch('')}>Cancella ricerca</Button>}</div>}
      {cliente && invoicesQ.data && invoicesQ.data.items.length > 0 && <>
        <div className={s.documents} aria-busy={invoicesQ.isFetching}>{invoicesQ.data.items.map(document => <InvoiceCard key={document.id} document={document} />)}</div>
        {totalPages > 1 && <nav className={s.pagination} aria-label="Paginazione fatture"><span>Pagina {page} di {totalPages}</span><div><Button variant="secondary" size="sm" leftIcon={<Icon name="chevron-left" size={16} />} disabled={page <= 1 || invoicesQ.isFetching} onClick={() => setPage(p => p - 1)}>Precedente</Button><Button variant="secondary" size="sm" rightIcon={<Icon name="chevron-right" size={16} />} disabled={page >= totalPages || invoicesQ.isFetching} onClick={() => setPage(p => p + 1)}>Successiva</Button></div></nav>}
      </>}
    </div>
  );
}
