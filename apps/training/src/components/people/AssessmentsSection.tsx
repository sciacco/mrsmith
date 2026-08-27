// Valutazioni di competenza della scheda persona (#162, §Persone 2): ultima
// valutazione per area in evidenza, storico consultabile. Il 409 su
// (area, data) duplicata si mostra per intero (nessuna previsione locale).

import { useMemo, useState } from 'react';
import { Button, Icon, Modal, SingleSelect, VisuallyHidden, useToast } from '@mrsmith/ui';
import {
  useCreateAssessment,
  useDeleteAssessment,
  useTrainingSkillAreas,
  useUpdateAssessment,
} from '../../api/queries';
import type { AssessmentInput, AssessmentUpdateInput, PersonAssessmentRef } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly } from '../events/eventFormat';
import { ASSESSMENT_LEVEL_LABELS, VALIDATION_SOURCE_LABELS } from '../../lib/labels';
import cardStyles from '../requests/drawerShared.module.css';
import formStyles from '../requests/requestShared.module.css';
import listStyles from '../../pages/RequestsPage/listPage.module.css';
import styles from './AssessmentsSection.module.css';

const LEVEL_OPTIONS = Object.entries(ASSESSMENT_LEVEL_LABELS).map(([value, label]) => ({
  value: Number(value),
  label: `${value} · ${label}`,
}));
const SOURCE_OPTIONS = Object.entries(VALIDATION_SOURCE_LABELS).map(([value, label]) => ({ value, label }));

function latestPerArea(assessments: PersonAssessmentRef[]): PersonAssessmentRef[] {
  const latest = new Map<string, PersonAssessmentRef>();
  for (const a of assessments) {
    const current = latest.get(a.skillAreaId);
    if (!current || a.assessedOn > current.assessedOn) latest.set(a.skillAreaId, a);
  }
  return [...latest.values()].sort((a, b) => a.skillAreaName.localeCompare(b.skillAreaName));
}

interface AssessmentsSectionProps {
  personId: string;
  assessments: PersonAssessmentRef[];
}

