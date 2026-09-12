import { useState, type FormEvent } from 'react';
import { Button, Modal, SingleSelect } from '@mrsmith/ui';
import { formatInstant, formatNumber } from '@mrsmith/format';
import { useAcceptNeed, useTrainingEvents } from '../../api/queries';
import type { NeedAcceptResponse, NeedDetail } from '../../api/types';
import { REQUEST_OUTCOME_LABELS } from '../../lib/labels';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import form from '../requests/requestShared.module.css';
import styles from '../../pages/NeedDetailPage/NeedDetailPage.module.css';

export function NeedAcceptModal({ need, onClose, onAccepted }: {
  need: NeedDetail;
  onClose: () => void;
  onAccepted: (response: NeedAcceptResponse) => void;
}) {
  const events = useTrainingEvents();
  const accept = useAcceptNeed();
  const [eventId, setEventId] = useState('new');
  const [reason, setReason] = useState('');
  const [error, setError] = useState<string | null>(null);
  const open = need.requests.filter((r) => !r.outcome);
  const excluded = need.requests.filter((r) => r.outcome);
  const available = (events.data ?? []).filter((e) => e.courseId === need.finalCourseId && !e.cancelledAt);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      const result = await accept.mutateAsync({ id: need.id, input: { eventId: eventId === 'new' ? undefined : eventId, reason } });
      onAccepted(result);
      onClose();
    } catch (e) { setError(describeApiError(e, 'Accoglimento non riuscito')); }
  }

  return (
    <Modal open onClose={onClose} title="Accogli su questo corso ed evento" size="lg">
      <form className={`${form.body} ${form.bodyModal}`} onSubmit={submit}>
        <div><span className={form.field}>Corso definitivo</span><strong>{need.finalCourseTitle}</strong></div>
        <div className={form.field}>Evento
          <SingleSelect<string> ariaLabel="Evento" selected={eventId} onChange={(v) => setEventId(v ?? 'new')} disabled={events.isPending || events.isError}
            options={[{ value: 'new', label: 'Nuovo evento' }, ...available.map((e) => ({ value: e.id, label: `${e.title} · creato il ${formatInstant(e.createdAt)} · ${formatNumber(e.enrollmentsCount)} ${e.enrollmentsCount === 1 ? 'iscrizione' : 'iscrizioni'}` }))]} />
        </div>
        {events.isError && <ErrorPanel message="Eventi non disponibili. Riapri il modulo per riprovare." />}
        <div>
          <h2 className={styles.sectionTitle}>Richieste da accogliere</h2>
          <ul className={styles.requestList}>{open.map((r) => <li key={r.id}><strong>{r.employeeName}</strong> · {r.description}{r.suspendedAt ? ' · Sospesa' : ''}</li>)}</ul>
          {!open.length && <p className={styles.muted}>Nessuna richiesta da accogliere.</p>}
        </div>
        {excluded.length > 0 && <div>
          <h2 className={styles.sectionTitle}>Escluse dall’accoglimento</h2>
          <ul className={styles.requestList}>{excluded.map((r) => <li key={r.id}>{r.employeeName} · {r.description} · {REQUEST_OUTCOME_LABELS[r.outcome!]}{r.acceptedCourseTitle ? ` · ${r.acceptedCourseTitle}` : ''}</li>)}</ul>
        </div>}
        <label className={form.field}>Motivazione della decisione<textarea className={form.textarea} rows={2} value={reason} onChange={(e) => setReason(e.target.value)} /></label>
        {error && <ErrorPanel message={error} />}
        <div className={form.actions}>
          <Button variant="ghost" onClick={onClose} disabled={accept.isPending}>Annulla</Button>
          <Button type="submit" loading={accept.isPending} disabled={!open.length || !need.finalCourseId || events.isPending || events.isError}>Accogli su questo corso ed evento</Button>
        </div>
      </form>
    </Modal>
  );
}
