import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCreateOrigin } from '../../api/queries';
import styles from '../../components/Compliance.module.css';

interface OriginCreateModalProps {
  open: boolean;
  onClose: () => void;
}

export function OriginCreateModal({ open, onClose }: OriginCreateModalProps) {
  const [methodId, setMethodId] = useState('');
  const [description, setDescription] = useState('');
  const { toast } = useToast();
  const createOrigin = useCreateOrigin();

  useEffect(() => {
    if (!open) {
      setMethodId('');
      setDescription('');
    }
  }, [open]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    createOrigin.mutate(
      { method_id: methodId.trim(), description: description.trim() },
      {
        onSuccess: () => {
          toast('Provenienza creata');
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
    <Modal open={open} onClose={onClose} title="Nuova provenienza">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Codice</label>
          <input className={styles.input} type="text" value={methodId} onChange={(e) => setMethodId(e.target.value)} placeholder="Es. AGCOM" required />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Descrizione</label>
          <input className={styles.input} type="text" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Descrizione della provenienza" required />
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>Annulla</button>
          <button type="submit" className={styles.btnPrimary} disabled={createOrigin.isPending || !methodId.trim() || !description.trim()}>
            {createOrigin.isPending ? 'Creazione...' : 'Crea'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
