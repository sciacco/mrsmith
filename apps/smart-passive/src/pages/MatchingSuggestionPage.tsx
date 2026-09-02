import { useMemo, useState } from 'react';
import { formatCurrency, formatLocalDate, formatNumber } from '@mrsmith/format';
import { Button, Drawer, Icon, SearchInput, Skeleton, ToggleSwitch, VisuallyHidden } from '@mrsmith/ui';
import { useMatchingSuggestions } from '../api/queries';
import type {
  MatchingCascadeReason,
  MatchingFunnelFilter,
  MatchingFunnelInvoice,
  MatchingFunnelSupplier,
  MatchingSuggestionCheck,
  MatchingSuggestionInvoice,
  MatchingSuggestionLevelKey,
} from '../types';
import base from './MatchingFunnelPage.module.css';
import styles from './MatchingSuggestionPage.module.css';

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

const levelTitles: Record<MatchingSuggestionLevelKey, string> = {
  sdi: 'Riferimento ordine nell’XML',
  contracts: 'Contratti di acquisto',
  orders: 'Riga per riga sugli ordini aperti',
  fixed_fee: 'Canone fisso per ripetizione',
};

const levelShort: Record<MatchingSuggestionLevelKey, string> = {
  sdi: '1 · XML',
  contracts: '2 · Contratti',
  orders: '3 · Ordini',
  fixed_fee: '4 · Canone',
};

const levelNotes: Record<MatchingSuggestionLevelKey, string> = {
  sdi: 'Il codice ordine scritto dal fornitore nella fattura elettronica, risolto sugli ordini Alyante tramite il codice RDA o PA nel numero originale. Verifica se ogni riferimento dichiarato è risolto, nessuno riempie i 20 caratteri del campo e il residuo degli ordini copre l’imponibile. Gli ordini trovati senza queste condizioni restano un suggerimento.',
  contracts: 'I contratti che AFC registra in Alyante per i canoni ricorrenti, uno per contratto con il suo canone. Verifica se un solo contratto, o una sola combinazione, somma esattamente all’imponibile. Esistono da luglio 2026: prima, e per i fornitori non censiti, valgono i livelli seguenti.',
  orders: 'Stesso articolo, stesso importo o prezzo unitario, quantità entro il residuo, fra gli ordini aperti dello stesso fornitore, esclusi gli ordini di beni: sono caricati alla consegna e non confermano nulla. Verifica se una combinazione sola regge; se le righe reggono su più ordini, quegli ordini restano un suggerimento. Oppure una riga con articolo e descrizione presenti in un solo ordine del fornitore (pratica, targa, contratto) nomina quell’ordine, qualunque sia il prezzo: verificato se l’ordine ha ancora residuo per l’imponibile.',
  fixed_fee: 'Serie di fatture dello stesso fornitore con lo stesso imponibile, una al mese per almeno tre mesi. Prende l’ordine collegato da AFC sulla precedente della serie e verifica che abbia ancora residuo per l’imponibile; altrimenti resta un suggerimento.',
};

function outcomeLabel(s: MatchingSuggestionInvoice): string {
  if (s.level === 'residual') return 'Nessun suggerimento';
  return `${s.verified ? 'Verificata' : 'Suggerimento'} · ${levelShort[s.level]}`;
}

function verdictLabel(verdict: MatchingSuggestionInvoice['verdict']): string {
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
    case 'unresolved': return 'Un riferimento non trovato';
    case 'truncated': return 'Riferimento troncato a 20 caratteri';
    case 'not_covered': return 'Ordini in fattura senza residuo sufficiente';
    default: return '—';
  }
}

function contractsReasonLabel(reason: string): string {
  switch (reason) {
    case 'no_contracts': return 'Nessun contratto del fornitore alla data';
    case 'no_match': return 'Nessuna combinazione di canoni torna';
    case 'ambiguous': return 'Più combinazioni tornano';
    default: return '—';
  }
}

function ordersReasonLabel(reason: string): string {
  switch (reason) {
    case 'no_orders': return 'Fornitore senza ordini';
    case 'goods_only': return 'Solo ordini di beni, caricati alla consegna';
    case 'no_open_orders': return 'Nessun ordine aperto';
    case 'no_match': return 'Nessuna riga corrisponde';
    case 'ambiguous': return 'Più ordini possibili';
    case 'not_covered': return 'Ordine per descrizione senza residuo sufficiente';
    default: return '—';
  }
}

