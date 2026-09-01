import { useMemo, useState } from 'react';
import { formatCurrency, formatLocalDate, formatNumber } from '@mrsmith/format';
import { Button, Drawer, Icon, Skeleton, VisuallyHidden } from '@mrsmith/ui';
import { useArakRDAs } from '../api/queries';
import type { ArakRDARow } from '../types';
import styles from './RdaArakPage.module.css';

/** Una RDA = testata + righe dall'estrazione Arak. */
interface RdaGroup {
  key: string;
  header: ArakRDARow;
  lines: ArakRDARow[];
}

type LineKind = 'text' | 'integer' | 'decimal' | 'money';

interface LineColumnDef {
  key: keyof ArakRDARow;
  kind: LineKind;
  label?: string;
}

const lineColumns: LineColumnDef[] = [
  { key: 'row_id', kind: 'integer', label: 'Riga' },
  { key: 'product_code', kind: 'text', label: 'Cod. prodotto' },
  { key: 'row_type', kind: 'text', label: 'Tipo' },
  { key: 'qty', kind: 'decimal' },
  { key: 'nrc', kind: 'money' },
  { key: 'mrc', kind: 'money' },
  { key: 'price', kind: 'money' },
  { key: 'total', kind: 'money' },
];

function hasRowData(row: ArakRDARow): boolean {
  return row.row_id !== null;
}

function groupRDAs(rows: ArakRDARow[]): RdaGroup[] {
  const groups: RdaGroup[] = [];
  const byKey = new Map<string, RdaGroup>();
  rows.forEach((row, index) => {
    const key = row.id !== null && row.id !== undefined ? `${row.id}` : `noidx-${index}`;
    const existing = byKey.get(key);
    if (existing) {
      if (hasRowData(row)) existing.lines.push(row);
      return;
    }
    const group: RdaGroup = { key, header: row, lines: hasRowData(row) ? [row] : [] };
    byKey.set(key, group);
    groups.push(group);
  });
  return groups;
}

function money(value: number | null, currency: string | null): string {
  return formatCurrency(value, currency ?? undefined) ?? '—';
}

function integer(value: number | null): string {
  return formatNumber(value, { format: { maximumFractionDigits: 0 } }) ?? '—';
}

function decimal(value: number | null): string {
  return formatNumber(value, { format: { maximumFractionDigits: 4 } }) ?? '—';
}

function text(value: string | number | boolean | null): string {
  if (value === null || value === undefined || value === '') return '—';
  if (typeof value === 'boolean') return value ? 'Sì' : 'No';
  return String(value).trim() || '—';
}

function bool(value: boolean | null): string {
  if (value === null) return '—';
  return value ? 'Sì' : 'No';
}

function date(value: string | null): string {
  return formatLocalDate(value) ?? text(value);
}

function providerLabel(h: ArakRDARow): string {
  const name = text(h.provider_company_name);
  const erp = text(h.erp_id);
  if (name === '—' && erp === '—') return '—';
  if (name === '—') return `ERP ${erp}`;
  if (erp === '—') return name;
  return `${name} (ERP ${erp})`;
}

function budgetLabel(h: ArakRDARow): string {
  const name = text(h.budget_name);
  const year = h.budget_year !== null ? integer(h.budget_year) : '—';
  if (name === '—' && year === '—') return '—';
  if (name === '—') return year;
  if (year === '—') return name;
  return `${name} ${year}`;
}

interface DetailField {
  label: string;
  value: string;
  wide?: boolean;
}

function detailFields(h: ArakRDARow): DetailField[] {
  return [
    // Identificazione e contenuto
    { label: 'ID', value: integer(h.id) },
    { label: 'Codice', value: text(h.code) },
    { label: 'Tipo', value: text(h.type) },
    { label: 'Progetto', value: text(h.project) },
    { label: 'Oggetto', value: text(h.object) },
    { label: 'Descrizione', value: text(h.description), wide: true },
    { label: 'Note', value: text(h.note), wide: true },
    // Ciclo e stato
    { label: 'Stato', value: text(h.state) },
    { label: 'Documento creato', value: text(h.created_document) },
    { label: 'Livello approvazione corrente', value: integer(h.current_approval_level) },
    { label: 'Data creazione', value: date(h.created) },
    { label: 'Data aggiornamento', value: date(h.updated) },
    { label: 'Data eliminazione', value: date(h.deleted) },
    // Soggetti e classificazioni
    { label: 'Richiedente', value: text(h.requester_email) },
    { label: 'Fornitore', value: providerLabel(h) },
    { label: 'Stato fornitore', value: text(h.provider_state) },
    { label: 'Budget', value: budgetLabel(h) },
    { label: 'Centro di costo', value: text(h.cost_center) },
    { label: 'Budget user', value: integer(h.budget_user_id) },
    // Condizioni e riferimenti economici
    { label: 'Valuta', value: text(h.currency) },
    { label: 'Importo totale', value: money(h.total_price, h.currency) },
    { label: 'Metodo di pagamento', value: text(h.payment_method_code) },
    { label: 'Descrizione pagamento', value: text(h.payment_method_description) },
    { label: 'Leasing', value: bool(h.leasing) },
    { label: 'Anticipo', value: bool(h.advance_payment) },
    { label: 'Data offerta fornitore', value: date(h.provider_offer_date) },
    { label: 'Codice offerta fornitore', value: text(h.provider_offer_code) },
    { label: 'Magazzino di riferimento', value: text(h.reference_warehouse) },
    { label: 'Incremento budget', value: integer(h.budget_increment_id) },
    { label: 'Sottratto dal budget', value: bool(h.subtracted_from_budget) },
  ];
}

