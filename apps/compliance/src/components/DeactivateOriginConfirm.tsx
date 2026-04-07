import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useDeleteOrigin } from '../api/queries';
import type { Origin } from '../api/types';
import styles from './Compliance.module.css';

interface DeactivateOriginConfirmProps {
  open: boolean;
  onClose: () => void;
  origin: Origin | null;
}

export function DeactivateOriginConfirm({ open, onClose, origin }: DeactivateOriginConfirmProps) {
  const { toast } = useToast();
  const deleteOrigin = useDeleteOrigin();

  function handleConfirm() {
    if (!origin) return;
    deleteOrigin.mutate(origin.method_id, {
      onSuccess: () => {
        toast('Provenienza disabilitata');
        onClose();
      },
      onError: (error) => {
        if (error instanceof ApiError) {
          toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
        } else {
          toast('Errore di connessione', 'error');
        }
      },
    });
  }

  return (
    <Modal open={open} onClose={onClose} title="Disabilita provenienza">
      <p className={styles.confirmMessage}>
        Disabilitare la provenienza <strong>{origin?.method_id}</strong>?
        <br />
        Non sara piu selezionabile per nuove richieste di blocco. Le richieste esistenti non saranno modificate.
      </p>
      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary} onClick={onClose}>
          Annulla
        </button>
        <button
          type="button"
          className={styles.btnDanger}
          onClick={handleConfirm}
          disabled={deleteOrigin.isPending}
        >
          {deleteOrigin.isPending ? 'Disabilitazione...' : 'Disabilita'}
        </button>
      </div>
    </Modal>
  );
}
