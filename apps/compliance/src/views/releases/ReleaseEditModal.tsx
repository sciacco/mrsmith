import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useRelease, useUpdateRelease, useOrigins } from '../../api/queries';
import styles from '../../components/Compliance.module.css';

interface ReleaseEditModalProps {
  open: boolean;
  onClose: () => void;
  releaseId: number;
}

export function ReleaseEditModal({ open, onClose, releaseId }: ReleaseEditModalProps) {
  const [date, setDate] = useState('');
  const [reference, setReference] = useState('');
  const [methodId, setMethodId] = useState('');
  const { toast } = useToast();
  const { data: release } = useRelease(open ? releaseId : null);
  const { data: origins, isLoading: originsLoading } = useOrigins();
  const updateRelease = useUpdateRelease();

  useEffect(() => {
    if (release) {
      setDate(release.request_date.split('T')[0]!);
      setReference(release.reference);
      setMethodId(release.method_id ?? '');
    }
  }, [release]);

  const hasSelectedOrigin = !!methodId && !!origins?.some((o) => o.method_id === methodId);
  const showCurrentOrigin = !!methodId && !hasSelectedOrigin;
  const noOrigins = !originsLoading && (!origins || origins.length === 0) && !showCurrentOrigin;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    updateRelease.mutate(
      { id: releaseId, request_date: date, reference: reference.trim(), method_id: methodId },
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
        <div className={styles.formGroup}>
          <label className={styles.label}>Provenienza</label>
          <select className={styles.input} value={methodId} onChange={(e) => setMethodId(e.target.value)} disabled={originsLoading || noOrigins}>
            {!methodId && <option value="" disabled>Non indicata</option>}
            {originsLoading && <option disabled>Caricamento...</option>}
            {noOrigins && <option disabled>Nessuna provenienza disponibile</option>}
            {showCurrentOrigin && (
              <option value={methodId}>{release?.method_description ?? methodId}</option>
            )}
            {origins?.map((o) => (
              <option key={o.method_id} value={o.method_id}>{o.description}</option>
            ))}
          </select>
          {noOrigins && (
            <p className={styles.errorText}>Vai alla sezione Provenienze per crearne una</p>
          )}
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>Annulla</button>
          <button
            type="submit"
            className={styles.btnPrimary}
            disabled={updateRelease.isPending || !reference.trim() || !methodId || originsLoading || noOrigins}
          >
            {updateRelease.isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
