import { Button, Icon, Modal } from '@mrsmith/ui';
import { useState, type FormEvent } from 'react';
import { normalizeManualVat, validateManualDomain, validateManualVat } from '../../lib/companyIdentifiers';
import styles from './DirectCompanyModal.module.css';

type Errors = Partial<Record<'vatCode' | 'domain' | 'form', string>>;

export function DirectCompanyModal({ open, submitting, onClose, onSubmit }: {
  open: boolean;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (value: { vatCode: string; domain?: string }, setError: (message: string) => void) => Promise<void>;
}) {
  const [vatCode, setVatCode] = useState('');
  const [domain, setDomain] = useState('');
  const [errors, setErrors] = useState<Errors>({});

  function close() {
    if (submitting) return;
    setVatCode(''); setDomain(''); setErrors({}); onClose();
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next: Errors = { vatCode: validateManualVat(vatCode) ?? undefined, domain: validateManualDomain(domain) ?? undefined };
    if (next.vatCode || next.domain) { setErrors(next); return; }
    setErrors({});
    await onSubmit(
      { vatCode: normalizeManualVat(vatCode), ...(domain.trim() ? { domain: domain.trim() } : {}) },
      (message) => setErrors({ form: message }),
    );
  }

  return (
    <Modal open={open} onClose={close} title="Aggiungi azienda" size="md" dismissible={!submitting}>
      <form className={styles.form} onSubmit={(event) => void submit(event)} noValidate>
        <p className={styles.copy}>Aggiungi una segnalazione direttamente all’iniziativa. Il dominio è opzionale e verrà verificato senza bloccare la creazione.</p>
        <div className={styles.field}>
          <label htmlFor="direct-card-vat" className={styles.label}><span className={styles.requiredDot} aria-hidden="true" /><span>P.IVA / codice fiscale</span><span className={styles.srOnly}>obbligatorio</span></label>
          <input id="direct-card-vat" className={`${styles.input} ${errors.vatCode ? styles.inputError : ''}`} value={vatCode} onChange={(event) => { setVatCode(event.target.value); if (errors.vatCode) setErrors((current) => ({ ...current, vatCode: validateManualVat(event.target.value) ?? undefined })); }} required autoComplete="off" placeholder="01234567890" aria-invalid={Boolean(errors.vatCode) || undefined} aria-describedby={errors.vatCode ? 'direct-card-vat-error' : 'direct-card-vat-hint'} />
          <p id="direct-card-vat-hint" className={styles.hint}>11 cifre oppure 16 caratteri alfanumerici.</p>
          {errors.vatCode ? <p id="direct-card-vat-error" className={styles.error}>{errors.vatCode}</p> : null}
        </div>
        <div className={styles.field}>
          <label htmlFor="direct-card-domain" className={styles.label}>Dominio</label>
          <input id="direct-card-domain" className={`${styles.input} ${errors.domain ? styles.inputError : ''}`} value={domain} onChange={(event) => { setDomain(event.target.value); if (errors.domain) setErrors((current) => ({ ...current, domain: validateManualDomain(event.target.value) ?? undefined })); }} autoComplete="off" inputMode="url" placeholder="azienda.it" aria-invalid={Boolean(errors.domain) || undefined} aria-describedby={errors.domain ? 'direct-card-domain-error' : 'direct-card-domain-hint'} />
          <p id="direct-card-domain-hint" className={styles.hint}>Indica il sito ufficiale solo se lo conosci.</p>
          {errors.domain ? <p id="direct-card-domain-error" className={styles.error}>{errors.domain}</p> : null}
        </div>
        {errors.form ? <div className={styles.formError} role="alert"><Icon name="triangle-alert" size={16} /><span>{errors.form}</span></div> : null}
        <div className={styles.actions}><Button variant="secondary" onClick={close} disabled={submitting}>Annulla</Button><Button type="submit" loading={submitting} leftIcon={<Icon name="plus" />}>Aggiungi azienda</Button></div>
      </form>
    </Modal>
  );
}
