import { useMemo, useState } from 'react';
import { formatCurrency, formatLocalDate, formatNumber } from '@mrsmith/format';
import { Button, Drawer, Icon, SearchInput, Skeleton, TabNav, VisuallyHidden } from '@mrsmith/ui';
import { useMatchingFunnel } from '../api/queries';
import type {
  MatchingCascadeInvoice,
  MatchingCascadeLevel,
  MatchingCascadeReason,
  MatchingFunnelInvoice,
  MatchingFunnelScope,
  MatchingFunnelSupplier,
} from '../types';
import base from './MatchingFunnelPage.module.css';
import styles from './MatchingCascadePage.module.css';

function integer(value: number): string {
  return formatNumber(value, { format: { maximumFractionDigits: 0 } }) ?? String(value);
}

function percent(part: number, total: number): string {
  if (total === 0) return '—';
  return `${Math.round((part / total) * 100)}%`;
}

function date(value: string | null): string {
  return formatLocalDate(value) ?? '—';
}

function money(value: number | null, currency = 'EUR'): string {
  return formatCurrency(value, currency) ?? '—';
}

const levelTitles: Record<MatchingCascadeLevel['key'], string> = {
  sdi: 'Riferimento ordine nell’XML',
  orders: 'Riga per riga sugli ordini aperti',
  fixed_fee: 'Canone fisso per ripetizione',
};

const levelNotes: Record<MatchingCascadeLevel['key'], string> = {
  sdi: 'Il codice ordine scritto dal fornitore nella fattura elettronica, risolto sugli ordini Alyante tramite il codice RDA o PA nel numero originale.',
  orders: 'Stesso articolo, stesso importo o prezzo unitario, quantità entro il residuo, fra gli ordini aperti dello stesso fornitore. Chiude solo se una combinazione sola regge.',
  fixed_fee: 'Serie di fatture dello stesso fornitore con lo stesso imponibile, una al mese per almeno tre mesi. La fattura prende l’ordine collegato da AFC sulla precedente della serie. La serie è letta su tutte le fatture 2026 del fornitore, saldate comprese.',
};

function levelLabel(level: MatchingCascadeInvoice['level']): string {
  if (level === 'sdi') return '1 · XML';
  if (level === 'orders') return '2 · Ordini';
  if (level === 'fixed_fee') return '3 · Canone';
  return 'Residuo';
}

function verdictLabel(verdict: MatchingCascadeInvoice['verdict']): string {
  switch (verdict) {
    case 'match': return 'Stessi ordini di AFC';
    case 'partial': return 'Parte degli ordini di AFC';
    case 'wrong': return 'Ordini diversi da AFC';
    case 'no_truth': return 'AFC non ha collegato';
    case 'afc_linked': return 'AFC ha collegato';
    case 'afc_unlinked': return 'AFC non ha collegato';
  }
}

function sdiReasonLabel(reason: string): string {
  switch (reason) {
    case 'no_xml': return 'Nessun XML agganciato';
    case 'no_ref': return 'XML senza riferimento ordine';
    case 'no_code': return 'Riferimento senza codice PO o PA';
    case 'unresolved': return 'Codice non trovato';
    default: return '—';
  }
}

function ordersReasonLabel(reason: string): string {
  switch (reason) {
    case 'no_orders': return 'Fornitore senza ordini';
    case 'no_open_orders': return 'Nessun ordine aperto';
    case 'no_match': return 'Nessuna riga corrisponde';
    case 'ambiguous': return 'Più ordini possibili';
    default: return '—';
  }
}

function fixedFeeReasonLabel(reason: string): string {
  switch (reason) {
    case 'no_series': return 'Nessuna serie mensile a importo fisso';
    case 'no_anchor': return 'Canone fisso senza ordine collegato';
    default: return '—';
  }
}

function seriesText(cascade: MatchingCascadeInvoice): string {
  return cascade.series_size > 0 ? `${integer(cascade.series_size)} fatture` : '—';
}

function familyLabel(family: string): string {
  switch (family) {
    case 'recurring_in_course': return 'Contratto ricorrente in corso';
    case 'goods_orders': return 'Ordini di beni mai collegati';
    case 'service_orders_expired': return 'Ordini a servizi, nessuno in corso';
    case 'rda_only': return 'Solo RDA, senza ordine';
    case 'unknown': return 'Fornitore senza ordini né RDA';
    default: return '—';
  }
}

function pairLabel(reason: string): string {
  const [sdi = '', orders = ''] = reason.split(' / ');
  return `${sdiReasonLabel(sdi)} · ${ordersReasonLabel(orders)}`;
}

