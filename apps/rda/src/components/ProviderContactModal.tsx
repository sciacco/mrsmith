import { Button, Icon, Modal, PhoneInput } from '@mrsmith/ui';
import { useEffect, useId, useMemo, useState, type FormEvent } from 'react';
import type { ProviderReference } from '../api/types';
import styles from './ProviderContactModal.module.css';
import {
  PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE,
  availableReferenceTypes,
  isValidOptionalProviderRefPhone,
  referenceTypeLabel,
} from '../lib/provider-refs';

type ContactMode = 'create' | 'edit';

interface ContactDraft {
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
  reference_type: string;
}

interface ContactErrors {
  email?: string;
  phone?: string;
}

function draftFromContact(contact: ProviderReference | null | undefined): ContactDraft {
  return {
    first_name: contact?.first_name ?? '',
    last_name: contact?.last_name ?? '',
    email: contact?.email ?? '',
    phone: contact?.phone ?? '',
    reference_type: contact?.reference_type ?? availableReferenceTypes()[0]?.value ?? 'ADMINISTRATIVE_REF',
  };
}

function toProviderReference(draft: ContactDraft): ProviderReference {
  return {
    first_name: draft.first_name.trim(),
    last_name: draft.last_name.trim(),
    email: draft.email.trim(),
    phone: draft.phone.trim(),
    reference_type: draft.reference_type,
  };
}

function validateContactDraft(draft: ContactDraft): ContactErrors {
  const errors: ContactErrors = {};
  const email = draft.email.trim();
  if (!email) {
    errors.email = 'Inserisci l\'email del contatto';
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    errors.email = 'Inserisci un\'email valida';
  }
  if (!isValidOptionalProviderRefPhone(draft.phone)) {
    errors.phone = PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE;
  }
  return errors;
}

export function ProviderContactForm({
  mode = 'create',
  contact,
  saving,
  submitError,
  onCancel,
  onSubmit,
}: {
  mode?: ContactMode;
  contact?: ProviderReference | null;
  saving?: boolean;
  submitError?: string;
  onCancel: () => void;
  onSubmit: (body: ProviderReference) => void;
}) {
  const [draft, setDraft] = useState<ContactDraft>(() => draftFromContact(contact));
  const [errors, setErrors] = useState<ContactErrors>({});
  const fieldId = useId();
  const emailId = `${fieldId}-email`;
  const firstNameId = `${fieldId}-first-name`;
  const lastNameId = `${fieldId}-last-name`;
  const phoneId = `${fieldId}-phone`;
  const typeId = `${fieldId}-type`;
  const emailErrorId = `${fieldId}-email-error`;
  const submitErrorId = `${fieldId}-submit-error`;
  const typeOptions = useMemo(() => availableReferenceTypes(), []);
  const editing = mode === 'edit';

  useEffect(() => {
    setDraft(draftFromContact(contact));
    setErrors({});
  }, [contact]);

  function update<K extends keyof ContactDraft>(key: K, value: ContactDraft[K]) {
    setDraft((current) => ({ ...current, [key]: value }));
    if (key === 'email' || key === 'phone') {
      setErrors((current) => ({ ...current, [key]: undefined }));
    }
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextErrors = validateContactDraft(draft);
    setErrors(nextErrors);
    if (Object.values(nextErrors).some(Boolean)) return;
    onSubmit(toProviderReference(draft));
  }

  return (
    <form className={styles.form} onSubmit={submit} noValidate>
      <div className={`${styles.field} ${styles.wide}`}>
        <label htmlFor={emailId}>
          Email
          <span className={styles.requiredMarker} aria-hidden="true" />
          <span className={styles.srOnly}> obbligatorio</span>
        </label>
        <input
          id={emailId}
          value={draft.email}
          type="email"
          required
          disabled={saving}
          aria-invalid={errors.email ? 'true' : undefined}
          aria-describedby={errors.email ? emailErrorId : undefined}
          onChange={(event) => update('email', event.target.value)}
        />
        {errors.email ? <p id={emailErrorId} className={styles.fieldError}>{errors.email}</p> : null}
      </div>
      <div className={styles.field}>
        <label htmlFor={firstNameId}>Nome</label>
        <input id={firstNameId} value={draft.first_name} disabled={saving} onChange={(event) => update('first_name', event.target.value)} />
      </div>
      <div className={styles.field}>
        <label htmlFor={lastNameId}>Cognome</label>
        <input id={lastNameId} value={draft.last_name} disabled={saving} onChange={(event) => update('last_name', event.target.value)} />
      </div>
      <PhoneInput
        id={phoneId}
        value={draft.phone}
        onChange={(val) => update('phone', val)}
        label="Telefono"
        error={errors.phone}
        disabled={saving}
      />
      <div className={styles.field}>
        <label htmlFor={typeId}>Tipo</label>
        {editing ? (
          <output id={typeId} className={styles.readonly}>{referenceTypeLabel(draft.reference_type)}</output>
        ) : (
          <select id={typeId} value={draft.reference_type} disabled={saving} onChange={(event) => update('reference_type', event.target.value)}>
            {typeOptions.map((item) => (
              <option key={item.value} value={item.value}>{item.label}</option>
            ))}
          </select>
        )}
      </div>
      {submitError ? <p id={submitErrorId} className={`${styles.fieldError} ${styles.wide}`} role="alert">{submitError}</p> : null}
      <div className={`${styles.actions} ${styles.wide}`}>
        <Button variant="secondary" disabled={saving} onClick={onCancel}>
          Annulla
        </Button>
        <Button type="submit" leftIcon={<Icon name={editing ? 'check' : 'plus'} />} loading={saving} aria-describedby={submitError ? submitErrorId : undefined}>
          {editing ? 'Salva modifiche' : 'Aggiungi contatto'}
        </Button>
      </div>
    </form>
  );
}

export function ProviderContactModal({
  open,
  mode,
  contact,
  saving,
  onClose,
  onSubmit,
}: {
  open: boolean;
  mode: ContactMode;
  contact?: ProviderReference | null;
  saving?: boolean;
  onClose: () => void;
  onSubmit: (body: ProviderReference) => void;
}) {
  return (
    <Modal open={open} onClose={onClose} title={mode === 'edit' ? 'Modifica contatto' : 'Nuovo contatto'} size="lg">
      <ProviderContactForm
        key={`${mode}-${open ? 'open' : 'closed'}-${contact?.id ?? 'new'}`}
        mode={mode}
        contact={contact}
        saving={saving}
        onCancel={onClose}
        onSubmit={onSubmit}
      />
    </Modal>
  );
}