function fixedFeeReasonLabel(reason: string): string {
  switch (reason) {
    case 'no_series': return 'Nessuna serie mensile a importo fisso';
    case 'no_anchor': return 'Canone fisso senza ordine collegato';
    case 'not_covered': return 'Ordine della serie senza residuo sufficiente';
    default: return '—';
  }
}

const reasonLabels: Record<MatchingSuggestionLevelKey, (reason: string) => string> = {
  sdi: sdiReasonLabel,
  contracts: contractsReasonLabel,
  orders: ordersReasonLabel,
  fixed_fee: fixedFeeReasonLabel,
};

function checkText(check: MatchingSuggestionCheck): string {
  const name = levelShort[check.level];
  if (check.outcome === 'verified') return `${name}: verificato`;
  const reason = reasonLabels[check.level](check.reason).toLocaleLowerCase('it-IT');
  return check.outcome === 'hint' ? `${name}: suggerimento, ${reason}` : `${name}: ${reason}`;
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

function proposalsText(s: MatchingSuggestionInvoice): string {
  if (s.proposals.length === 0) return '—';
  const shown = s.proposals.slice(0, 4).join(' + ');
  const more = s.proposals.length - 4;
  return more > 0 ? `${shown} e altri ${integer(more)}` : shown;
}

function afcLinksText(invoice: MatchingFunnelInvoice): string {
  return invoice.order_rules.afc_links.length === 0 ? '—' : invoice.order_rules.afc_links.join(' + ');
}

interface SupplierCounts {
  verified: number;
  hint: number;
  residual: number;
  residualLinked: number;
  wrong: number;
}

function supplierCounts(row: MatchingFunnelSupplier): SupplierCounts {
  const counts: SupplierCounts = { verified: 0, hint: 0, residual: 0, residualLinked: 0, wrong: 0 };
  for (const invoice of row.invoices) {
    const s = invoice.suggestion;
    if (s.level === 'residual') {
      counts.residual++;
      if (s.verdict === 'afc_linked') counts.residualLinked++;
    } else if (s.verified) {
      counts.verified++;
    } else {
      counts.hint++;
    }
    if (s.verdict === 'wrong' || s.verdict === 'partial') counts.wrong++;
  }
  return counts;
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

function firstOfYear(): string {
  return `${new Date().getFullYear()}-01-01`;
}

function today(): string {
  return new Date().toISOString().slice(0, 10);
}

export function MatchingSuggestionPage() {
  const [from, setFrom] = useState(firstOfYear);
  const [to, setTo] = useState(today);
  const [openOnly, setOpenOnly] = useState(true);
  const [filter, setFilter] = useState<MatchingFunnelFilter | null>(null);
  const query = useMatchingSuggestions(filter);
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
  const summary = query.data?.suggestions;
  const invalidRange = from !== '' && to !== '' && to < from;

  function load() {
    if (invalidRange) return;
    setActiveSupplierID(null);
    setFilter({ scope: openOnly ? 'open' : 'all', from, to });
  }

  return (
    <section className={base.page}>
      <div className={base.header}>
        <div>
          <p className={base.eyebrow}>Diagnostica</p>
          <h1>Suggerimenti</h1>
          <p className={base.description}>
            Ogni fattura scende per livelli. Un livello la ferma solo se verifica la sua proposta; se trova qualcosa
            senza poterlo verificare lo annota come suggerimento e passa la mano. Alla fine ogni fattura è verificata,
            ha solo un suggerimento, o non ne ha. Le proposte sono confrontate con gli ordini che AFC ha collegato in Alyante.
          </p>
        </div>
      </div>

      <div className={styles.filters}>
        <div className={styles.field}>
          <span className={styles.fieldLabel}>Data documento da</span>
          <div className={styles.dateInputWrap}>
            <input
              type="date"
              className={styles.dateInput}
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              aria-label="Data documento da"
            />
          </div>
        </div>
        <div className={styles.field}>
          <span className={styles.fieldLabel}>Data documento a</span>
          <div className={styles.dateInputWrap}>
            <input
              type="date"
              className={styles.dateInput}
              value={to}
              onChange={(e) => setTo(e.target.value)}
              aria-label="Data documento a"
            />
          </div>
        </div>
        <div className={styles.toggleField}>
          <ToggleSwitch id="suggestion-open-only" checked={openOnly} onChange={setOpenOnly} label="Solo fatture da saldare" />
        </div>
        <div className={styles.filterActions}>
          {invalidRange && <span className={base.codeWarn}>La data finale precede quella iniziale</span>}
          {!invalidRange && query.isSuccess && query.data && (
            <span className={styles.filterHint}>{integer(query.data.suggestions.invoices)} fatture caricate</span>
          )}
          <Button
            variant="primary"
            size="sm"
            onClick={load}
            disabled={invalidRange || query.isFetching}
            leftIcon={<Icon name={query.isFetching ? 'refresh-cw' : 'search'} size={14} />}
          >
            {query.isFetching ? 'Caricamento…' : 'Carica'}
          </Button>
        </div>
      </div>

      {filter === null && !query.isFetching && (
        <div className={base.panel}>
          <p className={styles.empty}>Scegliere l’intervallo di date e premere «Carica».</p>
        </div>
      )}

      {query.isFetching && (
        <div className={base.panel}>
          <Skeleton rows={10} />
        </div>
      )}

      {query.isError && !query.isFetching && (
        <div className={base.error} role="alert">
          <span className={base.errorIcon}><Icon name="triangle-alert" size={20} /></span>
          <div>
            <strong>Impossibile costruire i suggerimenti.</strong>
            <p>Verificare le connessioni Alyante, Arak e Anisetta, quindi riprovare.</p>
          </div>
        </div>
      )}

      {query.isSuccess && !query.isFetching && summary && (
        <>
          <section className={base.summaryPanel}>
            <h2>Esito finale</h2>
            <div className={styles.finalFigures}>
              <div className={styles.figure}>
                <strong>{integer(summary.verified)}</strong>
                <span>verificate su {integer(summary.invoices)} ({percent(summary.verified, summary.invoices)})</span>
              </div>
              <div className={styles.figure}>
                <strong>{integer(summary.hint_only)}</strong>
                <span>con un suggerimento non verificato ({percent(summary.hint_only, summary.invoices)})</span>
              </div>
              <div className={styles.figure}>
                <strong>{integer(summary.residual)}</strong>
                <span>senza suggerimento ({percent(summary.residual, summary.invoices)}), di cui {integer(summary.residual_with_afc_link)} collegate da AFC</span>
              </div>
            </div>
            <table className={base.summaryTable}>
              <thead>
                <tr>
                  <th scope="col">Livello che ha dato l’esito</th>
                  <th scope="col">Verificate</th>
                  <th scope="col">Solo suggerimento</th>
                  <th scope="col">Stessi ordini di AFC</th>
                  <th scope="col">Diverse da AFC</th>
                  <th scope="col">AFC non ha collegato</th>
                </tr>
              </thead>
              <tbody>
                {summary.final.map((row) => (
                  <tr key={row.level}>
                    <th scope="row">{levelTitles[row.level]}</th>
                    <td>{integer(row.verified)}</td>
                    <td>{integer(row.hint)}</td>
                    <td>{integer(row.match)}</td>
                    <td className={row.wrong + row.partial > 0 ? base.codeWarn : undefined}>{integer(row.wrong + row.partial)}</td>
                    <td>{integer(row.no_truth)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p className={styles.levelNote}>
              «Diverse da AFC» conta le proposte che non coincidono con gli ordini collegati da AFC, per difetto o per eccesso: misura l’errore della regola, non è un dato da lavorare. Le fatture senza lavorazione AFC non sono giudicabili.
            </p>
          </section>

          <div className={styles.levels}>
            {summary.levels.map((level, index) => (
              <section key={level.key} className={base.summaryPanel}>
                <div className={styles.levelHead}>
                  <h2>{levelTitles[level.key]}</h2>
                  <span className={styles.levelStep}>Livello {index + 1}</span>
                </div>
                <div className={styles.figure}>
                  <strong>{integer(level.verified)}</strong>
                  <span>verificate su {integer(level.tested)} provate ({percent(level.verified, level.tested)})</span>
                </div>
                <table className={base.summaryTable}>
                  <tbody>
                    <tr><th scope="row">Stessi ordini di AFC</th><td>{integer(level.match)}</td></tr>
                    <tr><th scope="row">Diverse da AFC</th><td className={level.wrong + level.partial > 0 ? base.codeWarn : undefined}>{integer(level.wrong + level.partial)}</td></tr>
                    <tr><th scope="row">AFC non ha collegato</th><td>{integer(level.no_truth)}</td></tr>
                    <tr><th scope="row">Solo suggerimento, passate oltre</th><td>{integer(level.hint)}</td></tr>
                    <tr><th scope="row">Nessun esito, passate oltre</th><td>{integer(level.tested - level.verified - level.hint)}</td></tr>
                  </tbody>
                </table>
                <p className={styles.levelNote}>{levelNotes[level.key]}</p>
              </section>
            ))}
          </div>

          <div className={styles.residualGrid}>
            <ReasonTable title="Senza suggerimento, per famiglia" rows={summary.residual_by_family} label={familyLabel} />
            <ReasonTable title="Senza suggerimento, esito del livello 1" rows={summary.residual_by_sdi} label={sdiReasonLabel} />
            <ReasonTable title="Senza suggerimento, esito del livello 2" rows={summary.residual_by_contracts} label={contractsReasonLabel} />
            <ReasonTable title="Senza suggerimento, esito del livello 3" rows={summary.residual_by_orders} label={ordersReasonLabel} />
            <ReasonTable title="Senza suggerimento, esito del livello 4" rows={summary.residual_by_fixed_fee} label={fixedFeeReasonLabel} />
          </div>

          <section className={base.tablePanel}>
            <div className={base.toolbar}>
              <div>
                <h2>Per fornitore</h2>
                <p>Come finiscono le fatture di ogni fornitore nell’intervallo caricato.</p>
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
                  <VisuallyHidden as="caption">Suggerimenti per fornitore</VisuallyHidden>
                  <thead>
                    <tr>
                      <th>Codice ERP</th>
                      <th>Fornitore</th>
                      <th className={base.numeric}>Fatture</th>
                      <th className={base.numeric}>Verificate</th>
                      <th className={base.numeric}>Solo suggerimento</th>
                      <th className={base.numeric}>Senza suggerimento</th>
                      <th className={base.numeric}>di cui collegate da AFC</th>
                      <th className={base.numeric}>Diverse da AFC</th>
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
                        <td className={base.numeric}>{integer(counts.verified)}</td>
                        <td className={base.numeric}>{integer(counts.hint)}</td>
                        <td className={base.numeric}>{integer(counts.residual)}</td>
                        <td className={base.numeric}>{integer(counts.residualLinked)}</td>
                        <td className={`${base.numeric} ${counts.wrong > 0 ? base.codeWarn : ''}`}>{integer(counts.wrong)}</td>
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
                          <th>Esito</th>
                          <th>Ordini o contratti proposti</th>
                          <th>Ordini collegati da AFC</th>
                          <th>Confronto</th>
                          <th>Livelli provati</th>
                          <th>Famiglia</th>
                        </tr>
                      </thead>
                      <tbody>
                        {activeSupplier.invoices.map((invoice) => {
                          const s = invoice.suggestion;
                          const warn = s.verdict === 'wrong' || s.verdict === 'partial' || s.verdict === 'afc_linked';
                          return (
                            <tr key={invoice.registration}>
                              <td>{date(invoice.document_date)}</td>
                              <td>{invoice.document_number || '—'}</td>
                              <td className={base.numeric}>{money(invoice.taxable_amount)}</td>
                              <td>
                                <span className={`${styles.outcomeTag} ${s.verified ? styles.outcomeVerified : ''}`}>{outcomeLabel(s)}</span>
                              </td>
                              <td className={base.codesCell}>{proposalsText(s)}</td>
                              <td className={base.codesCell}>{afcLinksText(invoice)}</td>
                              <td className={warn ? base.codeWarn : undefined}>{verdictLabel(s.verdict)}</td>
                              <td>
                                <ul className={styles.checkList}>
                                  {s.checks.map((check) => (
                                    <li key={check.level}>{checkText(check)}</li>
                                  ))}
                                </ul>
                              </td>
                              <td>{familyLabel(s.family)}</td>
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
