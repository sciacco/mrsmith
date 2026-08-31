// Dettaglio corso di catalogo (#158, §Catalogo 4): dati completi, regole ed
// eventi collegati con rimando alle rispettive superfici. Archiviazione
// separata dalla modifica (idioma POST /courses/{id}/archive del backend).

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { formatCurrency } from '@mrsmith/format';
import { Button, Drawer, Modal, Skeleton, StatusBadge, useToast } from '@mrsmith/ui';
import { useArchiveCourse, useCourseDetail, useResumeCourse, useSuspendCourse } from '../../api/queries';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly, formatInstantDate } from '../events/eventFormat';
import { DELIVERY_MODE_LABELS, PROVIDER_KIND_LABELS } from '../../lib/labels';
import { CourseEditorModal } from './CourseEditorModal';
import styles from '../requests/drawerShared.module.css';
import formStyles from '../requests/requestShared.module.css';
import localStyles from './CourseDetailDrawer.module.css';

export function CourseDetailDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const { toast } = useToast();
  const detail = useCourseDetail(id);
  const archiveCourse = useArchiveCourse();
  const suspendCourse = useSuspendCourse();
  const resumeCourse = useResumeCourse();
  const [showEdit, setShowEdit] = useState(false);
  const [suspendReason, setSuspendReason] = useState('');
  const [showSuspend, setShowSuspend] = useState(false);
  const [archiveError, setArchiveError] = useState<string | null>(null);

  const course = detail.data;
  const isSuspended = !!course?.suspendedAt;

  async function suspend() {
    setArchiveError(null);
    try {
      await suspendCourse.mutateAsync({ id, input: { reason: suspendReason.trim() } });
      setShowSuspend(false);
      setSuspendReason('');
      toast('Corso sospeso');
    } catch (e) {
      setArchiveError(describeApiError(e, 'Sospensione non riuscita'));
    }
  }

  async function resume() {
    setArchiveError(null);
    try {
      await resumeCourse.mutateAsync(id);
      toast('Corso riattivato');
    } catch (e) {
      setArchiveError(describeApiError(e, 'Riattivazione non riuscita'));
    }
  }

  async function archive() {
    setArchiveError(null);
    try {
      await archiveCourse.mutateAsync(id);
      toast('Corso archiviato');
    } catch (e) {
      setArchiveError(describeApiError(e, 'Archiviazione non riuscita'));
    }
  }

  return (
    <>
      <Drawer
        open
        onClose={onClose}
        title={course?.title ?? 'Corso'}
        subtitle={course?.vendorName}
        size="lg"
        footer={
          course ? (
            <div className={styles.footerActions}>
              <Button variant="ghost" size="md" onClick={() => setShowEdit(true)}>
                Modifica
              </Button>
              {isSuspended ? (
                <Button variant="secondary" size="md" loading={resumeCourse.isPending} onClick={resume}>
                  Riattiva
                </Button>
              ) : (
                <Button variant="ghost" size="md" onClick={() => setShowSuspend(true)}>
                  Sospendi
                </Button>
              )}
              {course.active && (
                <Button variant="danger" size="md" loading={archiveCourse.isPending} onClick={archive}>
                  Archivia
                </Button>
              )}
            </div>
          ) : undefined
        }
      >
        <div className={styles.body}>
          {detail.isLoading && <Skeleton rows={6} />}
          {detail.isError && (
            <p className={styles.errorNotice}>{describeApiError(detail.error, 'Lettura del corso non riuscita')}</p>
          )}
          {course && (
            <>
              <ErrorPanel message={archiveError} onDismiss={() => setArchiveError(null)} />
              <section className={styles.card}>
                <h3 className={styles.cardTitle}>Corso</h3>
                <dl className={styles.grid}>
                  <div className={styles.item}>
                    <dt>Aree di competenza</dt>
                    <dd>{course.skillAreas.length > 0 ? course.skillAreas.map((a) => a.name).join(', ') : '—'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Erogazione</dt>
                    <dd>{PROVIDER_KIND_LABELS[course.providerKind] ?? course.providerKind}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Fornitore abituale</dt>
                    <dd>{course.vendorName || '—'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Tag</dt>
                    <dd>{course.tags.length > 0 ? course.tags.join(', ') : '—'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Modalità</dt>
                    <dd>{DELIVERY_MODE_LABELS[course.deliveryMode] ?? course.deliveryMode}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Durata / prezzo indicativi</dt>
                    <dd>
                      {course.defaultHours !== undefined ? `${course.defaultHours} h` : '—'}
                      {course.defaultCost !== undefined ? ` · ${formatCurrency(course.defaultCost) ?? '—'}` : ''}
                    </dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Certificazione collegata</dt>
                    <dd>{course.leadsToCertName || '—'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Compliance</dt>
                    <dd>{course.complianceRelated ? course.complianceFramework || 'Sì' : 'No'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Stato</dt>
                    <dd>
                      {course.active ? 'Attivo' : 'Archiviato'}
                      {isSuspended &&
                        ` · sospeso${course.suspensionReason ? `: ${course.suspensionReason}` : ''}`}
                    </dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Formatori designati</dt>
                    <dd>{course.trainers.length > 0 ? course.trainers.map((t) => t.name).join(', ') : '—'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Promemoria</dt>
                    <dd>
                      {course.reminderText
                        ? `${course.reminderText}${course.reminderAt ? ` · richiamo ${formatDateOnly(course.reminderAt)}` : ''}`
                        : '—'}
                    </dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Nota</dt>
                    <dd>{course.notes || '—'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Visibilità</dt>
                    <dd>
                      {!course.visibility
                        ? 'Riservato a People'
                        : course.visibility.kind === 'all'
                          ? 'Tutti'
                          : course.visibility.kind === 'people'
                            ? `Persone scelte (${course.visibility.employeeIds?.length ?? 0})`
                            : 'Cerchia dedicata'}
                    </dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Origine</dt>
                    <dd>
                      {course.factorialTrainingId
                        ? course.active
                          ? 'Importato dal sync Factorial'
                          : 'Importato dal sync Factorial — da curare'
                        : 'Locale'}
                    </dd>
                  </div>
                </dl>
                {course.description && <p>{course.description}</p>}
              </section>

              <section className={styles.card}>
                <h3 className={styles.cardTitle}>Regole collegate</h3>
                {course.rules.length === 0 ? (
                  <p>Nessuna regola collegata.</p>
                ) : (
                  <ul className={localStyles.ruleList}>
                    {course.rules.map((r) => (
                      <li key={r.id}>
                        <Link to={`/regole?id=${r.id}`}>{r.name}</Link>{' '}
                        <StatusBadge
                          value={r.isActive ? 'active' : 'inactive'}
                          label={r.isActive ? 'Attiva' : 'Disattiva'}
                          variant={r.isActive ? 'success' : 'neutral'}
                        />
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              <section className={styles.card}>
                <h3 className={styles.cardTitle}>Eventi collegati</h3>
                {course.events.length === 0 ? (
                  <p>Nessun evento collegato.</p>
                ) : (
                  <div className={localStyles.tableWrap}>
                    <table className={localStyles.miniTable}>
                      <thead>
                        <tr>
                          <th>Evento</th>
                          <th>Iscrizioni</th>
                          <th>Sessioni</th>
                          <th>Annullato</th>
                        </tr>
                      </thead>
                      <tbody>
                        {course.events.map((e) => (
                          <tr key={e.id}>
                            <td>
                              <Link to={`/eventi/${e.id}`}>{formatInstantDate(e.createdAt)}</Link>
                            </td>
                            <td>{e.enrollmentsCount}</td>
                            <td>{e.sessionsCount}</td>
                            <td>{e.cancelledAt ? 'Sì' : 'No'}</td>
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

      {showSuspend && (
        <Modal open onClose={() => setShowSuspend(false)} title="Sospendi corso" size="sm">
          <div className={formStyles.body}>
            <p>Il tema resta a catalogo ma esce dalle viste operative finché non viene riattivato.</p>
            <label className={formStyles.field}>
              Motivo (facoltativo)
              <textarea
                className={formStyles.textarea}
                value={suspendReason}
                onChange={(e) => setSuspendReason(e.target.value)}
                rows={2}
                placeholder="Es. standby per accordo con partner"
              />
            </label>
            <div className={formStyles.actions}>
              <Button variant="ghost" size="md" onClick={() => setShowSuspend(false)} disabled={suspendCourse.isPending}>
                Annulla
              </Button>
              <Button variant="primary" size="md" loading={suspendCourse.isPending} onClick={suspend}>
                Sospendi
              </Button>
            </div>
          </div>
        </Modal>
      )}
      {course && showEdit && (
        <CourseEditorModal
          mode="edit"
          courseId={course.id}
          initial={course}
          open={showEdit}
          onClose={() => setShowEdit(false)}
          onSaved={() => setShowEdit(false)}
        />
      )}
    </>
  );
}
