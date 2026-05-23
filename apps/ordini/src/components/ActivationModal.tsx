import { useState } from 'react';
import { Button, Modal } from '@mrsmith/ui';
import type { OrderRow } from '../api/types';
import styles from '../pages/OrderDetailPage.module.css';

interface ActivationModalProps {
  row: OrderRow | null;
  open: boolean;
  loading?: boolean;
  onClose: () => void;
  onConfirm: (date: string) => void;
}

export function ActivationModal({ row, open, loading, onClose, onConfirm }: ActivationModalProps) {
  const [activationDate, setActivationDate] = useState('');

  function close() {
    setActivationDate('');
    onClose();
  }

  return (
    <Modal open={open} onClose={close} title="Conferma attivazione">
      <div className={styles.modalBody}>
        <p className={styles.modalText}>Imposta la data di attivazione per la riga selezionata.</p>
        {row ? (
          <div className={styles.modalSummary}>
            <span>{row.cdlan_codart ?? 'Riga ordine'}</span>
            <strong>{row.cdlan_descart ?? '—'}</strong>
          </div>
        ) : null}
        <label className={styles.fieldLabel}>
          <span>Data attivazione <span className={styles.requiredDot} aria-hidden="true" /></span>
          <span className={styles.visuallyHidden}>obbligatorio</span>
          <input
            className={styles.input}
            type="date"
            required
            value={activationDate}
            onChange={(event) => setActivationDate(event.target.value)}
          />
        </label>
        <div className={styles.modalActions}>
          <Button variant="secondary" onClick={close}>Annulla</Button>
          <Button loading={loading} disabled={!activationDate} onClick={() => onConfirm(activationDate)}>
            Conferma
          </Button>
        </div>
      </div>
    </Modal>
  );
}
