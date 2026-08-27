import { useEffect, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { formatCurrency } from '@mrsmith/format';
import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useCancelEvent, useEventDetail, useFeedEvent, useTrainingLookups, useUpdateEvent } from '../../api/queries';
import type { EventInput } from '../../api/types';
import { describeApiError } from '../../components/events/apiErrors';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { EventConditionBadges } from '../../components/events/EventConditionBadges';
import { EventFormModal } from '../../components/events/EventFormModal';
import { formatInstantDate } from '../../components/events/eventFormat';
import { ReasonDialog } from '../../components/events/ReasonDialog';
import { EVENT_ORIGIN_LABELS } from '../../lib/labels';
import { EnrollmentsSection } from './EnrollmentsSection';
import { ExpensesSection } from './ExpensesSection';
import { ParticipationsSection } from './ParticipationsSection';
import { SessionsSection } from './SessionsSection';
import styles from './EventDetailPage.module.css';

export function EventDetailPage() {
  const { id = '' } = useParams();
  const { toast } = useToast();
  const [params] = useSearchParams();
  const highlightParam = params.get('highlight');
  const [highlighted, setHighlighted] = useState<string | null>(null);

  const detail = useEventDetail(id);
  const lookups = useTrainingLookups();
  const updateEvent = useUpdateEvent();
  const cancelEvent = useCancelEvent();
  const feedEvent = useFeedEvent();

  const [showEdit, setShowEdit] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);
  const [showCancel, setShowCancel] = useState(false);
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [feedError, setFeedError] = useState<string | null>(null);

  useEffect(() => {
    if (!highlightParam || !detail.isSuccess) return;
    setHighlighted(highlightParam);
    document.getElementById(`section-${highlightParam}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    const timer = setTimeout(() => setHighlighted(null), 2500);
    return () => clearTimeout(timer);
  }, [highlightParam, detail.isSuccess]);

  if (detail.isLoading) {
    return (
      <main className={styles.page}>
        <Skeleton rows={8} />
      </main>
    );
  }

  if (detail.isError || !detail.data) {
    return (
      <main className={styles.page}>
        <Link to="/eventi" className={styles.backLink}>
          <Icon name="arrow-left" size={16} /> Eventi
        </Link>
        <p className={styles.errorNotice}>
          Evento non disponibile: la lettura non è riuscita. Riprovare più tardi o tornare all'elenco.
        </p>
      </main>
    );
  }

  const event = detail.data;

  async function handleUpdate(input: EventInput) {
    setEditError(null);
    try {
      await updateEvent.mutateAsync({ id, input });
      setShowEdit(false);
      toast('Evento aggiornato');
    } catch (error) {
      setEditError(describeApiError(error, 'Salvataggio non riuscito'));
    }
  }

  async function handleCancel(reason: string) {
    setCancelError(null);
    try {
      await cancelEvent.mutateAsync({ id, input: { reason } });
      setShowCancel(false);
      toast('Evento annullato');
    } catch (error) {
      setCancelError(describeApiError(error, 'Annullamento non riuscito'));
    }
  }

  async function handleFeed() {
    setFeedError(null);
    try {
      const response = await feedEvent.mutateAsync(id);
      toast(
        response.added.length > 0
          ? `${response.added.length} persone aggiunte alla platea`
          : 'Nessuna persona da aggiungere: la platea è già coperta',
      );
    } catch (error) {
      setFeedError(describeApiError(error, 'Alimentazione della platea non riuscita'));
    }
  }

  return (
    <main className={styles.page}>
      <Link to="/eventi" className={styles.backLink}>
        <Icon name="arrow-left" size={16} /> Eventi
      </Link>

      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>{event.courseTitle}</h1>
          <p className={styles.subtitle}>{event.vendorName || 'Nessun fornitore'}</p>
        </div>
        <div className={styles.headerActions}>
          {event.origin === 'rule' && (
            <Button variant="secondary" size="md" loading={feedEvent.isPending} onClick={handleFeed}>
              Alimenta platea
            </Button>
          )}
          <Button variant="secondary" size="md" onClick={() => setShowEdit(true)}>
            Modifica
          </Button>
          {!event.flags.cancelled && (
            <Button variant="danger" size="md" onClick={() => setShowCancel(true)}>
              Annulla evento
            </Button>
          )}
        </div>
      </header>

      <EventConditionBadges flags={event.flags} />
      <ErrorPanel message={feedError} onDismiss={() => setFeedError(null)} />

      <section className={styles.card}>
        <dl className={styles.detailGrid}>
          <div className={styles.detailItem}>
            <dt>Prezzo pattuito</dt>
            <dd>{event.agreedPrice !== undefined ? (formatCurrency(event.agreedPrice) ?? '—') : '—'}</dd>
          </div>
          <div className={styles.detailItem}>
            <dt>Condizioni</dt>
            <dd>{event.agreedConditions || '—'}</dd>
          </div>
          <div className={styles.detailItem}>
            <dt>Origine</dt>
            <dd>
              {EVENT_ORIGIN_LABELS[event.origin] ?? event.origin}
              {event.sourceRuleId && <span className={styles.mutedInline}> · regola {event.sourceRuleId}</span>}
              {event.sourceRequestId && (
                <span className={styles.mutedInline}> · richiesta {event.sourceRequestId}</span>
              )}
              {event.factorialClassId && (
                <span className={styles.mutedInline}> · classe Factorial {event.factorialClassId}</span>
              )}
              {event.ruleDeadline && (
                <span className={styles.mutedInline}> · scadenza tornata {formatInstantDate(event.ruleDeadline)}</span>
              )}
            </dd>
          </div>
          <div className={styles.detailItem}>
            <dt>Note</dt>
            <dd>{event.notes || '—'}</dd>
          </div>
        </dl>
        {event.flags.cancelled && (
          <p className={styles.cancelledNotice}>
            Annullato: {event.cancellationReason || 'nessuna motivazione registrata'}
          </p>
        )}
      </section>

      <SessionsSection
        eventId={id}
        sessions={event.sessions}
        highlighted={highlighted === 'sessions'}
      />
      <EnrollmentsSection
        eventId={id}
        enrollments={event.enrollments}
        participations={event.participations}
        employees={lookups.data?.employees ?? []}
        highlighted={highlighted === 'enrollments'}
      />
      <ParticipationsSection
        eventId={id}
        sessions={event.sessions}
        enrollments={event.enrollments}
        participations={event.participations}
        highlighted={highlighted === 'participations'}
      />
      <ExpensesSection
        eventId={id}
        expenses={event.expenses}
        enrollments={event.enrollments}
        highlighted={highlighted === 'expenses'}
      />

      {showEdit && (
        <EventFormModal
          key={event.updatedAt}
          open={showEdit}
          mode="edit"
          initial={{
            courseId: event.courseId,
            vendorId: event.vendorId,
            agreedPrice: event.agreedPrice,
            agreedConditions: event.agreedConditions,
            notes: event.notes,
          }}
          courses={lookups.data?.courses ?? []}
          vendors={lookups.data?.vendors ?? []}
          pending={updateEvent.isPending}
          error={editError}
          onSubmit={handleUpdate}
          onClose={() => {
            setShowEdit(false);
            setEditError(null);
          }}
        />
      )}

      {showCancel && (
        <ReasonDialog
          open
          title="Annulla evento"
          description="Le iscrizioni non terminali verranno annullate con la stessa motivazione. Sessioni e presenze restano registrate."
          confirmLabel="Annulla evento"
          pending={cancelEvent.isPending}
          error={cancelError}
          onConfirm={handleCancel}
          onClose={() => {
            setShowCancel(false);
            setCancelError(null);
          }}
        />
      )}
    </main>
  );
}
