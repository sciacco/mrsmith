import { useState, type FormEvent } from 'react';
import { Button, Modal, MultiSelect, VisuallyHidden, useToast } from '@mrsmith/ui';
import { useSaveNeed, useTrainingSkillAreas } from '../../api/queries';
import type { Need } from '../../api/types';
import { needInput } from '../../lib/needs';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import form from '../requests/requestShared.module.css';

export function NeedEditorModal({ need, onClose, onCreated }: {
  need?: Need;
  onClose: () => void;
  onCreated?: (id: string) => void;
}) {
  const save = useSaveNeed();
  const areas = useTrainingSkillAreas();
  const { toast } = useToast();
  const [description, setDescription] = useState(need?.description ?? '');
  const [skillAreaIds, setSkillAreaIds] = useState(need?.skillAreas.map((a) => a.id) ?? []);
  const [notes, setNotes] = useState(need?.notes ?? '');
  const [reminderText, setReminderText] = useState(need?.reminderText ?? '');
  const [reminderAt, setReminderAt] = useState(need?.reminderAt ?? '');
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      const result = await save.mutateAsync({ id: need?.id, input: {
        ...(need ? needInput(need) : { status: 'new' as const }),
        description: description.trim(), skillAreaIds, notes, reminderText, reminderAt,
      } });
      toast(need ? 'Esigenza aggiornata' : 'Esigenza creata');
      onClose();
      if (!need && result.id) onCreated?.(result.id);
    } catch (e) { setError(describeApiError(e, 'Salvataggio non riuscito')); }
  }

  return (
    <Modal open onClose={onClose} title={need ? 'Modifica esigenza' : 'Nuova esigenza'} size="lg">
      <form className={`${form.body} ${form.bodyModal}`} onSubmit={submit}>
        <label className={form.field}>
          <span className={form.labelHead}>Descrizione<span className={form.requiredMarker} aria-hidden="true" /><VisuallyHidden>obbligatorio</VisuallyHidden></span>
          <textarea className={form.textarea} required rows={2} value={description} onChange={(e) => setDescription(e.target.value)} autoFocus />
        </label>
        <div className={form.field}>Aree di competenza
          <MultiSelect<string> options={(areas.data ?? []).map((a) => ({ value: a.id, label: a.name }))} selected={skillAreaIds} onChange={setSkillAreaIds} placeholder="Seleziona aree…" />
        </div>
        {areas.isError && <ErrorPanel message="Aree non disponibili. Riapri il modulo per riprovare." />}
        <label className={form.field}>Note<textarea className={form.textarea} rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} /></label>
        <div className={form.row}>
          <label className={form.field}>Promemoria<input className={form.input} value={reminderText} onChange={(e) => setReminderText(e.target.value)} /></label>
          <label className={form.field}>Data promemoria<input className={form.input} type="date" value={reminderAt} onChange={(e) => setReminderAt(e.target.value)} /></label>
        </div>
        {error && <ErrorPanel message={error} />}
        <div className={form.actions}>
          <Button variant="ghost" onClick={onClose}>Annulla</Button>
          <Button type="submit" loading={save.isPending} disabled={!description.trim()}>{need ? 'Salva modifiche' : 'Crea esigenza'}</Button>
        </div>
      </form>
    </Modal>
  );
}
