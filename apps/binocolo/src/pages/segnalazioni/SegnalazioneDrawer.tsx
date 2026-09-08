import { useState, type CSSProperties, type FormEvent } from 'react';
import { Button, Drawer, Icon } from '@mrsmith/ui';
import { formatInstant } from '@mrsmith/format';
import type { Segnalazione, SegnalazioneState, SegnalazioneWrite } from '../../api/types';
import { useSegnalazioneMutations } from '../../hooks/useSegnalazioni';
import {
  SEGNALAZIONE_EMPTY_RULE,
  SEGNALAZIONE_STATES,
  segnalazioneEditable,
  segnalazioneHasContent,
  segnalazioneStateLabel,
  segnalazioneStateVars,
  segnalazioneTitle,
  toSegnalazioneWrite,
} from '../../lib/segnalazioneStates';
import { errorLabel } from '../ricerche/helpers';
import { SegnalazioneFields, SegnalazioneReadonly } from '../../components/segnalazioni/SegnalazioneFields';
import form from '../../components/segnalazioni/segnalazioni.module.css';
import styles from './Segnalazioni.module.css';

/** Corpo modificabile. Rimontato dal padre (chiave id+stato) così la bozza
 *  parte sempre dai contenuti persistiti quando la segnalazione cambia. */
function EditableBody({ item, onSaved }: { item: Segnalazione; onSaved: () => void }) {
  const { updateContent } = useSegnalazioneMutations();
  const [value, setValue] = useState<SegnalazioneWrite>(() => toSegnalazioneWrite(item));
  const [error, setError] = useState('');
  const saving = updateContent.isPending;
  const dirty = JSON.stringify(value) !== JSON.stringify(toSegnalazioneWrite(item));

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!segnalazioneHasContent(value)) {
      setError(SEGNALAZIONE_EMPTY_RULE);
      return;
    }
    setError('');
    try {
      await updateContent.mutateAsync({ id: item.id, body: value });
      onSaved();
    } catch (cause) {
      setError(errorLabel(cause));
    }
  }

  return (
    <form id="segnalazione-edit-form" className={form.form} onSubmit={(event) => void submit(event)} noValidate>
      <SegnalazioneFields
        idPrefix="segnalazione-edit"
        value={value}
        onChange={(next) => { setValue(next); if (error && segnalazioneHasContent(next)) setError(''); }}
        disabled={saving}
        invalid={error === SEGNALAZIONE_EMPTY_RULE}
        describedBy={error ? 'segnalazione-edit-error' : 'segnalazione-edit-hint'}
      />
      <p id="segnalazione-edit-hint" className={form.hint}>Serve almeno uno tra nome, sito web, identificativo fiscale e note.</p>
      {error ? <div id="segnalazione-edit-error" className={form.formError} role="alert"><Icon name="triangle-alert" size={16} /><span>{error}</span></div> : null}
      <div className={form.actions}>
        <Button type="submit" loading={saving} disabled={!dirty}>Salva contenuti</Button>
      </div>
    </form>
  );
}

export function SegnalazioneDrawer({ item, onClose, onSaved }: {
  item: Segnalazione;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { setState } = useSegnalazioneMutations();
  const [stateError, setStateError] = useState('');
  const editable = segnalazioneEditable(item.state);
  const vars = segnalazioneStateVars(item.state) as CSSProperties;

  async function move(target: SegnalazioneState) {
    if (target === item.state || setState.isPending) return;
    setStateError('');
    try {
      await setState.mutateAsync({ id: item.id, state: target });
    } catch (cause) {
      setStateError(errorLabel(cause));
    }
  }

  return (
    <Drawer
      open
      onClose={onClose}
      size="lg"
      title={segnalazioneTitle(item)}
      subtitle={
        <div className={styles.drawerMeta}>
          <span className={styles.statePill} style={vars}>{segnalazioneStateLabel(item.state)}</span>
          <span>Inserita da {item.createdByEmail || 'autore non disponibile'} il {formatInstant(item.createdAt)}</span>
          <span>· Aggiornata il {formatInstant(item.updatedAt)}{item.updatedByEmail ? ` da ${item.updatedByEmail}` : ''}</span>
        </div>
      }
      footer={
        <div className={styles.footer}>
          <div className={styles.stateGroup} role="group" aria-label="Cambia stato">
            <span className={styles.stateGroupLabel}>Stato</span>
            {SEGNALAZIONE_STATES.map((state) => (
              <button
                key={state.key}
                type="button"
                className={styles.stateBtn}
                style={segnalazioneStateVars(state.key) as CSSProperties}
                aria-pressed={state.key === item.state}
                disabled={setState.isPending}
                onClick={() => void move(state.key)}
              >
                {state.label}
              </button>
            ))}
          </div>
          {stateError ? <div className={form.formError} role="alert"><Icon name="triangle-alert" size={16} /><span>{stateError}</span></div> : null}
        </div>
      }
    >
      <div className={styles.drawerBody}>
        {editable ? (
          <EditableBody key={`${item.id}:${item.state}:${item.updatedAt}`} item={item} onSaved={onSaved} />
        ) : (
          <>
            <div className={form.lockNote}><Icon name="lock" size={16} /><span>Contenuti bloccati: la segnalazione è chiusa. Cambia stato per modificarli.</span></div>
            <SegnalazioneReadonly value={toSegnalazioneWrite(item)} />
          </>
        )}
      </div>
    </Drawer>
  );
}
