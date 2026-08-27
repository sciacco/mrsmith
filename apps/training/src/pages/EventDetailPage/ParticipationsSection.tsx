// Matrice presenze iscritti x sessioni con i gesti massivi della slice 6.2
// (#153). Cella vuota = non assegnata; cella valorizzata = stato di
// partecipazione modificabile. Ogni esito 409/422 del backend si mostra per
// intero in un pannello inline, mai solo in un toast (#156).

import { useState } from 'react';
import { Button, Icon, Modal, MultiSelect, SingleSelect, useToast, VisuallyHidden } from '@mrsmith/ui';
import {
  useAssignParticipation,
  useBulkAssignments,
  useBulkParticipation,
  useRemoveParticipation,
  useUpdateParticipation,
} from '../../api/queries';
import type {
  BulkAssignMode,
  BulkAssignmentsResponse,
  BulkParticipationResponse,
  EnrollmentDetail,
  ParticipationRow,
  ParticipationStatus,
  SessionDetail,
} from '../../api/types';
import { describeApiError } from '../../components/events/apiErrors';
import { formatInstantDate } from '../../components/events/eventFormat';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { BULK_ASSIGN_MODE_LABELS, PARTICIPATION_STATUS_LABELS } from '../../lib/labels';
import styles from './EventDetailPage.module.css';

interface ParticipationsSectionProps {
  eventId: string;
  sessions: SessionDetail[];
  enrollments: EnrollmentDetail[];
  participations: ParticipationRow[];
  highlighted: boolean;
}

function sessionLabel(session: SessionDetail, index: number): string {
  if (session.startsAt) return `#${index + 1} · ${formatInstantDate(session.startsAt)}`;
  if (session.dueAt) return `#${index + 1} · scad. ${formatInstantDate(session.dueAt)}`;
  return `Sessione ${index + 1}`;
}

// I 409/422 del backend portano UUID grezzi nel messaggio (sessioni per il
// 422 session_capacity_insufficient: "sessione <uuid>: richiesti N,
// disponibili M"; iscrizioni per il 409 event_expense_po_not_approved:
// "avvio bloccato per N iscrizioni (<uuid>, ...): <PO bloccanti>"). Li
// sostituiamo con le etichette leggibili quando l'id è tra quelli
// dell'evento, mantenendo il resto del testo del backend.
const UUID_PATTERN = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi;

function describeIdError(error: unknown, fallback: string, labelById: Map<string, string>): string {
  const message = describeApiError(error, fallback);
  return message.replace(UUID_PATTERN, (match) => labelById.get(match) ?? match);
}

function describeBulkAssignError(error: unknown, sessions: SessionDetail[]): string {
  const labelById = new Map(sessions.map((s, i) => [s.id, sessionLabel(s, i)]));
  return describeIdError(error, 'Assegnazione non riuscita', labelById);
}

function describeBulkParticipationError(error: unknown, enrollments: EnrollmentDetail[]): string {
  const labelById = new Map(enrollments.map((e) => [e.id, e.employeeName]));
  return describeIdError(error, 'Aggiornamento non riuscito', labelById);
}

const STATUS_OPTIONS: { value: ParticipationStatus; label: string }[] = [
  { value: 'assigned', label: PARTICIPATION_STATUS_LABELS.assigned ?? 'assigned' },
  { value: 'in_progress', label: PARTICIPATION_STATUS_LABELS.in_progress ?? 'in_progress' },
  { value: 'completed', label: PARTICIPATION_STATUS_LABELS.completed ?? 'completed' },
  { value: 'not_attended', label: PARTICIPATION_STATUS_LABELS.not_attended ?? 'not_attended' },
];

