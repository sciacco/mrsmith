import { useState } from 'react';
import { Button, Modal, useToast } from '@mrsmith/ui';
import type { MAInitiativeCardView } from '../../../api/types';
import { ESITO_SUGGESTIONS, REGISTRY_BRIDGE_FACTS, stateLabel } from '../../../lib/cardStates';
import { errorLabel } from '../../ricerche/helpers';
import type { CloseCardInput } from './useBoardData';
import styles from './board.module.css';

export type TerminalTarget = 'won' | 'ko_nostro' | 'ko_target';

/** Modale della transizione terminale (KANBAN-V2-PLAN.md §T). WON = conferma secca
 *  senza esito; KO = esito obbligatorio (vocabolario suggerito + testo libero);
 *  ponte registro SOLO per ko_target. Non applica lo stato finché non si conferma. */
export function TerminalModal({
  card,
  targetState,
  onClose,
  onSubmit,
}: {
  card: MAInitiativeCardView;
  targetState: TerminalTarget;
  onClose: () => void;
  onSubmit: (input: CloseCardInput) => Promise<void>;
}) {
  const { toast } = useToast();
  const [esito, setEsito] = useState('');
  const [note, setNote] = useState('');
  const [facts, setFacts] = useState<Set<string>>(new Set());
  const [saving, setSaving] = useState(false);

  const isWon = targetState === 'won';
  const suggestions = isWon ? [] : ESITO_SUGGESTIONS[targetState];
  const showBridge = targetState === 'ko_target';

  const toggleFact = (kind: string) =>
    setFacts((prev) => {
      const next = new Set(prev);
      if (next.has(kind)) next.delete(kind);
      else next.add(kind);
      return next;
    });

  const submit = async () => {
    if (!isWon && !esito.trim()) return;
    setSaving(true);
    try {
      await onSubmit({
        companyKey: card.companyKey,
        state: targetState,
        esito: isWon ? undefined : esito.trim(),
        note: note.trim() || undefined,
        registerFacts: showBridge ? [...facts] : undefined,
      });
      onClose();
    } catch (e) {
      toast(errorLabel(e), 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`${stateLabel(targetState)} — ${card.companyName}`}>
      {isWon ? (
        <p className={styles.modalSub}>Operazione conclusa. Conferma secca: WON non porta esito.</p>
      ) : (
        <>
          <p className={styles.modalSub}>
            {targetState === 'ko_target' ? 'La controparte non procede.' : 'Chiusura dalla nostra parte.'} L&apos;esito calibra lo
            scoring{showBridge ? '; il ponte registro segnala l’azienda alle altre iniziative' : ''}.
          </p>
          <div className={styles.field}>
            <label>
              Esito <span style={{ color: 'var(--color-danger)' }} aria-hidden="true">•</span>
            </label>
            <div className={styles.tcombo}>
              {suggestions.map((s) => (
                <button
                  key={s.key}
                  type="button"
                  className={[styles.opt, esito === s.key ? styles.optOn : ''].filter(Boolean).join(' ')}
                  onClick={() => setEsito(s.key)}
                  title={s.description}
                >
                  {s.label}
                </button>
              ))}
            </div>
            <input
              className={styles.input}
              placeholder="…oppure scrivi un esito libero"
              value={esito}
              onChange={(e) => setEsito(e.target.value)}
              maxLength={120}
            />
          </div>
          {showBridge ? (
            <div className={styles.bridge}>
              <p className={styles.lab}>
                Ponte registro azienda <span className={styles.hint}>(vale per tutte le iniziative)</span>
              </p>
              {REGISTRY_BRIDGE_FACTS.map((k) => (
                <label key={k.key} className={styles.checkrow}>
                  <input type="checkbox" checked={facts.has(k.key)} onChange={() => toggleFact(k.key)} />
                  <span>
                    <b>{k.label}</b>
                    {k.hint ? ` — ${k.hint}` : ''}
                  </span>
                </label>
              ))}
            </div>
          ) : null}
          <div className={styles.field}>
            <label>Nota</label>
            <input
              className={styles.input}
              placeholder="Contesto della chiusura (opzionale)"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              maxLength={500}
            />
          </div>
        </>
      )}
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>
          Annulla
        </Button>
        <Button
          variant={isWon ? 'primary' : 'danger'}
          onClick={() => void submit()}
          loading={saving}
          disabled={!isWon && !esito.trim()}
        >
          {isWon ? 'Conferma WON' : `Registra ${stateLabel(targetState)}`}
        </Button>
      </div>
    </Modal>
  );
}

/** Rimozione = uscita per errore di triage (§4.3): nessun verdetto. Opzione di
 *  correzione stella sulla provenienza + motivo. */
export function RemoveModal({
  card,
  onClose,
  onSubmit,
}: {
  card: MAInitiativeCardView;
  onClose: () => void;
  onSubmit: (input: { companyKey: string; correctRating: boolean; reason?: string }) => Promise<void>;
}) {
  const { toast } = useToast();
  const [correctRating, setCorrectRating] = useState(false);
  const [reason, setReason] = useState('');
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    setSaving(true);
    try {
      await onSubmit({ companyKey: card.companyKey, correctRating, reason: reason.trim() || undefined });
      onClose();
    } catch (e) {
      toast(errorLabel(e), 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Rimuovi dalla lavorazione — ${card.companyName}`}>
      <p className={styles.modalSub}>
        La rimozione è per le aziende aggiunte per errore: <b>nessun verdetto</b> viene registrato.
      </p>
      <div className={styles.bridge}>
        <label className={styles.checkrow}>
          <input type="checkbox" checked={correctRating} onChange={(e) => setCorrectRating(e.target.checked)} />
          <span>
            Correggi anche la valutazione nella ricerca di provenienza: <b>escludi (−1)</b>
          </span>
        </label>
      </div>
      <div className={styles.field}>
        <label>Motivo dell&apos;esclusione</label>
        <input
          className={styles.input}
          placeholder="Es. non è un MSP, rivende solo licenze…"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          maxLength={500}
        />
      </div>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>
          Annulla
        </Button>
        <Button variant="danger" onClick={() => void submit()} loading={saving}>
          Rimuovi
        </Button>
      </div>
    </Modal>
  );
}
