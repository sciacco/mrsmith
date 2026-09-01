import { useMemo, useState } from 'react';
import { formatCurrency, formatLocalDate, formatNumber } from '@mrsmith/format';
import { Button, Drawer, Icon, Skeleton, VisuallyHidden } from '@mrsmith/ui';
import { useAlyanteInvoices } from '../api/queries';
import type { AlyanteInvoiceRow } from '../types';
import styles from './AlyanteInvoicesPage.module.css';

/** Una fattura = testata + righe DO30 ripetute dall'estrazione Alyante. */
interface InvoiceGroup {
  key: string;
  header: AlyanteInvoiceRow;
  lines: AlyanteInvoiceRow[];
}

type LineKind = 'text' | 'integer' | 'decimal' | 'money';

interface LineColumnDef {
  key: keyof AlyanteInvoiceRow;
  kind: LineKind;
  label?: string;
}

const lineMainColumns: LineColumnDef[] = [
  { key: 'DO30_INDTIPORIGA', kind: 'integer', label: 'Tipo' },
  { key: 'DO30_UM1', kind: 'text' },
  { key: 'DO30_QTA1', kind: 'decimal' },
  { key: 'DO30_PREZZO1', kind: 'money' },
  { key: 'DO30_SCIMP', kind: 'money' },
  { key: 'DO30_IMPORTO', kind: 'money' },
  { key: 'DO30_IMPNETSCP', kind: 'money' },
  { key: 'DO30_ALIVA_CG28', kind: 'text' },
  { key: 'DO30_IMPORTOIVA', kind: 'money' },
];

function lineSubParts(line: AlyanteInvoiceRow): { codArt: string | null; descArt: string | null } {
  const codArt =
    line.DO30_CODART_MG66 !== null && String(line.DO30_CODART_MG66).trim() !== ''
      ? String(line.DO30_CODART_MG66).trim()
      : null;
  const descArt =
    line.DO30_DESCART !== null && String(line.DO30_DESCART).trim() !== ''
      ? String(line.DO30_DESCART).trim()
      : null;
  return { codArt, descArt };
}

function hasLineData(row: AlyanteInvoiceRow): boolean {
  return (
    row.DO30_PROGRIGA !== null ||
    row.DO30_CODART_MG66 !== null ||
    row.DO30_DESCART !== null ||
    row.DO30_IMPORTO !== null
  );
}

function groupInvoices(rows: AlyanteInvoiceRow[]): InvoiceGroup[] {
  const groups: InvoiceGroup[] = [];
  const byKey = new Map<string, InvoiceGroup>();
  rows.forEach((row, index) => {
    const key =
      row.DO11_NUMREG_CO99 !== null && row.DO11_NUMREG_CO99 !== undefined
        ? `${row.DO11_DITTA_CG18 ?? 'd'}-${row.DO11_NUMREG_CO99}`
        : `noidx-${index}`;
    const existing = byKey.get(key);
    if (existing) {
      if (hasLineData(row)) existing.lines.push(row);
      return;
    }
    const group: InvoiceGroup = { key, header: row, lines: hasLineData(row) ? [row] : [] };
    byKey.set(key, group);
    groups.push(group);
  });
  return groups;
}

function money(value: number | null): string {
  return formatCurrency(value) ?? '—';
}

function integer(value: number | null): string {
  return formatNumber(value, { format: { maximumFractionDigits: 0 } }) ?? '—';
}

function decimal(value: number | null): string {
  return formatNumber(value, { format: { maximumFractionDigits: 4 } }) ?? '—';
}

function text(value: string | number | null): string {
  if (value === null || value === undefined || value === '') return '—';
  return String(value).trim() || '—';
}

function date(value: string | null): string {
  return formatLocalDate(value) ?? text(value);
}

interface DetailField {
  label: string;
  value: string;
  wide?: boolean;
}