function lineValue(row: ArakRDARow, column: LineColumnDef): string {
  const value = row[column.key];
  if (value === null || value === undefined || value === '') return '—';
  if (column.kind === 'integer' && typeof value === 'number') return integer(value);
  if (column.kind === 'decimal' && typeof value === 'number') return decimal(value);
  if (column.kind === 'money' && typeof value === 'number') return formatNumber(value, { format: { minimumFractionDigits: 2, maximumFractionDigits: 2 } }) ?? '—';
  return text(value as string | number | boolean | null);
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

export function RdaArakPage() {
  const query = useArakRDAs();
  const rows = query.data ?? [];
  const groups = useMemo(() => groupRDAs(rows), [rows]);
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const activeGroup = groups.find((g) => g.key === activeKey) ?? null;

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Utility</p>
          <h1>RDA Arak</h1>
          <p className={styles.description}>
            Estrazione delle richieste di acquisto (RDA) Arak non in bozza e non annullate.
            Una riga per RDA: testata e righe sono consultabili dal dettaglio.
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
            <strong>Impossibile caricare le RDA Arak.</strong>
            <p>Verificare la configurazione della connessione Arak e riprovare.</p>
          </div>
        </div>
      )}

      {query.isSuccess && groups.length === 0 && (
        <div className={styles.empty}>
          <span className={styles.emptyIcon}><Icon name="database" size={32} /></span>
          <strong>Nessuna RDA da mostrare</strong>
          <p>L’estrazione non ha restituito richieste di acquisto non in bozza o non annullate.</p>
        </div>
      )}

      {query.isSuccess && groups.length > 0 && (
        <div className={styles.tablePanel}>
          <div className={styles.tableMeta}>
            <span>Granularità: riga RDA — dettaglio con righe di acquisto</span>
            <span>Valori grezzi Arak, formattati solo per consultazione</span>
          </div>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <VisuallyHidden as="caption">
                RDA Arak non in bozza e non annullate
              </VisuallyHidden>
              <thead>
                <tr>
                  <th className={styles.detailHead}><VisuallyHidden>Dettaglio</VisuallyHidden></th>
                  <th>Codice</th>
                  <th>Stato</th>
                  <th>Oggetto</th>
                  <th>Fornitore</th>
                  <th>Budget</th>
                  <th>Richiedente</th>
                  <th>Creazione</th>
                  <th className={styles.numTh}>Importo totale</th>
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
                          <VisuallyHidden>Dettagli RDA {text(h.code)}</VisuallyHidden>
                        </button>
                      </td>
                      <td>{text(h.code)}</td>
                      <td>{text(h.state)}</td>
                      <td>{text(h.object)}</td>
                      <td>{providerLabel(h)}</td>
                      <td>{budgetLabel(h)}</td>
                      <td>{text(h.requester_email)}</td>
                      <td className={styles.cellDate}>{date(h.created)}</td>
                      <td className={styles.numeric}>{money(h.total_price, h.currency)}</td>
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
          title={`RDA ${text(activeGroup.header.code)}`}
          subtitle={
            <span className={styles.drawerSubtitle}>
              {text(activeGroup.header.state)}
              {' · ID '}
              {integer(activeGroup.header.id)}
            </span>
          }
        >
          <div className={styles.drawerBody}>
            <section className={styles.card}>
              <DetailGrid fields={detailFields(activeGroup.header)} />
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Righe RDA</h3>
              {activeGroup.lines.length === 0 ? (
                <p className={styles.noLines}>Nessuna riga in estrazione.</p>
              ) : (
                <div className={styles.linesWrap}>
                  <table className={styles.miniTable}>
                    <thead>
                      <tr>
                        {lineColumns.map((column) => (
                          <th
                            key={column.key}
                            className={column.kind === 'text' ? undefined : styles.numTh}
                          >
                            {column.label ?? column.key}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    {activeGroup.lines.map((line, index) => (
                      <tbody key={`${line.row_id ?? 'riga'}-${index}`}>
                        <tr>
                          {lineColumns.map((column) => (
                            <td
                              key={column.key}
                              className={column.kind === 'text' ? undefined : styles.numeric}
                            >
                              {lineValue(line, column)}
                            </td>
                          ))}
                        </tr>
                        {(text(line.product_description) !== '—' || text(line.row_description) !== '—') && (
                          <tr>
                            <td colSpan={lineColumns.length}>
                              {text(line.product_description)}
                              {text(line.row_description) !== '—' && (
                                <span> · {text(line.row_description)}</span>
                              )}
                            </td>
                          </tr>
                        )}
                      </tbody>
                    ))}
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
