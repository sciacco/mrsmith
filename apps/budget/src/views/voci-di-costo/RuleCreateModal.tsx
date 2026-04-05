import { useState } from 'react';
import { Modal, SingleSelect, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCreateUserRule, useCreateCcRule, useUsers } from './queries';
import { isValidMoneyInput } from '../../utils/format';
import styles from './BudgetDetailPage.module.css';

interface RuleCreateModalProps {
  open: boolean;
  onClose: () => void;
  type: 'user' | 'cc';
  budgetId: number;
  userId?: number;
  costCenter?: string;
}

const LEVEL_OPTIONS = [
  { value: 1, label: 'Livello 1' },
  { value: 2, label: 'Livello 2' },
  { value: 3, label: 'Livello 3' },
];

export function RuleCreateModal({ open, onClose, type, budgetId, userId, costCenter }: RuleCreateModalProps) {
  const [level, setLevel] = useState<number | null>(1);
  const [threshold, setThreshold] = useState('');
  const [approverId, setApproverId] = useState<number | null>(null);
  const [sendEmail, setSendEmail] = useState(true);
  const [thresholdError, setThresholdError] = useState('');
  const { toast } = useToast();

  const { data: users } = useUsers();
  const createUserRule = useCreateUserRule(budgetId, userId ?? 0);
  const createCcRule = useCreateCcRule(budgetId, costCenter ?? '');

  const approverOptions = (users ?? []).map((u) => ({
    value: u.id,
    label: `${u.first_name} ${u.last_name} (${u.email})`,
  }));

  function reset() {
    setLevel(1);
    setThreshold('');
    setApproverId(null);
    setSendEmail(true);
    setThresholdError('');
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!threshold.trim() || !isValidMoneyInput(threshold.trim())) {
      setThresholdError('Formato non valido (es. 500.00)');
      return;
    }
    if (!level || !approverId) return;
    setThresholdError('');

    const handlers = {
      onSuccess: () => {
        toast('Regola creata');
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

    if (type === 'user' && userId != null) {
      createUserRule.mutate({
        threshold: threshold.trim(),
        approver_id: approverId,
        budget_id: budgetId,
        user_id: userId,
        level,
        send_email: sendEmail,
      }, handlers);
    } else if (type === 'cc' && costCenter) {
      createCcRule.mutate({
        threshold: threshold.trim(),
        approver_id: approverId,
        budget_id: budgetId,
        cost_center: costCenter,
        level,
        send_email: sendEmail,
      }, handlers);
    }
  }

  const isPending = createUserRule.isPending || createCcRule.isPending;

  return (
    <Modal open={open} onClose={onClose} title="Nuova regola di approvazione">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Livello</label>
          <SingleSelect options={LEVEL_OPTIONS} selected={level} onChange={setLevel} placeholder="Seleziona livello..." />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Soglia</label>
          <input
            className={`${styles.input} ${thresholdError ? styles.inputError : ''}`}
            type="text"
            value={threshold}
            onChange={(e) => { setThreshold(e.target.value); setThresholdError(''); }}
            placeholder="es. 500.00"
            required
          />
          {thresholdError ? (
            <p className={styles.errorText}>{thresholdError}</p>
          ) : (
            <p className={styles.helpText}>Inserire il valore in formato decimale (es. 500.00)</p>
          )}
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Approvatore</label>
          <SingleSelect options={approverOptions} selected={approverId} onChange={setApproverId} placeholder="Seleziona approvatore..." />
        </div>
        <div className={styles.formGroup}>
          <div className={styles.toggle}>
            <button
              type="button"
              className={`${styles.toggleSwitch} ${sendEmail ? styles.toggleActive : ''}`}
              onClick={() => setSendEmail(!sendEmail)}
            />
            <span className={styles.toggleLabel}>Invio email</span>
          </div>
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>Annulla</button>
          <button
            type="submit"
            className={styles.btnPrimary}
            disabled={!level || !threshold.trim() || !approverId || isPending}
          >
            {isPending ? 'Creazione...' : 'Conferma'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
