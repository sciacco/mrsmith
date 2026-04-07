import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCreateRelease } from '../../api/queries';
import { parseDomains } from '../../utils/fqdn';
import { DomainPreview } from '../../components/DomainPreview';
import styles from '../../components/Compliance.module.css';

interface ReleaseCreateModalProps {
  open: boolean;
  onClose: () => void;
}

export function ReleaseCreateModal({ open, onClose }: ReleaseCreateModalProps) {
  const [date, setDate] = useState(new Date().toISOString().split('T')[0]!);
  const [reference, setReference] = useState('');
  const [domainsText, setDomainsText] = useState('');
  const { toast } = useToast();
  const createRelease = useCreateRelease();

  useEffect(() => {
    if (!open) {
      setDate(new Date().toISOString().split('T')[0]!);
      setReference('');
      setDomainsText('');
    }
  }, [open]);

  const parsed = parseDomains(domainsText);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    createRelease.mutate(
      { request_date: date, reference: reference.trim(), domains: parsed.valid },
      {
        onSuccess: () => {
          toast('Richiesta di rilascio creata');
          onClose();
        },
        onError: (error) => {
          if (error instanceof ApiError) {
            const body = error.body as { error?: string; invalid?: string[]; message?: string } | undefined;
            if (body?.error === 'invalid_domains' && body.invalid) {
              toast(`Alcuni domini non sono validi: ${body.invalid.join(', ')}`, 'error');
            } else {
              toast(body?.message ?? error.statusText, 'error');
            }
          } else {
            toast('Errore di connessione', 'error');
          }
        },
      },
    );
  }

  return (
    <Modal open={open} onClose={onClose} title="Nuova richiesta di rilascio">
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
          <label className={styles.label}>Domini</label>
          <textarea
            className={styles.textarea}
            value={domainsText}
            onChange={(e) => setDomainsText(e.target.value)}
            placeholder="Inserisci un dominio per riga"
          />
          {domainsText.trim() && <DomainPreview valid={parsed.valid} invalid={parsed.invalid} />}
        </div>
        <div className={styles.actions}>
          <button type="button" className={styles.btnSecondary} onClick={onClose}>Annulla</button>
          <button
            type="submit"
            className={styles.btnPrimary}
            disabled={createRelease.isPending || !reference.trim() || parsed.valid.length === 0 || parsed.invalid.length > 0}
          >
            {createRelease.isPending ? 'Creazione...' : 'Crea richiesta'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
