import { useEffect, useMemo, useState } from 'react';
import { Button, Drawer, Skeleton } from '@mrsmith/ui';
import { usePlanAudit } from '../../api/queries';
import type { PlanAuditEvent, PlanningSummary } from '../../api/types';
import styles from './PlanHistoryDrawer.module.css';

interface PlanHistoryDrawerProps {
  open: boolean;
  plan: PlanningSummary | null;
  onClose: () => void;
}

export function PlanHistoryDrawer({ open, plan, onClose }: PlanHistoryDrawerProps) {
  const [cursor, setCursor] = useState<string | undefined>();
  const [events, setEvents] = useState<PlanAuditEvent[]>([]);
  const audit = usePlanAudit(plan?.plan_id, cursor, open && !!plan?.plan_id);

  useEffect(() => {
    if (open) {
      setCursor(undefined);
      setEvents([]);
    }
  }, [open, plan?.plan_id]);

  useEffect(() => {
    if (!audit.data) return;
    setEvents((current) => {
      const seen = new Set(current.map((event) => event.id));
      const next = audit.data.events.filter((event) => !seen.has(event.id));
      return [...current, ...next];
    });
  }, [audit.data]);

  const nextCursor = audit.data?.next_cursor;

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size="lg"
      title={plan ? `Storico piano ${plan.year}` : 'Storico piano'}
      subtitle={plan ? <span className={styles.subtitle}>{statusLabel(plan.status)}</span> : undefined}
      footer={
        <div className={styles.footer}>
          <span className={styles.footerCount}>{events.length} eventi</span>
          <Button
            variant="secondary"
            size="sm"
            disabled={!nextCursor || audit.isFetching}
            loading={audit.isFetching && !!nextCursor}
            onClick={() => setCursor(nextCursor)}
          >
            Mostra precedenti
          </Button>
        </div>
      }
    >
      {audit.isLoading && events.length === 0 ? (
        <Skeleton rows={5} />
      ) : events.length === 0 ? (
        <p className={styles.empty}>Nessun evento disponibile.</p>
      ) : (
        <ol className={styles.timeline}>
          {events.map((event) => (
            <li key={event.id} className={styles.item}>
              <div className={styles.dot} aria-hidden="true" />
              <div className={styles.itemBody}>
                <div className={styles.itemHead}>
                  <span className={styles.itemTitle}>{eventLabel(event)}</span>
                  <time className={styles.time}>{formatDateTime(event.created_at)}</time>
                </div>
                <p className={styles.meta}>{event.actor.display_name}</p>
                <EventDetail event={event} />
              </div>
            </li>
          ))}
        </ol>
      )}
    </Drawer>
  );
}

export function PlanClosedSummary({ plan, onOpenHistory }: { plan: PlanningSummary; onOpenHistory: () => void }) {
  const audit = usePlanAudit(plan.plan_id, undefined, plan.status === 'closed');
  const milestones = useMemo(
    () => (audit.data?.events ?? []).filter((event) =>
      event.event_type === 'plan_created' ||
      event.event_type === 'plan_status_changed' ||
      event.event_type === 'plan_budget_changed',
    ).filter((event) => {
      if (event.event_type !== 'plan_status_changed') return true;
      return event.payload.to === 'closed' || event.payload.to === 'open';
    }).slice(0, 4),
    [audit.data],
  );

  return (
    <section className={styles.summary} aria-label={`Riepilogo piano ${plan.year}`}>
      <div className={styles.summaryHead}>
        <div>
          <h2 className={styles.summaryTitle}>Riepilogo chiusura</h2>
          <p className={styles.summarySub}>
            {formatEuro(plan.budget_spent)} allocati su {formatEuro(plan.budget_total)}
          </p>
        </div>
        <Button variant="secondary" size="sm" onClick={onOpenHistory}>Storico completo</Button>
      </div>
      {audit.isLoading ? (
        <Skeleton rows={3} />
      ) : milestones.length === 0 ? (
        <p className={styles.emptyInline}>Storico non ancora disponibile.</p>
      ) : (
        <ul className={styles.milestones}>
          {milestones.map((event) => (
            <li key={event.id} className={styles.milestone}>
              <span>{eventLabel(event)}</span>
              <time>{formatDate(event.created_at)}</time>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function EventDetail({ event }: { event: PlanAuditEvent }) {
  const detail = eventDetail(event);
  if (!detail) return null;
  return <p className={styles.detail}>{detail}</p>;
}

function eventLabel(event: PlanAuditEvent): string {
  switch (event.event_type) {
    case 'plan_created':
      return 'Piano creato';
    case 'plan_status_changed':
      if (event.payload.to === 'open') return 'Piano aperto';
      if (event.payload.to === 'closed') return 'Piano chiuso';
      if (event.payload.to === 'frozen') return 'Piano congelato';
      return 'Stato piano aggiornato';
    case 'plan_budget_changed':
      return 'Budget aggiornato';
    case 'plan_notes_changed':
      return 'Note aggiornate';
    case 'plan_deleted':
      return 'Piano eliminato';
    case 'bulk_plan_applied':
      return 'Suggerimento pianificato';
    case 'suggestion_dismissed':
      return 'Suggerimento rimosso';
    case 'adhoc_created':
      return 'Iscrizione manuale creata';
    case 'enrollment_modified':
      return 'Iscrizione aggiornata';
    case 'enrollment_cancelled':
      return 'Iscrizione annullata';
    case 'bulk_review_applied':
      return 'Richieste revisionate';
  }
}

function eventDetail(event: PlanAuditEvent): string | null {
  const count = numericPayload(event, 'created') ?? numericPayload(event, 'succeeded');
  if (event.event_type === 'adhoc_created' && count !== undefined) return `${count} iscrizioni create`;
  if (event.event_type === 'bulk_plan_applied' && count !== undefined) return `${count} iscrizioni create`;
  if (event.event_type === 'bulk_review_applied' && count !== undefined) return `${count} richieste gestite`;
  if (event.event_type === 'plan_budget_changed') {
    const to = numericPayload(event, 'to');
    return to !== undefined ? `Nuovo budget ${formatEuro(to)}` : null;
  }
  if (event.event_type === 'plan_status_changed' && event.payload.expired_enrollments_count) {
    return `${event.payload.expired_enrollments_count} iscrizioni scadute`;
  }
  return null;
}

function numericPayload(event: PlanAuditEvent, key: string): number | undefined {
  const value = event.payload[key];
  return typeof value === 'number' ? value : undefined;
}

function statusLabel(status: PlanningSummary['status']): string {
  switch (status) {
    case 'draft':
      return 'In preparazione';
    case 'open':
      return 'Aperto';
    case 'frozen':
      return 'Congelato';
    case 'closed':
      return 'Chiuso';
    case 'missing':
      return 'Nessun piano';
  }
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat('it-IT', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value));
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'medium' }).format(new Date(value));
}

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}