function residualReason(cascade: MatchingCascadeInvoice): string {
  if (cascade.level !== 'residual') return '—';
  return `${sdiReasonLabel(cascade.sdi_reason)} · ${ordersReasonLabel(cascade.orders_reason)} · ${fixedFeeReasonLabel(cascade.fixed_fee_reason)}`;
}

function proposalsText(cascade: MatchingCascadeInvoice): string {
  if (cascade.proposals.length === 0) return '—';
  const shown = cascade.proposals.slice(0, 4).join(' + ');
  const more = cascade.proposals.length - 4;
  return more > 0 ? `${shown} e altri ${integer(more)}` : shown;
}

function afcLinksText(invoice: MatchingFunnelInvoice): string {
  return invoice.order_rules.afc_links.length === 0 ? '—' : invoice.order_rules.afc_links.join(' + ');
}

interface SupplierCounts {
  sdi: number;
  orders: number;
  fixedFee: number;
  residual: number;
  residualLinked: number;
  wrong: number;
}

function supplierCounts(row: MatchingFunnelSupplier): SupplierCounts {
  const counts: SupplierCounts = { sdi: 0, orders: 0, fixedFee: 0, residual: 0, residualLinked: 0, wrong: 0 };
  for (const invoice of row.invoices) {
    const c = invoice.cascade;
    if (c.level === 'sdi') counts.sdi++;
    else if (c.level === 'orders') counts.orders++;
    else if (c.level === 'fixed_fee') counts.fixedFee++;
    else {
      counts.residual++;
      if (c.verdict === 'afc_linked') counts.residualLinked++;
    }
    if (c.verdict === 'wrong' || c.verdict === 'partial') counts.wrong++;
  }
  return counts;
}

function decimal(value: number): string {
  return formatNumber(value, { format: { maximumFractionDigits: 1 } }) ?? String(value);
}

function share(value: number): string {
  return `${Math.round(value * 100)}%`;
}

function supplierLabel(row: MatchingFunnelSupplier): string {
  return row.alyante_supplier_name ?? row.provider_name ?? '—';
}

