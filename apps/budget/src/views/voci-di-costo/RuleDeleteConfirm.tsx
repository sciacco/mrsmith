import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useDeleteUserRule, useDeleteCcRule } from './queries';
import styles from './BudgetDetailPage.module.css';

interface RuleDeleteConfirmProps {
  open: boolean;
  onClose: () => void;
  type: 'user' | 'cc';
  budgetId: number;
  userId?: number;
  costCenter?: string;
  ruleId: number;
}

export function RuleDeleteConfirm({ open, onClose, type, budgetId, userId, costCenter, ruleId }: RuleDeleteConfirmProps) {
  const { toast } = useToast();
  const deleteUserRule = useDeleteUserRule(budgetId, userId ?? 0);
  const deleteCcRule = useDeleteCcRule(budgetId, costCenter ?? '');

  function handleDelete() {
    const handlers = {
      onSuccess: (res: { message: string }) => {
        toast(res.message);
        onClose();
      },
      onError: (error: Error) => {
        if (error instanceof ApiError) {
          toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
        } else {
          toast('Errore di connessione', 'error');
        }
      },
    };

    if (type === 'user') {
      deleteUserRule.mutate(ruleId, handlers);
    } else {
      deleteCcRule.mutate(ruleId, handlers);
    }
  }

  const isPending = deleteUserRule.isPending || deleteCcRule.isPending;

  return (
    <Modal open={open} onClose={onClose} title="Elimina regola">
      <p className={styles.confirmMessage}>
        Sei sicuro di voler eliminare questa regola di approvazione?
      </p>
      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary} onClick={onClose}>Annulla</button>
        <button className={styles.btnDanger} onClick={handleDelete} disabled={isPending}>
          {isPending ? 'Eliminazione...' : 'Elimina'}
        </button>
      </div>
    </Modal>
  );
}
