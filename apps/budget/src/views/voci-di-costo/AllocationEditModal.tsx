import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useEditUserBudget, useEditCcBudget } from './queries';
import { isValidMoneyInput } from '../../utils/format';
import styles from './BudgetDetailPage.module.css';

interface AllocationEditModalProps {
  open: boolean;
  onClose: () => void;
  type: 'user' | 'cc';
  budgetId: number;
  identifier: number | string; // user_id or cost_center
  currentLimit: string;
  currentEnabled: boolean;
}

export function AllocationEditModal({
  open, onClose, type, budgetId, identifier, currentLimit, currentEnabled,
}: AllocationEditModalProps) {
  const [limit, setLimit] = useState(currentLimit);
  const [enabled, setEnabled] = useState(currentEnabled);
  const [limitError, setLimitError] = useState('');
  const { toast } = useToast();

  const editUser = useEditUserBudget(budgetId);
  const editCc = useEditCcBudget(budgetId);

  useEffect(() => {
    if (open) {
      setLimit(currentLimit);
      setEnabled(currentEnabled);
      setLimitError('');
    }
  }, [open, currentLimit, currentEnabled]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (limit.trim() && !isValidMoneyInput(limit.trim())) {
      setLimitError('Formato non valido (es. 1500.00)');
      return;
    }
    setLimitError('');

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
      editUser.mutate({
        user_id: identifier as number,
        ...(limit.trim() !== currentLimit && { limit: limit.trim() }),
        ...(enabled !== currentEnabled && { enabled }),
      }, handlers);
    } else {
      editCc.mutate({
        cost_center: identifier as string,
        ...(limit.trim() !== currentLimit && { limit: limit.trim() }),
        ...(enabled !== currentEnabled && { enabled }),
      }, handlers);
    }
  }

  const isPending = editUser.isPending || editCc.isPending;

  return (
    <Modal open={open} onClose={onClose} title="Modifica allocazione">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Limite</label>
          <input
            className={`${styles.input} ${limitError ? styles.inputError : ''}`}
            type="text"
            value={limit}
            onChange={(e) => { setLimit(e.target.value); setLimitError(''); }}
            placeholder="es. 50000.00"
          />
          {limitError ? (
            <p className={styles.errorText}>{limitError}</p>
          ) : (
            <p className={styles.helpText}>Inserire il valore in formato decimale (es. 1500.00)</p>
          )}
        </div>
        <div className={styles.formGroup}>
          <div className={styles.toggle}>
            <button
              type="button"
              className={`${styles.toggleSwitch} ${enabled ? styles.toggleActive : ''}`}
              onClick={() => setEnabled(!enabled)}
            />
            <span className={styles.toggleLabel}>{enabled ? 'Attivo' : 'Disabilitato'}</span>
          </div>
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>
            Annulla
          </button>
          <button type="submit" className={styles.btnPrimary} disabled={isPending}>
            {isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
