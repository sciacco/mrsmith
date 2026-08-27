// Dettaglio percorso di catalogo (#162, §Catalogo 5): anagrafica con
// disattivazione tramite active, passi con ordinamento e obbligatorietà su
// corso XOR certificazione (sostituzione atomica, 422 mostrato per intero),
// assegnatari con progresso.

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Button, Drawer, Icon, Skeleton, SingleSelect, StatusBadge, ToggleSwitch, useToast } from '@mrsmith/ui';
import {
  usePathDetail,
  useReplacePathSteps,
  useTrainingCertifications,
  useTrainingCourses,
  useUpsertPath,
} from '../../api/queries';
import type { PathDetail, PathStepInput, PathStepsInput } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly } from '../events/eventFormat';
import { PathEditorModal } from './PathEditorModal';
import drawerStyles from '../requests/drawerShared.module.css';
import formStyles from '../requests/requestShared.module.css';
import tableStyles from './CourseDetailDrawer.module.css';
import styles from './PathsCatalogSection.module.css';

interface StepDraft {
  key: string;
  kind: 'course' | 'certification';
  courseId: string;
  certificationId: string;
  isRequired: boolean;
  notes: string;
}

let draftKeySeq = 0;
function nextDraftKey(): string {
  draftKeySeq += 1;
  return `step-${draftKeySeq}`;
}

function stepsToDrafts(path: PathDetail): StepDraft[] {
  return [...path.steps]
    .sort((a, b) => a.stepOrder - b.stepOrder)
    .map((s) => ({
      key: nextDraftKey(),
      kind: s.certificationId ? 'certification' : 'course',
      courseId: s.courseId ?? '',
      certificationId: s.certificationId ?? '',
      isRequired: s.isRequired,
      notes: s.notes ?? '',
    }));
}

