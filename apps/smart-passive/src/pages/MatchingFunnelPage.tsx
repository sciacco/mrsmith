import { useMemo, useState } from 'react';
import { formatCurrency, formatInstant, formatLocalDate, formatNumber } from '@mrsmith/format';
import { Button, Drawer, Icon, SearchInput, Skeleton, TabNav, VisuallyHidden } from '@mrsmith/ui';
import { useMatchingFunnel, useSDIImportStatus } from '../api/queries';
import type { MatchingFunnelInvoice, MatchingFunnelReferencedRDA, MatchingFunnelScope, MatchingFunnelSupplier, SDIImportStatus } from '../types';
import styles from './MatchingFunnelPage.module.css';

function integer(value: number): string {
  return formatNumber(value, { format: { maximumFractionDigits: 0 } }) ?? String(value);
}

function date(value: string | null): string {
  return formatLocalDate(value) ?? '—';
}

function money(value: number | null, currency = 'EUR'): string {
  return formatCurrency(value, currency) ?? '—';
}

function profileLabel(profile: string): string {
  if (profile === 'one_time') return 'Una tantum';
  if (profile === 'recurring') return 'Ricorrente';
  if (profile === 'mixed') return 'Mista';
  return 'Non classificata';
}

function outcomeLabel(outcome: MatchingFunnelSupplier['outcome']): string {
  if (outcome === 'none') return 'Nessuna candidata';
  if (outcome === 'one') return 'Una candidata';
  return 'Più candidate';
}

function referenceOutcomeLabel(outcome: MatchingFunnelInvoice['reference']['outcome']): string {
  switch (outcome) {
    case 'no_note': return 'Nessuna nota';
    case 'no_code': return 'Nota senza codice';
    case 'no_rda_declared': return 'Nessuna RDA dichiarata';
    case 'legacy_only': return 'Legacy senza successore Arak';
    case 'unresolved': return 'Codice non in Arak';
    case 'ambiguous': return 'Codice ambiguo';
    case 'one': return 'Una RDA';
    case 'multiple': return 'Più RDA';
  }
}

function referencedRDALabel(rda: MatchingFunnelReferencedRDA): string {
  if (rda.resolution === 'unresolved') return `${rda.code} (non in Arak)`;
  if (rda.resolution === 'ambiguous') return `${rda.code} (ambiguo)`;
  const base = rda.resolution === 'successor' ? `${rda.code} (da ${rda.legacy_code})` : rda.code;
  if (rda.supplier_match === false) {
    const erp = rda.supplier_erp_id === null ? 'senza codice ERP' : `ERP ${integer(rda.supplier_erp_id)}`;
    return `${base}, fornitore ${erp}`;
  }
  return base;
}

function ruleLabel(rule: MatchingFunnelInvoice['rules']['rule']): string {
  if (rule === 'full') return 'Importo intero';
  if (rule === 'installment') return 'Rata';
  if (rule === 'sum') return 'Somma di RDA';
  return '—';
}

function verdictLabel(verdict: MatchingFunnelInvoice['rules']['verdict']): string {
  switch (verdict) {
    case 'match': return 'Stessa RDA di AFC';
    case 'ambiguous': return 'Più proposte, una giusta';
    case 'wrong': return 'RDA diversa da AFC';
    case 'none': return 'Nessuna proposta';
    case 'false_positive': return 'Proposta su fattura senza RDA';
    case 'silent_ok': return 'Nessuna proposta, corretto';
    case 'no_truth': return 'Senza riscontro AFC';
  }
}

function proposalsText(rules: MatchingFunnelInvoice['rules']): string {
  if (rules.proposals.length === 0) {
    return rules.near_miss === null ? '—' : `Nessuna al centesimo, la più vicina a ${(rules.near_miss * 100).toFixed(1)}%`;
  }
  const shown = rules.proposals.slice(0, 3).map((proposal) => proposal.codes.join(' + '));
  const rest = rules.proposals.length - shown.length;
  return rest > 0 ? `${shown.join(' · ')} e altre ${integer(rest)}` : shown.join(' · ');
}

function orderRuleLabel(rule: MatchingFunnelInvoice['order_rules']['rule']): string {
  if (rule === 'header') return 'Totale ordine';
  if (rule === 'lines') return 'Riga per riga';
  return '—';
}