function ReasonTable({ title, rows, label }: { title: string; rows: MatchingCascadeReason[]; label: (reason: string) => string }) {
  return (
    <section className={base.summaryPanel}>
      <h2>{title}</h2>
      <table className={base.summaryTable}>
        <thead>
          <tr><th scope="col">Motivo</th><th scope="col">Fatture</th><th scope="col">Con ordine AFC</th></tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.reason}>
              <th scope="row">{label(row.reason)}</th>
              <td>{integer(row.count)}</td>
              <td>{integer(row.with_afc_link)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

const scopeTabs = [
  { key: 'open', label: 'Fatture da saldare' },
  { key: 'all', label: 'Tutte le fatture 2026' },
];

export function MatchingCascadePage() {
  const [scope, setScope] = useState<MatchingFunnelScope>('open');
  const query = useMatchingFunnel(scope);
  const [search, setSearch] = useState('');
  const [activeSupplierID, setActiveSupplierID] = useState<number | 'missing' | null>(null);
  const suppliers = query.data?.suppliers ?? [];
  const activeSupplier = suppliers.find((row) => (row.supplier_erp_id ?? 'missing') === activeSupplierID) ?? null;
  const rows = useMemo(() => suppliers.map((row) => ({ row, counts: supplierCounts(row) })), [suppliers]);
  const filteredRows = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase('it-IT');
    if (!needle) return rows;
    return rows.filter(({ row }) => {
      const values = [row.supplier_erp_id === null ? '' : String(row.supplier_erp_id), row.alyante_supplier_name ?? '', row.provider_name ?? ''];
      return values.some((value) => value.toLocaleLowerCase('it-IT').includes(needle));
    });
  }, [rows, search]);
  const cascade = query.data?.cascade;

  return (
    <section className={base.page}>
      <div className={base.header}>
        <div>
          <p className={base.eyebrow}>Diagnostica</p>
          <h1>Abbinamento a livelli</h1>
          <p className={base.description}>
            Ogni fattura entra in cima e scende per livelli: ogni livello prova a chiuderla e, se non ci riesce,
            la passa al successivo. Le fatture chiuse sono confrontate con gli ordini che AFC ha collegato in Alyante.
            In fondo resta il residuo, spezzato per motivo.
          </p>
        </div>
        <div className={base.headerActions}>
          <TabNav
            items={scopeTabs}
            activeKey={scope}
            onTabChange={(key) => {
              setScope(key as MatchingFunnelScope);
              setActiveSupplierID(null);
            }}
          />
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void query.refetch()}
            disabled={query.isFetching}
            leftIcon={<Icon name="refresh-cw" size={14} />}
          >
            Aggiorna
          </Button>
        </div>
      </div>

      {query.isLoading && (
        <div className={base.panel}>
          <Skeleton rows={10} />
        </div>
      )}

      {query.isError && (
        <div className={base.error} role="alert">
          <span className={base.errorIcon}><Icon name="triangle-alert" size={20} /></span>
          <div>
            <strong>Impossibile costruire l’abbinamento a livelli.</strong>
            <p>Verificare le connessioni Alyante, Arak e Anisetta, quindi riprovare.</p>
          </div>
        </div>
      )}

      {query.isSuccess && cascade && (
        <>
          <div className={styles.levels}>
            {cascade.levels.map((level, index) => (
              <section key={level.key} className={`${base.summaryPanel} ${styles.level}`}>
                <div className={styles.levelHead}>
                  <h2>{levelTitles[level.key]}</h2>
                  <span className={styles.levelStep}>Livello {index + 1}</span>
                </div>
                <div className={styles.levelFigure}>
                  <strong>{integer(level.closed)}</strong>
                  <span>chiuse su {integer(level.entered)} in ingresso ({percent(level.closed, level.entered)})</span>
                </div>
                <table className={base.summaryTable}>
                  <tbody>
                    <tr><th scope="row">Stessi ordini di AFC</th><td>{integer(level.match)}</td></tr>
                    <tr><th scope="row">Parte degli ordini di AFC</th><td>{integer(level.partial)}</td></tr>
                    <tr><th scope="row">Ordini diversi da AFC</th><td className={level.wrong > 0 ? base.codeWarn : undefined}>{integer(level.wrong)}</td></tr>
                    <tr><th scope="row">AFC non ha collegato</th><td>{integer(level.no_truth)}</td></tr>
                    <tr><th scope="row">Passate al livello successivo</th><td>{integer(level.passed)}</td></tr>
                  </tbody>
                </table>
                <p className={styles.levelNote}>{levelNotes[level.key]}</p>
              </section>
            ))}
            <section className={`${base.summaryPanel} ${styles.level}`}>
              <div className={styles.levelHead}>
                <h2>Residuo</h2>
                <span className={styles.levelStep}>Fine</span>
              </div>
              <div className={styles.levelFigure}>
                <strong>{integer(cascade.residual)}</strong>
                <span>fatture su {integer(cascade.invoices)} ({percent(cascade.residual, cascade.invoices)})</span>
              </div>
              <table className={base.summaryTable}>
                <tbody>
                  <tr><th scope="row">AFC ha collegato un ordine</th><td>{integer(cascade.residual_with_afc_link)}</td></tr>
                  <tr><th scope="row">AFC non ha collegato</th><td>{integer(cascade.residual - cascade.residual_with_afc_link)}</td></tr>
                </tbody>
              </table>
              <p className={styles.levelNote}>
                Le fatture collegate da AFC sono quelle che i livelli avrebbero dovuto chiudere. Per le altre il collegamento manca anche in Alyante: da classificare a loro volta, non da dare per «senza ordine».
              </p>
            </section>
          </div>

          <div className={styles.residualGrid}>
            <ReasonTable title="Residuo per famiglia" rows={cascade.residual_by_family} label={familyLabel} />
            <ReasonTable title="Residuo per esito del livello 1" rows={cascade.residual_by_sdi} label={sdiReasonLabel} />
            <ReasonTable title="Residuo per esito del livello 2" rows={cascade.residual_by_orders} label={ordersReasonLabel} />
            <ReasonTable title="Residuo per esito del livello 3" rows={cascade.residual_by_fixed_fee} label={fixedFeeReasonLabel} />
            <ReasonTable title="Residuo per coppia di esiti" rows={cascade.residual_by_pair} label={pairLabel} />
          </div>

          <section className={base.tablePanel}>
            <div className={base.toolbar}>
              <div>
                <h2>Per fornitore</h2>
                <p>Dove si fermano le fatture di ogni fornitore, e come il fornitore fattura: XML ricevuti al mese, righe per XML, quota di XML con lo stesso imponibile di un altro, quota con periodo di competenza.</p>
              </div>
              <SearchInput
                value={search}
                onChange={setSearch}
                placeholder="Cerca codice o fornitore…"
                ariaLabel="Filtra per fornitore"
                className={base.search}
              />
            </div>

            {filteredRows.length === 0 ? (
              <div className={base.filteredEmpty}>
                <strong>Nessun fornitore corrisponde alla ricerca</strong>
                <Button variant="secondary" size="sm" onClick={() => setSearch('')}>Cancella ricerca</Button>
              </div>
            ) : (
              <div className={base.tableWrap}>
                <table className={base.table}>
                  <VisuallyHidden as="caption">Abbinamento a livelli per fornitore</VisuallyHidden>
                  <thead>
                    <tr>
                      <th>Codice ERP</th>
                      <th>Fornitore</th>
                      <th className={base.numeric}>Fatture</th>
                      <th className={base.numeric}>Chiuse al livello 1</th>
                      <th className={base.numeric}>Chiuse al livello 2</th>
                      <th className={base.numeric}>Chiuse al livello 3</th>
                      <th className={base.numeric}>Residuo</th>
                      <th className={base.numeric}>di cui collegate da AFC</th>
                      <th className={base.numeric}>Diverse da AFC</th>
                      <th className={base.numeric}>XML al mese</th>
                      <th className={base.numeric}>Righe per XML</th>
                      <th className={base.numeric}>Stesso imponibile</th>
                      <th className={base.numeric}>Con periodo</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredRows.map(({ row, counts }) => (
                      <tr key={row.supplier_erp_id ?? 'missing'}>
                        <td>
                          <button
                            type="button"
                            className={base.detailButton}
                            onClick={() => setActiveSupplierID(row.supplier_erp_id ?? 'missing')}
                            aria-haspopup="dialog"
                          >
                            <Icon name="chevron-right" size={14} />
                            {row.supplier_erp_id === null ? '—' : integer(row.supplier_erp_id)}
                          </button>
                        </td>
                        <td>{supplierLabel(row)}</td>
                        <td className={base.numeric}>{integer(row.invoice_count)}</td>
                        <td className={base.numeric}>{integer(counts.sdi)}</td>
                        <td className={base.numeric}>{integer(counts.orders)}</td>
                        <td className={base.numeric}>{integer(counts.fixedFee)}</td>
                        <td className={base.numeric}>{integer(counts.residual)}</td>
                        <td className={base.numeric}>{integer(counts.residualLinked)}</td>
                        <td className={`${base.numeric} ${counts.wrong > 0 ? base.codeWarn : ''}`}>{integer(counts.wrong)}</td>
                        <td className={base.numeric}>{row.billing ? decimal(row.billing.per_month) : '—'}</td>
                        <td className={base.numeric}>{row.billing ? decimal(row.billing.median_lines) : '—'}</td>
                        <td className={base.numeric}>{row.billing ? share(row.billing.repeat_share) : '—'}</td>
                        <td className={base.numeric}>{row.billing ? share(row.billing.period_share) : '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          {activeSupplier && (
            <Drawer
              open
              onClose={() => setActiveSupplierID(null)}
              size="xl"
              title={activeSupplier.supplier_erp_id === null
                ? 'Fatture senza codice fornitore'
                : `Fornitore ERP ${integer(activeSupplier.supplier_erp_id)}`}
              subtitle={supplierLabel(activeSupplier)}
            >
              <div className={base.drawerBody}>
                <section className={base.detailSection}>
                  <div className={base.detailHeading}>
                    <h3>Fatture</h3>
                    <span>{integer(activeSupplier.invoices.length)}</span>
                  </div>
                  <div className={base.detailTableWrap}>
                    <table className={base.detailTable}>
                      <thead>
                        <tr>
                          <th>Data</th>
                          <th>Numero</th>
                          <th className={base.numeric}>Imponibile</th>
                          <th>Fermata a</th>
                          <th>Serie</th>
                          <th>Ordini proposti</th>
                          <th>Ordini collegati da AFC</th>
                          <th>Confronto</th>
                          <th>Motivo del residuo</th>
                          <th>Famiglia</th>
                        </tr>
                      </thead>
                      <tbody>
                        {activeSupplier.invoices.map((invoice) => {
                          const c = invoice.cascade;
                          const warn = c.verdict === 'wrong' || c.verdict === 'partial' || c.verdict === 'afc_linked';
                          return (
                            <tr key={invoice.registration}>
                              <td>{date(invoice.document_date)}</td>
                              <td>{invoice.document_number || '—'}</td>
                              <td className={base.numeric}>{money(invoice.taxable_amount)}</td>
                              <td><span className={styles.levelTag}>{levelLabel(c.level)}</span></td>
                              <td>{seriesText(c)}</td>
                              <td className={base.codesCell}>{proposalsText(c)}</td>
                              <td className={base.codesCell}>{afcLinksText(invoice)}</td>
                              <td className={warn ? base.codeWarn : undefined}>{verdictLabel(c.verdict)}</td>
                              <td>{residualReason(c)}</td>
                              <td>{familyLabel(c.family)}</td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                </section>
              </div>
            </Drawer>
          )}
        </>
      )}
    </section>
  );
}
