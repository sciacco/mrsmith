import { Button, Icon, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useProviderMutations } from '../api/queries';
import type { PoDetail, ProviderReference, ProviderSummary } from '../api/types';
import {
  PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE,
  PROVIDER_REFERENCE_PHONE_PATTERN,
  availableReferenceTypes,
  QUALIFICATION_REF,
  isValidOptionalProviderRefPhone,
  referenceTypeLabel,
} from '../lib/provider-refs';

function providerRefs(provider?: ProviderSummary): ProviderReference[] {
  if (!provider) return [];
  const refs = provider.refs?.length ? provider.refs : provider.ref ? [provider.ref] : [];
  return refs;
}

function recipientIDs(po: PoDetail): number[] {
  return (po.recipients ?? []).map((ref) => ref.id).filter((id): id is number => id != null);
}

function refFromForm(form: HTMLFormElement, referenceType: string): ProviderReference {
  const data = new FormData(form);
  return {
    first_name: String(data.get('first_name') ?? '').trim(),
    last_name: String(data.get('last_name') ?? '').trim(),
    email: String(data.get('email') ?? '').trim(),
    phone: String(data.get('phone') ?? '').trim(),
    reference_type: referenceType,
  };
}

function phoneInputFromForm(form: HTMLFormElement) {
  return form.elements.namedItem('phone') as HTMLInputElement | null;
}

function clearPhoneInvalid(event: React.FormEvent<HTMLInputElement>) {
  if (isValidOptionalProviderRefPhone(event.currentTarget.value)) {
    event.currentTarget.removeAttribute('aria-invalid');
  }
}

function refKey(ref: ProviderReference): string {
  return ref.id ? `ref-${ref.id}` : `email-${ref.email ?? 'unknown'}`;
}

function contactName(ref: ProviderReference): string {
  const name = [ref.first_name, ref.last_name].filter(Boolean).join(' ').trim();
  return name || ref.email || 'Contatto senza nome';
}

