import type { ReactNode } from 'react';
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { formatCurrency, formatInstant, formatLocalDate, formatNumber } from '@mrsmith/format';
import { Icon, Skeleton, StatusBadge, type StatusBadgeVariant } from '@mrsmith/ui';
import {
  useExpiringCoverage,
  useRequestsAwaitingDecision,
  useRequestsWithoutTLOpinion,
  useRoundsWithoutEvent,
  useSeatRuleCoverage,
  useStaleEnrollments,
  useTrainingEvents,
  useUnapprovedEventExpenses,
  useUnfedPopulation,
} from '../../api/queries';
import type {
  EconomicState,
  EventListRow,
  ExpiringPersonRow,
  ExpiringSeatRuleRow,
  RequestAwaitingDecisionRow,
  RequestWithoutTLOpinionRow,
  RoundWithoutEventRow,
  SeatRuleCoverageRow,
  StaleEnrollmentRow,
  TLOpinion,
  UnapprovedEventExpenseRow,
  UnfedPopulationRow,
} from '../../api/types';
import {
  ECONOMIC_STATE_LABELS,
  EVENT_CONDITION_LABELS,
  NEED_LABELS,
  TL_OPINION_LABELS,
} from '../../lib/labels';
import styles from './WorkQueuePage.module.css';

const DEFAULT_WITHIN_DAYS = 60;
const DEFAULT_OLDER_THAN_DAYS = 30;
const MIN_DAYS = 1;
const MAX_DAYS = 365;

function parseDaysParam(raw: string | null, fallback: number): number {
  if (!raw) return fallback;
  const parsed = Number(raw);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.min(MAX_DAYS, Math.max(MIN_DAYS, Math.round(parsed)));
}

function formatDate(value: string | undefined | null): string {
  if (!value) return '—';
  return formatLocalDate(value) ?? value;
}

// Normalizza il cast testuale di una colonna timestamptz di Postgres
// ("2026-08-27 21:00:00+00") in RFC 3339 ("2026-08-27T21:00:00+00:00"):
// separatore spazio → "T", offset a due cifre → offset a quattro cifre.
// "Z" resta invariato.
function toRfc3339Instant(value: string): string {
  const withT = value.includes('T') ? value : value.replace(' ', 'T');
  const offsetMatch = /^(.*)([+-]\d{2})$/.exec(withT);
  return offsetMatch ? `${offsetMatch[1]}${offsetMatch[2]}:00` : withT;
}

// Per i valori con offset (es. lastSessionDate, derivato da timestamptz):
// il giorno civile va calcolato nel fuso di Europe/Rome, non troncando il
// timestamp del server. Vedi docs/UI-UX.md §17.
function formatInstantDate(value: string | undefined | null): string {
  if (!value) return '—';
  return (
    formatInstant(toRfc3339Instant(value), {
      format: { day: '2-digit', month: '2-digit', year: 'numeric' },
    }) ?? value
  );
}

function ageLabel(value: number): string {
  return `${formatNumber(value) ?? value} gg`;
}

function names(list: { name?: string; employeeName?: string }[]): string {
  const values = list.map((item) => item.employeeName ?? item.name ?? '').filter(Boolean);
  return values.length > 0 ? values.join(', ') : '—';
}

function tlOpinionVariant(value: TLOpinion): StatusBadgeVariant {
  return value === 'favorable' ? 'success' : 'danger';
}

function economicStateVariant(value: EconomicState): StatusBadgeVariant {
  if (value === 'approved') return 'success';
  if (value === 'rejected') return 'danger';
  return 'warning';
}

// ── Controllo dell'orizzonte/soglia in giorni, stato nell'URL ──

function DaysField({
  id,
  label,
  value,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  onChange: (next: number) => void;
}) {
  const [draft, setDraft] = useState(String(value));

  useEffect(() => setDraft(String(value)), [value]);

  function commit() {
    const parsed = Number(draft);
    const clamped = Number.isFinite(parsed)
      ? Math.min(MAX_DAYS, Math.max(MIN_DAYS, Math.round(parsed)))
      : value;
    setDraft(String(clamped));
    if (clamped !== value) onChange(clamped);
  }

  return (
    <div className={styles.daysField}>
      <label htmlFor={id} className={styles.daysFieldLabel}>
        {label}
      </label>
      <input
        id={id}
        type="number"
        min={MIN_DAYS}
        max={MAX_DAYS}
        className={styles.daysFieldInput}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => {
          if (event.key === 'Enter') event.currentTarget.blur();
        }}
      />
      <span className={styles.daysFieldUnit}>giorni</span>
    </div>
  );
}

