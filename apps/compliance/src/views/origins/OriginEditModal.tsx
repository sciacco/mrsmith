import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useUpdateOrigin } from '../../api/queries';
import type { Origin } from '../../api/types';
import styles from '../../components/Compliance.module.css';

interface OriginEditModalProps {
  open: boolean;
  onClose: () => void;
  origin: Origin | null;
}

export function OriginEditModal({ open, onClose, origin }: OriginEditModalProps) {
  const [description, setDescription] = useState('');
  const { toast } = useToast();
  const updateOrigin = useUpdateOrigin();

  useEffect(() => {
    if (origin) setDescription(origin.description);
  }, [origin]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!origin) return;
    updateOrigin.mutate(
      { methodId: origin.method_id, description: description.trim() },
      {
        onSuccess: () => {
          toast('Provenienza aggiornata');
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
    <Modal open={open} onClose={onClose} title="Modifica provenienza">
      <div className={styles.ruleContext}>
        <span className={styles.ruleContextLabel}>Codice</span>
        <span className={styles.ruleContextValue}>{origin?.method_id}</span>
      </div>
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Descrizione</label>
          <input className={styles.input} type="text" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Descrizione della provenienza" required />
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>Annulla</button>
          <button type="submit" className={styles.btnPrimary} disabled={updateOrigin.isPending || !description.trim()}>
            {updateOrigin.isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
