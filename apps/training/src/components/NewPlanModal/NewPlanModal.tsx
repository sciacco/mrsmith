import { useEffect, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, Modal, useToast } from '@mrsmith/ui';
import { useCreatePlan } from '../../api/queries';
import styles from './NewPlanModal.module.css';

interface NewPlanModalProps {
  open: boolean;
  defaultYear: number;
  prevYearAvailable: boolean;
  onClose: () => void;
  onCreated?: (year: number) => void;
}

type Source = 'empty' | 'duplicate';

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    const body = error.body as { message?: string; error?: string } | undefined;
    return body?.message ?? 'Creazione non completata';
  }
  return error instanceof Error ? error.message : 'Errore nella creazione del piano';
}

export function NewPlanModal({ open, defaultYear, prevYearAvailable, onClose, onCreated }: NewPlanModalProps) {
  const { toast } = useToast();
  const createPlan = useCreatePlan();
  const [year, setYear] = useState(String(defaultYear));
  const [budget, setBudget] = useState('');
  const [source, setSource] = useState<Source>(prevYearAvailable ? 'duplicate' : 'empty');

  useEffect(() => {
    if (open) {
      setYear(String(defaultYear));
      setBudget('');
      setSource(prevYearAvailable ? 'duplicate' : 'empty');
    }
  }, [open, defaultYear, prevYearAvailable]);

  const parsedYear = Number.parseInt(year, 10);
  const yearValid = Number.isInteger(parsedYear) && parsedYear >= 2020 && parsedYear <= 2100;
  const budgetValue = budget.trim() ? Number(budget.replace(',', '.')) : null;
  const budgetValid = budgetValue === null || (Number.isFinite(budgetValue) && budgetValue >= 0);
  const canSubmit = yearValid && budgetValid && !createPlan.isPending;

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!canSubmit) return;
    try {
      await createPlan.mutateAsync({
        year: parsedYear,
        budget_total: budgetValue ?? undefined,
        duplicate_from: source === 'duplicate' ? parsedYear - 1 : undefined,
      });
      toast(`Piano ${parsedYear} creato in bozza`);
      onCreated?.(parsedYear);
      onClose();
    } catch (err) {
      toast(errorMessage(err), 'error');
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Nuovo piano">
      <form className={styles.form} onSubmit={handleSubmit}>
        <div className={styles.field}>
          <label htmlFor="np-year" className={styles.label}>
            Anno
            <span className={styles.req} aria-hidden="true" />
            <span className="sr-only"> (obbligatorio)</span>
          </label>
          <input
            id="np-year"
            type="number"
            min={2020}
            max={2100}
            value={year}
            onChange={(e) => setYear(e.target.value)}
            className={styles.input}
            required
          />
        </div>

        <div className={styles.field}>
          <label htmlFor="np-budget" className={styles.label}>Budget totale (€)</label>
          <input
            id="np-budget"
            type="text"
            inputMode="decimal"
            placeholder="es. 150000"
            value={budget}
            onChange={(e) => setBudget(e.target.value)}
            className={styles.input}
          />
          <p className={styles.hint}>Lascia vuoto per definire il budget in seguito.</p>
        </div>

        <fieldset className={styles.fieldset}>
          <legend className={styles.legend}>Origine</legend>
          <label className={styles.radio}>
            <input
              type="radio"
              name="source"
              value="empty"
              checked={source === 'empty'}
              onChange={() => setSource('empty')}
            />
            Crea piano vuoto
          </label>
          <label className={`${styles.radio} ${!prevYearAvailable ? styles.radioDisabled : ''}`}>
            <input
              type="radio"
              name="source"
              value="duplicate"
              checked={source === 'duplicate'}
              onChange={() => setSource('duplicate')}
              disabled={!prevYearAvailable}
            />
            Duplica budget da piano {parsedYear - 1}
            {!prevYearAvailable && <span className={styles.radioHint}> · non disponibile</span>}
          </label>
        </fieldset>

        <div className={styles.footer}>
          <Button type="button" variant="ghost" size="md" onClick={onClose}>Annulla</Button>
          <Button type="submit" variant="primary" size="md" disabled={!canSubmit} loading={createPlan.isPending}>
            Crea piano
          </Button>
        </div>
      </form>
    </Modal>
  );
}
