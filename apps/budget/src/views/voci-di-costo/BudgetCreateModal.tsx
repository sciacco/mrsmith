import { useState } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCreateBudget } from './queries';
import styles from './BudgetListPage.module.css';

const CURRENT_YEAR = new Date().getFullYear();

interface BudgetCreateModalProps {
  open: boolean;
  onClose: () => void;
  onCreated?: (id: number) => void;
}

export function BudgetCreateModal({ open, onClose, onCreated }: BudgetCreateModalProps) {
  const [name, setName] = useState('');
  const [year, setYear] = useState(String(CURRENT_YEAR));
  const { toast } = useToast();
  const createBudget = useCreateBudget();

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const yearNum = Number(year);
    if (!name.trim() || !yearNum) return;
    createBudget.mutate(
      { name: name.trim(), year: yearNum },
      {
        onSuccess: (res) => {
          toast('Budget creato');
          setName('');
          setYear(String(CURRENT_YEAR));
          onClose();
          onCreated?.(res.id);
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
    <Modal open={open} onClose={onClose} title="Nuovo budget">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Nome</label>
          <input
            className={styles.input}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
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
            required
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
            disabled={!name.trim() || !Number(year) || createBudget.isPending}
          >
            {createBudget.isPending ? 'Creazione...' : 'Conferma'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
