// Form evento completo, riusato da creazione (EventsPage) e modifica
// (EventDetailPage): il PUT e una sostituzione integrale, quindi il form
// invia sempre tutti i campi. Nessuna prevenzione locale degli invarianti
// del backend (corso legato a regola/richiesta, evento con sessioni): il
// campo corso resta sempre modificabile, l'eventuale 409 si mostra per
// intero.

import { useState } from 'react';
import { Button, Modal, MoneyInput, MultiSelect, SingleSelect, VisuallyHidden } from '@mrsmith/ui';
import type { EventInput, LookupItem } from '../../api/types';
import { ErrorPanel } from './ErrorPanel';
import styles from './EventFormModal.module.css';
import formStyles from '../requests/requestShared.module.css';

function selectableOptions(items: LookupItem[], currentId: string | undefined) {
  const options = items.filter((i) => i.active).map((i) => ({ value: i.id, label: i.label }));
  if (currentId && !options.some((o) => o.value === currentId)) {
    const current = items.find((i) => i.id === currentId);
    if (current) options.push({ value: current.id, label: `${current.label} (non attivo)` });
  }
  return options;
}

interface EventFormModalProps {
  open: boolean;
  mode: 'create' | 'edit';
  initial?: EventInput;
  courses: LookupItem[];
  vendors: LookupItem[];
  people?: LookupItem[];
  pending: boolean;
  error?: string | null;
  onSubmit: (input: EventInput) => void;
  onClose: () => void;
}

export function EventFormModal({
  open,
  mode,
  initial,
  courses,
  vendors,
  people = [],
  pending,
  error,
  onSubmit,
  onClose,
}: EventFormModalProps) {
  const [courseId, setCourseId] = useState(initial?.courseId ?? '');
  const [title, setTitle] = useState(initial?.title ?? '');
  const [vendorId, setVendorId] = useState(initial?.vendorId ?? '');
  const [agreedPrice, setAgreedPrice] = useState(
    initial?.agreedPrice !== undefined ? initial.agreedPrice.toFixed(2) : '',
  );
  const [agreedConditions, setAgreedConditions] = useState(initial?.agreedConditions ?? '');
  const [notes, setNotes] = useState(initial?.notes ?? '');
  const [reminderText, setReminderText] = useState(initial?.reminderText ?? '');
  const [reminderAt, setReminderAt] = useState(initial?.reminderAt ?? '');
  const [trainerIds, setTrainerIds] = useState<string[]>(initial?.trainerIds ?? []);

  if (!open) return null;

  function submit() {
    onSubmit({
      courseId,
      title: mode === 'edit' ? title.trim() || undefined : undefined,
      vendorId: vendorId || undefined,
      agreedPrice: agreedPrice === '' ? undefined : Number(agreedPrice),
      agreedConditions: agreedConditions.trim() || undefined,
      notes: notes.trim() || undefined,
      reminderText: reminderText.trim() || undefined,
      reminderAt: reminderAt || undefined,
      trainerIds: mode === 'edit' ? trainerIds : undefined,
    });
  }

  return (
    <Modal open={open} onClose={onClose} title={mode === 'create' ? 'Nuovo evento' : 'Modifica evento'} size="md">
      <div className={styles.body}>
        <label className={styles.field}>
          <span className={styles.labelHead}>
            Corso
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <SingleSelect
            options={selectableOptions(courses, initial?.courseId)}
            selected={courseId || null}
            onChange={(v) => setCourseId(v ?? '')}
            placeholder="Seleziona corso..."
          />
        </label>
        <label className={styles.field}>
          Fornitore
          <SingleSelect
            options={selectableOptions(vendors, initial?.vendorId)}
            selected={vendorId || null}
            onChange={(v) => setVendorId(v ?? '')}
            placeholder="Nessun fornitore"
            allowClear
          />
        </label>
        {mode === 'edit' && (
          <label className={styles.field}>
            Titolo dell'evento
            <input className={formStyles.input} value={title} onChange={(e) => setTitle(e.target.value)} />
          </label>
        )}
        {mode === 'create' && (
          <p className={formStyles.hint}>Il titolo viene ereditato dal corso; si può cambiare dopo la creazione.</p>
        )}
        {mode === 'edit' && (
          <label className={styles.field}>
            Formatori interni
            <MultiSelect<string>
              options={people.filter((p) => p.active).map((p) => ({ value: p.id, label: p.label }))}
              selected={trainerIds}
              onChange={setTrainerIds}
              placeholder="Nessuno"
            />
          </label>
        )}
        <div className={formStyles.row}>
          <label className={styles.field}>
            Promemoria (in attesa di / prossimo passo)
            <input className={formStyles.input} value={reminderText} onChange={(e) => setReminderText(e.target.value)} />
          </label>
          <label className={styles.field}>
            Data di richiamo
            <input type="date" className={formStyles.input} value={reminderAt} onChange={(e) => setReminderAt(e.target.value)} />
          </label>
        </div>
        <MoneyInput label="Prezzo pattuito" value={agreedPrice} onChange={setAgreedPrice} />
        <label className={styles.field}>
          Condizioni
          <textarea
            className={styles.textarea}
            value={agreedConditions}
            onChange={(e) => setAgreedConditions(e.target.value)}
            rows={2}
          />
        </label>
        <label className={styles.field}>
          Note
          <textarea className={styles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </label>
        <ErrorPanel message={error ?? null} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={pending} disabled={courseId === ''} onClick={submit}>
            {mode === 'create' ? 'Crea evento' : 'Salva modifiche'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
