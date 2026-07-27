import { useState } from 'react';
import { Button, Modal, useToast } from '@mrsmith/ui';
import type { MACompanyContact } from '../../../api/types';
import { useCompanyContactMutations } from '../../../hooks/useCompanyContacts';
import styles from './contacts.module.css';

export function ContactRemoveModal({ companyKey, contact, onClose }: { companyKey: string; contact: MACompanyContact | null; onClose: () => void }) {
  const { remove } = useCompanyContactMutations(companyKey);
  const { toast } = useToast();
  const [error, setError] = useState('');
  const confirm = async () => {
    if (!contact) return;
    setError('');
    try { await remove.mutateAsync(contact.id); toast('Interlocutore rimosso.', 'success'); onClose(); }
    catch { setError('Interlocutore non rimosso. Riprova.'); }
  };
  return (
    <Modal open={contact !== null} onClose={onClose} title="Rimuovi interlocutore" size="sm" dismissible={!remove.isPending}>
      <div className={styles.removeBody}>
        <p>Rimuovere <strong>{contact?.name}</strong> dalla rubrica aziendale?</p>
        {contact?.isPrimary ? <p className={styles.warning}>L’azienda resterà senza interlocutore principale.</p> : null}
        {error ? <p className={styles.formError} role="alert">{error}</p> : null}
        <div className={styles.modalActions}><Button variant="secondary" onClick={onClose}>Annulla</Button><Button variant="danger" loading={remove.isPending} onClick={() => void confirm()}>Rimuovi</Button></div>
      </div>
    </Modal>
  );
}
