// Report (#163, slice 4 del task 7): i due prospetti ratificati, consuntivo
// economico ed erogato, come sezioni indipendenti della stessa vista — filtri,
// stati ed export propri di ciascuna (§159: nessuna attribuzione economica per
// date di evento o sessione, nessun costo indicativo/pattuito qui — quel
// confronto resta nel dettaglio evento).

import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { formatCurrency, formatNumber } from '@mrsmith/format';
import { Button, Icon, Skeleton, SingleSelect, StatusBadge, useToast } from '@mrsmith/ui';
import { useApiClient } from '../../api/client';
import { useDeliveredReport, useEconomicReport } from '../../api/queries';
import type { DeliveredReportRow, EconomicReportRow } from '../../api/types';
import { describeApiError } from '../../components/events/apiErrors';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { formatDateOnly, formatInstantDate } from '../../components/events/eventFormat';
import { deliveryStatusVariant, economicStateVariant } from '../../components/events/statusVariants';
import { downloadFile } from '../../components/people/awardHelpers';
import { DELIVERY_STATUS_LABELS, ECONOMIC_STATE_LABELS, LEARNING_OUTCOME_LABELS } from '../../lib/labels';
import styles from './ReportPage.module.css';

const UNKNOWN_YEAR_KEY = 'sconosciuto';

function budgetYearKey(row: EconomicReportRow): string {
  return row.poError || row.budgetYear === undefined ? UNKNOWN_YEAR_KEY : String(row.budgetYear);
}

export function ReportPage() {
  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>Report</h1>
        <p className={styles.subtitle}>Consuntivo economico ed erogato della formazione.</p>
      </header>
      <EconomicReportSection />
      <DeliveredReportSection />
    </main>
  );
}

// ── Consuntivo economico: raggruppato per anno di competenza del budget RDA ──

function economicTotalsLabel(rows: EconomicReportRow[]): string {
  const totals = new Map<string, number>();
  for (const row of rows) {
    if (row.poError || row.amount === undefined || row.currency === undefined) continue;
    const amount = Number(row.amount);
    if (!Number.isFinite(amount)) continue;
    totals.set(row.currency, (totals.get(row.currency) ?? 0) + amount);
  }
  if (totals.size === 0) return '—';
  return [...totals.entries()]
    .map(([currency, amount]) => formatCurrency(amount, currency) ?? `${formatNumber(amount)} ${currency}`)
    .join(' · ');
}

function EconomicReportSection() {
  const api = useApiClient();
  const { toast } = useToast();
  const report = useEconomicReport();
  const [year, setYear] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  const rows = report.data ?? [];
  const years = useMemo(() => [...new Set(rows.map(budgetYearKey))].sort((a, b) => b.localeCompare(a)), [rows]);
  const filtered = useMemo(() => (year ? rows.filter((row) => budgetYearKey(row) === year) : rows), [rows, year]);
  const groups = useMemo(() => {
    const byYear = new Map<string, EconomicReportRow[]>();
    for (const row of filtered) {
      const key = budgetYearKey(row);
      (byYear.get(key) ?? byYear.set(key, []).get(key)!).push(row);
    }
    return [...byYear.entries()].sort(([a], [b]) => b.localeCompare(a));
  }, [filtered]);

  async function exportXlsx() {
    setExportError(null);
    setExporting(true);
    try {
      const qs = year ? `?${new URLSearchParams({ year }).toString()}` : '';
      await downloadFile(api, `/training/v1/exports/economic.xlsx${qs}`, 'formazione-consuntivo-economico.xlsx');
      toast('Esportazione avviata');
    } catch (error) {
      setExportError(describeApiError(error, 'Esportazione non riuscita'));
    } finally {
      setExporting(false);
    }
  }

  return (
    <section className={styles.section}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>Consuntivo economico</h2>
          <p className={styles.sectionNote}>
            L&apos;attribuzione segue il budget della RDA collegata a ciascuna voce, non le date dell&apos;evento.
          </p>
        </div>
        <div className={styles.sectionActions}>
          <SingleSelect
            options={years.map((y) => ({ value: y, label: y === UNKNOWN_YEAR_KEY ? 'Anno non disponibile' : y }))}
            selected={year}
            onChange={setYear}
            placeholder="Tutti gli anni"
            allowClear
            ariaLabel="Filtra per anno di competenza"
          />
          <Button
            variant="secondary"
            size="sm"
            leftIcon={<Icon name="download" size={14} />}
            loading={exporting}
            disabled={rows.length === 0}
            onClick={exportXlsx}
          >
            Esporta XLSX
          </Button>
        </div>
      </div>

      <ErrorPanel message={exportError} onDismiss={() => setExportError(null)} />

      {report.isLoading ? (
        <Skeleton rows={4} />
      ) : report.isError ? (
        <p className={styles.errorNotice}>{describeApiError(report.error, 'Lettura del consuntivo non riuscita')}</p>
      ) : filtered.length === 0 ? (
        <p className={styles.emptyNotice}>Nessuna voce di spesa registrata.</p>
      ) : (
        groups.map(([key, groupRows]) => (
          <div key={key} className={styles.yearGroup}>
            <div className={styles.yearGroupHeader}>
              <h3 className={styles.yearGroupTitle}>{key === UNKNOWN_YEAR_KEY ? 'Anno non disponibile' : key}</h3>
              <span className={styles.yearGroupTotal}>{economicTotalsLabel(groupRows)}</span>
            </div>
            <EconomicTable rows={groupRows} />
          </div>
        ))
      )}
    </section>
  );
}

