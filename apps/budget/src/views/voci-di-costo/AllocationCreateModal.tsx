import { useState } from 'react';
import { Modal, SingleSelect, useToast, MoneyInput } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCreateUserBudget, useCreateCcBudget, useUsers, useCostCenters } from './queries';
import styles from './BudgetDetailPage.module.css';

interface AllocationCreateModalProps {
  open: boolean;
  onClose: () => void;
  type: 'user' | 'cc';
  budgetId: number;
  excludeUserIds?: number[];
  excludeCostCenters?: string[];
}

export function AllocationCreateModal({
  open, onClose, type, budgetId, excludeUserIds = [], excludeCostCenters = [],
}: AllocationCreateModalProps) {
  const [userId, setUserId] = useState<number | null>(null);
  const [costCenter, setCostCenter] = useState('');
  const [limit, setLimit] = useState('');
  const { toast } = useToast();

  const { data: users } = useUsers();
  const { data: costCenters } = useCostCenters();
  const createUser = useCreateUserBudget(budgetId);
  const createCc = useCreateCcBudget(budgetId);

  const excludeSet = new Set(excludeUserIds);
  const userOptions = (users ?? [])
    .filter((u) => !excludeSet.has(u.id))
    .map((u) => ({ value: u.id, label: `${u.first_name} ${u.last_name} (${u.email})` }));

  const excludeCcSet = new Set(excludeCostCenters);
  const ccOptions = (costCenters ?? [])
    .filter((cc) => !excludeCcSet.has(cc.name))
    .map((cc) => ({ value: cc.name, label: cc.name }));

  function reset() {
    setUserId(null);
    setCostCenter('');
    setLimit('');
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    const handlers = {
      onSuccess: (res: { message: string }) => {
        toast(res.message);
        reset();
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
      if (!userId) return;
      createUser.mutate({ user_id: userId, limit: limit.trim() }, handlers);
    } else {
      if (!costCenter.trim()) return;
      createCc.mutate({ cost_center: costCenter.trim(), limit: limit.trim() }, handlers);
    }
  }

  const isPending = createUser.isPending || createCc.isPending;
  const isDisabled = type === 'user' ? !userId || !limit.trim() : !costCenter.trim() || !limit.trim();

  return (
    <Modal open={open} onClose={onClose} title="Nuova allocazione">
      <form onSubmit={handleSubmit}>
        {type === 'user' ? (
          <div className={styles.formGroup}>
            <label className={styles.label}>Utente</label>
            <SingleSelect
              options={userOptions}
              selected={userId}
              onChange={setUserId}
              placeholder="Seleziona utente..."
            />
          </div>
        ) : (
          <div className={styles.formGroup}>
            <label className={styles.label}>Centro di costo</label>
            <input
              className={styles.input}
              type="text"
              value={costCenter}
              onChange={(e) => setCostCenter(e.target.value)}
              placeholder="Nome del centro di costo"
              list="cc-options"
              required
            />
            <datalist id="cc-options">
              {ccOptions.map((o) => (
                <option key={o.value} value={o.value} />
              ))}
            </datalist>
          </div>
        )}
        <div className={styles.formGroup}>
          <MoneyInput
            label="Limite"
            value={limit}
            onChange={setLimit}
            locale="it-IT"
            currency="EUR"
            required
          />
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>
            Annulla
          </button>
          <button type="submit" className={styles.btnPrimary} disabled={isDisabled || isPending}>
            {isPending ? 'Creazione...' : 'Conferma'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
