import { useEffect, useState } from 'react';
import { Button, Modal, useToast, VisuallyHidden } from '@mrsmith/ui';
import type { MACompanyAgreement, MACompanyAgreementWrite } from '../../../api/types';
import { useCompanyAgreementMutations } from '../../../hooks/useCompanyAgreements';
import styles from './agreements.module.css';

const empty: MACompanyAgreementWrite = { kind: 'nda', signedOn: '', expiresOn: '' };

export function AgreementEditorModal({
  companyKey,
  open,
  onClose,
  agreement,
}: {
  companyKey: string;
  open: boolean;
  onClose: () => void;
  agreement?: MACompanyAgreement;
}) {
  const mutations = useCompanyAgreementMutations(companyKey);
  const { toast } = useToast();
  const [form, setForm] = useState<MACompanyAgreementWrite>(empty);
  const [error, setError] = useState('');
  const [errorField, setErrorField] = useState<'signedOn' | 'expiresOn' | null>(null);
  const pending = mutations.create.isPending || mutations.update.isPending;

  useEffect(() => {
    if (!open) return;
    setForm(agreement ? { kind: agreement.kind, signedOn: agreement.signedOn, expiresOn: agreement.expiresOn ?? '' } : empty);
    setError('');
    setErrorField(null);
  }, [agreement, open]);

  const set = (key: keyof MACompanyAgreementWrite, value: string) => {
    setForm((current) => ({ ...current, [key]: value }));
    setError('');
    setErrorField(null);
  };

  const save = async () => {
    if (!form.signedOn) {
      setError('Inserisci la data di sottoscrizione.');
      setErrorField('signedOn');
      return;
    }
    if (form.expiresOn && form.expiresOn < form.signedOn) {
      setError('La scadenza non può precedere la sottoscrizione.');
      setErrorField('expiresOn');
      return;
    }
    setError('');
    setErrorField(null);
    try {
      if (agreement) await mutations.update.mutateAsync({ id: agreement.id, body: form });
      else await mutations.create.mutateAsync(form);
      toast(agreement ? 'Accordo aggiornato.' : 'Accordo registrato.', 'success');
      onClose();
    } catch {
      setError('Accordo non salvato. Verifica le date e riprova.');
      setErrorField(null);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title={agreement ? 'Modifica accordo' : 'Registra accordo'} size="md" dismissible={!pending}>
      <div className={styles.form}>
        <label htmlFor="agreement-kind">
          <span>Tipo</span>
          <select id="agreement-kind" value={form.kind} onChange={(e) => set('kind', e.target.value as 'nda')}>
            <option value="nda">NDA</option>
          </select>
        </label>
        <label htmlFor="agreement-signed-on">
          <span>Data sottoscrizione <i aria-hidden="true" /><VisuallyHidden>obbligatorio</VisuallyHidden></span>
          <input
            id="agreement-signed-on"
            type="date"
            required
            value={form.signedOn}
            aria-invalid={errorField === 'signedOn'}
            aria-describedby={errorField === 'signedOn' ? 'agreement-form-error' : undefined}
            onChange={(e) => set('signedOn', e.target.value)}
          />
        </label>
        <div>
          <label htmlFor="agreement-expires-on">
            <span>Data scadenza</span>
            <input
              id="agreement-expires-on"
              type="date"
              value={form.expiresOn}
              aria-invalid={errorField === 'expiresOn'}
              aria-describedby={errorField === 'expiresOn' ? 'agreement-expires-on-helper agreement-form-error' : 'agreement-expires-on-helper'}
              onChange={(e) => set('expiresOn', e.target.value)}
            />
          </label>
          <p id="agreement-expires-on-helper" className={styles.helper}>Lascia vuoto se non è prevista una scadenza.</p>
        </div>
        {error ? <p id="agreement-form-error" className={styles.formError} role="alert">{error}</p> : null}
        <div className={styles.modalActions}>
          <Button variant="secondary" onClick={onClose} disabled={pending}>Annulla</Button>
          <Button variant="primary" loading={pending} onClick={() => void save()}>Salva</Button>
        </div>
      </div>
    </Modal>
  );
}