export function ParticipationsSection({
  eventId,
  sessions,
  enrollments,
  participations,
  highlighted,
}: ParticipationsSectionProps) {
  const assign = useAssignParticipation();
  const remove = useRemoveParticipation();
  const update = useUpdateParticipation();
  const [cellError, setCellError] = useState<string | null>(null);
  const [bulkAssignMode, setBulkAssignMode] = useState<BulkAssignMode | null>(null);
  const [showBulkParticipation, setShowBulkParticipation] = useState(false);

  const byPair = new Map<string, ParticipationRow>();
  for (const p of participations) byPair.set(`${p.enrollmentId}|${p.sessionId}`, p);

  function handleAssign(enrollmentId: string, sessionId: string) {
    setCellError(null);
    assign.mutate(
      { enrollmentId, sessionId },
      { onError: (error) => setCellError(describeApiError(error, 'Assegnazione non riuscita')) },
    );
  }

  function handleRemove(enrollmentId: string, sessionId: string) {
    setCellError(null);
    remove.mutate(
      { enrollmentId, sessionId },
      { onError: (error) => setCellError(describeApiError(error, 'Rimozione non riuscita')) },
    );
  }

  function handleUpdate(enrollmentId: string, sessionId: string, participationStatus: ParticipationStatus) {
    setCellError(null);
    update.mutate(
      { enrollmentId, sessionId, input: { participationStatus } },
      { onError: (error) => setCellError(describeApiError(error, 'Aggiornamento non riuscito')) },
    );
  }

  return (
    <section
      id="section-participations"
      className={`${styles.section} ${highlighted ? styles.sectionHighlighted : ''}`}
    >
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Presenze</h2>
        <div className={styles.bulkBar}>
          <Button variant="secondary" size="sm" onClick={() => setBulkAssignMode('all_to_all')} disabled={sessions.length === 0}>
            Assegna tutti a tutte
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setBulkAssignMode('distribute')} disabled={sessions.length === 0}>
            Distribuisci automaticamente
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setBulkAssignMode('fill_session')} disabled={sessions.length === 0}>
            Colloca i non assegnati
          </Button>
          <Button variant="secondary" size="sm" leftIcon={<Icon name="clipboard-check" size={14} />} onClick={() => setShowBulkParticipation(true)} disabled={sessions.length === 0}>
            Segna presenze
          </Button>
        </div>
      </div>

      <ErrorPanel message={cellError} onDismiss={() => setCellError(null)} />

      {sessions.length === 0 || enrollments.length === 0 ? (
        <p className={styles.emptyNotice}>
          {sessions.length === 0 ? "Nessuna sessione: aggiungine una per registrare presenze." : 'Nessun iscritto.'}
        </p>
      ) : (
        <div className={styles.matrixWrap}>
          <table className={styles.matrixTable}>
            <thead>
              <tr>
                <th className={styles.matrixPersonCell}>Persona</th>
                {sessions.map((session, index) => (
                  <th key={session.id}>{sessionLabel(session, index)}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {enrollments.map((enrollment) => (
                <tr key={enrollment.id}>
                  <td className={styles.matrixPersonCell}>{enrollment.employeeName}</td>
                  {sessions.map((session, sessionIndex) => {
                    const participation = byPair.get(`${enrollment.id}|${session.id}`);
                    return (
                      <td key={session.id} className={styles.matrixCell}>
                        {participation ? (
                          <>
                            <select
                              className={styles.matrixSelect}
                              value={participation.participationStatus}
                              onChange={(e) =>
                                handleUpdate(enrollment.id, session.id, e.target.value as ParticipationStatus)
                              }
                              aria-label={`Presenza di ${enrollment.employeeName} a ${sessionLabel(session, sessionIndex)}`}
                            >
                              {STATUS_OPTIONS.map((option) => (
                                <option key={option.value} value={option.value}>
                                  {option.label}
                                </option>
                              ))}
                            </select>
                            <button
                              type="button"
                              className={styles.matrixRemoveButton}
                              disabled={participation.participationStatus !== 'assigned'}
                              onClick={() => handleRemove(enrollment.id, session.id)}
                              aria-label={`Rimuovi assegnazione di ${enrollment.employeeName}`}
                            >
                              <Icon name="x" size={14} />
                            </button>
                          </>
                        ) : (
                          <button
                            type="button"
                            className={styles.matrixAssignButton}
                            onClick={() => handleAssign(enrollment.id, session.id)}
                            aria-label={`Assegna ${enrollment.employeeName} alla sessione`}
                          >
                            <Icon name="plus" size={14} />
                          </button>
                        )}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <BulkAssignDialog
        key={bulkAssignMode ?? 'closed'}
        eventId={eventId}
        mode={bulkAssignMode}
        sessions={sessions}
        onClose={() => setBulkAssignMode(null)}
      />
      {showBulkParticipation && (
        <BulkParticipationDialog
          sessions={sessions}
          enrollments={enrollments}
          open={showBulkParticipation}
          onClose={() => setShowBulkParticipation(false)}
        />
      )}
    </section>
  );
}

function BulkAssignDialog({
  eventId,
  mode,
  sessions,
  onClose,
}: {
  eventId: string;
  mode: BulkAssignMode | null;
  sessions: SessionDetail[];
  onClose: () => void;
}) {
  const { toast } = useToast();
  const bulkAssign = useBulkAssignments();
  const [sessionIds, setSessionIds] = useState<string[]>([]);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<BulkAssignmentsResponse | null>(null);

  if (!mode) return null;

  const sessionOptions = sessions.map((s, i) => ({ value: s.id, label: sessionLabel(s, i) }));

  function handleClose() {
    setSessionIds([]);
    setSessionId(null);
    setError(null);
    setResult(null);
    onClose();
  }

  async function submit() {
    if (!mode) return;
    setError(null);
    try {
      const response = await bulkAssign.mutateAsync({
        eventId,
        input:
          mode === 'fill_session'
            ? { mode, sessionId: sessionId ?? '' }
            : { mode, sessionIds: sessionIds.length > 0 ? sessionIds : undefined },
      });
      setResult(response);
      toast(`${response.assigned} assegnazioni create`);
    } catch (submitError) {
      setError(describeBulkAssignError(submitError, sessions));
    }
  }

  return (
    <Modal open={mode !== null} onClose={handleClose} title={BULK_ASSIGN_MODE_LABELS[mode]} size="sm">
      <div className={styles.formBody}>
        {result ? (
          <>
            <p className={styles.resultNotice}>{result.assigned} assegnazioni create.</p>
            {Object.keys(result.perSession).length > 0 && (
              <ul className={styles.resultList}>
                {sessions.map((s, i) => {
                  const count = result.perSession[s.id];
                  if (!count) return null;
                  return (
                    <li key={s.id}>
                      <span>{sessionLabel(s, i)}</span>
                      <span>{count}</span>
                    </li>
                  );
                })}
              </ul>
            )}
            <div className={styles.actions}>
              <Button variant="primary" size="md" onClick={handleClose}>
                Chiudi
              </Button>
            </div>
          </>
        ) : (
          <>
            {mode === 'fill_session' ? (
              <label className={styles.field}>
                <span className={styles.labelHead}>
                  Sessione
                  <span className={styles.requiredMarker} aria-hidden="true" />
                  <VisuallyHidden>obbligatorio</VisuallyHidden>
                </span>
                <SingleSelect options={sessionOptions} selected={sessionId} onChange={setSessionId} placeholder="Seleziona sessione..." />
              </label>
            ) : (
              <label className={styles.field}>
                Sessioni
                <MultiSelect options={sessionOptions} selected={sessionIds} onChange={setSessionIds} placeholder="Tutte le sessioni" />
                <span className={styles.helperText}>Nessuna selezione = tutte le sessioni dell'evento.</span>
              </label>
            )}
            <ErrorPanel message={error} />
            <div className={styles.actions}>
              <Button variant="ghost" size="md" onClick={handleClose} disabled={bulkAssign.isPending}>
                Annulla
              </Button>
              <Button
                variant="primary"
                size="md"
                loading={bulkAssign.isPending}
                disabled={mode === 'fill_session' && sessionId === null}
                onClick={submit}
              >
                Conferma
              </Button>
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}

function BulkParticipationDialog({
  sessions,
  enrollments,
  open,
  onClose,
}: {
  sessions: SessionDetail[];
  enrollments: EnrollmentDetail[];
  open: boolean;
  onClose: () => void;
}) {
  const { toast } = useToast();
  const bulkParticipation = useBulkParticipation();
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [status, setStatus] = useState<ParticipationStatus>('completed');
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<BulkParticipationResponse | null>(null);

  if (!open) return null;

  const sessionOptions = sessions.map((s, i) => ({ value: s.id, label: sessionLabel(s, i) }));

  function handleClose() {
    setSessionId(null);
    setError(null);
    setResult(null);
    onClose();
  }

  async function submit() {
    if (!sessionId) return;
    setError(null);
    try {
      const response = await bulkParticipation.mutateAsync({ sessionId, input: { participationStatus: status } });
      setResult(response);
      toast(`${response.updated} presenze aggiornate`);
    } catch (submitError) {
      setError(describeBulkParticipationError(submitError, enrollments));
    }
  }

  return (
    <Modal open={open} onClose={handleClose} title="Segna presenze" size="sm">
      <div className={styles.formBody}>
        {result ? (
          <>
            <p className={styles.resultNotice}>{result.updated} presenze aggiornate.</p>
            <div className={styles.actions}>
              <Button variant="primary" size="md" onClick={handleClose}>
                Chiudi
              </Button>
            </div>
          </>
        ) : (
          <>
            <label className={styles.field}>
              <span className={styles.labelHead}>
                Sessione
                <span className={styles.requiredMarker} aria-hidden="true" />
                <VisuallyHidden>obbligatorio</VisuallyHidden>
              </span>
              <SingleSelect options={sessionOptions} selected={sessionId} onChange={setSessionId} placeholder="Seleziona sessione..." />
            </label>
            <label className={styles.field}>
              Stato presenza
              <SingleSelect
                options={STATUS_OPTIONS}
                selected={status}
                onChange={(v) => setStatus((v as ParticipationStatus) ?? 'completed')}
              />
            </label>
            <span className={styles.helperText}>Si applica a tutte le iscrizioni non annullate della sessione.</span>
            <ErrorPanel message={error} />
            <div className={styles.actions}>
              <Button variant="ghost" size="md" onClick={handleClose} disabled={bulkParticipation.isPending}>
                Annulla
              </Button>
              <Button
                variant="primary"
                size="md"
                loading={bulkParticipation.isPending}
                disabled={sessionId === null}
                onClick={submit}
              >
                Conferma
              </Button>
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}
