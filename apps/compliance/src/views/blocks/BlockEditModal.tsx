import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useBlock, useUpdateBlock, useOrigins } from '../../api/queries';
import styles from '../../components/Compliance.module.css';

interface BlockEditModalProps {
  open: boolean;
  onClose: () => void;
  blockId: number;
}

export function BlockEditModal({ open, onClose, blockId }: BlockEditModalProps) {
  const [date, setDate] = useState('');
  const [reference, setReference] = useState('');
  const [methodId, setMethodId] = useState('');
  const { toast } = useToast();
  const { data: block } = useBlock(open ? blockId : null);
  const { data: origins } = useOrigins();
  const updateBlock = useUpdateBlock();

  useEffect(() => {
    if (block) {
      setDate(block.request_date);
      setReference(block.reference);
      setMethodId(block.method_id);
    }
  }, [block]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    updateBlock.mutate(
      { id: blockId, request_date: date, reference: reference.trim(), method_id: methodId },
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
    <Modal open={open} onClose={onClose} title="Modifica richiesta di blocco">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label}>Data</label>
          <input className={styles.input} type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Riferimento</label>
          <input className={styles.input} type="text" value={reference} onChange={(e) => setReference(e.target.value)} placeholder="Numero protocollo" required />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label}>Provenienza</label>
          <select className={styles.input} value={methodId} onChange={(e) => setMethodId(e.target.value)}>
            {origins?.map((o) => (
              <option key={o.method_id} value={o.method_id}>{o.description}</option>
            ))}
          </select>
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>Annulla</button>
          <button type="submit" className={styles.btnPrimary} disabled={updateBlock.isPending || !reference.trim()}>
            {updateBlock.isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
