import { Modal, useToast } from '@mrsmith/ui';
import { useDeleteGroup } from './queries';
import { ApiError } from '@mrsmith/api-client';
import styles from './GruppiPage.module.css';

interface GroupDeleteConfirmProps {
  open: boolean;
  onClose: () => void;
  groupName: string;
  onDeleted: () => void;
}

export function GroupDeleteConfirm({ open, onClose, groupName, onDeleted }: GroupDeleteConfirmProps) {
  const { toast } = useToast();
  const deleteGroup = useDeleteGroup();

  function handleConfirm() {
    deleteGroup.mutate(groupName, {
      onSuccess: (res) => {
        toast(res.message);
        onDeleted();
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
    <Modal open={open} onClose={onClose} title="Elimina gruppo">
      <p className={styles.deleteMessage}>
        Eliminare il gruppo <strong>{groupName}</strong>?
      </p>
      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary} onClick={onClose}>
          Annulla
        </button>
        <button
          type="button"
          className={styles.btnDanger}
          onClick={handleConfirm}
          disabled={deleteGroup.isPending}
        >
          {deleteGroup.isPending ? 'Eliminazione...' : 'Elimina'}
        </button>
      </div>
    </Modal>
  );
}
