import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useEditBudget } from './queries';
import type { BudgetDetails, BudgetEdit } from '../../api/types';
import styles from './BudgetListPage.module.css';

interface BudgetEditModalProps {
  open: boolean;
  onClose: () => void;
  budget: BudgetDetails;
}

export function BudgetEditModal({ open, onClose, budget }: BudgetEditModalProps) {
  const [name, setName] = useState(budget.name);
  const [year, setYear] = useState(String(budget.year));
  const { toast } = useToast();
  const editBudget = useEditBudget();

  useEffect(() => {
    if (open) {
      setName(budget.name);
      setYear(String(budget.year));
    }
  }, [open, budget.name, budget.year]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const body: BudgetEdit = {};
    if (name.trim() !== budget.name) body.name = name.trim();
    const yearNum = Number(year);
    if (yearNum && yearNum !== budget.year) body.year = yearNum;
    if (!body.name && !body.year) {
      onClose();
      return;
    }
    editBudget.mutate(
      { id: budget.id, body },
      {
        onSuccess: (res) => {
          toast(res.message);
          onClose();
        },
        onError: (error) => {
          if (error instanceof ApiError) {
            toast((error.body as { message?: string })?.message ?? error.statusText, 'error');
          } else {
            toast('Errore di connessione', 'error');
          }
        },
      },
    );
  }

  return (
    <Modal open={open} onClose={onClose} title="Modifica budget">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Nome</label>
          <input
            className={styles.input}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Nome del budget"
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Anno</label>
          <input
            className={styles.input}
            type="number"
            value={year}
            onChange={(e) => setYear(e.target.value)}
            min={2020}
            max={2099}
          />
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>
            Annulla
          </button>
          <button
            type="submit"
            className={styles.btnPrimary}
            disabled={editBudget.isPending}
          >
            {editBudget.isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