function orderVerdictLabel(verdict: MatchingFunnelInvoice['order_rules']['verdict']): string {
  switch (verdict) {
    case 'match': return 'Stesso ordine di AFC';
    case 'ambiguous': return 'Più ordini, tra cui quelli di AFC';
    case 'wrong': return 'Ordine diverso da AFC';
    case 'none': return 'Nessuna proposta';
    case 'unlinked_proposal': return 'Proposta, AFC non ha collegato';
    case 'unlinked_silent': return 'Nessun collegamento AFC';
  }
}

function orderProposalsText(rules: MatchingFunnelInvoice['order_rules']): string {
  if (rules.proposals.length === 0) {
    return rules.lines_total > 0 ? `— (righe trovate ${integer(rules.lines_matched)} su ${integer(rules.lines_total)})` : '—';
  }
  const shown = rules.proposals.slice(0, 2).map((proposal) => {
    const labels = proposal.slice(0, 4).join(' + ');
    const more = proposal.length - 4;
    return more > 0 ? `${labels} e altri ${integer(more)} ordini` : labels;
  });
  const rest = rules.proposals.length - shown.length;
  const text = rest > 0 ? `${shown.join(' · ')} e altre ${integer(rest)}` : shown.join(' · ');
  if (rules.ambiguous_lines === 0) return text;
  const lines = rules.ambiguous_lines === 1 ? '1 riga su più ordini' : `${integer(rules.ambiguous_lines)} righe su più ordini`;
  return `${text} (${lines})`;
}

function sdiVerdictLabel(verdict: MatchingFunnelInvoice['sdi']['verdict']): string {
  switch (verdict) {
    case 'match': return 'Stessi ordini di AFC';
    case 'partial': return 'Parte degli ordini di AFC';
    case 'wrong': return 'Ordini diversi da AFC';
    case 'none': return 'Riferimento non risolto';
    case 'no_truth': return 'Senza collegamento AFC';
  }
}

function sdiRefsText(sdi: MatchingFunnelInvoice['sdi']): string {
  if (!sdi.linked) return 'XML non trovato';
  if (sdi.order_refs.length === 0) return 'Nessun riferimento ordine';
  return sdi.order_refs.slice(0, 4).map((ref) => {
    const target = ref.orders.length > 0 ? ref.orders.join(' + ') : (ref.code ? 'nessun ordine' : '');
    return target ? `${ref.declared} → ${target}` : ref.declared;
  }).join(' · ');
}

function sdiPeriodText(sdi: MatchingFunnelInvoice['sdi']): string {
  if (!sdi.period_start) return '—';
  if (sdi.period_end && sdi.period_end !== sdi.period_start) return `${date(sdi.period_start)} – ${date(sdi.period_end)}`;
  return date(sdi.period_start);
}

function noteExcerpt(note: string): string {
  const flat = note.replace(/\s+/g, ' ').trim();
  if (!flat) return '—';
  return flat.length > 60 ? `${flat.slice(0, 57)}…` : flat;
}

function resolvedReferences(row: MatchingFunnelSupplier): number {
  return row.reference.one_rda + row.reference.multiple_rdas;
}

function alyanteSupplierLabel(row: MatchingFunnelSupplier): string {
  if (row.alyante_supplier_name) return row.alyante_supplier_name;
  return row.supplier_erp_id === null ? 'Codice fornitore assente' : 'Ragione sociale non disponibile';
}

function arakSupplierLabel(row: MatchingFunnelSupplier): string {
  return row.provider_name ?? 'Nessuna RDA candidata';
}

// The import runs every 8 hours: a success older than a day means three
// missed runs, which is the only signal worth showing.
const sdiImportStaleAfterMs = 24 * 60 * 60 * 1000;

function sdiImportModeLabel(mode: 'incremental' | 'full'): string {
  return mode === 'full' ? 'passata completa' : 'incrementale';
}

function sdiImportStatusLabel(status: SDIImportStatus | undefined): { text: string; stale: boolean } {
  if (!status) return { text: '—', stale: false };
  if (!status.last_success) {
    return { text: 'Nessun giro riuscito', stale: status.last_run !== null };
  }
  const run = status.last_success;
  const finished = run.finished_at ?? run.started_at;
  const when = formatInstant(finished) ?? finished;
  const stale = Date.now() - new Date(finished).getTime() > sdiImportStaleAfterMs;
  return { text: `${when} (${sdiImportModeLabel(run.mode)}, ${integer(run.inserted)} inseriti)`, stale };
}

const scopeTabs = [
  { key: 'open', label: 'Fatture da saldare' },
  { key: 'all', label: 'Tutte le fatture 2026' },
];