function detailFields(h: AlyanteInvoiceRow): DetailField[] {
  return [
    { label: 'Data documento', value: date(h.DO11_DATADOC) },
    { label: 'Fornitore (codice)', value: integer(h.DO11_CLIFOR_CG44) },
    { label: 'Riferimento fornitore', value: text(h.DO11_NUMDOCORIG) },
    { label: 'In scadenziario', value: h.IN_SCADENZIARIO ? 'Sì' : 'No' },
    { label: 'Prossima scadenza', value: date(h.EF01_SCADE_S) },
    { label: 'Rate aperte', value: integer(h.NUM_RATE_APERTE) },
    { label: 'Rate totali', value: integer(h.TOTRATE) },
    { label: 'Importo rate', value: money(h.EF01_IMPEFFORIG) },
    { label: 'Residuo', value: money(h.RESIDUO) },
    { label: 'Pagato su residuo', value: money(h.PAGATO_SU_RESIDUO) },
    { label: 'Totale a pagare', value: money(h.DO13_TOTAPAGARE) },
    { label: 'Totale documento', value: money(h.DO13_TOTDOCUMENTO) },
    { label: 'Note', value: text(h.DO11_NOTEDOCUM), wide: true },
  ];
}

function lineValue(row: AlyanteInvoiceRow, column: LineColumnDef): string {
  const value = row[column.key];
  if (value === null || value === undefined || value === '') return '—';
  if (column.kind === 'integer' && typeof value === 'number') return integer(value);
  if (column.kind === 'decimal' && typeof value === 'number') return decimal(value);
  if (column.kind === 'money' && typeof value === 'number') return money(value);
  return String(value);
}

function DetailGrid({ fields }: { fields: DetailField[] }) {
  return (
    <dl className={styles.grid}>
      {fields.map((field) => (
        <div key={field.label} className={field.wide ? styles.itemWide : styles.item}>
          <dt>{field.label}</dt>
          <dd>{field.value}</dd>
        </div>
      ))}
    </dl>
  );
}

function rateAperte(h: AlyanteInvoiceRow): string {
  if (h.NUM_RATE_APERTE === null || h.TOTRATE === null) return '—';
  return `${h.NUM_RATE_APERTE}/${h.TOTRATE}`;
}

