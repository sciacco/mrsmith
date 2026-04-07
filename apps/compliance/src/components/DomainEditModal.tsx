import { useState, useEffect } from 'react';
import { Modal } from '@mrsmith/ui';
import { isValidFQDN } from '../utils/fqdn';
import styles from './Compliance.module.css';

interface DomainEditModalProps {
  open: boolean;
  onClose: () => void;
  domain: { id: number; domain: string } | null;
  onSave: (id: number, newDomain: string) => void;
  isPending: boolean;
}

export function DomainEditModal({ open, onClose, domain, onSave, isPending }: DomainEditModalProps) {
  const [value, setValue] = useState('');

  useEffect(() => {
    if (open && domain) {
      setValue(domain.domain);
    }
  }, [open, domain]);

  const isValid = isValidFQDN(value.trim());
  const isChanged = domain ? value.trim() !== domain.domain : false;

  return (
    <Modal open={open} onClose={onClose} title="Modifica dominio">
      <div className={styles.formGroup}>
        <label className={styles.label}>Dominio</label>
        <input
          className={`${styles.input} ${value.trim() && !isValid ? styles.inputError : ''}`}
          type="text"
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
        {value.trim() && (
          isValid
            ? <p className={styles.helpText}>Dominio valido</p>
            : <p className={styles.errorText}>Dominio non valido</p>
        )}
      </div>
      <div className={styles.actions}>
        <button type="button" className={styles.btnSecondary} onClick={onClose}>
          Annulla
        </button>
        <button
          type="button"
          className={styles.btnPrimary}
          onClick={() => domain && onSave(domain.id, value.trim())}
          disabled={isPending || !value.trim() || !isValid || !isChanged}
        >
          {isPending ? 'Salvataggio...' : 'Salva'}
        </button>
      </div>
    </Modal>
  );
}
