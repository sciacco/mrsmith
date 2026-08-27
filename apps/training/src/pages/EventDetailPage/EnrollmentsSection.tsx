// Iscritti dell'evento: tabella, aggiunta massiva, fatti (drawer), annulla,
// storico-completato, riapri. Le azioni si offrono in base allo stato reale
// gia caricato (mai una previsione locale dell'esito): il drawer fatti resta
// sempre disponibile, coerente con lo store (i fatti restano modificabili
// anche sull'iscrizione annullata); il 4xx del backend, se arriva, si mostra
// per intero (#156).

import { useState } from 'react';
import { formatNumber } from '@mrsmith/format';
import {
  Button,
  Drawer,
  Icon,
  Modal,
  MultiSelect,
  SingleSelect,
  StatusBadge,
  useToast,
  VisuallyHidden,
} from '@mrsmith/ui';
import {
  useBulkEnrollments,
  useCancelEnrollment,
  useCompleteEnrollmentHistorical,
  useReopenEnrollment,
  useUpdateEnrollmentFacts,
} from '../../api/queries';
import type {
  BulkEnrollResponse,
  EnrollmentDetail,
  EnrollmentFactsInput,
  LearningOutcome,
  LookupItem,
  ParticipationRow,
} from '../../api/types';
import { describeApiError } from '../../components/events/apiErrors';
import { formatDateOnly } from '../../components/events/eventFormat';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { ReasonDialog } from '../../components/events/ReasonDialog';
import { deliveryStatusVariant } from '../../components/events/statusVariants';
import {
  DELIVERY_STATUS_LABELS,
  ENROLL_SKIP_REASON_LABELS,
  EVENT_ORIGIN_LABELS,
  LEARNING_OUTCOME_LABELS,
} from '../../lib/labels';
import styles from './EventDetailPage.module.css';

interface EnrollmentsSectionProps {
  eventId: string;
  enrollments: EnrollmentDetail[];
  participations: ParticipationRow[];
  employees: LookupItem[];
  highlighted: boolean;
}

