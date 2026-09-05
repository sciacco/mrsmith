import { useEffect, useId, useState } from "react";
import { Button, Modal, VisuallyHidden, useToast } from "@mrsmith/ui";
import { useUpdatePlanningReminder } from "../../api/queries";
import type { PlanningReminder } from "../../api/types";
import styles from "./PlanningPage.module.css";

interface Props {
  reminder: PlanningReminder | null;
  onClose: () => void;
  onCompleted: (message: string) => void;
}

export function PlanningReminderModal({
  reminder,
  onClose,
  onCompleted,
}: Props) {
  const { toast } = useToast();
  const mutation = useUpdatePlanningReminder();
  const errorId = useId();
  const [text, setText] = useState("");
  const [date, setDate] = useState("");
  const [removing, setRemoving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!reminder) return;
    setText(reminder.text);
    setDate(reminder.date ?? "");
    setRemoving(false);
    setError(null);
  }, [reminder]);

  if (!reminder) return null;
  const currentReminder = reminder;

  async function submit(remove: boolean) {
    setError(null);
    try {
      await mutation.mutateAsync({
        kind: currentReminder.ownerKind,
        id: currentReminder.ownerId,
        input: {
          text: remove ? "" : text.trim(),
          date: remove ? null : date || null,
        },
      });
      const message = remove ? "Promemoria rimosso." : "Promemoria salvato.";
      toast(message);
      onCompleted(message);
      onClose();
    } catch {
      setError(
        "Impossibile salvare il promemoria. Verificare i campi e riprovare.",
      );
    }
  }

  return (
    <Modal
      open
      title="Modifica promemoria"
      onClose={onClose}
      dismissible={!mutation.isPending}
    >
      <form
        className={styles.reminderForm}
        onSubmit={(event) => {
          event.preventDefault();
          void submit(false);
        }}
      >
        <p className={styles.modalOwner}>Promemoria di {reminder.ownerLabel}</p>
        {error && (
          <p id={errorId} className={styles.fieldError} role="alert">
            {error}
          </p>
        )}
        {removing ? (
          <>
            <p>
              Rimuovere questo promemoria? Corso, richiesta ed evento non
              verranno modificati.
            </p>
            <div className={styles.modalActions}>
              <Button
                variant="secondary"
                type="button"
                disabled={mutation.isPending}
                onClick={() => setRemoving(false)}
              >
                Annulla
              </Button>
              <Button
                variant="danger"
                type="button"
                loading={mutation.isPending}
                aria-describedby={error ? errorId : undefined}
                onClick={() => void submit(true)}
              >
                Rimuovi promemoria
              </Button>
            </div>
          </>
        ) : (
          <>
            <label htmlFor="planning-reminder-text">
              Testo <span className={styles.requiredDot} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </label>
            <textarea
              id="planning-reminder-text"
              required
              value={text}
              maxLength={2000}
              onChange={(event) => setText(event.target.value)}
              aria-invalid={Boolean(error)}
              aria-describedby={error ? errorId : undefined}
            />
            <label htmlFor="planning-reminder-date">Data facoltativa</label>
            <input
              id="planning-reminder-date"
              type="date"
              value={date}
              onChange={(event) => setDate(event.target.value)}
              aria-describedby={error ? errorId : undefined}
            />
            <div className={styles.modalActions}>
              <Button
                variant="ghost"
                type="button"
                disabled={mutation.isPending}
                onClick={() => setRemoving(true)}
              >
                Rimuovi promemoria
              </Button>
              <Button
                variant="primary"
                type="submit"
                loading={mutation.isPending}
                disabled={!text.trim()}
              >
                Salva
              </Button>
            </div>
          </>
        )}
      </form>
    </Modal>
  );
}
