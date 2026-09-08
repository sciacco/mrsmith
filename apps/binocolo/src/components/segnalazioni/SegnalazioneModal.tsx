import { Button, Icon, Modal, useToast } from '@mrsmith/ui';
import { useState, type FormEvent } from 'react';
import type { Segnalazione, SegnalazioneWrite } from '../../api/types';
import { useSegnalazioneMutations } from '../../hooks/useSegnalazioni';
import { SEGNALAZIONE_EMPTY_RULE, emptySegnalazioneWrite, segnalazioneHasContent } from '../../lib/segnalazioneStates';
import { errorLabel } from '../../pages/ricerche/helpers';
import { SegnalazioneFields } from './SegnalazioneFields';
import styles from './segnalazioni.module.css';

/** Form «Nuova segnalazione», riusato in Pipeline e nella pagina Segnalazioni.
 *  Salva direttamente via API; al successo toast, reset e `onCreated`. In caso
 *  di errore il messaggio resta inline e i dati digitati sono conservati. */
export function SegnalazioneModal({ open, onClose, onCreated }: {
  open: boolean;
  onClose: () => void;
  onCreated?: (segnalazione: Segnalazione) => void;
}) {
  const { create } = useSegnalazioneMutations();
  const { toast } = useToast();
  const [value, setValue] = useState<SegnalazioneWrite>(emptySegnalazioneWrite);
  const [error, setError] = useState('');
  const submitting = create.isPending;

  function close() {
    if (submitting) return;
    setValue(emptySegnalazioneWrite());
    setError('');
    onClose();
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!segnalazioneHasContent(value)) {
      setError(SEGNALAZIONE_EMPTY_RULE);
      return;
    }
    setError('');
    try {
      const created = await create.mutateAsync(value);
      toast('Segnalazione registrata', 'success');
      setValue(emptySegnalazioneWrite());
      onClose();
      onCreated?.(created);
    } catch (cause) {
      setError(errorLabel(cause));
    }
  }

  return (
    <Modal open={open} onClose={close} title="Nuova segnalazione" size="lg" dismissible={!submitting}>
      <form className={styles.form} onSubmit={(event) => void submit(event)} noValidate>
        <SegnalazioneFields
          idPrefix="segnalazione-new"
          value={value}
          onChange={(next) => { setValue(next); if (error && segnalazioneHasContent(next)) setError(''); }}
          disabled={submitting}
          invalid={error === SEGNALAZIONE_EMPTY_RULE}
          describedBy={error ? 'segnalazione-new-error' : 'segnalazione-new-hint'}
        />
        <p id="segnalazione-new-hint" className={styles.hint}>Tutti i campi sono facoltativi. Serve almeno uno tra nome, sito web, identificativo fiscale e note.</p>
        {error ? <div id="segnalazione-new-error" className={styles.formError} role="alert"><Icon name="triangle-alert" size={16} /><span>{error}</span></div> : null}
        <div className={styles.actions}>
          <Button type="button" variant="secondary" onClick={close} disabled={submitting}>Annulla</Button>
          <Button type="submit" loading={submitting} leftIcon={<Icon name="plus" />}>Registra segnalazione</Button>
        </div>
      </form>
    </Modal>
  );
}