export function EnrollmentsSection({
  eventId,
  enrollments,
  participations,
  employees,
  highlighted,
}: EnrollmentsSectionProps) {
  const { toast } = useToast();
  const cancelEnrollment = useCancelEnrollment();
  const completeHistorical = useCompleteEnrollmentHistorical();
  const reopenEnrollment = useReopenEnrollment();

  const [showAdd, setShowAdd] = useState(false);
  const [factsTarget, setFactsTarget] = useState<EnrollmentDetail | null>(null);
  const [cancelTarget, setCancelTarget] = useState<EnrollmentDetail | null>(null);
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [reopenTarget, setReopenTarget] = useState<EnrollmentDetail | null>(null);
  const [reopenError, setReopenError] = useState<string | null>(null);
  const [completeTarget, setCompleteTarget] = useState<EnrollmentDetail | null>(null);
  const [completeError, setCompleteError] = useState<string | null>(null);

  const participationCountByEnrollment = new Map<string, number>();
  for (const p of participations) {
    participationCountByEnrollment.set(p.enrollmentId, (participationCountByEnrollment.get(p.enrollmentId) ?? 0) + 1);
  }

  async function handleCancel(reason: string) {
    if (!cancelTarget) return;
    setCancelError(null);
    try {
      await cancelEnrollment.mutateAsync({ id: cancelTarget.id, input: { reason } });
      toast('Iscrizione annullata');
      setCancelTarget(null);
    } catch (error) {
      setCancelError(describeApiError(error, 'Annullamento non riuscito'));
    }
  }

  async function handleReopen(reason: string) {
    if (!reopenTarget) return;
    setReopenError(null);
    try {
      await reopenEnrollment.mutateAsync({ id: reopenTarget.id, input: { reason } });
      toast('Iscrizione riaperta');
      setReopenTarget(null);
    } catch (error) {
      setReopenError(describeApiError(error, 'Riapertura non riuscita'));
    }
  }

  async function handleComplete() {
    if (!completeTarget) return;
    setCompleteError(null);
    try {
      await completeHistorical.mutateAsync(completeTarget.id);
      toast('Iscrizione registrata come storico-completata');
      setCompleteTarget(null);
    } catch (error) {
      setCompleteError(describeApiError(error, 'Registrazione non riuscita'));
    }
  }

  return (
    <section id="section-enrollments" className={`${styles.section} ${highlighted ? styles.sectionHighlighted : ''}`}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Iscritti</h2>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={() => setShowAdd(true)}>
          Aggiungi persone
        </Button>
      </div>

      {enrollments.length === 0 ? (
        <p className={styles.emptyNotice}>Nessuna persona iscritta a questo evento.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Persona</th>
                <th>Stato</th>
                <th>Esito prova</th>
                <th>Date effettive</th>
                <th className={styles.numCell}>Ore</th>
                <th>Origine</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {enrollments.map((enrollment) => {
                const canCancel = enrollment.deliveryStatus === 'planned' || enrollment.deliveryStatus === 'in_progress';
                const canComplete = enrollment.deliveryStatus === 'planned';
                const canReopen =
                  enrollment.deliveryStatus === 'cancelled' ||
                  (enrollment.deliveryStatus === 'completed' &&
                    (participationCountByEnrollment.get(enrollment.id) ?? 0) === 0);
                return (
                  <tr key={enrollment.id}>
                    <td>{enrollment.employeeName}</td>
                    <td>
                      <StatusBadge
                        value={enrollment.deliveryStatus}
                        label={DELIVERY_STATUS_LABELS[enrollment.deliveryStatus]}
                        variant={deliveryStatusVariant(enrollment.deliveryStatus)}
                      />
                    </td>
                    <td>{enrollment.learningOutcome ? LEARNING_OUTCOME_LABELS[enrollment.learningOutcome] : '—'}</td>
                    <td>
                      {enrollment.actualStart ? formatDateOnly(enrollment.actualStart) : '—'}
                      {enrollment.actualEnd ? ` – ${formatDateOnly(enrollment.actualEnd)}` : ''}
                    </td>
                    <td className={styles.numCell}>
                      {enrollment.hoursActual !== undefined ? (formatNumber(enrollment.hoursActual) ?? '—') : '—'}
                    </td>
                    <td>{EVENT_ORIGIN_LABELS[enrollment.origin] ?? enrollment.origin}</td>
                    <td className={styles.rowActions}>
                      <button type="button" className={styles.linkButton} onClick={() => setFactsTarget(enrollment)}>
                        Fatti
                      </button>
                      {canComplete && (
                        <button
                          type="button"
                          className={styles.linkButton}
                          onClick={() => {
                            setCompleteTarget(enrollment);
                            setCompleteError(null);
                          }}
                        >
                          Storico-completato
                        </button>
                      )}
                      {canCancel && (
                        <button
                          type="button"
                          className={styles.linkButtonDanger}
                          onClick={() => {
                            setCancelTarget(enrollment);
                            setCancelError(null);
                          }}
                        >
                          Annulla
                        </button>
                      )}
                      {canReopen && (
                        <button
                          type="button"
                          className={styles.linkButton}
                          onClick={() => {
                            setReopenTarget(enrollment);
                            setReopenError(null);
                          }}
                        >
                          Riapri
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <AddEnrollmentsModal
        open={showAdd}
        eventId={eventId}
        employees={employees}
        alreadyEnrolled={new Set(enrollments.map((e) => e.employeeId))}
        onClose={() => setShowAdd(false)}
      />

      {factsTarget && (
        <FactsDrawerForm key={factsTarget.id} enrollment={factsTarget} onClose={() => setFactsTarget(null)} />
      )}

      {cancelTarget && (
        <ReasonDialog
          key={cancelTarget.id}
          open
          title="Annulla iscrizione"
          confirmLabel="Annulla iscrizione"
          pending={cancelEnrollment.isPending}
          error={cancelError}
          onConfirm={handleCancel}
          onClose={() => setCancelTarget(null)}
        />
      )}

      {reopenTarget && (
        <ReasonDialog
          key={reopenTarget.id}
          open
          title="Riapri iscrizione"
          confirmLabel="Riapri"
          danger={false}
          pending={reopenEnrollment.isPending}
          error={reopenError}
          onConfirm={handleReopen}
          onClose={() => setReopenTarget(null)}
        />
      )}

      <Modal open={completeTarget !== null} onClose={() => setCompleteTarget(null)} title="Storico-completato" size="sm">
        <div className={styles.formBody}>
          <p>Registrare questa iscrizione come formazione già avvenuta, senza tracciamento a sessioni?</p>
          <ErrorPanel message={completeError} />
          <div className={styles.actions}>
            <Button variant="ghost" size="md" onClick={() => setCompleteTarget(null)} disabled={completeHistorical.isPending}>
              Annulla
            </Button>
            <Button variant="primary" size="md" loading={completeHistorical.isPending} onClick={handleComplete}>
              Conferma
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function AddEnrollmentsModal({
  open,
  eventId,
  employees,
  alreadyEnrolled,
  onClose,
}: {
  open: boolean;
  eventId: string;
  employees: LookupItem[];
  alreadyEnrolled: Set<string>;
  onClose: () => void;
}) {
  const { toast } = useToast();
  const bulkEnroll = useBulkEnrollments();
  const [selected, setSelected] = useState<string[]>([]);
  const [objective, setObjective] = useState('');
  const [notes, setNotes] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<BulkEnrollResponse | null>(null);

  if (!open) return null;

  const options = employees
    .filter((e) => e.active && !alreadyEnrolled.has(e.id))
    .map((e) => ({ value: e.id, label: e.label }));
  const employeeName = (id: string) => employees.find((e) => e.id === id)?.label ?? id;

  function handleClose() {
    setSelected([]);
    setObjective('');
    setNotes('');
    setError(null);
    setResult(null);
    onClose();
  }

  async function submit() {
    setError(null);
    try {
      const response = await bulkEnroll.mutateAsync({
        eventId,
        input: { employeeIds: selected, objective: objective.trim() || undefined, notes: notes.trim() || undefined },
      });
      setResult(response);
      if (response.created.length > 0) toast(`${response.created.length} persone aggiunte`);
    } catch (submitError) {
      setError(describeApiError(submitError, 'Aggiunta non riuscita'));
    }
  }

  return (
    <Modal open={open} onClose={handleClose} title="Aggiungi persone" size="md">
      <div className={styles.formBody}>
        {result ? (
          <>
            <p className={styles.resultNotice}>{result.created.length} persone aggiunte.</p>
            {result.skipped.length > 0 && (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Persona</th>
                      <th>Motivo esclusione</th>
                    </tr>
                  </thead>
                  <tbody>
                    {result.skipped.map((row) => (
                      <tr key={row.employeeId}>
                        <td>{employeeName(row.employeeId)}</td>
                        <td>{ENROLL_SKIP_REASON_LABELS[row.reason] ?? row.reason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
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
                Persone
                <span className={styles.requiredMarker} aria-hidden="true" />
                <VisuallyHidden>obbligatorio</VisuallyHidden>
              </span>
              <MultiSelect options={options} selected={selected} onChange={setSelected} placeholder="Seleziona persone..." />
            </label>
            <label className={styles.field}>
              Obiettivo
              <input className={styles.input} value={objective} onChange={(e) => setObjective(e.target.value)} />
            </label>
            <label className={styles.field}>
              Note
              <textarea className={styles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
            </label>
            <ErrorPanel message={error} />
            <div className={styles.actions}>
              <Button variant="ghost" size="md" onClick={handleClose} disabled={bulkEnroll.isPending}>
                Annulla
              </Button>
              <Button
                variant="primary"
                size="md"
                loading={bulkEnroll.isPending}
                disabled={selected.length === 0}
                onClick={submit}
              >
                Aggiungi
              </Button>
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}

const OUTCOME_OPTIONS: { value: LearningOutcome | ''; label: string }[] = [
  { value: '', label: 'Nessuno' },
  { value: 'passed', label: LEARNING_OUTCOME_LABELS.passed ?? 'passed' },
  { value: 'failed', label: LEARNING_OUTCOME_LABELS.failed ?? 'failed' },
  { value: 'not_taken', label: LEARNING_OUTCOME_LABELS.not_taken ?? 'not_taken' },
  { value: 'not_required', label: LEARNING_OUTCOME_LABELS.not_required ?? 'not_required' },
];

function FactsDrawerForm({ enrollment, onClose }: { enrollment: EnrollmentDetail; onClose: () => void }) {
  const { toast } = useToast();
  const updateFacts = useUpdateEnrollmentFacts();
  const [objective, setObjective] = useState(enrollment.objective ?? '');
  const [notes, setNotes] = useState(enrollment.notes ?? '');
  const [actualStart, setActualStart] = useState(enrollment.actualStart ?? '');
  const [actualEnd, setActualEnd] = useState(enrollment.actualEnd ?? '');
  const [hoursActual, setHoursActual] = useState(
    enrollment.hoursActual !== undefined ? String(enrollment.hoursActual) : '',
  );
  const [learningOutcome, setLearningOutcome] = useState<LearningOutcome | ''>(enrollment.learningOutcome ?? '');
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setError(null);
    const input: EnrollmentFactsInput = {
      objective: objective.trim() || undefined,
      notes: notes.trim() || undefined,
      actualStart: actualStart || undefined,
      actualEnd: actualEnd || undefined,
      hoursActual: hoursActual === '' ? undefined : Number(hoursActual),
      learningOutcome,
    };
    try {
      await updateFacts.mutateAsync({ id: enrollment.id, input });
      toast('Fatti aggiornati');
      onClose();
    } catch (submitError) {
      setError(describeApiError(submitError, 'Salvataggio non riuscito'));
    }
  }

  return (
    <Drawer open onClose={onClose} title={`Fatti — ${enrollment.employeeName}`} size="md">
      <div className={styles.formBody}>
        <label className={styles.field}>
          Obiettivo
          <input className={styles.input} value={objective} onChange={(e) => setObjective(e.target.value)} />
        </label>
        <div className={styles.fieldRow}>
          <label className={styles.field}>
            Data inizio effettiva
            <input
              type="date"
              className={styles.input}
              value={actualStart}
              onChange={(e) => setActualStart(e.target.value)}
            />
          </label>
          <label className={styles.field}>
            Data fine effettiva
            <input
              type="date"
              className={styles.input}
              value={actualEnd}
              onChange={(e) => setActualEnd(e.target.value)}
            />
          </label>
        </div>
        <label className={styles.field}>
          Ore effettive
          <input
            type="number"
            min={0}
            className={styles.input}
            value={hoursActual}
            onChange={(e) => setHoursActual(e.target.value)}
          />
        </label>
        <label className={styles.field}>
          Esito prova
          <SingleSelect
            options={OUTCOME_OPTIONS}
            selected={learningOutcome}
            onChange={(v) => setLearningOutcome((v as LearningOutcome | '') ?? '')}
          />
        </label>
        <label className={styles.field}>
          Note
          <textarea className={styles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={3} />
        </label>
        <ErrorPanel message={error} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={updateFacts.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={updateFacts.isPending} onClick={submit}>
            Salva fatti
          </Button>
        </div>
      </div>
    </Drawer>
  );
}
