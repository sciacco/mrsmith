// Creazione/modifica persona (#158, §Persone 1): stato-dai-fatti, nessuna
// prevenzione locale sul blocco di gestione manuale — se nome, email, stato
// o team provengono dalla directory esterna il backend risponde
// person_managed_by_directory e l'errore si mostra per intero.
// «Gestione manuale» (directoryExempt) è la valvola che sblocca la modifica:
// disponibile solo in modifica, mai in creazione (non esiste ancora una
// persona da sganciare dalla sync).
// Il form invia un `teamId` singolo: quando la persona ha più di
// un'appartenenza attiva il backend (membershipReplacementNeeded) le chiude
// tutte e ne ricrea una sola. Nessuna previsione locale di quale tenere:
// l'effetto va solo dichiarato in chiaro prima del salvataggio (QA task
// 6.7). Lo stesso vale per il campo Note, che sostituisce integralmente il
// valore esistente senza poterlo mostrare (PersonListRow non espone `notes`
// in lettura).

import { useState } from 'react';
import { Button, Modal, SingleSelect, ToggleSwitch, VisuallyHidden } from '@mrsmith/ui';
import { useCreatePerson, useTrainingTeams, useUpdatePerson } from '../../api/queries';
import type { PersonListRow } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { PERSON_STATUS_LABELS } from '../../lib/labels';
import styles from '../requests/requestShared.module.css';
import peopleStyles from './people.module.css';

const STATUS_OPTIONS = Object.entries(PERSON_STATUS_LABELS).map(([value, label]) => ({ value, label }));

interface PersonEditorModalProps {
  mode: 'create' | 'edit';
  personId?: string;
  initial?: PersonListRow;
  open: boolean;
  onClose: () => void;
  onSaved: (id: string) => void;
}

export function PersonEditorModal({ mode, personId, initial, open, onClose, onSaved }: PersonEditorModalProps) {
  const teams = useTrainingTeams();
  const createPerson = useCreatePerson();
  const updatePerson = useUpdatePerson();

  const [firstName, setFirstName] = useState(initial?.firstName ?? '');
  const [lastName, setLastName] = useState(initial?.lastName ?? '');
  const [email, setEmail] = useState(initial?.email ?? '');
  const [status, setStatus] = useState(initial?.status ?? 'active');
  const [teamId, setTeamId] = useState(initial?.teams[0]?.id ?? '');
  const [notes, setNotes] = useState('');
  const [directoryExempt, setDirectoryExempt] = useState(initial?.directoryExempt ?? false);
  const [error, setError] = useState<string | null>(null);

  if (!open) return null;

  const pending = createPerson.isPending || updatePerson.isPending;
  const canSubmit = firstName.trim() !== '' && lastName.trim() !== '' && email.trim() !== '';

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    const base = {
      firstName: firstName.trim(),
      lastName: lastName.trim(),
      email: email.trim(),
      status,
      teamId: teamId || undefined,
      notes: notes.trim() || undefined,
    };
    try {
      if (mode === 'edit' && personId) {
        await updatePerson.mutateAsync({ id: personId, input: { ...base, directoryExempt } });
        onSaved(personId);
      } else {
        const response = await createPerson.mutateAsync(base);
        if (response.id) onSaved(response.id);
      }
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={mode === 'create' ? 'Nuova persona' : 'Modifica persona'} size="md">
      <div className={styles.body}>
        <div className={styles.row}>
          <label className={styles.field}>
            <span className={styles.labelHead}>
              Nome
              <span className={styles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input className={styles.input} value={firstName} onChange={(e) => setFirstName(e.target.value)} />
          </label>
          <label className={styles.field}>
            <span className={styles.labelHead}>
              Cognome
              <span className={styles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input className={styles.input} value={lastName} onChange={(e) => setLastName(e.target.value)} />
          </label>
        </div>

        <label className={styles.field}>
          <span className={styles.labelHead}>
            Email
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <input type="email" className={styles.input} value={email} onChange={(e) => setEmail(e.target.value)} />
        </label>

        <div className={styles.row}>
          <label className={styles.field}>
            Stato
            <SingleSelect
              options={STATUS_OPTIONS}
              selected={status}
              onChange={(v) => setStatus(v ?? 'active')}
              placeholder="Seleziona stato..."
            />
          </label>
          <label className={styles.field}>
            Team
            <SingleSelect
              options={(teams.data ?? []).filter((t) => t.active).map((t) => ({ value: t.id, label: t.name }))}
              selected={teamId || null}
              onChange={(v) => setTeamId(v ?? '')}
              placeholder="Nessun team"
              allowClear
            />
          </label>
        </div>

        {mode === 'edit' && initial && initial.teams.length > 1 && (
          <div className={peopleStyles.multiTeamNotice} role="note">
            <p className={peopleStyles.multiTeamNoticeTitle}>Appartenenze attuali multiple</p>
            <p className={peopleStyles.multiTeamNoticeText}>
              {initial.teams.map((t) => (t.role === 'lead' ? `${t.name} (lead)` : t.name)).join(', ')}
            </p>
            <p className={peopleStyles.multiTeamNoticeText}>
              Salvando resta solo il team scelto sopra: le altre appartenenze vengono chiuse.
            </p>
          </div>
        )}

        <label className={styles.field}>
          Note
          <textarea className={styles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
          <span className={styles.hint}>Le note esistenti non sono leggibili qui: salvando vengono sostituite da questo campo.</span>
        </label>

        {mode === 'edit' && (
          <div className={styles.toggleGroup}>
            <ToggleSwitch
              id="person-directory-exempt"
              checked={directoryExempt}
              onChange={setDirectoryExempt}
              label="Gestione manuale (esclude dalla sincronizzazione anagrafica)"
            />
          </div>
        )}

        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={pending} disabled={!canSubmit} onClick={submit}>
            {mode === 'create' ? 'Crea persona' : 'Salva modifiche'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
