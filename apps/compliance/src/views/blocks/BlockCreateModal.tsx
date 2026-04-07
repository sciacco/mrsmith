import { useState, useEffect } from 'react';
import { Modal, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCreateBlock, useOrigins } from '../../api/queries';
import { parseDomains } from '../../utils/fqdn';
import { DomainPreview } from '../../components/DomainPreview';
import styles from '../../components/Compliance.module.css';

interface BlockCreateModalProps {
  open: boolean;
  onClose: () => void;
}

export function BlockCreateModal({ open, onClose }: BlockCreateModalProps) {
  const [date, setDate] = useState(new Date().toISOString().split('T')[0]!);
  const [reference, setReference] = useState('');
  const [methodId, setMethodId] = useState('');
  const [domainsText, setDomainsText] = useState('');
  const { toast } = useToast();
  const createBlock = useCreateBlock();
  const { data: origins, isLoading: originsLoading } = useOrigins();

  useEffect(() => {
    if (origins && origins.length > 0 && !methodId) {
      setMethodId(origins[0]!.method_id);
    }
  }, [origins, methodId]);

  useEffect(() => {
    if (!open) {
      setDate(new Date().toISOString().split('T')[0]!);
      setReference('');
      setMethodId('');
      setDomainsText('');
    }
  }, [open]);

  const parsed = parseDomains(domainsText);
  const noOrigins = !originsLoading && (!origins || origins.length === 0);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    createBlock.mutate(
      { request_date: date, reference: reference.trim(), method_id: methodId, domains: parsed.valid },
      {
        onSuccess: () => {
          toast('Richiesta di blocco creata');
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
    <Modal open={open} onClose={onClose} title="Nuova richiesta di blocco">
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
            {originsLoading && <option disabled>Caricamento...</option>}
            {noOrigins && <option disabled>Nessuna provenienza disponibile</option>}
            {origins?.map((o) => (
              <option key={o.method_id} value={o.method_id}>{o.description}</option>
            ))}
          </select>
          {noOrigins && (
            <p className={styles.errorText}>Vai alla sezione Provenienze per crearne una</p>
          )}
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
            disabled={
              createBlock.isPending ||
              !reference.trim() ||
              originsLoading ||
              noOrigins ||
              parsed.valid.length === 0 ||
              parsed.invalid.length > 0
            }
          >
            {createBlock.isPending ? 'Creazione...' : 'Crea richiesta'}
          </button>
        </div>
      </form>
    </Modal>
  );
}
