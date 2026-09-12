import { useState, type FormEvent } from 'react';
import { Button, Modal, SingleSelect, VisuallyHidden, useToast } from '@mrsmith/ui';
import { useSaveNeedCandidate, useTrainingCourses } from '../../api/queries';
import type { NeedCandidate, NeedDetail } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import form from '../requests/requestShared.module.css';

export function NeedCandidateModal({ need, candidate, onClose }: { need: NeedDetail; candidate?: NeedCandidate; onClose: () => void }) {
  const courses = useTrainingCourses();
  const save = useSaveNeedCandidate();
  const { toast } = useToast();
  const [mode, setMode] = useState('existing');
  const [courseId, setCourseId] = useState('');
  const [title, setTitle] = useState('');
  const [notes, setNotes] = useState(candidate?.notes ?? '');
  const [rank, setRank] = useState(candidate?.rank?.toString() ?? '');
  const [error, setError] = useState<string | null>(null);
  const available = (courses.data ?? []).filter((c) => !need.candidates.some((n) => n.courseId === c.id));

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await save.mutateAsync({ id: need.id, courseId: candidate?.courseId, input: {
        courseId: !candidate && mode === 'existing' ? courseId : undefined,
        newCourseTitle: !candidate && mode === 'new' ? title.trim() : undefined,
        notes, rank: rank === '' ? undefined : Number(rank),
      } });
      toast(candidate ? 'Candidato aggiornato' : 'Candidato aggiunto');
      onClose();
    } catch (e) { setError(describeApiError(e, 'Salvataggio candidato non riuscito')); }
  }

  return (
    <Modal open onClose={onClose} title={candidate ? 'Modifica candidato' : 'Aggiungi candidato'} size="lg">
      <form className={`${form.body} ${form.bodyModal}`} onSubmit={submit}>
        {candidate ? <strong>{candidate.courseTitle}</strong> : <>
          <div className={form.field}>Candidato
            <SingleSelect<string> ariaLabel="Candidato" options={[{ value: 'existing', label: 'Corso esistente' }, { value: 'new', label: 'Nuovo corso da titolo' }]} selected={mode} onChange={(v) => setMode(v ?? 'existing')} />
          </div>
          {mode === 'existing' ? <>
            <div className={form.field}>Corso
              <SingleSelect<string> ariaLabel="Corso" options={available.map((c) => ({ value: c.id, label: `${c.title}${c.active ? '' : ' · Non attivo'}` }))} selected={courseId || null} onChange={(v) => setCourseId(v ?? '')} disabled={courses.isPending || courses.isError} />
            </div>
            {courses.isError && <ErrorPanel message="Corsi non disponibili. Riapri il modulo per riprovare." />}
          </> : <label className={form.field}>
            <span className={form.labelHead}>Titolo del corso<span className={form.requiredMarker} aria-hidden="true" /><VisuallyHidden>obbligatorio</VisuallyHidden></span>
            <input className={form.input} required value={title} onChange={(e) => setTitle(e.target.value)} />
            <span className={form.hint}>Fornitore, prezzo, ore e link si completano nella scheda corso.</span>
          </label>}
        </>}
        <label className={form.field}>Nota sul candidato<textarea className={form.textarea} rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} /></label>
        <label className={form.field}>Rango<input className={form.input} type="number" step="1" value={rank} onChange={(e) => setRank(e.target.value)} /></label>
        {error && <ErrorPanel message={error} />}
        <div className={form.actions}>
          <Button variant="ghost" onClick={onClose}>Annulla</Button>
          <Button type="submit" loading={save.isPending} disabled={!candidate && !(mode === 'existing' ? courseId : title.trim())}>{candidate ? 'Salva modifiche' : 'Aggiungi candidato'}</Button>
        </div>
      </form>
    </Modal>
  );
}
