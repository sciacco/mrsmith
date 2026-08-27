// Dialog di conferma con motivazione obbligatoria (annullamento evento,
// annullamento iscrizione, riapertura). Danger come azione primaria,
// ghost/secondary come via di uscita sicura (docs/UI-UX.md §14.2).

import { useState } from 'react';
import { Button, Modal, VisuallyHidden } from '@mrsmith/ui';
import { ErrorPanel } from './ErrorPanel';
import styles from './ReasonDialog.module.css';

interface ReasonDialogProps {
  open: boolean;
  title: string;
  description?: string;
  confirmLabel: string;
  danger?: boolean;
  pending?: boolean;
  error?: string | null;
  onConfirm: (reason: string) => void;
  onClose: () => void;
}

export function ReasonDialog({
  open,
  title,
  description,
  confirmLabel,
  danger = true,
  pending = false,
  error,
  onConfirm,
  onClose,
}: ReasonDialogProps) {
  const [reason, setReason] = useState('');

  if (!open) return null;

  function handleClose() {
    setReason('');
    onClose();
  }

  return (
    <Modal open={open} onClose={handleClose} title={title} size="sm">
      <div className={styles.body}>
        {description && <p className={styles.description}>{description}</p>}
        <label className={styles.field}>
          <span className={styles.labelHead}>
            Motivazione
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <textarea
            className={styles.textarea}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            required
            aria-invalid={error ? 'true' : undefined}
          />
        </label>
        <ErrorPanel message={error ?? null} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={handleClose} disabled={pending}>
            Annulla
          </Button>
          <Button
            variant={danger ? 'danger' : 'primary'}
            size="md"
            loading={pending}
            disabled={reason.trim() === ''}
            onClick={() => onConfirm(reason.trim())}
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
