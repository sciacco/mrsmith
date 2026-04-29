import { Button, Icon, Modal, useToast } from '@mrsmith/ui';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useProviderMutations } from '../api/queries';
import type { ProviderPayload, ProviderSummary } from '../api/types';
import styles from './ProviderRequestModal.module.css';

interface ProviderRequestModalProps {
  open: boolean;
  initialCompanyName: string;
  onClose: () => void;
  onCreated: (provider: ProviderSummary) => void;
}

interface FieldErrors {
  company_name?: string;
  email?: string;
}

export function ProviderRequestModal({ open, initialCompanyName, onClose, onCreated }: ProviderRequestModalProps) {
  const { createProvider } = useProviderMutations();
  const { toast } = useToast();
  const formRef = useRef<HTMLFormElement>(null);
  const [errors, setErrors] = useState<FieldErrors>({});

  useEffect(() => {
    if (!open) return;
    const form = formRef.current;
    form?.reset();
    setErrors({});

    const companyInput = form?.elements.namedItem('company_name');
    if (companyInput instanceof HTMLInputElement) {
      companyInput.value = initialCompanyName.trim();
      companyInput.focus();
    }
  }, [initialCompanyName, open]);

  function closeModal() {
    if (createProvider.isPending) return;
    onClose();
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const formData = new FormData(form);
    const companyName = String(formData.get('company_name') ?? '').trim();
    const email = String(formData.get('email') ?? '').trim();
    const nextErrors: FieldErrors = {};

    if (!companyName) nextErrors.company_name = 'Inserisci la ragione sociale';
    if (!email) nextErrors.email = 'Inserisci il contatto qualifica';

    if (nextErrors.company_name || nextErrors.email) {
      setErrors(nextErrors);
      toast(nextErrors.company_name ?? nextErrors.email ?? 'Completa i campi obbligatori', 'warning');
      const fieldName = nextErrors.company_name ? 'company_name' : 'email';
      const field = form.elements.namedItem(fieldName);
      if (field instanceof HTMLElement) field.focus();
      return;
    }

    const payload: ProviderPayload = {
      company_name: companyName,
      state: 'DRAFT',
      country: 'IT',
      language: 'it',
      vat_number: String(formData.get('vat_number') ?? '').trim() || undefined,
      cf: String(formData.get('cf') ?? '').trim() || undefined,
      postal_code: String(formData.get('postal_code') ?? '').trim() || undefined,
      province: String(formData.get('province') ?? '').trim() || undefined,
      city: String(formData.get('city') ?? '').trim() || undefined,
      address: String(formData.get('address') ?? '').trim() || undefined,
      ref: {
        first_name: String(formData.get('first_name') ?? '').trim(),
        last_name: String(formData.get('last_name') ?? '').trim(),
        email,
        phone: String(formData.get('phone') ?? '').trim(),
        reference_type: 'QUALIFICATION_REF',
      },
    };

    try {
      const provider = await createProvider.mutateAsync(payload);
      toast('Richiesta di censimento creata');
      form.reset();
      setErrors({});
      onCreated(provider);
      onClose();
    } catch {
      toast('Dati fornitore non disponibili in questo momento', 'error');
    }
  }

  return (
    <Modal open={open} onClose={closeModal} title="Nuovo fornitore" size="wide" dismissible={!createProvider.isPending}>
      <form ref={formRef} className={`formGrid three ${styles.form}`} noValidate onSubmit={(event) => void submit(event)}>
        <div className="field">
          <label>Ragione sociale</label>
          <input name="company_name" aria-invalid={Boolean(errors.company_name)} onChange={() => setErrors((current) => ({ ...current, company_name: undefined }))} />
          {errors.company_name ? <p className="fieldError">{errors.company_name}</p> : null}
        </div>
        <div className="field"><label>P.IVA</label><input name="vat_number" /></div>
        <div className="field"><label>CF</label><input name="cf" /></div>
        <div className="field"><label>Indirizzo</label><input name="address" /></div>
        <div className="field"><label>Citta</label><input name="city" /></div>
        <div className="field"><label>CAP</label><input name="postal_code" /></div>
        <div className="field"><label>Provincia</label><input name="province" maxLength={2} /></div>
        <div className="field"><label>Nome qualifica</label><input name="first_name" /></div>
        <div className="field"><label>Cognome qualifica</label><input name="last_name" /></div>
        <div className="field">
          <label>Email qualifica</label>
          <input name="email" type="email" aria-invalid={Boolean(errors.email)} onChange={() => setErrors((current) => ({ ...current, email: undefined }))} />
          {errors.email ? <p className="fieldError">{errors.email}</p> : null}
        </div>
        <div className="field"><label>Telefono qualifica</label><input name="phone" /></div>
        <div className={`modalActions fullWidth ${styles.actions}`}>
          <Button type="button" variant="secondary" disabled={createProvider.isPending} onClick={closeModal}>
            Annulla
          </Button>
          <Button type="submit" leftIcon={<Icon name="check" />} loading={createProvider.isPending}>
            Invia richiesta di censimento
          </Button>
        </div>
      </form>
    </Modal>
  );
}
