import { useState, useEffect, useRef } from 'react';
import { Modal } from '@mrsmith/ui';
import { parseDomains } from '../utils/fqdn';
import { DomainPreview } from './DomainPreview';
import styles from './Compliance.module.css';

interface AddDomainsModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (domains: string[]) => void;
  isPending: boolean;
  title: string;
}

export function AddDomainsModal({ open, onClose, onSubmit, isPending, title }: AddDomainsModalProps) {
  const [text, setText] = useState('');
  const [parsed, setParsed] = useState<{ valid: string[]; invalid: string[] }>({ valid: [], invalid: [] });
  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    if (!open) {
      setText('');
      setParsed({ valid: [], invalid: [] });
    }
  }, [open]);

  function handleChange(value: string) {
    setText(value);
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      setParsed(parseDomains(value));
    }, 150);
  }

  return (
    <Modal open={open} onClose={onClose} title={title}>
      <div className={styles.formGroup}>
        <label className={styles.label}>Domini</label>
        <textarea
          className={styles.textarea}
          value={text}
          onChange={(e) => handleChange(e.target.value)}
          placeholder="Inserisci un dominio per riga"
        />
        {text.trim() && <DomainPreview valid={parsed.valid} invalid={parsed.invalid} />}
      </div>
      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary} onClick={onClose}>
          Annulla
        </button>
        <button
          type="button"
          className={styles.btnPrimary}
          onClick={() => onSubmit(parsed.valid)}
          disabled={isPending || !text.trim() || parsed.invalid.length > 0 || parsed.valid.length === 0}
        >
          {isPending ? 'Aggiunta...' : 'Aggiungi'}
        </button>
      </div>
    </Modal>
  );
}