export function MatchingFunnelPage() {
  const [scope, setScope] = useState<MatchingFunnelScope>('open');
  const query = useMatchingFunnel(scope);
  const importStatus = useSDIImportStatus();
  const importLabel = sdiImportStatusLabel(importStatus.data);
  const [search, setSearch] = useState('');
  const [activeSupplierID, setActiveSupplierID] = useState<number | 'missing' | null>(null);
  const suppliers = query.data?.suppliers ?? [];
  const activeSupplier = suppliers.find((row) => (row.supplier_erp_id ?? 'missing') === activeSupplierID) ?? null;
  const filteredSuppliers = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase('it-IT');
    if (!needle) return suppliers;
    return suppliers.filter((row) => {
      const values = [
        row.supplier_erp_id === null ? '' : String(row.supplier_erp_id),
        row.alyante_supplier_name ?? '',
        row.provider_name ?? '',
        outcomeLabel(row.outcome),
      ];
      return values.some((value) => value.toLocaleLowerCase('it-IT').includes(needle));
    });
  }, [search, suppliers]);

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Diagnostica</p>
          <h1>Funnel dati</h1>
          <p className={styles.description}>
            Prima misura del bacino di matching. Le fatture sono confrontate con le RDA esclusivamente
            tramite il codice fornitore condiviso, senza importi, date, scoring o interpretazioni AI.
            Le note scritte da AFC sulle fatture già lavorate sono lette solo come verità di riferimento
            per misurare il funnel, mai come ingresso del matching.
          </p>
        </div>
        <div className={styles.headerActions}>
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
        <div className={styles.panel}>
          <Skeleton rows={10} />
        </div>
      )}

      {query.isError && (
        <div className={styles.error} role="alert">
          <span className={styles.errorIcon}><Icon name="triangle-alert" size={20} /></span>
          <div>
            <strong>Impossibile costruire il funnel dati.</strong>
            <p>Verificare le connessioni Alyante e Arak, quindi riprovare.</p>
          </div>
        </div>
      )}

      {query.isSuccess && (
        <>
          <div className={styles.summaryGrid}>
            <section className={styles.summaryPanel}>
              <h2>Universo osservato</h2>
              <table className={styles.summaryTable}>
                <tbody>
                  <tr><th scope="row">Fatture Alyante</th><td>{integer(query.data.summary.invoice_count)}</td></tr>
                  <tr><th scope="row">RDA Arak</th><td>{integer(query.data.summary.rda_count)}</td></tr>
                  <tr><th scope="row">RDA senza codice ERP fornitore</th><td>{integer(query.data.summary.rdas_without_erp_id)}</td></tr>
                  <tr><th scope="row">Codici RDA condivisi da più RDA</th><td>{integer(query.data.summary.duplicate_rda_codes)}</td></tr>
                  <tr><th scope="row">RDA con PA legacy nell'oggetto</th><td>{integer(query.data.summary.rdas_with_legacy_predecessor)}</td></tr>
                </tbody>
              </table>
            </section>

            <section className={styles.summaryPanel}>
              <h2>Esito del filtro fornitore</h2>
              <table className={styles.summaryTable}>
                <tbody>
                  <tr><th scope="row">Nessuna RDA candidata</th><td>{integer(query.data.summary.no_candidates)}</td></tr>
                  <tr><th scope="row">Una RDA candidata</th><td>{integer(query.data.summary.one_candidate)}</td></tr>
                  <tr><th scope="row">Più RDA candidate</th><td>{integer(query.data.summary.multiple_candidates)}</td></tr>
                </tbody>
              </table>
            </section>

            <section className={styles.summaryPanel}>
              <h2>Profilo contrattuale RDA</h2>
              <table className={styles.summaryTable}>
                <tbody>
                  <tr><th scope="row">Una tantum</th><td>{integer(query.data.profiles.one_time)}</td></tr>
                  <tr><th scope="row">Ricorrenti</th><td>{integer(query.data.profiles.recurring)}</td></tr>
                  <tr><th scope="row">Miste</th><td>{integer(query.data.profiles.mixed)}</td></tr>
                  <tr><th scope="row">Non classificabili</th><td>{integer(query.data.profiles.unknown)}</td></tr>
                </tbody>
              </table>
              <p className={styles.summaryNote}>
                Classificazione preliminare basata su tipo riga, ricorrenza, durata e rinnovo.
              </p>
            </section>

            <section className={styles.summaryPanel}>
              <h2>Verità di riferimento AFC</h2>
              <table className={styles.summaryTable}>
                <tbody>
                  <tr><th scope="row">Una RDA indicata</th><td>{integer(query.data.reference.one_rda)}</td></tr>
                  <tr><th scope="row">Più RDA indicate</th><td>{integer(query.data.reference.multiple_rdas)}</td></tr>
                  <tr><th scope="row">Codice non in Arak</th><td>{integer(query.data.reference.unresolved)}</td></tr>
                  <tr><th scope="row">Codice ambiguo</th><td>{integer(query.data.reference.ambiguous)}</td></tr>
                  <tr><th scope="row">Legacy sostituito da RDA Arak</th><td>{integer(query.data.reference.legacy_promoted)}</td></tr>
                  <tr><th scope="row">Legacy senza successore Arak</th><td>{integer(query.data.reference.legacy_only)}</td></tr>
                  <tr><th scope="row">Arak e legacy insieme</th><td>{integer(query.data.reference.arak_and_legacy)}</td></tr>
                  <tr><th scope="row">Nessuna RDA dichiarata</th><td>{integer(query.data.reference.no_rda_declared)}</td></tr>
                  <tr><th scope="row">Nota senza codice</th><td>{integer(query.data.reference.no_code)}</td></tr>
                  <tr><th scope="row">Nessuna nota</th><td>{integer(query.data.reference.no_note)}</td></tr>
                  <tr><th scope="row">RDA indicata con fornitore diverso</th><td>{integer(query.data.reference.supplier_mismatch)}</td></tr>
                </tbody>
              </table>
              <p className={styles.summaryNote}>
                Codici PO e PA letti dalle note testata. Un PA legacy citato nell'oggetto di una RDA Arak conta come quella RDA. «Nessuna RDA dichiarata» = nota «PA mai creati».
              </p>
            </section>

            <section className={styles.summaryPanel}>
              <h2>Prova delle regole</h2>
              <table className={styles.summaryTable}>
                <tbody>
                  <tr><th scope="row">Fatture con RDA indicata da AFC</th><td>{integer(query.data.rules.truth_invoices)}</td></tr>
                  <tr><th scope="row">Stessa RDA di AFC</th><td>{integer(query.data.rules.match)}</td></tr>
                  <tr><th scope="row">di cui per importo intero</th><td>{integer(query.data.rules.match_by_full)}</td></tr>
                  <tr><th scope="row">di cui per rata</th><td>{integer(query.data.rules.match_by_installment)}</td></tr>
                  <tr><th scope="row">di cui per somma di RDA</th><td>{integer(query.data.rules.match_by_sum)}</td></tr>
                  <tr><th scope="row">Più proposte, una giusta</th><td>{integer(query.data.rules.ambiguous)}</td></tr>
                  <tr><th scope="row">RDA diversa da AFC</th><td>{integer(query.data.rules.wrong)}</td></tr>
                  <tr><th scope="row">Nessuna proposta</th><td>{integer(query.data.rules.none)}</td></tr>
                  <tr><th scope="row">Fatture senza RDA (AFC)</th><td>{integer(query.data.rules.no_rda_invoices)}</td></tr>
                  <tr><th scope="row">di cui con proposta sbagliata</th><td>{integer(query.data.rules.false_positive)}</td></tr>
                  <tr><th scope="row">Fatture senza riscontro AFC con proposta</th><td>{integer(query.data.rules.no_truth_proposals)} / {integer(query.data.rules.no_truth_invoices)}</td></tr>
                  <tr><th scope="row">Nessuna proposta ma RDA entro il 2%</th><td>{integer(query.data.rules.near_misses)}</td></tr>
                </tbody>
              </table>
              <p className={styles.summaryNote}>
                Confronto al centesimo fra imponibile e RDA dello stesso fornitore: importo intero, poi rata di un periodo, poi somma di due o tre RDA.
              </p>
            </section>

            <section className={styles.summaryPanel}>
              <h2>Ordini Alyante</h2>
              <table className={styles.summaryTable}>
                <tbody>
                  <tr><th scope="row">Ordini a fornitore (tutti gli anni)</th><td>{integer(query.data.orders.order_count)}</td></tr>
                  {Object.entries(query.data.orders.by_doc_code).sort().map(([code, count]) => (
                    <tr key={code}><th scope="row">di cui {code}</th><td>{integer(count)}</td></tr>
                  ))}
                  <tr><th scope="row">Con codice RDA nel numero originale</th><td>{integer(query.data.orders.with_rda_code)}</td></tr>
                  <tr><th scope="row">di cui RDA trovata in Arak</th><td>{integer(query.data.orders.rda_resolved)}</td></tr>
                  <tr><th scope="row">di cui con fornitore diverso dalla RDA</th><td>{integer(query.data.orders.supplier_mismatch)}</td></tr>
                  <tr><th scope="row">Con codice del gestionale precedente</th><td>{integer(query.data.orders.with_legacy_code)}</td></tr>
                  <tr><th scope="row">Senza codice</th><td>{integer(query.data.orders.without_code)}</td></tr>
                  <tr><th scope="row">Ordini aperti (residuo da fatturare)</th><td>{integer(query.data.orders.open_orders)}</td></tr>
                  <tr><th scope="row">Ordini chiusi (tutto fatturato)</th><td>{integer(query.data.orders.closed_orders)}</td></tr>
                  <tr><th scope="row">di cui fatturati oltre l'ordinato</th><td>{integer(query.data.orders.over_consumed_orders)}</td></tr>
                  <tr><th scope="row">Ordini senza righe</th><td>{integer(query.data.orders.orders_without_lines)}</td></tr>
                  <tr><th scope="row">Fatture senza ordine dello stesso fornitore</th><td>{integer(query.data.orders.invoices_no_candidates)}</td></tr>
                  <tr><th scope="row">Fatture con un solo ordine aperto</th><td>{integer(query.data.orders.invoices_one_open_candidate)}</td></tr>
                  <tr><th scope="row">Fatture con più ordini aperti</th><td>{integer(query.data.orders.invoices_multiple_open_candidates)}</td></tr>
                  <tr><th scope="row">Fatture senza ordini aperti</th><td>{integer(query.data.orders.invoices_no_open_candidates)}</td></tr>
                  <tr><th scope="row">Fatture collegate a un ordine da AFC</th><td>{integer(query.data.orders.invoices_with_afc_link)}</td></tr>
                </tbody>
              </table>
              <p className={styles.summaryNote}>
                Ordini di tipo 22 letti da Alyante, tutti gli anni. Il residuo di ogni riga è quantità ordinata meno quantità delle righe di fattura che AFC vi ha collegato. Un ordine è aperto se una riga ha residuo.
              </p>
            </section>

            <section className={styles.summaryPanel}>
              <h2>Prova delle regole sugli ordini</h2>
              <table className={styles.summaryTable}>
                <tbody>
                  <tr><th scope="row">Fatture collegate a un ordine da AFC</th><td>{integer(query.data.order_rules.linked_invoices)}</td></tr>
                  <tr><th scope="row">Stesso ordine di AFC</th><td>{integer(query.data.order_rules.match)}</td></tr>
                  <tr><th scope="row">di cui riga per riga</th><td>{integer(query.data.order_rules.match_by_lines)}</td></tr>
                  <tr><th scope="row">di cui per totale ordine</th><td>{integer(query.data.order_rules.match_by_header)}</td></tr>
                  <tr><th scope="row">Più ordini, tra cui quelli di AFC</th><td>{integer(query.data.order_rules.ambiguous)}</td></tr>
                  <tr><th scope="row">Ordine diverso da AFC</th><td>{integer(query.data.order_rules.wrong)}</td></tr>
                  <tr><th scope="row">Nessuna proposta</th><td>{integer(query.data.order_rules.none)}</td></tr>
                  <tr><th scope="row">Fatture non collegate da AFC</th><td>{integer(query.data.order_rules.unlinked_invoices)}</td></tr>
                  <tr><th scope="row">di cui con una proposta</th><td>{integer(query.data.order_rules.unlinked_proposal)}</td></tr>
                </tbody>
              </table>
              <p className={styles.summaryNote}>
                Candidati: solo ordini aperti del fornitore, valutati come se la fattura non fosse ancora collegata. Prima riga per riga (stesso articolo, stesso imponibile o stesso prezzo unitario, quantità entro il residuo), poi il totale imponibile dell'ordine.
              </p>
            </section>

            <section className={styles.summaryPanel}>
              <h2>Fattura elettronica (SDI)</h2>
              <table className={styles.summaryTable}>
                <tbody>
                  <tr>
                    <th scope="row">Ultima importazione riuscita</th>
                    <td className={importLabel.stale ? styles.codeWarn : undefined}>
                      {importStatus.isError ? 'Stato non disponibile' : importLabel.text}
                    </td>
                  </tr>
                  <tr><th scope="row">Fatture con XML agganciato</th><td>{integer(query.data.sdi.invoices_linked)}</td></tr>
                  <tr><th scope="row">di cui solo per numero e data</th><td>{integer(query.data.sdi.linked_number_only)}</td></tr>
                  <tr><th scope="row">Fatture senza XML</th><td>{integer(query.data.sdi.invoices_not_linked)}</td></tr>
                  <tr><th scope="row">Con riferimento ordine del fornitore</th><td>{integer(query.data.sdi.with_order_ref)}</td></tr>
                  <tr><th scope="row">di cui con codice PO o PA</th><td>{integer(query.data.sdi.with_usable_code)}</td></tr>
                  <tr><th scope="row">di cui risolto su un ordine Alyante</th><td>{integer(query.data.sdi.resolved_to_order)}</td></tr>
                  <tr><th scope="row">di cui risolto su una RDA Arak</th><td>{integer(query.data.sdi.resolved_to_rda)}</td></tr>
                  <tr><th scope="row">di cui con codice non trovato</th><td>{integer(query.data.sdi.unresolved_code)}</td></tr>
                  <tr><th scope="row">Con riferimento contratto</th><td>{integer(query.data.sdi.with_contract_ref)}</td></tr>
                  <tr><th scope="row">Con periodo di competenza</th><td>{integer(query.data.sdi.with_period)}</td></tr>
                  <tr><th scope="row">Con codice articolo del fornitore</th><td>{integer(query.data.sdi.with_article_codes)}</td></tr>
                  <tr><th scope="row">Confronto con AFC: fatture collegate</th><td>{integer(query.data.sdi.afc_linked_invoices)}</td></tr>
                  <tr><th scope="row">Stessi ordini di AFC</th><td>{integer(query.data.sdi.match)}</td></tr>
                  <tr><th scope="row">di cui prima ambigue per importo</th><td>{integer(query.data.sdi.order_rule_ambiguous_resolved)}</td></tr>
                  <tr><th scope="row">Parte degli ordini di AFC</th><td>{integer(query.data.sdi.partial)}</td></tr>
                  <tr><th scope="row">Ordini diversi da AFC</th><td>{integer(query.data.sdi.wrong)}</td></tr>
                  <tr><th scope="row">Riferimento non risolto</th><td>{integer(query.data.sdi.none)}</td></tr>
                </tbody>
              </table>
              <p className={styles.summaryNote}>
                XML ricevuti dallo SDI, agganciati alla registrazione Alyante per partita IVA, numero e data. Il riferimento ordine scritto dal fornitore viene risolto sugli ordini Alyante tramite il codice RDA o PA nel numero originale.
              </p>
            </section>
          </div>

          <section className={styles.tablePanel}>
            <div className={styles.toolbar}>
              <div>
                <h2>Dettaglio per fornitore</h2>
                <p>Ogni riga mostra il bacino di RDA disponibile per le fatture dello stesso codice ERP.</p>
              </div>
              <SearchInput
                value={search}
                onChange={setSearch}
                placeholder="Cerca codice, fornitore o esito…"
                ariaLabel="Filtra il dettaglio per fornitore"
                className={styles.search}
              />
            </div>

            {filteredSuppliers.length === 0 ? (
              <div className={styles.filteredEmpty}>
                <strong>Nessun fornitore corrisponde alla ricerca</strong>
                <Button variant="secondary" size="sm" onClick={() => setSearch('')}>Cancella ricerca</Button>
              </div>
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <VisuallyHidden as="caption">Funnel di matching aggregato per codice fornitore</VisuallyHidden>
                  <thead>
                    <tr>
                      <th>Codice ERP</th>
                      <th>Fornitore Alyante</th>
                      <th>Fornitore Arak</th>
                      <th className={styles.numeric}>Fatture</th>
                      <th className={styles.numeric}>RDA candidate</th>
                      <th>Esito</th>
                      <th className={styles.numeric}>Una tantum</th>
                      <th className={styles.numeric}>Ricorrenti</th>
                      <th className={styles.numeric}>Miste</th>
                      <th className={styles.numeric}>Non class.</th>
                      <th className={styles.numeric}>Rif. AFC risolti</th>
                      <th className={styles.numeric}>Forn. diverso</th>
                      <th className={styles.numeric}>Ordini Alyante</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredSuppliers.map((row) => (
                      <tr key={row.supplier_erp_id ?? 'missing'}>
                        <td>
                          <button
                            type="button"
                            className={styles.detailButton}
                            onClick={() => setActiveSupplierID(row.supplier_erp_id ?? 'missing')}
                            aria-haspopup="dialog"
                          >
                            <Icon name="chevron-right" size={14} />
                            {row.supplier_erp_id === null ? '—' : integer(row.supplier_erp_id)}
                          </button>
                        </td>
                        <td>{alyanteSupplierLabel(row)}</td>
                        <td>{arakSupplierLabel(row)}</td>
                        <td className={styles.numeric}>{integer(row.invoice_count)}</td>
                        <td className={styles.numeric}>{integer(row.candidate_count)}</td>
                        <td>{outcomeLabel(row.outcome)}</td>
                        <td className={styles.numeric}>{integer(row.profiles.one_time)}</td>
                        <td className={styles.numeric}>{integer(row.profiles.recurring)}</td>
                        <td className={styles.numeric}>{integer(row.profiles.mixed)}</td>
                        <td className={styles.numeric}>{integer(row.profiles.unknown)}</td>
                        <td className={styles.numeric}>{integer(resolvedReferences(row))}</td>
                        <td className={styles.numeric}>{integer(row.reference.supplier_mismatch)}</td>
                        <td className={styles.numeric}>{integer(row.order_candidate_count)}</td>
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
              subtitle={[
                activeSupplier.alyante_supplier_name
                  ? `Alyante: ${activeSupplier.alyante_supplier_name}`
                  : null,
                activeSupplier.provider_name
                  ? `Arak: ${activeSupplier.provider_name}`
                  : 'Nessuna RDA candidata per questo fornitore',
              ].filter(Boolean).join(' · ')}
            >
              <div className={styles.drawerBody}>
                <section className={styles.detailSection}>
                  <div className={styles.detailHeading}>
                    <h3>Fatture Alyante</h3>
                    <span>{integer(activeSupplier.invoices.length)}</span>
                  </div>
                  <div className={styles.detailTableWrap}>
                    <table className={styles.detailTable}>
                      <thead>
                        <tr>
                          <th>Data</th>
                          <th>Numero</th>
                          <th>Rif. fornitore</th>
                          <th>Registrazione</th>
                          <th className={styles.numeric}>Imponibile</th>
                          <th>Stato AFC</th>
                          <th>Riferimenti AFC</th>
                          <th>Esito riferimento</th>
                          <th>Regola</th>
                          <th>RDA proposte</th>
                          <th>Confronto con AFC</th>
                          <th>Ordine collegato da AFC</th>
                          <th>Regola ordini</th>
                          <th>Ordini proposti</th>
                          <th>Confronto ordini</th>
                          <th>Riferimento ordine SDI</th>
                          <th>Periodo SDI</th>
                          <th>Confronto SDI</th>
                        </tr>
                      </thead>
                      <tbody>
                        {activeSupplier.invoices.map((invoice) => (
                          <tr key={invoice.registration}>
                            <td>{date(invoice.document_date)}</td>
                            <td>{invoice.document_number || '—'}</td>
                            <td>{invoice.supplier_reference || '—'}</td>
                            <td>{String(invoice.registration)}</td>
                            <td className={styles.numeric}>{money(invoice.taxable_amount)}</td>
                            <td>{invoice.reference.afc_status || '—'}</td>
                            <td className={styles.codesCell}>
                              {invoice.reference.rdas.length === 0 && invoice.reference.legacy_codes.length === 0
                                ? <span className={styles.codeMuted} title={invoice.reference.note}>{noteExcerpt(invoice.reference.note)}</span>
                                : <div className={styles.codes}>{[
                                    ...invoice.reference.rdas.map((rda) => (
                                      <span
                                        key={rda.code}
                                        className={rda.resolution === 'unresolved' || rda.resolution === 'ambiguous' || rda.supplier_match === false ? styles.codeWarn : undefined}
                                      >
                                        {referencedRDALabel(rda)}
                                      </span>
                                    )),
                                    ...invoice.reference.legacy_codes.map((code) => (
                                      <span key={code} className={styles.codeMuted}>{code}</span>
                                    )),
                                  ]}</div>}
                            </td>
                            <td>{referenceOutcomeLabel(invoice.reference.outcome)}</td>
                            <td>{ruleLabel(invoice.rules.rule)}</td>
                            <td className={styles.codesCell}>{proposalsText(invoice.rules)}</td>
                            <td className={
                              invoice.rules.verdict === 'match' || invoice.rules.verdict === 'silent_ok'
                                ? styles.verdictGood
                                : invoice.rules.verdict === 'wrong' || invoice.rules.verdict === 'false_positive'
                                  ? styles.codeWarn
                                  : undefined
                            }>
                              {verdictLabel(invoice.rules.verdict)}
                            </td>
                            <td className={styles.codesCell}>{invoice.order_rules.afc_links.length === 0 ? '—' : invoice.order_rules.afc_links.join(' · ')}</td>
                            <td>{orderRuleLabel(invoice.order_rules.rule)}</td>
                            <td className={styles.codesCell}>{orderProposalsText(invoice.order_rules)}</td>
                            <td className={
                              invoice.order_rules.verdict === 'match'
                                ? styles.verdictGood
                                : invoice.order_rules.verdict === 'wrong'
                                  ? styles.codeWarn
                                  : undefined
                            }>
                              {orderVerdictLabel(invoice.order_rules.verdict)}
                            </td>
                            <td className={styles.codesCell}>{sdiRefsText(invoice.sdi)}</td>
                            <td>{sdiPeriodText(invoice.sdi)}</td>
                            <td className={
                              invoice.sdi.verdict === 'match'
                                ? styles.verdictGood
                                : invoice.sdi.verdict === 'wrong'
                                  ? styles.codeWarn
                                  : undefined
                            }>
                              {invoice.sdi.linked ? sdiVerdictLabel(invoice.sdi.verdict) : '—'}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </section>

                <section className={styles.detailSection}>
                  <div className={styles.detailHeading}>
                    <h3>RDA candidate</h3>
                    <span>{integer(activeSupplier.candidates.length)}</span>
                  </div>
                  {activeSupplier.candidates.length === 0 ? (
                    <p className={styles.noCandidates}>Nessuna RDA nell’universo osservato con lo stesso codice fornitore.</p>
                  ) : (
                    <div className={styles.detailTableWrap}>
                      <table className={styles.detailTable}>
                        <thead>
                          <tr>
                            <th>Codice</th>
                            <th>Stato</th>
                            <th>Oggetto</th>
                            <th>Creazione</th>
                            <th>Profilo</th>
                            <th className={styles.numeric}>Totale</th>
                          </tr>
                        </thead>
                        <tbody>
                          {activeSupplier.candidates.map((candidate) => (
                            <tr key={candidate.id}>
                              <td>{candidate.code || `ID ${integer(candidate.id)}`}</td>
                              <td>{candidate.state || '—'}</td>
                              <td className={styles.objectCell}>{candidate.object || '—'}</td>
                              <td>{date(candidate.created)}</td>
                              <td>{profileLabel(candidate.profile)}</td>
                              <td className={styles.numeric}>{money(candidate.total, candidate.currency || 'EUR')}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </section>

                <section className={styles.detailSection}>
                  <div className={styles.detailHeading}>
                    <h3>Ordini Alyante dello stesso fornitore</h3>
                    <span>{integer(activeSupplier.orders.filter((order) => order.open).length)} aperti su {integer(activeSupplier.orders.length)}</span>
                  </div>
                  {activeSupplier.orders.length === 0 ? (
                    <p className={styles.noCandidates}>Nessun ordine a fornitore in Alyante con lo stesso codice fornitore.</p>
                  ) : (
                    <div className={styles.detailTableWrap}>
                      <table className={styles.detailTable}>
                        <thead>
                          <tr>
                            <th>Ordine</th>
                            <th>Documento</th>
                            <th>Data</th>
                            <th>Codice RDA</th>
                            <th>Stato</th>
                            <th className={styles.numeric}>Righe</th>
                            <th className={styles.numeric}>Residuo (q.tà)</th>
                            <th className={styles.numeric}>Imponibile</th>
                            <th className={styles.numeric}>Totale</th>
                          </tr>
                        </thead>
                        <tbody>
                          {activeSupplier.orders.map((order) => (
                            <tr key={order.registration}>
                              <td>{order.label}</td>
                              <td>{order.doc_code}</td>
                              <td>{date(order.date)}</td>
                              <td>{order.rda_code ? `${order.rda_code}${order.rda_id === null ? ' (non in Arak)' : ''}` : '—'}</td>
                              <td>{order.open ? 'Aperto' : 'Chiuso'}</td>
                              <td className={styles.numeric}>{integer(order.line_count)}</td>
                              <td className={styles.numeric}>{formatNumber(order.residual_qty, { format: { maximumFractionDigits: 2 } }) ?? String(order.residual_qty)}</td>
                              <td className={styles.numeric}>{money(order.taxable)}</td>
                              <td className={styles.numeric}>{money(order.total)}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </section>
              </div>
            </Drawer>
          )}
        </>
      )}
    </section>
  );
}
