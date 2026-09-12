import { useState, type FormEvent } from 'react';
import { Button, Modal, SingleSelect, VisuallyHidden, useToast } from '@mrsmith/ui';
import { useSaveNeedOption, useTrainingCourses } from '../../api/queries';
import type { NeedOption, NeedDetail } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import form from '../requests/requestShared.module.css';

export function NeedOptionModal({ need, option, onClose }: { need: NeedDetail; option?: NeedOption; onClose: () => void }) {
  const courses = useTrainingCourses();
  const save = useSaveNeedOption();
  const { toast } = useToast();
  const [mode, setMode] = useState('existing');
  const [courseId, setCourseId] = useState('');
  const [title, setTitle] = useState('');
  const [notes, setNotes] = useState(option?.notes ?? '');
  const [rank, setRank] = useState(option?.rank?.toString() ?? '');
  const [error, setError] = useState<string | null>(null);
  const available = (courses.data ?? []).filter((c) => !need.candidates.some((n) => n.courseId === c.id));

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await save.mutateAsync({ id: need.id, courseId: option?.courseId, input: {
        courseId: !option && mode === 'existing' ? courseId : undefined,
        newCourseTitle: !option && mode === 'new' ? title.trim() : undefined,
        notes, rank: rank === '' ? undefined : Number(rank),
      } });
      toast(option ? 'Corso aggiornato' : 'Corso aggiunto');
      onClose();
    } catch (e) { setError(describeApiError(e, 'Salvataggio del corso non riuscito')); }
  }

  return (
    <Modal open onClose={onClose} title={option ? 'Modifica corso' : 'Corso da aggiungere'} size="lg">
      <form className={`${form.body} ${form.bodyModal}`} onSubmit={submit}>
        {option ? <strong>{option.courseTitle}</strong> : <>
          <div className={form.field}>
            <SingleSelect<string> options={[{ value: 'existing', label: 'Corso esistente' }, { value: 'new', label: 'Nuovo corso da titolo' }]} selected={mode} onChange={(v) => setMode(v ?? 'existing')} />
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
        <label className={form.field}>Nota<textarea className={form.textarea} rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} /></label>
        <label className={form.field}>Posizione<input className={form.input} type="number" step="1" value={rank} onChange={(e) => setRank(e.target.value)} /></label>
        {error && <ErrorPanel message={error} />}
        <div className={form.actions}>
          <Button variant="ghost" onClick={onClose}>Annulla</Button>
          <Button type="submit" loading={save.isPending} disabled={!option && !(mode === 'existing' ? courseId : title.trim())}>{option ? 'Salva modifiche' : 'Aggiungi corso'}</Button>
        </div>
      </form>
    </Modal>
  );
}
