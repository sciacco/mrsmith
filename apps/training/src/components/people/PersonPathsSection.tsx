// Percorsi assegnati nella scheda persona (#162, §Persone 3): progresso per
// passo (coperto/scoperto col riferimento che copre) e totali obbligatori.
// Un solo PUT copre aggiornamento date/note, registrazione della
// conclusione e relativa correzione/azzeramento (PathAssignmentUpdateInput
// e sempre una sostituzione integrale).

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Button, Icon, Modal, SingleSelect, StatusBadge, VisuallyHidden, useToast } from '@mrsmith/ui';
import {
  useAssignPersonPath,
  useRemovePersonPath,
  useTrainingPaths,
  useUpdatePersonPath,
} from '../../api/queries';
import type { PathAssignmentInput, PathAssignmentUpdateInput, PersonPathRef } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly } from '../events/eventFormat';
import cardStyles from '../requests/drawerShared.module.css';
import formStyles from '../requests/requestShared.module.css';
import listStyles from '../../pages/RequestsPage/listPage.module.css';
import styles from './PersonPathsSection.module.css';

interface PersonPathsSectionProps {
  personId: string;
  paths: PersonPathRef[];
}

export function PersonPathsSection({ personId, paths }: PersonPathsSectionProps) {
  const { toast } = useToast();
  const removePath = useRemovePersonPath();
  const [showAssign, setShowAssign] = useState(false);
  const [editing, setEditing] = useState<PersonPathRef | null>(null);
  const [removeTarget, setRemoveTarget] = useState<PersonPathRef | null>(null);
  const [removeError, setRemoveError] = useState<string | null>(null);

  async function confirmRemove() {
    if (!removeTarget) return;
    setRemoveError(null);
    try {
      await removePath.mutateAsync({ personId, pathId: removeTarget.pathId });
      toast('Percorso rimosso');
      setRemoveTarget(null);
    } catch (e) {
      setRemoveError(describeApiError(e, 'Rimozione non riuscita'));
    }
  }

  return (
    <section className={cardStyles.card}>
      <header className={listStyles.header}>
        <h3 className={cardStyles.cardTitle}>Percorsi</h3>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={() => setShowAssign(true)}>
          Assegna percorso
        </Button>
      </header>

      {paths.length === 0 ? (
        <p>Nessun percorso assegnato.</p>
      ) : (
        paths.map((p) => {
          const showSignal = p.allRequiredCovered && !p.completedOn;
          return (
            <div key={p.pathId} className={styles.pathCard}>
              <div className={styles.pathHeader}>
                <h4 className={styles.pathName}>{p.pathName}</h4>
                <div className={styles.pathActions}>
                  <Button variant="ghost" size="sm" onClick={() => setEditing(p)}>
                    Aggiorna
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setRemoveTarget(p)}>
                    Rimuovi
                  </Button>
                </div>
              </div>
              <div className={styles.pathMeta}>
                <span>Inizio {formatDateOnly(p.startedOn)}</span>
                <span>Obiettivo {p.targetCompletion ? formatDateOnly(p.targetCompletion) : '—'}</span>
                <span>
                  Obbligatori coperti {p.requiredCovered}/{p.requiredTotal}
                </span>
                {p.completedOn ? (
                  <StatusBadge value="completed" label={`Concluso il ${formatDateOnly(p.completedOn)}`} variant="success" />
                ) : showSignal ? (
                  <StatusBadge value="ready" label="Obbligatori coperti" variant="success" />
                ) : null}
              </div>
              <ul className={styles.stepList}>
                {p.steps.map((s) => (
                  <li key={s.stepId} className={styles.step}>
                    <Icon
                      name={s.covered ? 'check-circle' : 'circle'}
                      size={14}
                      color={s.covered ? 'var(--color-success)' : 'var(--color-text-faint)'}
                    />
                    <span className={styles.stepLabel}>
                      {s.courseTitle || s.certificationName} {s.isRequired ? '' : '(facoltativo)'}
                    </span>
                    {s.covered && s.eventId && (
                      <span className={styles.stepRef}>
                        · <Link to={`/eventi/${s.eventId}`}>Apri evento</Link>
                      </span>
                    )}
                    {s.covered && !s.eventId && s.awardId && <span className={styles.stepRef}>· da conseguimento</span>}
                  </li>
                ))}
              </ul>
              {p.notes && <p>{p.notes}</p>}
            </div>
          );
        })
      )}

      {showAssign && <AssignPathModal personId={personId} existing={paths} onClose={() => setShowAssign(false)} />}
      {editing && <UpdatePathModal personId={personId} assignment={editing} onClose={() => setEditing(null)} />}

      <Modal
        open={removeTarget !== null}
        onClose={() => setRemoveTarget(null)}
        title="Rimuovi percorso"
        size="sm"
        dismissible={!removePath.isPending}
      >
        <div className={formStyles.body}>
          <p>Rimuovere l'assegnazione al percorso «{removeTarget?.pathName}»? L'operazione non è reversibile.</p>
          <ErrorPanel message={removeError} onDismiss={() => setRemoveError(null)} />
          <div className={formStyles.actions}>
            <Button variant="ghost" size="md" onClick={() => setRemoveTarget(null)} disabled={removePath.isPending}>
              Annulla
            </Button>
            <Button variant="danger" size="md" loading={removePath.isPending} onClick={confirmRemove}>
              Rimuovi
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function AssignPathModal({
  personId,
  existing,
  onClose,
}: {
  personId: string;
  existing: PersonPathRef[];
  onClose: () => void;
}) {
  const { toast } = useToast();
  const paths = useTrainingPaths();
  const assign = useAssignPersonPath();
  const assignedIds = new Set(existing.map((p) => p.pathId));

  const [pathId, setPathId] = useState('');
  const [startedOn, setStartedOn] = useState('');
  const [targetCompletion, setTargetCompletion] = useState('');
  const [notes, setNotes] = useState('');
  const [error, setError] = useState<string | null>(null);

  const canSubmit = pathId !== '';

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    const input: PathAssignmentInput = {
      pathId,
      startedOn: startedOn || undefined,
      targetCompletion: targetCompletion || undefined,
      notes: notes.trim() || undefined,
    };
    try {
      await assign.mutateAsync({ personId, input });
      toast('Percorso assegnato');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Assegnazione non riuscita'));
    }
  }

  return (
    <Modal open onClose={onClose} title="Assegna percorso" size="sm">
      <div className={formStyles.body}>
        <label className={formStyles.field}>
          <span className={formStyles.labelHead}>
            Percorso
            <span className={formStyles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <SingleSelect
            options={(paths.data ?? [])
              .filter((p) => p.active && !assignedIds.has(p.id))
              .map((p) => ({ value: p.id, label: p.name }))}
            selected={pathId || null}
            onChange={(v) => setPathId(v ?? '')}
            placeholder="Seleziona percorso..."
            searchable
          />
        </label>
        <div className={formStyles.row}>
          <label className={formStyles.field}>
            Inizio
            <input type="date" className={formStyles.input} value={startedOn} onChange={(e) => setStartedOn(e.target.value)} />
          </label>
          <label className={formStyles.field}>
            Obiettivo
            <input
              type="date"
              className={formStyles.input}
              value={targetCompletion}
              onChange={(e) => setTargetCompletion(e.target.value)}
            />
          </label>
        </div>
        <label className={formStyles.field}>
          Note
          <textarea className={formStyles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </label>
        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={assign.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={assign.isPending} disabled={!canSubmit} onClick={submit}>
            Assegna
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function UpdatePathModal({
  personId,
  assignment,
  onClose,
}: {
  personId: string;
  assignment: PersonPathRef;
  onClose: () => void;
}) {
  const { toast } = useToast();
  const updatePath = useUpdatePersonPath();

  const [startedOn, setStartedOn] = useState(assignment.startedOn);
  const [targetCompletion, setTargetCompletion] = useState(assignment.targetCompletion ?? '');
  const [completedOn, setCompletedOn] = useState(assignment.completedOn ?? '');
  const [notes, setNotes] = useState(assignment.notes ?? '');
  const [error, setError] = useState<string | null>(null);

  const canSubmit = startedOn !== '';
  const showSignal = assignment.allRequiredCovered && !assignment.completedOn;

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    const input: PathAssignmentUpdateInput = {
      startedOn,
      targetCompletion: targetCompletion || undefined,
      completedOn: completedOn || undefined,
      notes: notes.trim() || undefined,
    };
    try {
      await updatePath.mutateAsync({ personId, pathId: assignment.pathId, input });
      toast('Percorso aggiornato');
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Aggiornamento non riuscito'));
    }
  }

  return (
    <Modal open onClose={onClose} title={`Aggiorna percorso · ${assignment.pathName}`} size="sm">
      <div className={formStyles.body}>
        {showSignal && (
          <p className={formStyles.hint}>
            Tutti i passi obbligatori sono coperti: è possibile registrare la conclusione.
          </p>
        )}
        <div className={formStyles.row}>
          <label className={formStyles.field}>
            <span className={formStyles.labelHead}>
              Inizio
              <span className={formStyles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input type="date" className={formStyles.input} value={startedOn} onChange={(e) => setStartedOn(e.target.value)} />
          </label>
          <label className={formStyles.field}>
            Obiettivo
            <input
              type="date"
              className={formStyles.input}
              value={targetCompletion}
              onChange={(e) => setTargetCompletion(e.target.value)}
            />
          </label>
        </div>
        <label className={formStyles.field}>
          Conclusione
          <input type="date" className={formStyles.input} value={completedOn} onChange={(e) => setCompletedOn(e.target.value)} />
          <span className={formStyles.hint}>Vuoto = percorso ancora in corso.</span>
        </label>
        <label className={formStyles.field}>
          Note
          <textarea className={formStyles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </label>
        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={updatePath.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={updatePath.isPending} disabled={!canSubmit} onClick={submit}>
            Salva modifiche
          </Button>
        </div>
      </div>
    </Modal>
  );
}
