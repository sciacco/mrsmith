import { Button, Icon, Modal } from '@mrsmith/ui';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  loading?: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Conferma',
  danger,
  loading,
  onConfirm,
  onClose,
}: ConfirmDialogProps) {
  return (
    <Modal open={open} onClose={onClose} title={title} size="sm">
      <p className="modalText">{message}</p>
      <div className="modalActions">
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button
          variant={danger ? 'danger' : 'primary'}
          leftIcon={<Icon name={danger ? 'trash' : 'check'} />}
          loading={loading}
          onClick={onConfirm}
        >
          {confirmLabel}
        </Button>
      </div>
    </Modal>
  );
}