export function PathDetailDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const { toast } = useToast();
  const detail = usePathDetail(id);
  const courses = useTrainingCourses();
  const certifications = useTrainingCertifications();
  const upsertPath = useUpsertPath();
  const replaceSteps = useReplacePathSteps();

  const [showEdit, setShowEdit] = useState(false);
  const [activeError, setActiveError] = useState<string | null>(null);
  const [stepsError, setStepsError] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<StepDraft[]>([]);

  const path = detail.data;

  // Le bozze si risincronizzano solo al cambio di percorso, non a ogni
  // refetch: ogni mutazione Training invalida la cache e il drawer contiene
  // altri comandi mutanti (Attivo, Modifica) oltre a «Salva passi» — una
  // dipendenza su `path` intero cancellerebbe le modifiche pendenti dei
  // passi a ogni invalidazione indotta da quei comandi.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (path) setDrafts(stepsToDrafts(path));
  }, [path?.id]);

  async function toggleActive() {
    if (!path) return;
    setActiveError(null);
    try {
      await upsertPath.mutateAsync({
        id: path.id,
        input: { code: path.code, name: path.name, skillAreaId: path.skillAreaId, description: path.description, active: !path.active },
      });
      toast(path.active ? 'Percorso disattivato' : 'Percorso attivato');
    } catch (e) {
      setActiveError(describeApiError(e, `Impossibile ${path.active ? 'disattivare' : 'attivare'} il percorso`));
    }
  }

  function updateDraft(key: string, patch: Partial<StepDraft>) {
    setDrafts((prev) => prev.map((d) => (d.key === key ? { ...d, ...patch } : d)));
  }

  function moveDraft(index: number, direction: -1 | 1) {
    setDrafts((prev) => {
      const next = [...prev];
      const target = index + direction;
      if (target < 0 || target >= next.length) return prev;
      const moved = next[index];
      const swapped = next[target];
      if (!moved || !swapped) return prev;
      next[index] = swapped;
      next[target] = moved;
      return next;
    });
  }

  function removeDraft(key: string) {
    setDrafts((prev) => prev.filter((d) => d.key !== key));
  }

  function addDraft() {
    setDrafts((prev) => [...prev, { key: nextDraftKey(), kind: 'course', courseId: '', certificationId: '', isRequired: true, notes: '' }]);
  }

  const draftsValid = drafts.every((d) => (d.kind === 'course' ? d.courseId !== '' : d.certificationId !== ''));

  async function saveSteps() {
    if (!path || !draftsValid) return;
    setStepsError(null);
    const input: PathStepsInput = {
      steps: drafts.map(
        (d, index): PathStepInput => ({
          stepOrder: index + 1,
          courseId: d.kind === 'course' ? d.courseId : undefined,
          certificationId: d.kind === 'certification' ? d.certificationId : undefined,
          isRequired: d.isRequired,
          notes: d.notes.trim() || undefined,
        }),
      ),
    };
    try {
      await replaceSteps.mutateAsync({ pathId: path.id, input });
      const refetched = await detail.refetch();
      if (refetched.data) setDrafts(stepsToDrafts(refetched.data));
      toast('Passi aggiornati');
    } catch (e) {
      setStepsError(describeApiError(e, 'Aggiornamento dei passi non riuscito'));
    }
  }

  return (
    <>
      <Drawer open onClose={onClose} title={path?.name ?? 'Percorso'} subtitle={path?.code} size="lg">
        <div className={drawerStyles.body}>
          {detail.isLoading && <Skeleton rows={6} />}
          {detail.isError && (
            <p className={drawerStyles.errorNotice}>{describeApiError(detail.error, 'Lettura del percorso non riuscita')}</p>
          )}
          {path && (
            <>
              <section className={drawerStyles.card}>
                <div className={styles.cardHeader}>
                  <h3 className={drawerStyles.cardTitle}>Percorso</h3>
                  <div className={styles.cardHeader}>
                    <ToggleSwitch id={`path-active-${path.id}`} checked={path.active} onChange={toggleActive} label="Attivo" />
                    <Button variant="ghost" size="sm" onClick={() => setShowEdit(true)}>
                      Modifica
                    </Button>
                  </div>
                </div>
                <ErrorPanel message={activeError} onDismiss={() => setActiveError(null)} />
                <dl className={drawerStyles.grid}>
                  <div className={drawerStyles.item}>
                    <dt>Area di competenza</dt>
                    <dd>{path.skillAreaName || '—'}</dd>
                  </div>
                  <div className={drawerStyles.item}>
                    <dt>Assegnatari</dt>
                    <dd>{path.assigneesCount}</dd>
                  </div>
                </dl>
                {path.description && <p>{path.description}</p>}
              </section>

              <section className={drawerStyles.card}>
                <div className={styles.cardHeader}>
                  <h3 className={drawerStyles.cardTitle}>Passi</h3>
                  <Button variant="ghost" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={addDraft}>
                    Aggiungi passo
                  </Button>
                </div>
                <ErrorPanel message={stepsError} onDismiss={() => setStepsError(null)} />
                {drafts.length === 0 ? (
                  <p>Nessun passo definito.</p>
                ) : (
                  <div className={styles.stepList}>
                    {drafts.map((d, index) => (
                      <div key={d.key} className={styles.stepRow}>
                        <div className={styles.stepOrder}>
                          <span className={styles.stepOrderNumber}>{index + 1}</span>
                          <div className={styles.stepMoveButtons}>
                            <Button
                              variant="ghost"
                              size="sm"
                              disabled={index === 0}
                              onClick={() => moveDraft(index, -1)}
                              aria-label="Sposta il passo in alto"
                            >
                              <Icon name="chevron-up" size={14} />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              disabled={index === drafts.length - 1}
                              onClick={() => moveDraft(index, 1)}
                              aria-label="Sposta il passo in basso"
                            >
                              <Icon name="chevron-down" size={14} />
                            </Button>
                          </div>
                        </div>
                        <div className={styles.stepFields}>
                          <div className={styles.stepFieldsRow}>
                            <SingleSelect
                              options={[
                                { value: 'course', label: 'Corso' },
                                { value: 'certification', label: 'Certificazione' },
                              ]}
                              selected={d.kind}
                              onChange={(v) => updateDraft(d.key, { kind: (v as 'course' | 'certification') ?? 'course', courseId: '', certificationId: '' })}
                            />
                            {d.kind === 'course' ? (
                              <SingleSelect
                                options={(courses.data ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.title }))}
                                selected={d.courseId || null}
                                onChange={(v) => updateDraft(d.key, { courseId: v ?? '' })}
                                placeholder="Seleziona corso..."
                                searchable
                              />
                            ) : (
                              <SingleSelect
                                options={(certifications.data ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.name }))}
                                selected={d.certificationId || null}
                                onChange={(v) => updateDraft(d.key, { certificationId: v ?? '' })}
                                placeholder="Seleziona certificazione..."
                                searchable
                              />
                            )}
                          </div>
                          <input
                            className={formStyles.input}
                            placeholder="Note del passo (facoltative)"
                            value={d.notes}
                            onChange={(e) => updateDraft(d.key, { notes: e.target.value })}
                          />
                          <div className={styles.stepToggleRow}>
                            <ToggleSwitch
                              id={`step-required-${d.key}`}
                              checked={d.isRequired}
                              onChange={(v) => updateDraft(d.key, { isRequired: v })}
                              label="Obbligatorio"
                            />
                          </div>
                        </div>
                        <Button
                          variant="ghost"
                          size="sm"
                          className={styles.stepRemove}
                          onClick={() => removeDraft(d.key)}
                          aria-label="Rimuovi il passo"
                        >
                          <Icon name="trash" size={14} />
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
                <Button
                  variant="primary"
                  size="md"
                  loading={replaceSteps.isPending}
                  disabled={!draftsValid}
                  onClick={saveSteps}
                >
                  Salva passi
                </Button>
              </section>

              <section className={drawerStyles.card}>
                <h3 className={drawerStyles.cardTitle}>Assegnatari</h3>
                {path.assignees.length === 0 ? (
                  <p>Nessun assegnatario.</p>
                ) : (
                  <div className={tableStyles.tableWrap}>
                    <table className={tableStyles.miniTable}>
                      <thead>
                        <tr>
                          <th>Persona</th>
                          <th>Inizio</th>
                          <th>Obiettivo</th>
                          <th className={styles.numCell}>Obbligatori coperti</th>
                          <th>Conclusione</th>
                        </tr>
                      </thead>
                      <tbody>
                        {path.assignees.map((a) => (
                          <tr key={a.employeeId}>
                            <td>
                              <Link to={`/persone/${a.employeeId}`}>{a.employeeName}</Link>
                            </td>
                            <td>{formatDateOnly(a.startedOn)}</td>
                            <td>{a.targetCompletion ? formatDateOnly(a.targetCompletion) : '—'}</td>
                            <td className={styles.numCell}>
                              {a.requiredCovered}/{a.requiredTotal}
                            </td>
                            <td>
                              {a.completedOn ? (
                                formatDateOnly(a.completedOn)
                              ) : a.allRequiredCovered ? (
                                <StatusBadge value="ready" label="Obbligatori coperti" variant="success" />
                              ) : (
                                '—'
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>
            </>
          )}
        </div>
      </Drawer>

      {path && showEdit && (
        <PathEditorModal mode="edit" pathId={path.id} initial={path} open={showEdit} onClose={() => setShowEdit(false)} onSaved={() => setShowEdit(false)} />
      )}
    </>
  );
}
