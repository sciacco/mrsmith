import { Modal, Button } from '@mrsmith/ui';
import styles from './ConfirmDialog.module.css';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  variant?: 'danger' | 'primary';
  onConfirm: () => void;
  onCancel: () => void;
  /** Optional middle action, e.g. "Scarta modifiche" in a 3-way dirty prompt. */
  discardLabel?: string;
  onDiscard?: () => void;
  confirmLoading?: boolean;
}

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  variant = 'danger',
  onConfirm,
  onCancel,
  discardLabel,
  onDiscard,
  confirmLoading = false,
}: ConfirmDialogProps) {
  return (
    <Modal open={open} onClose={onCancel} title={title} size="sm">
      <p className={styles.message}>{message}</p>
      <div className={styles.actions}>
        <Button variant="ghost" onClick={onCancel}>Annulla</Button>
        {discardLabel && onDiscard && (
          <Button variant="secondary" onClick={onDiscard}>{discardLabel}</Button>
        )}
        <Button variant={variant} onClick={onConfirm} loading={confirmLoading}>
          {confirmLabel}
        </Button>
      </div>
    </Modal>
  );
}