function EconomicTable({ rows }: { rows: EconomicReportRow[] }) {
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Corso</th>
            <th>Evento</th>
            <th>PO</th>
            <th>Stato economico</th>
            <th>Budget</th>
            <th className={styles.numCell}>Iscrizioni coperte</th>
            <th className={styles.numCell}>Importo</th>
            <th>Creata il</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.expenseId}>
              <td>
                {row.courseTitle}
                {row.eventCancelled && (
                  <StatusBadge
                    className={styles.inlineBadge}
                    value="cancelled"
                    label="Evento annullato"
                    variant="danger"
                  />
                )}
              </td>
              <td>
                <Link to={`/eventi/${row.eventId}`} className={styles.rowLink}>
                  Apri evento
                </Link>
              </td>
              <td>{row.poCode || row.poId}</td>
              <td>
                {row.poError ? (
                  <StatusBadge value="poError" label="Errore PO" variant="danger" tooltip={row.poError} />
                ) : row.economicState ? (
                  <StatusBadge
                    value={row.economicState}
                    label={ECONOMIC_STATE_LABELS[row.economicState]}
                    variant={economicStateVariant(row.economicState)}
                  />
                ) : (
                  '—'
                )}
              </td>
              <td>{row.budgetName || '—'}</td>
              <td className={styles.numCell}>{formatNumber(row.coveredEnrollments)}</td>
              <td className={styles.numCell}>
                {row.amount !== undefined && row.currency !== undefined
                  ? (formatCurrency(Number(row.amount), row.currency) ?? row.amount)
                  : '—'}
              </td>
              <td>{formatInstantDate(row.createdAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ── Erogato: intervallo di date sulla data di riferimento del fatto ──

type GroupBy = 'course' | 'skillArea' | 'team';

const GROUP_BY_OPTIONS: { value: GroupBy; label: string }[] = [
  { value: 'course', label: 'Corso' },
  { value: 'skillArea', label: 'Area' },
  { value: 'team', label: 'Team' },
];

function currentYearRange(): { from: string; to: string } {
  const year = new Date().getFullYear();
  return { from: `${year}-01-01`, to: `${year}-12-31` };
}

function groupKeysFor(row: DeliveredReportRow, groupBy: GroupBy): { key: string; label: string }[] {
  if (groupBy === 'course') return [{ key: row.courseId, label: row.courseTitle }];
  if (groupBy === 'skillArea') {
    if (row.skillAreaNames.length === 0) return [{ key: 'none', label: 'Nessuna area' }];
    return row.skillAreaNames.map((name) => ({ key: name, label: name }));
  }
  if (row.teams.length === 0) return [{ key: 'none', label: 'Nessun team' }];
  return row.teams.map((team) => ({ key: team.id, label: team.name }));
}

interface DeliveredGroup {
  key: string;
  label: string;
  completions: number;
  hours: number;
  hasHours: boolean;
}

function groupDeliveredRows(rows: DeliveredReportRow[], groupBy: GroupBy): DeliveredGroup[] {
  const groups = new Map<string, DeliveredGroup>();
  for (const row of rows) {
    for (const { key, label } of groupKeysFor(row, groupBy)) {
      const entry = groups.get(key) ?? { key, label, completions: 0, hours: 0, hasHours: false };
      entry.completions += 1;
      if (row.hours !== undefined) {
        entry.hours += row.hours;
        entry.hasHours = true;
      }
      groups.set(key, entry);
    }
  }
  return [...groups.values()].sort((a, b) => b.completions - a.completions);
}

function DeliveredReportSection() {
  const api = useApiClient();
  const { toast } = useToast();
  const [range, setRange] = useState(currentYearRange());
  const [groupBy, setGroupBy] = useState<GroupBy>('course');
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);
  const report = useDeliveredReport(range.from, range.to);
  const rows = report.data?.rows ?? [];

  const hasAnyHours = rows.some((row) => row.hours !== undefined);
  const totalHours = rows.reduce((sum, row) => sum + (row.hours ?? 0), 0);
  const grouped = useMemo(() => groupDeliveredRows(rows, groupBy), [rows, groupBy]);
  const groupColumnLabel = GROUP_BY_OPTIONS.find((option) => option.value === groupBy)?.label ?? '';

  async function exportXlsx() {
    setExportError(null);
    setExporting(true);
    try {
      const qs = new URLSearchParams({ from: range.from, to: range.to }).toString();
      await downloadFile(api, `/training/v1/exports/delivered.xlsx?${qs}`, 'formazione-erogato.xlsx');
      toast('Esportazione avviata');
    } catch (error) {
      setExportError(describeApiError(error, 'Esportazione non riuscita'));
    } finally {
      setExporting(false);
    }
  }

  return (
    <section className={styles.section}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>Erogato</h2>
          <p className={styles.sectionNote}>
            Nel raggruppamento per team la persona conta in ciascuno dei suoi team correnti: i totali per team non
            sommano al totale complessivo.
          </p>
        </div>
        <div className={styles.sectionActions}>
          <div className={styles.dateRange}>
            <label className={styles.dateField}>
              Dal
              <input
                type="date"
                className={styles.dateInput}
                value={range.from}
                onChange={(e) => setRange((current) => ({ ...current, from: e.target.value }))}
              />
            </label>
            <label className={styles.dateField}>
              Al
              <input
                type="date"
                className={styles.dateInput}
                value={range.to}
                onChange={(e) => setRange((current) => ({ ...current, to: e.target.value }))}
              />
            </label>
          </div>
          <SingleSelect
            options={GROUP_BY_OPTIONS}
            selected={groupBy}
            onChange={(v) => setGroupBy(v ?? 'course')}
            ariaLabel="Raggruppa per"
          />
          <Button
            variant="secondary"
            size="sm"
            leftIcon={<Icon name="download" size={14} />}
            loading={exporting}
            disabled={rows.length === 0}
            onClick={exportXlsx}
          >
            Esporta XLSX
          </Button>
        </div>
      </div>

      <ErrorPanel message={exportError} onDismiss={() => setExportError(null)} />

      {report.isLoading ? (
        <Skeleton rows={4} />
      ) : report.isError ? (
        <p className={styles.errorNotice}>{describeApiError(report.error, "Lettura dell'erogato non riuscita")}</p>
      ) : rows.length === 0 ? (
        <p className={styles.emptyNotice}>Nessun completamento nel periodo selezionato.</p>
      ) : (
        <>
          <div className={styles.statsRow}>
            <div className={styles.statTile}>
              <span className={styles.statLabel}>Completamenti</span>
              <span className={styles.statValue}>{formatNumber(rows.length)}</span>
            </div>
            <div className={styles.statTile}>
              <span className={styles.statLabel}>Ore erogate</span>
              <span className={styles.statValue}>{hasAnyHours ? formatNumber(totalHours) : '—'}</span>
            </div>
          </div>

          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>{groupColumnLabel}</th>
                  <th className={styles.numCell}>Completamenti</th>
                  <th className={styles.numCell}>Ore</th>
                </tr>
              </thead>
              <tbody>
                {grouped.map((group) => (
                  <tr key={group.key}>
                    <td>{group.label}</td>
                    <td className={styles.numCell}>{formatNumber(group.completions)}</td>
                    <td className={styles.numCell}>{group.hasHours ? formatNumber(group.hours) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Persona</th>
                  <th>Team</th>
                  <th>Corso</th>
                  <th>Evento</th>
                  <th>Stato</th>
                  <th>Esito</th>
                  <th>Data riferimento</th>
                  <th className={styles.numCell}>Ore</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.enrollmentId}>
                    <td>
                      <Link to={`/persone/${row.employeeId}`} className={styles.rowLink}>
                        {row.employeeName}
                      </Link>
                    </td>
                    <td>{row.teams.map((t) => t.name).join(', ') || '—'}</td>
                    <td>{row.courseTitle}</td>
                    <td>
                      <Link to={`/eventi/${row.eventId}`} className={styles.rowLink}>
                        Apri evento
                      </Link>
                    </td>
                    <td>
                      <StatusBadge
                        value={row.deliveryStatus}
                        label={DELIVERY_STATUS_LABELS[row.deliveryStatus]}
                        variant={deliveryStatusVariant(row.deliveryStatus)}
                      />
                    </td>
                    <td>{row.learningOutcome ? LEARNING_OUTCOME_LABELS[row.learningOutcome] : '—'}</td>
                    <td>{formatDateOnly(row.referenceDate)}</td>
                    <td className={styles.numCell}>{row.hours !== undefined ? formatNumber(row.hours) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </section>
  );
}
