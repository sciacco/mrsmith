import { useEffect, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, Modal, useToast } from '@mrsmith/ui';
import { useUpdatePlan } from '../../api/queries';
import type { PlanningSummary } from '../../api/types';
import styles from './PlanEditModal.module.css';

interface PlanEditModalProps {
  open: boolean;
  plan: PlanningSummary | null;
  onClose: () => void;
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    const body = error.body as { message?: string; error?: string } | undefined;
    return body?.message ?? 'Operazione non completata';
  }
  return error instanceof Error ? error.message : 'Errore nel salvataggio del piano';
}

export function PlanEditModal({ open, plan, onClose }: PlanEditModalProps) {
  const { toast } = useToast();
  const updatePlan = useUpdatePlan();
  const [budget, setBudget] = useState('');
  const [notes, setNotes] = useState('');
  const [confirmBelowSpent, setConfirmBelowSpent] = useState(false);

  useEffect(() => {
    if (open && plan) {
      setBudget(String(plan.budget_total || ''));
      setNotes(plan.notes ?? '');
      setConfirmBelowSpent(false);
    }
  }, [open, plan]);

  if (!plan) {
    return (
      <Modal open={false} onClose={onClose} title="Modifica piano">
        {null}
      </Modal>
    );
  }

  const budgetValue = budget.trim() ? Number(budget.replace(',', '.')) : 0;
  const budgetValid = Number.isFinite(budgetValue) && budgetValue >= 0;
  const belowSpent = budgetValid && budgetValue < plan.budget_spent;
  const canSubmit = budgetValid && (!belowSpent || confirmBelowSpent) && !updatePlan.isPending;
  const activePlan = plan;

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!canSubmit) return;
    try {
      const response = await updatePlan.mutateAsync({
        planId: activePlan.plan_id,
        body: {
          budget_total: budgetValue,
          notes,
        },
      });
      if (response.warnings?.includes('budget_below_spent')) {
        toast('Piano salvato con budget inferiore allo speso');
      } else {
        toast('Piano aggiornato');
      }
      onClose();
    } catch (err) {
      toast(errorMessage(err), 'error');
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Modifica piano">
      <form className={styles.form} onSubmit={handleSubmit}>
        <div className={styles.field}>
          <label htmlFor="pe-year" className={styles.label}>Anno</label>
          <input id="pe-year" className={styles.input} type="text" value={plan.year} readOnly />
        </div>

        <div className={styles.field}>
          <label htmlFor="pe-budget" className={styles.label}>Budget totale (€)</label>
          <input
            id="pe-budget"
            className={styles.input}
            type="text"
            inputMode="decimal"
            value={budget}
            onChange={(event) => {
              setBudget(event.target.value);
              setConfirmBelowSpent(false);
            }}
            placeholder="es. 150000"
          />
          <p className={styles.hint}>Speso attuale: {formatEuro(plan.budget_spent)}</p>
        </div>

        <div className={styles.field}>
          <label htmlFor="pe-notes" className={styles.label}>Note</label>
          <textarea
            id="pe-notes"
            className={styles.textarea}
            rows={4}
            value={notes}
            onChange={(event) => setNotes(event.target.value)}
            placeholder="Es. aggiornamento budget dopo revisione HR."
          />
        </div>

        {belowSpent && (
          <label className={styles.confirm}>
            <input
              type="checkbox"
              checked={confirmBelowSpent}
              onChange={(event) => setConfirmBelowSpent(event.target.checked)}
            />
            Confermo il budget inferiore allo speso gia allocato.
          </label>
        )}

        <div className={styles.footer}>
          <Button type="button" variant="ghost" size="md" onClick={onClose}>Annulla</Button>
          <Button type="submit" variant="primary" size="md" disabled={!canSubmit} loading={updatePlan.isPending}>
            Salva modifiche
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}
