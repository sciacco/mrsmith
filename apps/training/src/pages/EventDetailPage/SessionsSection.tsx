// Sessioni dell'evento: tabella, creazione/modifica, eliminazione con
// conferma. I 409/422 del backend (relazioni presenti, capienza sotto
// occupazione) si mostrano per intero, nessuna prevenzione locale (#156).

import { useState } from 'react';
import { formatNumber } from '@mrsmith/format';
import { Button, Icon, Modal, SingleSelect, useToast, VisuallyHidden } from '@mrsmith/ui';
import { useCreateSession, useDeleteSession, useUpdateSession } from '../../api/queries';
import type { ScheduleType, SessionDetail, SessionInput } from '../../api/types';
import { describeApiError } from '../../components/events/apiErrors';
import {
  dateValueToUtcMidnightInstant,
  formatInstantDate,
  formatInstantDateTime,
  instantToLocalDateTimeValue,
  instantToUtcDateValue,
  localDateTimeValueToInstant,
} from '../../components/events/eventFormat';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { SCHEDULE_TYPE_LABELS } from '../../lib/labels';
import styles from './EventDetailPage.module.css';

interface SessionsSectionProps {
  eventId: string;
  sessions: SessionDetail[];
  highlighted: boolean;
}

export function SessionsSection({ eventId, sessions, highlighted }: SessionsSectionProps) {
  const { toast } = useToast();
  const createSession = useCreateSession();
  const updateSession = useUpdateSession();
  const deleteSession = useDeleteSession();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<SessionDetail | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SessionDetail | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  function openCreate() {
    setEditing(null);
    setFormError(null);
    setFormOpen(true);
  }

  function openEdit(session: SessionDetail) {
    setEditing(session);
    setFormError(null);
    setFormOpen(true);
  }

  async function submitForm(input: SessionInput) {
    setFormError(null);
    try {
      if (editing) {
        await updateSession.mutateAsync({ id: editing.id, input });
        toast('Sessione aggiornata');
      } else {
        await createSession.mutateAsync({ eventId, input });
        toast('Sessione creata');
      }
      setFormOpen(false);
    } catch (error) {
      setFormError(describeApiError(error, 'Salvataggio non riuscito'));
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleteError(null);
    try {
      await deleteSession.mutateAsync(deleteTarget.id);
      toast('Sessione eliminata');
      setDeleteTarget(null);
    } catch (error) {
      setDeleteError(describeApiError(error, 'Eliminazione non riuscita'));
    }
  }

  return (
    <section id="section-sessions" className={`${styles.section} ${highlighted ? styles.sectionHighlighted : ''}`}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Sessioni</h2>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={openCreate}>
          Aggiungi sessione
        </Button>
      </div>

      {sessions.length === 0 ? (
        <p className={styles.emptyNotice}>Nessuna sessione: l'evento non ha ancora un calendario.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Tipo</th>
                <th>Date</th>
                <th>Scadenza</th>
                <th className={styles.numCell}>Capienza</th>
                <th className={styles.numCell}>Occupazione</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {sessions.map((session) => (
                <tr key={session.id}>
                  <td>{SCHEDULE_TYPE_LABELS[session.scheduleType ?? ''] ?? '—'}</td>
                  <td>
                    {session.startsAt ? formatInstantDateTime(session.startsAt) : '—'}
                    {session.endsAt ? ` – ${formatInstantDateTime(session.endsAt)}` : ''}
                  </td>
                  <td>{session.dueAt ? formatInstantDate(session.dueAt) : '—'}</td>
                  <td className={styles.numCell}>
                    {session.maxCapacity !== undefined ? (formatNumber(session.maxCapacity) ?? '—') : 'Illimitata'}
                  </td>
                  <td className={styles.numCell}>{formatNumber(session.occupancy) ?? session.occupancy}</td>
                  <td className={styles.rowActions}>
                    <button type="button" className={styles.linkButton} onClick={() => openEdit(session)}>
                      Modifica
                    </button>
                    <button
                      type="button"
                      className={styles.linkButtonDanger}
                      onClick={() => {
                        setDeleteTarget(session);
                        setDeleteError(null);
                      }}
                    >
                      Elimina
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {formOpen && (
        <SessionFormModal
          key={editing?.id ?? 'new'}
          open={formOpen}
          initial={editing}
          pending={createSession.isPending || updateSession.isPending}
          error={formError}
          onSubmit={submitForm}
          onClose={() => setFormOpen(false)}
        />
      )}

      <Modal open={deleteTarget !== null} onClose={() => setDeleteTarget(null)} title="Elimina sessione" size="sm">
        <div className={styles.formBody}>
          <p>Eliminare questa sessione? L'operazione non è reversibile.</p>
          <ErrorPanel message={deleteError} />
          <div className={styles.actions}>
            <Button variant="ghost" size="md" onClick={() => setDeleteTarget(null)} disabled={deleteSession.isPending}>
              Annulla
            </Button>
            <Button variant="danger" size="md" loading={deleteSession.isPending} onClick={confirmDelete}>
              Elimina
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function SessionFormModal({
  open,
  initial,
  pending,
  error,
  onSubmit,
  onClose,
}: {
  open: boolean;
  initial: SessionDetail | null;
  pending: boolean;
  error: string | null;
  onSubmit: (input: SessionInput) => void;
  onClose: () => void;
}) {
  const [scheduleType, setScheduleType] = useState<ScheduleType>(initial?.scheduleType ?? 'scheduled');
  const [startsAt, setStartsAt] = useState(instantToLocalDateTimeValue(initial?.startsAt));
  const [endsAt, setEndsAt] = useState(instantToLocalDateTimeValue(initial?.endsAt));
  const [dueAt, setDueAt] = useState(instantToUtcDateValue(initial?.dueAt));
  const [maxCapacity, setMaxCapacity] = useState(
    initial?.maxCapacity !== undefined ? String(initial.maxCapacity) : '',
  );
  const [notes, setNotes] = useState(initial?.notes ?? '');

  if (!open) return null;

  function submit() {
    onSubmit({
      scheduleType,
      startsAt: scheduleType === 'scheduled' && startsAt ? localDateTimeValueToInstant(startsAt) : undefined,
      endsAt: scheduleType === 'scheduled' && endsAt ? localDateTimeValueToInstant(endsAt) : undefined,
      dueAt: scheduleType === 'self_paced' && dueAt ? dateValueToUtcMidnightInstant(dueAt) : undefined,
      maxCapacity: maxCapacity === '' ? undefined : Number(maxCapacity),
      notes: notes.trim() || undefined,
    });
  }

  return (
    <Modal open={open} onClose={onClose} title={initial ? 'Modifica sessione' : 'Aggiungi sessione'} size="md">
      <div className={styles.formBody}>
        <label className={styles.field}>
          <span className={styles.labelHead}>
            Tipo sessione
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <SingleSelect
            options={[
              { value: 'scheduled', label: SCHEDULE_TYPE_LABELS.scheduled ?? 'scheduled' },
              { value: 'self_paced', label: SCHEDULE_TYPE_LABELS.self_paced ?? 'self_paced' },
            ]}
            selected={scheduleType}
            onChange={(v) => setScheduleType((v as ScheduleType) ?? 'scheduled')}
          />
        </label>
        {scheduleType === 'scheduled' ? (
          <div className={styles.fieldRow}>
            <label className={styles.field}>
              Inizio
              <input
                type="datetime-local"
                className={styles.input}
                value={startsAt}
                onChange={(e) => setStartsAt(e.target.value)}
              />
            </label>
            <label className={styles.field}>
              Fine
              <input
                type="datetime-local"
                className={styles.input}
                value={endsAt}
                onChange={(e) => setEndsAt(e.target.value)}
              />
            </label>
          </div>
        ) : (
          <label className={styles.field}>
            Scadenza
            <input type="date" className={styles.input} value={dueAt} onChange={(e) => setDueAt(e.target.value)} />
            <span className={styles.helperText}>Registrata a mezzanotte UTC.</span>
          </label>
        )}
        <label className={styles.field}>
          Capienza
          <input
            type="number"
            min={1}
            className={styles.input}
            value={maxCapacity}
            onChange={(e) => setMaxCapacity(e.target.value)}
            placeholder="Illimitata"
          />
        </label>
        <label className={styles.field}>
          Note
          <textarea className={styles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </label>
        <ErrorPanel message={error} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={pending} onClick={submit}>
            {initial ? 'Salva modifiche' : 'Crea sessione'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
