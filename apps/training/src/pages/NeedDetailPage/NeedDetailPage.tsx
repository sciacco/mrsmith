import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Button, Modal, SingleSelect, Skeleton, StatusBadge } from '@mrsmith/ui';
import { formatLocalDate, formatNumber } from '@mrsmith/format';
import { useDeleteNeed, useNeedDetail, useRemoveNeedOption, useRemoveNeedRequest, useSaveNeed } from '../../api/queries';
import type { NeedAcceptResponse, NeedOption, NeedStatus } from '../../api/types';
import { NEED_STATES, needInput } from '../../lib/needs';
import { REQUEST_OUTCOME_LABELS } from '../../lib/labels';
import { NeedEditorModal } from '../../components/needs/NeedEditorModal';
import { NeedOptionModal } from '../../components/needs/NeedOptionModal';
import { NeedRequestsModal } from '../../components/needs/NeedRequestsModal';
import { NeedAcceptModal } from '../../components/needs/NeedAcceptModal';
import { CourseDetailDrawer } from '../../components/catalog/CourseDetailDrawer';
import { RequestDetailDrawer } from '../../components/requests/RequestDetailDrawer';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { describeApiError } from '../../components/events/apiErrors';
import { outcomeVariant } from '../../components/requests/requestVariants';
import page from '../NeedsPage/NeedsPage.module.css';
import table from '../RequestsPage/listPage.module.css';
import styles from './NeedDetailPage.module.css';

