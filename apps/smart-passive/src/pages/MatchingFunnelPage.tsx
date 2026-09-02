import { useMemo, useState } from 'react';
import { formatCurrency, formatLocalDate, formatNumber } from '@mrsmith/format';
import { Button, Drawer, Icon, SearchInput, Skeleton, VisuallyHidden } from '@mrsmith/ui';
import { useMatchingFunnel } from '../api/queries';
import type { MatchingFunnelInvoice, MatchingFunnelReferencedRDA, MatchingFunnelSupplier } from '../types';
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

export function MatchingFunnelPage() {
  const query = useMatchingFunnel();
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
              </div>
            </Drawer>
          )}
        </>
      )}
    </section>
  );
}
