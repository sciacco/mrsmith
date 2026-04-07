import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useRelease, useUpdateRelease } from '../../api/queries';
import styles from '../../components/Compliance.module.css';

interface ReleaseEditModalProps {
  open: boolean;
  onClose: () => void;
  releaseId: number;
}

export function ReleaseEditModal({ open, onClose, releaseId }: ReleaseEditModalProps) {
  const [date, setDate] = useState('');
  const [reference, setReference] = useState('');
  const { toast } = useToast();
  const { data: release } = useRelease(open ? releaseId : null);
  const updateRelease = useUpdateRelease();

  useEffect(() => {
    if (release) {
      setDate(release.request_date);
      setReference(release.reference);
    }
  }, [release]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    updateRelease.mutate(
      { id: releaseId, request_date: date, reference: reference.trim() },
      {
        onSuccess: () => {
          toast('Richiesta aggiornata');
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
    <Modal open={open} onClose={onClose} title="Modifica richiesta di rilascio">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Data</label>
          <input className={styles.input} type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Riferimento</label>
          <input className={styles.input} type="text" value={reference} onChange={(e) => setReference(e.target.value)} placeholder="Numero protocollo" required />
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>Annulla</button>
          <button type="submit" className={styles.btnPrimary} disabled={updateRelease.isPending || !reference.trim()}>
            {updateRelease.isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