export function NeedDetailPage() {
  const { id } = useParams();
  const detail = useNeedDetail(id);
  const save = useSaveNeed();
  const removeOption = useRemoveNeedOption();
  const removeRequest = useRemoveNeedRequest();
  const deleteNeed = useDeleteNeed();
  const navigate = useNavigate();
  const [editing, setEditing] = useState(false);
  const [option, setOption] = useState<NeedOption | 'new' | null>(null);
  const [linking, setLinking] = useState(false);
  const [accepting, setAccepting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [courseId, setCourseId] = useState<string>();
  const [requestId, setRequestId] = useState<string>();
  const [error, setError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [result, setResult] = useState<NeedAcceptResponse | null>(null);
  const need = detail.data;

  async function update(fields: { status?: NeedStatus; finalCourseId?: string }) {
    if (!need) return;
    setError(null);
    try { await save.mutateAsync({ id: need.id, input: { ...needInput(need), ...fields } }); }
    catch (e) { setError(describeApiError(e, 'Salvataggio non riuscito')); }
  }
  async function unlinkCourse(courseId: string) {
    if (!need) return;
    setError(null);
    try { await removeOption.mutateAsync({ id: need.id, courseId }); }
    catch (e) { setError(describeApiError(e, 'Rimozione del corso non riuscita')); }
  }
  async function unlinkRequest(requestId: string) {
    if (!need) return;
    setError(null);
    try { await removeRequest.mutateAsync({ id: need.id, requestId }); }
    catch (e) { setError(describeApiError(e, 'Scollegamento richiesta non riuscito')); }
  }
  async function confirmDelete() {
    if (!need) return;
    setDeleteError(null);
    try { await deleteNeed.mutateAsync(need.id); navigate('/esigenze'); }
    catch (e) { setDeleteError(describeApiError(e, 'Eliminazione non riuscita')); }
  }

  return (
    <div className={page.page}>
      <Link to="/esigenze">← Esigenze formative</Link>
      {detail.isPending ? <Skeleton rows={6} /> : detail.isError ? (
        <div><ErrorPanel message={describeApiError(detail.error, 'Esigenza non disponibile')} /><Button variant="secondary" onClick={() => void detail.refetch()}>Riprova</Button></div>
      ) : need ? <>
        <header className={page.header}>
          <h1 className={page.title}>{need.description}</h1>
          <div className={styles.actions}>
            <Button variant="secondary" onClick={() => setEditing(true)}>Modifica esigenza</Button>
            <Button variant="ghost" onClick={() => { setDeleteError(null); setDeleting(true); }}>Elimina esigenza</Button>
          </div>
        </header>
        {error && <ErrorPanel message={error} />}
        <section className={styles.section} aria-label="Dati dell’esigenza">
          <div className={styles.status}>
            <SingleSelect<NeedStatus> ariaLabel="Stato" options={NEED_STATES} selected={need.status} onChange={(status) => status && void update({ status })} disabled={save.isPending} />
          </div>
          {need.skillAreas.length > 0 && <p className={styles.muted}>Aree: {need.skillAreas.map((a) => a.name).join(' · ')}</p>}
          {need.notes && <p className={styles.note}>{need.notes}</p>}
          {need.reminderText && <p className={styles.note}><strong>Promemoria{need.reminderAt ? ` · ${formatLocalDate(need.reminderAt)}` : ''}</strong><br />{need.reminderText}</p>}
        </section>
        {result && <div className={styles.result} role="status">
          <strong>{formatNumber(result.acceptedRequestIds.length)} {result.acceptedRequestIds.length === 1 ? 'richiesta accolta' : 'richieste accolte'}</strong>
          {result.eventId && <> · <Link to={`/eventi/${result.eventId}`}>Apri evento</Link></>}
          {result.excludedRequests.length > 0 && <>
            <p>Escluse dall’accoglimento:</p>
            <ul className={styles.requestList}>{result.excludedRequests.map((r) => <li key={r.id}>{r.employeeName} · {r.description} · {REQUEST_OUTCOME_LABELS[r.outcome!]}{r.acceptedCourseTitle ? ` · ${r.acceptedCourseTitle}` : ''}</li>)}</ul>
          </>}
        </div>}
        <section className={styles.section} aria-labelledby="need-candidates-title">
          <div className={page.header}>
            <h2 id="need-candidates-title" className={styles.sectionTitle}>Corsi da valutare</h2>
            <Button variant="secondary" onClick={() => setOption('new')}>Aggiungi Corso</Button>
          </div>
          {need.candidates.length > 0 ? <>
            <div className={styles.finalCourse}>
              <p className={styles.muted}>Corso definitivo</p>
              <SingleSelect<string> ariaLabel="Corso definitivo" placeholder="Scegli tra i corsi da valutare…" selected={need.finalCourseId ?? null} allowClear clearLabel="Nessun definitivo"
                options={need.candidates.map((c) => ({ value: c.courseId, label: c.courseTitle }))}
                onChange={(value) => void update({ finalCourseId: value ?? '' })} disabled={save.isPending} />
            </div>
            {need.candidates.map((c) => (
              <article key={c.courseId} className={styles.option}>
                <div className={page.header}>
                  <div className={styles.actions}>
                    <button className={styles.courseButton} onClick={() => setCourseId(c.courseId)}>{c.courseTitle}</button>
                    {c.courseId === need.finalCourseId && <StatusBadge value="Definitivo" variant="success" />}
                    {!c.isActive && <StatusBadge value="Non attivo" variant="neutral" />}
                    {c.rank != null && <span className={styles.muted}>Posizione {formatNumber(c.rank)}</span>}
                  </div>
                  <div className={styles.actions}>
                    <Button variant="ghost" onClick={() => setOption(c)}>Modifica nota e posizione</Button>
                    <Button variant="ghost" onClick={() => void unlinkCourse(c.courseId)} disabled={removeOption.isPending || c.courseId === need.finalCourseId} title={c.courseId === need.finalCourseId ? 'Cambia o rimuovi prima il definitivo' : undefined}>Rimuovi</Button>
                  </div>
                </div>
                {c.notes && <p className={styles.note}>{c.notes}</p>}
                {need.requests.length > 0 && <details className={styles.coverage}>
                  <summary>Anti-doppione per persona{need.coverage.some((coverage) => coverage.courseId === c.courseId && (coverage.completedEnrollments.length > 0 || coverage.validAwards.length > 0)) ? ' · Copertura esistente' : ''}</summary>
                  <ul>{need.coverage.filter((coverage) => coverage.courseId === c.courseId).map((coverage) => <li key={coverage.employeeId}>
                    <strong>{coverage.employeeName}</strong>
                    {!coverage.completedEnrollments.length && !coverage.validAwards.length ? ' · Nessuna copertura esistente' : <ul>
                      {coverage.completedEnrollments.map((en) => <li key={en.enrollmentId}><Link to={`/eventi/${en.eventId}`}>Corso completato</Link> · {formatLocalDate(en.completedOn)}</li>)}
                      {coverage.validAwards.map((award) => <li key={award.awardId}>{award.certificationName} · conseguita il {formatLocalDate(award.awardedOn)}{award.expiresOn ? ` · valida fino al ${formatLocalDate(award.expiresOn)}` : ' · senza scadenza'}</li>)}
                    </ul>}
                  </li>)}</ul>
                </details>}
              </article>
            ))}
          </> : <p className={styles.muted}>Aggiungi un corso esistente o un corso da solo titolo.</p>}
        </section>
        <section className={styles.section} aria-labelledby="need-requests-title">
          <div className={page.header}>
            <h2 id="need-requests-title" className={styles.sectionTitle}>Richieste collegate</h2>
            <div className={styles.actions}>
              <Button variant="secondary" onClick={() => setLinking(true)}>Collega richieste</Button>
              <Button onClick={() => setAccepting(true)} disabled={!need.finalCourseId || !need.requests.some((r) => !r.outcome) || save.isPending}>Accogli su questo corso ed evento</Button>
            </div>
          </div>
          {!need.finalCourseId && need.requests.some((r) => !r.outcome) && <p className={styles.muted}>Scegli il corso definitivo tra i corsi da valutare per accogliere le richieste insieme.</p>}
          {need.requests.length > 0 ? <div className={table.tableWrap}>
            <table className={table.table}>
              <thead><tr><th>Persona / richiesta</th><th>Stato</th><th>Corso accolto</th><th>Azioni</th></tr></thead>
              <tbody>{need.requests.map((r) => <tr key={r.id}>
                <td className={table.wrapCell}>
                  <button className={styles.courseButton} onClick={() => setRequestId(r.id)}>{r.employeeName}</button>
                  <span className={table.cellSecondary}>{r.description}</span>
                </td>
                <td>
                  <StatusBadge value={r.outcome ? REQUEST_OUTCOME_LABELS[r.outcome] : r.suspendedAt ? 'Sospesa' : 'Aperta'} variant={r.outcome ? outcomeVariant(r.outcome) : 'neutral'} />
                  {r.outcome && <span className={table.cellSecondary}>Esclusa dall’accoglimento</span>}
                </td>
                <td className={table.wrapCell}>{r.acceptedCourseId ? <button className={styles.courseButton} onClick={() => setCourseId(r.acceptedCourseId)}>{r.acceptedCourseTitle}</button> : '—'}</td>
                <td><Button variant="ghost" onClick={() => void unlinkRequest(r.id)} disabled={removeRequest.isPending}>Scollega</Button></td>
              </tr>)}</tbody>
            </table>
          </div> : <p className={styles.muted}>Nessuna richiesta collegata. Puoi aggiungerle anche in seguito.</p>}
        </section>
        {editing && <NeedEditorModal need={need} onClose={() => setEditing(false)} />}
        {option && <NeedOptionModal need={need} option={option === 'new' ? undefined : option} onClose={() => setOption(null)} />}
        {linking && <NeedRequestsModal need={need} onClose={() => setLinking(false)} />}
        {accepting && <NeedAcceptModal need={need} onClose={() => setAccepting(false)} onAccepted={setResult} />}
        <Modal open={deleting} onClose={() => setDeleting(false)} title="Elimina esigenza" size="md">
          <p>L’esigenza scompare dalle liste. Le richieste e i loro collegamenti vengono conservati.</p>
          {deleteError && <ErrorPanel message={deleteError} />}
          <div className={styles.actions}>
            <Button variant="ghost" onClick={() => setDeleting(false)}>Annulla</Button>
            <Button variant="danger" onClick={() => void confirmDelete()} loading={deleteNeed.isPending}>Elimina esigenza</Button>
          </div>
        </Modal>
      </> : null}
      {courseId && <CourseDetailDrawer id={courseId} onClose={() => setCourseId(undefined)} />}
      {requestId && <RequestDetailDrawer id={requestId} onClose={() => setRequestId(undefined)} />}
    </div>
  );
}
