import { useState, useEffect } from 'react';
import { Modal, SingleSelect, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useEditUserRule, useEditCcRule, useUsers } from './queries';
import { isValidMoneyInput } from '../../utils/format';
import type { UserBudgetApprovalRule, CcBudgetApprovalRule } from '../../api/types';
import styles from './BudgetDetailPage.module.css';

interface RuleEditModalProps {
  open: boolean;
  onClose: () => void;
  type: 'user' | 'cc';
  budgetId: number;
  userId?: number;
  userEmail?: string;
  costCenter?: string;
  rule: UserBudgetApprovalRule | CcBudgetApprovalRule;
}

const LEVEL_OPTIONS = [
  { value: 1, label: 'Livello 1' },
  { value: 2, label: 'Livello 2' },
  { value: 3, label: 'Livello 3' },
];

export function RuleEditModal({ open, onClose, type, budgetId, userId, userEmail, costCenter, rule }: RuleEditModalProps) {
  const [level, setLevel] = useState<number | null>(rule.level);
  const [threshold, setThreshold] = useState(rule.threshold);
  const [approverId, setApproverId] = useState<number | null>(rule.approver_id);
  const [sendEmail, setSendEmail] = useState(rule.send_email);
  const [thresholdError, setThresholdError] = useState('');
  const { toast } = useToast();

  const { data: users } = useUsers();
  const editUserRule = useEditUserRule(budgetId, userId ?? 0);
  const editCcRule = useEditCcRule(budgetId, costCenter ?? '');

  const approverOptions = (users ?? []).map((u) => ({
    value: u.id,
    label: `${u.first_name} ${u.last_name} (${u.email})`,
  }));

  useEffect(() => {
    if (open) {
      setLevel(rule.level);
      setThreshold(rule.threshold);
      setApproverId(rule.approver_id);
      setSendEmail(rule.send_email);
      setThresholdError('');
    }
  }, [open, rule]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (threshold.trim() && !isValidMoneyInput(threshold.trim())) {
      setThresholdError('Formato non valido (es. 500.00)');
      return;
    }
    setThresholdError('');

    const body: Record<string, unknown> = {};
    if (threshold.trim() !== rule.threshold) body.threshold = threshold.trim();
    if (approverId && approverId !== rule.approver_id) body.approver_id = approverId;
    if (level && level !== rule.level) body.level = level;
    if (sendEmail !== rule.send_email) body.send_email = sendEmail;

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
      editUserRule.mutate({ ruleId: rule.id, body }, handlers);
    } else {
      editCcRule.mutate({ ruleId: rule.id, body }, handlers);
    }
  }

  const isPending = editUserRule.isPending || editCcRule.isPending;

  return (
    <Modal open={open} onClose={onClose} title="Modifica regola">
      <form onSubmit={handleSubmit}>
        <div className={styles.ruleContext}>
          <span className={styles.ruleContextLabel}>
            {type === 'cc' ? 'Centro di costo' : 'Utente'}
          </span>
          <span className={styles.ruleContextValue}>
            {type === 'cc' ? costCenter : userEmail}
          </span>
        </div>
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
          <button type="submit" className={styles.btnPrimary} disabled={isPending}>
            {isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
