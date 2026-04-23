import { Button, Modal } from '@mrsmith/ui';
import type { ReactNode } from 'react';
import styles from './ConfirmDialog.module.css';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  variant?: 'danger' | 'primary';
  details?: ReactNode;
  busy?: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  variant = 'danger',
  details,
  busy = false,
  onConfirm,
  onClose,
}: ConfirmDialogProps) {
  return (
    <Modal open={open} onClose={onClose} title={title} size="sm">
      <div className={styles.body}>
        <p>{message}</p>
        {details ? <div className={styles.details}>{details}</div> : null}
        <div className={styles.actions}>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Annulla
          </Button>
          <Button variant={variant} onClick={onConfirm} loading={busy}>
            {confirmLabel}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
