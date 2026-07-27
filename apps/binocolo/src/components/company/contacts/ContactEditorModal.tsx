import { useEffect, useState } from 'react';
import { Button, Modal, ToggleSwitch, useToast } from '@mrsmith/ui';
import type { MACompanyContact, MACompanyContactWrite } from '../../../api/types';
import { useCompanyContactMutations } from '../../../hooks/useCompanyContacts';
import styles from './contacts.module.css';

const empty: MACompanyContactWrite = { name: '', relationship: '', contactDetails: '', note: '', isPrimary: false };

export function ContactEditorModal({ companyKey, open, onClose, contact, initial }: { companyKey: string; open: boolean; onClose: () => void; contact?: MACompanyContact; initial?: Partial<MACompanyContactWrite> }) {
  const mutations = useCompanyContactMutations(companyKey);
  const { toast } = useToast();
  const [form, setForm] = useState<MACompanyContactWrite>(empty);
  const [error, setError] = useState('');
  const pending = mutations.create.isPending || mutations.update.isPending;

  useEffect(() => {
    if (!open) return;
    setForm(contact ? { name: contact.name, relationship: contact.relationship, contactDetails: contact.contactDetails, note: contact.note, isPrimary: contact.isPrimary } : {
      ...empty,
      name: initial?.name ?? '',
      relationship: initial?.relationship ?? '',
      contactDetails: initial?.contactDetails ?? '',
      note: initial?.note ?? '',
      isPrimary: initial?.isPrimary ?? false,
    });
    setError('');
  }, [contact, initial?.contactDetails, initial?.isPrimary, initial?.name, initial?.note, initial?.relationship, open]);

  const set = (key: keyof MACompanyContactWrite, value: string | boolean) => { setForm((current) => ({ ...current, [key]: value })); setError(''); };
  const save = async () => {
    if (!form.name.trim()) { setError('Inserisci il nome dell’interlocutore.'); return; }
    setError('');
    try {
      if (contact) await mutations.update.mutateAsync({ id: contact.id, body: form });
      else await mutations.create.mutateAsync(form);
      toast(contact ? 'Interlocutore aggiornato.' : 'Interlocutore aggiunto.', 'success');
      onClose();
    } catch { setError('Interlocutore non salvato. Verifica i dati e riprova.'); }
  };

  return (
    <Modal open={open} onClose={onClose} title={contact ? 'Modifica interlocutore' : 'Aggiungi interlocutore'} size="md" dismissible={!pending}>
      <div className={styles.form}>
        <label><span>Nome <i aria-hidden="true" /></span><input autoFocus required maxLength={200} value={form.name} aria-invalid={Boolean(error && !form.name.trim())} onChange={(e) => set('name', e.target.value)} /></label>
        <label><span>Relazione</span><input maxLength={100} value={form.relationship} placeholder="Es. Socio, CFO, consulente" onChange={(e) => set('relationship', e.target.value)} /></label>
        <label><span>Recapiti</span><textarea rows={5} maxLength={2000} value={form.contactDetails} placeholder={'Cellulare: +39…\nEmail: nome@azienda.it'} onChange={(e) => set('contactDetails', e.target.value)} /></label>
        <label><span>Nota</span><textarea rows={3} maxLength={2000} value={form.note} onChange={(e) => set('note', e.target.value)} /></label>
        <div className={styles.primaryToggle}><div><strong>Interlocutore principale</strong><p>Può essercene uno solo per azienda.</p></div><ToggleSwitch id="contact-primary" checked={form.isPrimary} onChange={(checked) => set('isPrimary', checked)} aria-label="Interlocutore principale" /></div>
        {error ? <p className={styles.formError} role="alert">{error}</p> : null}
        <div className={styles.modalActions}><Button variant="secondary" onClick={onClose} disabled={pending}>Annulla</Button><Button variant="primary" loading={pending} onClick={() => void save()}>Salva</Button></div>
      </div>
    </Modal>
  );
}