export function AlyanteInvoicesPage() {
  const query = useAlyanteInvoices();
  const rows = query.data ?? [];
  const groups = useMemo(() => groupInvoices(rows), [rows]);
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const activeGroup = groups.find((g) => g.key === activeKey) ?? null;

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Utility</p>
          <h1>Fatture Alyante</h1>
          <p className={styles.description}>
            Estrazione delle fatture di acquisto non saldate o non presenti nello scadenziario.
            Una riga per fattura: testata, scadenziario e righe documento sono consultabili dal dettaglio.
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
          <Skeleton rows={12} />
        </div>
      )}

      {query.isError && (
        <div className={styles.error} role="alert">
          <span className={styles.errorIcon}><Icon name="triangle-alert" size={20} /></span>
          <div>
            <strong>Impossibile caricare le fatture Alyante.</strong>
            <p>Verificare la configurazione della connessione Alyante e riprovare.</p>
          </div>
        </div>
      )}

      {query.isSuccess && groups.length === 0 && (
        <div className={styles.empty}>
          <span className={styles.emptyIcon}><Icon name="database" size={32} /></span>
          <strong>Nessuna fattura da mostrare</strong>
          <p>L’estrazione non ha restituito fatture non saldate o mancanti nello scadenziario.</p>
        </div>
      )}

      {query.isSuccess && groups.length > 0 && (
        <div className={styles.tablePanel}>
          <div className={styles.tableMeta}>
            <span>Granularità: fattura — dettaglio con righe documento DO30</span>
            <span>Valori grezzi Alyante, formattati solo per consultazione</span>
          </div>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <VisuallyHidden as="caption">
                Fatture Alyante non saldate o non presenti nello scadenziario
              </VisuallyHidden>
              <thead>
                <tr>
                  <th className={styles.detailHead}><VisuallyHidden>Dettaglio</VisuallyHidden></th>
                  <th>Data documento</th>
                  <th>Numero</th>
                  <th>Tipo</th>
                  <th className={styles.numTh}>Fornitore</th>
                  <th>Rif. fornitore</th>
                  <th>Scadenza</th>
                  <th className={styles.numTh}>Rate aperte</th>
                  <th className={styles.numTh}>Totale documento</th>
                  <th className={styles.numTh}>Residuo</th>
                </tr>
              </thead>
              <tbody>
                {groups.map((group, index) => {
                  const h = group.header;
                  const selected = group.key === activeKey;
                  return (
                    <tr
                      key={group.key}
                      className={selected ? `${styles.row} ${styles.rowSelected}` : styles.row}
                      onClick={() => setActiveKey(group.key)}
                      style={{ animationDelay: `${Math.min(index * 40, 600)}ms` }}
                    >
                      <td className={styles.detailCell}>
                        <div className={styles.accentBar} />
                        <button
                          type="button"
                          className={styles.detailBtn}
                          onClick={() => setActiveKey(group.key)}
                          aria-haspopup="dialog"
                          aria-expanded={selected}
                        >
                          <Icon name="chevron-right" size={14} />
                          <VisuallyHidden>Dettagli fattura {text(h.DO11_NUMDOC)}</VisuallyHidden>
                        </button>
                      </td>
                      <td className={styles.cellDate}>{date(h.DO11_DATADOC)}</td>
                      <td>{text(h.DO11_NUMDOC)}</td>
                      <td>{text(h.DO11_DOCUM_MG36)}</td>
                      <td className={styles.numeric}>{integer(h.DO11_CLIFOR_CG44)}</td>
                      <td>{text(h.DO11_NUMDOCORIG)}</td>
                      <td className={styles.cellDate}>{date(h.EF01_SCADE_S)}</td>
                      <td className={styles.numeric}>{rateAperte(h)}</td>
                      <td className={styles.numeric}>{money(h.DO13_TOTDOCUMENTO)}</td>
                      <td className={styles.numeric}>{money(h.RESIDUO)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {activeGroup && (
        <Drawer
          open
          onClose={() => setActiveKey(null)}
          size="xl"
          title={`Fattura ${text(activeGroup.header.DO11_NUMDOC)}`}
          subtitle={
            <span className={styles.drawerSubtitle}>
              {text(activeGroup.header.DO11_DOCUM_MG36)}
              {' · '}
              {text(activeGroup.header.MG36_DESCDOCUM)}
              {' · '}
              Reg. {text(activeGroup.header.DO11_NUMREG_CO99)}
            </span>
          }
        >
          <div className={styles.drawerBody}>
            <section className={styles.card}>
              <DetailGrid fields={detailFields(activeGroup.header)} />
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Righe documento</h3>
              {activeGroup.lines.length === 0 ? (
                <p className={styles.noLines}>Nessuna riga documento in estrazione.</p>
              ) : (
                <div className={styles.linesWrap}>
                  <table className={styles.miniTable}>
                    <thead>
                      <tr>
                        {lineMainColumns.map((column) => (
                          <th
                            key={column.key}
                            className={column.kind === 'text' ? undefined : styles.numTh}
                          >
                            {column.label ?? column.key}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    {activeGroup.lines.map((line, index) => {
                      const { codArt, descArt } = lineSubParts(line);
                      return (
                        <tbody key={`${line.DO30_PROGRIGA ?? 'riga'}-${index}`}>
                          <tr>
                            {lineMainColumns.map((column) => (
                              <td
                                key={column.key}
                                className={column.kind === 'text' ? undefined : styles.numeric}
                              >
                                {lineValue(line, column)}
                              </td>
                            ))}
                          </tr>
                          {(codArt !== null || descArt !== null) && (
                            <tr className={styles.lineSub}>
                              <td aria-hidden="true" />
                              <td colSpan={lineMainColumns.length - 1}>
                                {descArt ?? '—'}
                                {codArt !== null && (
                                  <span className={styles.lineCode}> COD: {codArt}</span>
                                )}
                              </td>
                            </tr>
                          )}
                        </tbody>
                      );
                    })}
                  </table>
                </div>
              )}
            </section>
          </div>
        </Drawer>
      )}
    </section>
  );
}
