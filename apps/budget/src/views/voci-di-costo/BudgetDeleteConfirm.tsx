import { useNavigate } from 'react-router-dom';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useDeleteBudget } from './queries';
import styles from './BudgetListPage.module.css';

interface BudgetDeleteConfirmProps {
  open: boolean;
  onClose: () => void;
  budgetId: number;
  budgetName: string;
}

export function BudgetDeleteConfirm({ open, onClose, budgetId, budgetName }: BudgetDeleteConfirmProps) {
  const { toast } = useToast();
  const navigate = useNavigate();
  const deleteBudget = useDeleteBudget();

  function handleDelete() {
    deleteBudget.mutate(budgetId, {
      onSuccess: (res) => {
        toast(res.message);
        onClose();
        navigate('/budgets');
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
    <Modal open={open} onClose={onClose} title="Elimina budget">
      <p className={styles.confirmMessage}>
        Sei sicuro di voler eliminare il budget <strong>{budgetName}</strong>?
        Questa azione non può essere annullata.
      </p>
      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary} onClick={onClose}>
          Annulla
        </button>
        <button
          className={styles.btnDanger}
          onClick={handleDelete}
          disabled={deleteBudget.isPending}
        >
          {deleteBudget.isPending ? 'Eliminazione...' : 'Elimina'}
        </button>
      </div>
    </Modal>
  );
}