// ── Tabella generica a colonne, riusata da tutte le sottosezioni ──

interface QueueColumn<T> {
  header: string;
  align?: 'right';
  render: (row: T) => ReactNode;
}

function QueueTable<T>({
  rows,
  columns,
  rowKey,
}: {
  rows: T[];
  columns: QueueColumn<T>[];
  rowKey: (row: T) => string;
}) {
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            {columns.map((col) => (
              <th key={col.header} className={col.align === 'right' ? styles.numCell : undefined}>
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={rowKey(row)}>
              {columns.map((col) => (
                <td key={col.header} className={col.align === 'right' ? styles.numCell : undefined}>
                  {col.render(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ── Sezione di coda: intestazione con conteggio + stati loading/error/empty ──

function QueueSection({
  title,
  count,
  isLoading,
  isError,
  isEmpty,
  emptyMessage,
  children,
}: {
  title: string;
  count?: number;
  isLoading: boolean;
  isError: boolean;
  isEmpty: boolean;
  emptyMessage: string;
  children: ReactNode;
}) {
  return (
    <div className={styles.section}>
      <div className={styles.sectionHeading}>
        <h3 className={styles.sectionTitle}>{title}</h3>
        {!isLoading && !isError && (
          <span className={styles.sectionCount}>{formatNumber(count ?? 0) ?? count}</span>
        )}
      </div>
      {isLoading ? (
        <Skeleton rows={3} />
      ) : isError ? (
        <p className={styles.errorNotice}>Lettura non riuscita. Riprovare più tardi.</p>
      ) : isEmpty ? (
        <p className={styles.emptyNotice}>{emptyMessage}</p>
      ) : (
        children
      )}
    </div>
  );
}

export function WorkQueuePage() {
  const [params, setParams] = useSearchParams();
  const withinDays = parseDaysParam(params.get('withinDays'), DEFAULT_WITHIN_DAYS);
  const olderThanDays = parseDaysParam(params.get('olderThanDays'), DEFAULT_OLDER_THAN_DAYS);

  function setWithinDays(next: number) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('withinDays', String(next));
    setParams(nextParams, { replace: true });
  }

  function setOlderThanDays(next: number) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('olderThanDays', String(next));
    setParams(nextParams, { replace: true });
  }

  const withoutOpinion = useRequestsWithoutTLOpinion();
  const awaitingDecision = useRequestsAwaitingDecision();
  const seatCoverage = useSeatRuleCoverage();
  const expiring = useExpiringCoverage(withinDays);
  const unfed = useUnfedPopulation();
  const rounds = useRoundsWithoutEvent(withinDays);
  const expenses = useUnapprovedEventExpenses();
  const stale = useStaleEnrollments(olderThanDays);
  const events = useTrainingEvents();

  const needsReconciliation = (events.data ?? []).filter((e) => e.flags.needsReconciliation);
  const withoutSessions = (events.data ?? []).filter((e) => e.flags.withoutSessions);
  const unassignedEnrollments = (events.data ?? []).filter((e) => e.flags.unassignedEnrollments);

  const queries = [withoutOpinion, awaitingDecision, seatCoverage, expiring, unfed, rounds, expenses, stale, events];
  const allLoaded = queries.every((q) => !q.isLoading);
  const anyError = queries.some((q) => q.isError);
  const totalCount =
    (withoutOpinion.data?.length ?? 0) +
    (awaitingDecision.data?.length ?? 0) +
    (seatCoverage.data?.length ?? 0) +
    (expiring.data ? expiring.data.people.length + expiring.data.seatRules.length : 0) +
    (unfed.data?.length ?? 0) +
    (rounds.data?.rules.length ?? 0) +
    (expenses.data?.length ?? 0) +
    (stale.data?.enrollments.length ?? 0) +
    needsReconciliation.length +
    withoutSessions.length +
    unassignedEnrollments.length;
  const showPageEmpty = allLoaded && !anyError && totalCount === 0;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>Coda di lavoro</h1>
        <p className={styles.subtitle}>
          Richieste, coperture, spese ed eventi che richiedono attenzione, raggruppati per tipo.
        </p>
      </header>

      {showPageEmpty ? (
        <>
          <div className={styles.pageEmptyControls}>
            <DaysField id="within-days" label="Orizzonte" value={withinDays} onChange={setWithinDays} />
            <DaysField id="older-than-days" label="Soglia" value={olderThanDays} onChange={setOlderThanDays} />
          </div>
          <div className={styles.pageEmpty}>
            <div className={styles.pageEmptyIcon}>
              <Icon name="check-circle" size={32} />
            </div>
            <p className={styles.pageEmptyTitle}>Nessun elemento in coda</p>
            <p className={styles.pageEmptyDescription}>
              Nessuna richiesta, copertura, spesa o iscrizione in attesa di intervento entro la finestra impostata.
            </p>
          </div>
        </>
      ) : (
        <>
          <section className={styles.group}>
            <h2 className={styles.groupTitle}>Richieste</h2>
            <QueueSection
              title="Senza parere TL"
              count={withoutOpinion.data?.length}
              isLoading={withoutOpinion.isLoading}
              isError={withoutOpinion.isError}
              isEmpty={(withoutOpinion.data?.length ?? 0) === 0}
              emptyMessage="Nessuna richiesta senza parere TL."
            >
              <QueueTable<RequestWithoutTLOpinionRow>
                rows={withoutOpinion.data ?? []}
                rowKey={(r) => r.requestId}
                columns={[
                  { header: 'Persona', render: (r) => r.employeeName },
                  { header: 'Corso / titolo', render: (r) => r.courseTitle || r.freeTextTitle || '—' },
                  { header: 'Team', render: (r) => r.selectedTeamName },
                  { header: 'Lead abilitati', render: (r) => names(r.teamLeads) },
                  { header: 'Età', align: 'right', render: (r) => ageLabel(r.ageDays) },
                ]}
              />
            </QueueSection>

            <QueueSection
              title="Con parere, senza decisione"
              count={awaitingDecision.data?.length}
              isLoading={awaitingDecision.isLoading}
              isError={awaitingDecision.isError}
              isEmpty={(awaitingDecision.data?.length ?? 0) === 0}
              emptyMessage="Nessuna richiesta in attesa di decisione People."
            >
              <QueueTable<RequestAwaitingDecisionRow>
                rows={awaitingDecision.data ?? []}
                rowKey={(r) => r.requestId}
                columns={[
                  { header: 'Persona', render: (r) => r.employeeName },
                  { header: 'Corso / titolo', render: (r) => r.courseTitle || r.freeTextTitle || '—' },
                  { header: 'Team', render: (r) => r.selectedTeamName },
                  {
                    header: 'Parere',
                    render: (r) => (
                      <StatusBadge
                        value={r.tlOpinion}
                        label={TL_OPINION_LABELS[r.tlOpinion]}
                        variant={tlOpinionVariant(r.tlOpinion)}
                      />
                    ),
                  },
                  { header: 'Motivazione', render: (r) => r.tlOpinionReason || '—' },
                  { header: 'Età', align: 'right', render: (r) => ageLabel(r.ageDays) },
                ]}
              />
            </QueueSection>
          </section>

          <section className={styles.group}>
            <div className={styles.groupHeader}>
              <h2 className={styles.groupTitle}>Coperture</h2>
              <DaysField id="within-days" label="Orizzonte" value={withinDays} onChange={setWithinDays} />
            </div>
            <QueueSection
              title="In scadenza"
              count={expiring.data ? expiring.data.people.length + expiring.data.seatRules.length : undefined}
              isLoading={expiring.isLoading}
              isError={expiring.isError}
              isEmpty={!!expiring.data && expiring.data.people.length === 0 && expiring.data.seatRules.length === 0}
              emptyMessage="Nessuna copertura in scadenza entro l'orizzonte impostato."
            >
              {expiring.data && expiring.data.people.length > 0 && (
                <div className={styles.subgroup}>
                  <h4 className={styles.subgroupTitle}>
                    Persone ({formatNumber(expiring.data.people.length)})
                  </h4>
                  <QueueTable<ExpiringPersonRow>
                    rows={expiring.data.people}
                    rowKey={(r) => `${r.ruleId}-${r.employeeId}`}
                    columns={[
                      { header: 'Persona', render: (r) => r.employeeName },
                      { header: 'Regola', render: (r) => r.ruleName },
                      { header: 'Corso', render: (r) => r.courseTitle },
                      { header: 'Bisogno', render: (r) => NEED_LABELS[r.need] ?? r.need },
                      { header: 'Scadenza', render: (r) => formatDate(r.deadline) },
                      { header: 'Giorni', align: 'right', render: (r) => ageLabel(r.daysUntil) },
                    ]}
                  />
                </div>
              )}
              {expiring.data && expiring.data.seatRules.length > 0 && (
                <div className={styles.subgroup}>
                  <h4 className={styles.subgroupTitle}>
                    Regole a posizioni ({formatNumber(expiring.data.seatRules.length)})
                  </h4>
                  <QueueTable<ExpiringSeatRuleRow>
                    rows={expiring.data.seatRules}
                    rowKey={(r) => r.ruleId}
                    columns={[
                      { header: 'Regola', render: (r) => r.ruleName },
                      { header: 'Corso', render: (r) => r.courseTitle },
                      { header: 'Posizioni', align: 'right', render: (r) => formatNumber(r.seatCount) },
                      { header: 'Valide oggi', align: 'right', render: (r) => formatNumber(r.validToday) },
                      {
                        header: 'Valide a orizzonte',
                        align: 'right',
                        render: (r) => formatNumber(r.validAtHorizon),
                      },
                    ]}
                  />
                </div>
              )}
            </QueueSection>

            <QueueSection
              title="Posizioni scoperte"
              count={seatCoverage.data?.length}
              isLoading={seatCoverage.isLoading}
              isError={seatCoverage.isError}
              isEmpty={(seatCoverage.data?.length ?? 0) === 0}
              emptyMessage="Nessuna regola attiva a posizioni."
            >
              <QueueTable<SeatRuleCoverageRow>
                rows={seatCoverage.data ?? []}
                rowKey={(r) => r.ruleId}
                columns={[
                  { header: 'Regola', render: (r) => r.ruleName },
                  { header: 'Corso', render: (r) => r.courseTitle },
                  { header: 'Bisogno', render: (r) => NEED_LABELS[r.need] ?? r.need },
                  { header: 'Scadenza', render: (r) => formatDate(r.deadline) },
                  { header: 'Posizioni', align: 'right', render: (r) => formatNumber(r.seatCount) },
                  { header: 'Coperte', align: 'right', render: (r) => formatNumber(r.covered) },
                  { header: 'Mancanti', align: 'right', render: (r) => formatNumber(r.missing) },
                  { header: 'In formazione', render: (r) => names(r.inTraining) },
                ]}
              />
            </QueueSection>

            <QueueSection
              title="Platea non alimentata"
              count={unfed.data?.length}
              isLoading={unfed.isLoading}
              isError={unfed.isError}
              isEmpty={(unfed.data?.length ?? 0) === 0}
              emptyMessage="Nessuna tornata in corso con platea da alimentare."
            >
              <QueueTable<UnfedPopulationRow>
                rows={unfed.data ?? []}
                rowKey={(r) => `${r.ruleId}-${r.eventId}`}
                columns={[
                  { header: 'Regola', render: (r) => r.ruleName },
                  { header: 'Corso', render: (r) => r.courseTitle },
                  { header: 'Scadenza tornata', render: (r) => formatDate(r.roundDeadline) },
                  { header: 'Persone da aggiungere', align: 'right', render: (r) => formatNumber(r.members.length) },
                  { header: 'Persone', render: (r) => names(r.members) },
                ]}
              />
            </QueueSection>

            <QueueSection
              title="Tornata in arrivo senza evento"
              count={rounds.data?.rules.length}
              isLoading={rounds.isLoading}
              isError={rounds.isError}
              isEmpty={(rounds.data?.rules.length ?? 0) === 0}
              emptyMessage="Nessuna tornata in arrivo priva di evento."
            >
              <QueueTable<RoundWithoutEventRow>
                rows={rounds.data?.rules ?? []}
                rowKey={(r) => r.ruleId}
                columns={[
                  { header: 'Regola', render: (r) => r.ruleName },
                  { header: 'Corso', render: (r) => r.courseTitle },
                  { header: 'Bisogno', render: (r) => NEED_LABELS[r.need] ?? r.need },
                  { header: 'Prossima scadenza', render: (r) => formatDate(r.nextRoundDeadline) },
                  { header: 'Giorni', align: 'right', render: (r) => ageLabel(r.daysUntil) },
                  { header: 'Prima tornata', render: (r) => (r.firstRound ? 'Sì' : 'No') },
                ]}
              />
            </QueueSection>
          </section>

          <section className={styles.group}>
            <h2 className={styles.groupTitle}>Spese</h2>
            <QueueSection
              title="Voci non approvate"
              count={expenses.data?.length}
              isLoading={expenses.isLoading}
              isError={expenses.isError}
              isEmpty={(expenses.data?.length ?? 0) === 0}
              emptyMessage="Nessuna voce di spesa in attesa di approvazione."
            >
              <QueueTable<UnapprovedEventExpenseRow>
                rows={expenses.data ?? []}
                rowKey={(r) => r.expenseId}
                columns={[
                  { header: 'Corso', render: (r) => r.courseTitle },
                  { header: 'PO', render: (r) => r.poCode || String(r.poId) },
                  {
                    header: 'Stato',
                    render: (r) => (
                      <StatusBadge
                        value={r.economicState}
                        label={ECONOMIC_STATE_LABELS[r.economicState]}
                        variant={economicStateVariant(r.economicState)}
                      />
                    ),
                  },
                  {
                    header: 'Importo',
                    align: 'right',
                    render: (r) => formatCurrency(Number(r.totalPrice), r.currency || 'EUR') ?? r.totalPrice,
                  },
                  { header: 'Budget', render: (r) => r.budget.name || '—' },
                  {
                    header: 'Iscrizioni coperte',
                    align: 'right',
                    render: (r) => formatNumber(r.enrollmentCount),
                  },
                ]}
              />
            </QueueSection>
          </section>

          <section className={styles.group}>
            <div className={styles.groupHeader}>
              <h2 className={styles.groupTitle}>Iscrizioni ferme</h2>
              <DaysField id="older-than-days" label="Soglia" value={olderThanDays} onChange={setOlderThanDays} />
            </div>
            <QueueSection
              title="Oltre la soglia"
              count={stale.data?.enrollments.length}
              isLoading={stale.isLoading}
              isError={stale.isError}
              isEmpty={(stale.data?.enrollments.length ?? 0) === 0}
              emptyMessage="Nessuna iscrizione ferma oltre la soglia impostata."
            >
              <QueueTable<StaleEnrollmentRow>
                rows={stale.data?.enrollments ?? []}
                rowKey={(r) => r.enrollmentId}
                columns={[
                  { header: 'Persona', render: (r) => r.employeeName },
                  { header: 'Corso', render: (r) => r.courseTitle },
                  { header: 'Età', align: 'right', render: (r) => ageLabel(r.ageDays) },
                  {
                    header: 'Ultima sessione',
                    render: (r) => (r.lastSessionDate ? formatInstantDate(r.lastSessionDate) : 'Nessuna'),
                  },
                ]}
              />
            </QueueSection>
          </section>

          <section className={styles.group}>
            <h2 className={styles.groupTitle}>Eventi da sistemare</h2>
            {(
              [
                ['needsReconciliation', needsReconciliation, 'Nessun evento da riconciliare.'],
                ['withoutSessions', withoutSessions, 'Nessun evento senza sessioni.'],
                ['unassignedEnrollments', unassignedEnrollments, 'Nessun evento con iscrizioni non assegnate.'],
              ] as const
            ).map(([flag, rows, emptyMessage]) => (
              <QueueSection
                key={flag}
                title={EVENT_CONDITION_LABELS[flag]}
                count={events.isSuccess ? rows.length : undefined}
                isLoading={events.isLoading}
                isError={events.isError}
                isEmpty={rows.length === 0}
                emptyMessage={emptyMessage}
              >
                <QueueTable<EventListRow>
                  rows={rows}
                  rowKey={(r) => r.id}
                  columns={[
                    { header: 'Corso', render: (r) => r.courseTitle },
                    { header: 'Fornitore', render: (r) => r.vendorName || '—' },
                    { header: 'Sessioni', align: 'right', render: (r) => formatNumber(r.sessionsCount) },
                    { header: 'Iscrizioni', align: 'right', render: (r) => formatNumber(r.enrollmentsCount) },
                    {
                      header: 'Iscrizioni annullate',
                      align: 'right',
                      render: (r) => formatNumber(r.cancelledEnrollmentsCount),
                    },
                  ]}
                />
              </QueueSection>
            ))}
          </section>
        </>
      )}
    </main>
  );
}
