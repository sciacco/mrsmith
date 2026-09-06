import { useEffect, useState } from 'react';
import { formatLocalDate } from '@mrsmith/format';
import { Button, Modal, useToast } from '@mrsmith/ui';
import type { MACompanyAgreement } from '../../../api/types';
import { useCompanyAgreementMutations } from '../../../hooks/useCompanyAgreements';
import styles from './agreements.module.css';

export function AgreementRemoveModal({
  companyKey,
  agreement,
  onClose,
}: {
  companyKey: string;
  agreement: MACompanyAgreement | null;
  onClose: () => void;
}) {
  const { remove } = useCompanyAgreementMutations(companyKey);
  const { toast } = useToast();
  const [error, setError] = useState('');
  useEffect(() => setError(''), [agreement]);
  const confirm = async () => {
    if (!agreement) return;
    setError('');
    try {
      await remove.mutateAsync(agreement.id);
      toast('Accordo rimosso.', 'success');
      onClose();
    } catch {
      setError('Accordo non rimosso. Riprova.');
    }
  };
  return (
    <Modal open={agreement !== null} onClose={onClose} title="Rimuovi accordo" size="sm" dismissible={!remove.isPending}>
      <div className={styles.removeBody}>
        <p>Rimuovere l’NDA sottoscritto il {agreement ? formatLocalDate(agreement.signedOn) ?? '' : ''}?</p>
        <p>La rimozione resta tracciata nello storico eventi.</p>
        {error ? <p className={styles.formError} role="alert">{error}</p> : null}
        <div className={styles.modalActions}>
          <Button variant="secondary" onClick={onClose}>Annulla</Button>
          <Button variant="danger" loading={remove.isPending} onClick={() => void confirm()}>Rimuovi</Button>
        </div>
      </div>
    </Modal>
  );
}