export function ProviderRefTable({
  po,
  provider,
  editable,
  onSelectionChange,
  onSaveRecipients,
  showSaveAction = true,
}: {
  po: PoDetail;
  provider?: ProviderSummary;
  editable: boolean;
  onSelectionChange?: (ids: number[]) => void;
  onSaveRecipients?: (ids: number[]) => void;
  showSaveAction?: boolean;
}) {
  const [selected, setSelected] = useState<number[]>(() => recipientIDs(po));
  const [newType, setNewType] = useState<string>(availableReferenceTypes()[0]?.value ?? 'ADMINISTRATIVE_REF');
  const [editing, setEditing] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const mutations = useProviderMutations();
  const { toast } = useToast();
  const refs = useMemo(() => providerRefs(provider), [provider]);

  useEffect(() => {
    setSelected(recipientIDs(po));
  }, [po]);

  function toggle(id: number, checked: boolean) {
    setSelected((current) => {
      const next = checked ? [...new Set([...current, id])] : current.filter((item) => item !== id);
      onSelectionChange?.(next);
      return next;
    });
  }

  function rejectInvalidPhone(form: HTMLFormElement) {
    const phoneInput = phoneInputFromForm(form);
    if (!phoneInput || isValidOptionalProviderRefPhone(phoneInput.value)) {
      phoneInput?.removeAttribute('aria-invalid');
      return false;
    }
    phoneInput.setAttribute('aria-invalid', 'true');
    phoneInput.focus();
    toast(PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE, 'warning');
    return true;
  }

  async function update(ref: ProviderReference, form: HTMLFormElement) {
    if (!provider || !ref.id || ref.reference_type === QUALIFICATION_REF) return;
    if (rejectInvalidPhone(form)) return;
    try {
      await mutations.updateReference.mutateAsync({ providerId: provider.id, refId: ref.id, body: refFromForm(form, ref.reference_type ?? newType) });
      setEditing(null);
      toast('Contatto aggiornato');
    } catch {
      toast('Salvataggio contatto non riuscito', 'error');
    }
  }

  async function add(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!provider) return;
    if (rejectInvalidPhone(event.currentTarget)) return;
    try {
      await mutations.createReference.mutateAsync({ providerId: provider.id, body: refFromForm(event.currentTarget, newType) });
      event.currentTarget.reset();
      setAdding(false);
      toast('Contatto aggiunto');
    } catch {
      toast('Salvataggio contatto non riuscito', 'error');
    }
  }

  return (
    <div className="contactPicker">
      <div className="contactPickerHeader">
        <div>
          <h3>Destinatari ordine</h3>
          <p className="muted">Seleziona i contatti che riceveranno il PO, incluso Qualifica se serve. Senza selezione viene usato Qualifica automaticamente.</p>
        </div>
        {editable ? (
          <Button size="sm" variant="secondary" leftIcon={<Icon name={adding ? 'x' : 'plus'} />} onClick={() => setAdding((current) => !current)}>
            {adding ? 'Chiudi' : 'Aggiungi contatto'}
          </Button>
        ) : null}
      </div>

      {adding && editable ? (
        <form className="contactEditorForm contactEditorPanel" onSubmit={(event) => void add(event)} noValidate>
          <div className="field"><label>Email</label><input name="email" type="email" placeholder="nome@azienda.it" /></div>
          <div className="field"><label>Nome</label><input name="first_name" placeholder="Nome" /></div>
          <div className="field"><label>Cognome</label><input name="last_name" placeholder="Cognome" /></div>
          <div className="field">
            <label>Telefono</label>
            <input
              name="phone"
              type="tel"
              inputMode="tel"
              pattern={PROVIDER_REFERENCE_PHONE_PATTERN}
              title={PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE}
              placeholder="+39..."
              onInput={clearPhoneInvalid}
            />
          </div>
          <div className="field">
            <label>Tipo</label>
            <select value={newType} onChange={(event) => setNewType(event.target.value)}>
              {availableReferenceTypes().map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
            </select>
          </div>
          <div className="contactEditorActions">
            <Button size="sm" type="submit" leftIcon={<Icon name="plus" />} loading={mutations.createReference.isPending}>Aggiungi</Button>
          </div>
        </form>
      ) : null}

      <div className="contactCardList">
        {refs.map((ref) => {
          const key = refKey(ref);
          const isQualification = ref.reference_type === QUALIFICATION_REF;
          const isSelected = Boolean(ref.id && selected.includes(ref.id));
          const canSelect = editable && Boolean(ref.id);
          const canEdit = editable && !isQualification && Boolean(ref.id);

          return (
            <article key={key} className={`providerContactCard ${isSelected ? 'selected' : ''} ${isQualification ? 'fallback' : ''}`}>
              <label className="providerContactSelect">
                <input
                  type="checkbox"
                  checked={isSelected}
                  disabled={!canSelect}
                  aria-label={`Seleziona ${contactName(ref)} come destinatario`}
                  onChange={(event) => ref.id && toggle(ref.id, event.target.checked)}
                />
              </label>
              <div className="providerContactMain">
                <div className="providerContactTitleRow">
                  <strong>{contactName(ref)}</strong>
                  <span className={`badge ${isQualification ? 'info' : ''}`}>
                    {referenceTypeLabel(ref.reference_type)}
                  </span>
                </div>
                <div className="providerContactMeta">
                  <span><Icon name="mail" size={15} />{ref.email || '-'}</span>
                  <span><Icon name="phone" size={15} />{ref.phone || '-'}</span>
                </div>
              </div>
              {canEdit ? (
                <button
                  className="iconButton"
                  type="button"
                  aria-label={editing === key ? 'Chiudi modifica contatto' : 'Modifica contatto'}
                  title={editing === key ? 'Chiudi' : 'Modifica'}
                  onClick={() => setEditing((current) => (current === key ? null : key))}
                >
                  <Icon name={editing === key ? 'x' : 'pencil'} size={16} />
                </button>
              ) : null}
              {editing === key ? (
                <div className="providerContactEditor">
                  <form
                    className="contactEditorForm"
                    noValidate
                    onSubmit={(event) => {
                      event.preventDefault();
                      void update(ref, event.currentTarget);
                    }}
                  >
                    <div className="field"><label>Email</label><input name="email" type="email" defaultValue={ref.email} placeholder="Email" /></div>
                    <div className="field"><label>Nome</label><input name="first_name" defaultValue={ref.first_name} placeholder="Nome" /></div>
                    <div className="field"><label>Cognome</label><input name="last_name" defaultValue={ref.last_name} placeholder="Cognome" /></div>
                    <div className="field">
                      <label>Telefono</label>
                      <input
                        name="phone"
                        type="tel"
                        inputMode="tel"
                        pattern={PROVIDER_REFERENCE_PHONE_PATTERN}
                        title={PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE}
                        defaultValue={ref.phone}
                        placeholder="Telefono"
                        onInput={clearPhoneInvalid}
                      />
                    </div>
                    <div className="contactEditorActions">
                      <Button size="sm" type="submit" leftIcon={<Icon name="check" />} loading={mutations.updateReference.isPending}>Salva</Button>
                    </div>
                  </form>
                </div>
              ) : null}
            </article>
          );
        })}

        {refs.length === 0 ? (
          <div className="contactEmptyState">
            <Icon name="info" size={18} />
            <span>Nessun contatto disponibile per questo fornitore.</span>
          </div>
        ) : null}
      </div>

      {showSaveAction && onSaveRecipients ? (
        <div className="actionRow">
          <Button leftIcon={<Icon name="check" />} disabled={!editable} onClick={() => onSaveRecipients(selected)}>Salva destinatari</Button>
        </div>
      ) : null}
    </div>
  );
}
