import { Button, Icon, Modal } from '@mrsmith/ui';
import { useEffect, useMemo, useState, type FormEvent } from 'react';
import type { ProviderReference } from '../api/types';
import {
  PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE,
  PROVIDER_REFERENCE_PHONE_PATTERN,
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
  const [draft, setDraft] = useState<ContactDraft>(() => draftFromContact(contact));
  const [errors, setErrors] = useState<ContactErrors>({});
  const typeOptions = useMemo(() => availableReferenceTypes(), []);
  const editing = mode === 'edit';

  useEffect(() => {
    if (!open) return;
    setDraft(draftFromContact(contact));
    setErrors({});
  }, [contact, open]);

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
    <Modal open={open} onClose={onClose} title={editing ? 'Modifica contatto' : 'Nuovo contatto'} size="lg">
      <form className="contactModalForm" onSubmit={submit} noValidate>
        <div className="field wide">
          <label>Email</label>
          <input
            value={draft.email}
            type="email"
            required
            aria-invalid={errors.email ? 'true' : undefined}
            onChange={(event) => update('email', event.target.value)}
          />
          {errors.email ? <p className="fieldError">{errors.email}</p> : null}
        </div>
        <div className="field">
          <label>Nome</label>
          <input value={draft.first_name} onChange={(event) => update('first_name', event.target.value)} />
        </div>
        <div className="field">
          <label>Cognome</label>
          <input value={draft.last_name} onChange={(event) => update('last_name', event.target.value)} />
        </div>
        <div className="field">
          <label>Telefono</label>
          <input
            value={draft.phone}
            type="tel"
            inputMode="tel"
            pattern={PROVIDER_REFERENCE_PHONE_PATTERN}
            title={PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE}
            placeholder="+391234567890"
            aria-invalid={errors.phone ? 'true' : undefined}
            onChange={(event) => update('phone', event.target.value)}
          />
          {errors.phone ? <p className="fieldError">{errors.phone}</p> : null}
        </div>
        <div className="field">
          <label>Tipo</label>
          {editing ? (
            <div className="contactModalReadonly">{referenceTypeLabel(draft.reference_type)}</div>
          ) : (
            <select value={draft.reference_type} onChange={(event) => update('reference_type', event.target.value)}>
              {typeOptions.map((item) => (
                <option key={item.value} value={item.value}>{item.label}</option>
              ))}
            </select>
          )}
        </div>
        <div className="modalActions fullWidth">
          <Button variant="secondary" onClick={onClose}>
            Annulla
          </Button>
          <Button type="submit" leftIcon={<Icon name={editing ? 'check' : 'plus'} />} loading={saving}>
            {editing ? 'Salva modifiche' : 'Aggiungi contatto'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