export function AssessmentsSection({ personId, assessments }: AssessmentsSectionProps) {
  const { toast } = useToast();
  const deleteAssessment = useDeleteAssessment();
  const [editing, setEditing] = useState<PersonAssessmentRef | 'create' | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<PersonAssessmentRef | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const latest = useMemo(() => latestPerArea(assessments), [assessments]);
  const history = useMemo(
    () => [...assessments].sort((a, b) => b.assessedOn.localeCompare(a.assessedOn)),
    [assessments],
  );

  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleteError(null);
    try {
      await deleteAssessment.mutateAsync(deleteTarget.id);
      toast('Valutazione eliminata');
      setDeleteTarget(null);
    } catch (e) {
      setDeleteError(describeApiError(e, 'Eliminazione non riuscita'));
    }
  }

  return (
    <section className={cardStyles.card}>
      <header className={listStyles.header}>
        <h3 className={cardStyles.cardTitle}>Valutazioni</h3>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={() => setEditing('create')}>
          Registra valutazione
        </Button>
      </header>

      {assessments.length === 0 ? (
        <p>Nessuna valutazione registrata.</p>
      ) : (
        <>
          <div className={styles.latestRow}>
            {latest.map((a) => (
              <div key={a.skillAreaId} className={styles.latestChip}>
                <span className={styles.latestArea}>{a.skillAreaName}</span>
                <span className={styles.latestLevel}>{a.level}/5 · {ASSESSMENT_LEVEL_LABELS[a.level] ?? a.level}</span>
                <span className={styles.latestDate}>{formatDateOnly(a.assessedOn)}</span>
              </div>
            ))}
          </div>

          <h4 className={styles.historyTitle}>Storico</h4>
          <div className={listStyles.tableWrap}>
            <table className={listStyles.table}>
              <thead>
                <tr>
                  <th>Area</th>
                  <th>Livello</th>
                  <th>Data</th>
                  <th>Fonte</th>
                  <th>Note</th>
                  <th>Azioni</th>
                </tr>
              </thead>
              <tbody>
                {history.map((a) => (
                  <tr key={a.id}>
                    <td>{a.skillAreaName}</td>
                    <td>{a.level}/5 · {ASSESSMENT_LEVEL_LABELS[a.level] ?? a.level}</td>
                    <td>{formatDateOnly(a.assessedOn)}</td>
                    <td>{VALIDATION_SOURCE_LABELS[a.source] ?? a.source}</td>
                    <td>{a.notes || '—'}</td>
                    <td>
                      <div className={styles.rowActions}>
                        <Button variant="ghost" size="sm" onClick={() => setEditing(a)}>
                          Correggi
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(a)}>
                          Elimina
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {editing && (
        <AssessmentEditorModal
          personId={personId}
          assessment={editing === 'create' ? null : editing}
          onClose={() => setEditing(null)}
        />
      )}

      <Modal
        open={deleteTarget !== null}
        onClose={() => setDeleteTarget(null)}
        title="Elimina valutazione"
        size="sm"
        dismissible={!deleteAssessment.isPending}
      >
        <div className={formStyles.body}>
          <p>Eliminare la valutazione di «{deleteTarget?.skillAreaName}» del {deleteTarget ? formatDateOnly(deleteTarget.assessedOn) : ''}? L'operazione non è reversibile.</p>
          <ErrorPanel message={deleteError} onDismiss={() => setDeleteError(null)} />
          <div className={formStyles.actions}>
            <Button variant="ghost" size="md" onClick={() => setDeleteTarget(null)} disabled={deleteAssessment.isPending}>
              Annulla
            </Button>
            <Button variant="danger" size="md" loading={deleteAssessment.isPending} onClick={confirmDelete}>
              Elimina
            </Button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function AssessmentEditorModal({
  personId,
  assessment,
  onClose,
}: {
  personId: string;
  assessment: PersonAssessmentRef | null;
  onClose: () => void;
}) {
  const { toast } = useToast();
  const skillAreas = useTrainingSkillAreas();
  const createAssessment = useCreateAssessment();
  const updateAssessment = useUpdateAssessment();

  const [skillAreaId, setSkillAreaId] = useState(assessment?.skillAreaId ?? '');
  const [level, setLevel] = useState<number | null>(assessment?.level ?? null);
  const [assessedOn, setAssessedOn] = useState(assessment?.assessedOn ?? '');
  const [source, setSource] = useState(assessment?.source ?? '');
  const [notes, setNotes] = useState(assessment?.notes ?? '');
  const [error, setError] = useState<string | null>(null);

  const pending = createAssessment.isPending || updateAssessment.isPending;
  const canSubmit = (assessment ? true : skillAreaId !== '') && level !== null && (assessment ? assessedOn !== '' : true);

  async function submit() {
    if (!canSubmit || level === null) return;
    setError(null);
    try {
      if (assessment) {
        const input: AssessmentUpdateInput = { level, assessedOn, source: source || undefined, notes: notes.trim() || undefined };
        await updateAssessment.mutateAsync({ id: assessment.id, input });
        toast('Valutazione aggiornata');
      } else {
        const input: AssessmentInput = {
          skillAreaId,
          level,
          assessedOn: assessedOn || undefined,
          source: source || undefined,
          notes: notes.trim() || undefined,
        };
        await createAssessment.mutateAsync({ personId, input });
        toast('Valutazione registrata');
      }
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <Modal open onClose={onClose} title={assessment ? 'Correggi valutazione' : 'Registra valutazione'} size="sm">
      <div className={formStyles.body}>
        {assessment ? (
          <p className={formStyles.hint}>Area: {assessment.skillAreaName}</p>
        ) : (
          <label className={formStyles.field}>
            <span className={formStyles.labelHead}>
              Area di competenza
              <span className={formStyles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <SingleSelect
              options={(skillAreas.data ?? []).filter((a) => a.active).map((a) => ({ value: a.id, label: a.name }))}
              selected={skillAreaId || null}
              onChange={(v) => setSkillAreaId(v ?? '')}
              placeholder="Seleziona area..."
              searchable
            />
          </label>
        )}

        <label className={formStyles.field}>
          <span className={formStyles.labelHead}>
            Livello
            <span className={formStyles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <SingleSelect
            options={LEVEL_OPTIONS}
            selected={level}
            onChange={(v) => setLevel(v === null ? null : Number(v))}
            placeholder="Seleziona livello..."
          />
        </label>

        <div className={formStyles.row}>
          <label className={formStyles.field}>
            {assessment ? (
              <span className={formStyles.labelHead}>
                Data
                <span className={formStyles.requiredMarker} aria-hidden="true" />
                <VisuallyHidden>obbligatorio</VisuallyHidden>
              </span>
            ) : (
              'Data'
            )}
            <input type="date" className={formStyles.input} value={assessedOn} onChange={(e) => setAssessedOn(e.target.value)} />
          </label>
          <label className={formStyles.field}>
            Fonte
            <SingleSelect
              options={SOURCE_OPTIONS}
              selected={source || null}
              onChange={(v) => setSource(v ?? '')}
              placeholder="Dichiarata da questionario"
              allowClear
            />
          </label>
        </div>

        <label className={formStyles.field}>
          Note
          <textarea className={formStyles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </label>

        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={pending} disabled={!canSubmit} onClick={submit}>
            {assessment ? 'Salva modifiche' : 'Registra'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
