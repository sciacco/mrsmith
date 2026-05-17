import { Button, Icon, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useProviderMutations } from '../api/queries';
import type { PoDetail, ProviderReference, ProviderSummary } from '../api/types';
import {
  QUALIFICATION_REF,
  referenceTypeLabel,
} from '../lib/provider-refs';
import { ProviderContactModal } from './ProviderContactModal';

function providerRefs(provider?: ProviderSummary): ProviderReference[] {
  if (!provider) return [];
  const refs = provider.refs?.length ? provider.refs : provider.ref ? [provider.ref] : [];
  return [...refs].sort(compareProviderRefs);
}

const referenceTypeOrder: Record<string, number> = {
  [QUALIFICATION_REF]: 0,
  ADMINISTRATIVE_REF: 1,
  TECHNICAL_REF: 2,
  OTHER_REF: 3,
};

function compareProviderRefs(left: ProviderReference, right: ProviderReference): number {
  const leftOrder = referenceTypeOrder[left.reference_type ?? ''] ?? 4;
  const rightOrder = referenceTypeOrder[right.reference_type ?? ''] ?? 4;
  if (leftOrder !== rightOrder) return leftOrder - rightOrder;
  return contactName(left).localeCompare(contactName(right), 'it', { sensitivity: 'base' });
}

function recipientIDs(po: PoDetail): number[] {
  return (po.recipients ?? []).map((ref) => ref.id).filter((id): id is number => id != null);
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
  savingRecipients = false,
}: {
  po: PoDetail;
  provider?: ProviderSummary;
  editable: boolean;
  onSelectionChange?: (ids: number[]) => void;
  onSaveRecipients?: (ids: number[]) => void;
  showSaveAction?: boolean;
  savingRecipients?: boolean;
}) {
  const [selected, setSelected] = useState<number[]>(() => recipientIDs(po));
  const [contactModal, setContactModal] = useState<{ mode: 'create' } | { mode: 'edit'; ref: ProviderReference } | null>(null);
  const mutations = useProviderMutations();
  const { toast } = useToast();
  const refs = useMemo(() => providerRefs(provider), [provider]);
  const savingContact = mutations.createReference.isPending || mutations.updateReference.isPending;

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

  async function saveContact(body: ProviderReference) {
    if (!provider || !contactModal) return;
    try {
      if (contactModal.mode === 'edit') {
        const ref = contactModal.ref;
        if (!ref.id || ref.reference_type === QUALIFICATION_REF) return;
        await mutations.updateReference.mutateAsync({
          providerId: provider.id,
          refId: ref.id,
          body: { ...body, reference_type: ref.reference_type },
        });
        toast('Contatto aggiornato');
      } else {
        await mutations.createReference.mutateAsync({ providerId: provider.id, body });
        toast('Contatto aggiunto');
      }
      setContactModal(null);
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
          <Button size="sm" variant="secondary" leftIcon={<Icon name="plus" />} onClick={() => setContactModal({ mode: 'create' })}>
            Aggiungi contatto
          </Button>
        ) : null}
      </div>

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
                  aria-label="Modifica contatto"
                  title="Modifica"
                  onClick={() => setContactModal({ mode: 'edit', ref })}
                >
                  <Icon name="pencil" size={16} />
                </button>
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
          <Button leftIcon={<Icon name="check" />} disabled={!editable} loading={savingRecipients} onClick={() => onSaveRecipients(selected)}>Salva destinatari</Button>
        </div>
      ) : null}

      <ProviderContactModal
        open={contactModal != null}
        mode={contactModal?.mode ?? 'create'}
        contact={contactModal?.mode === 'edit' ? contactModal.ref : null}
        saving={savingContact}
        onClose={() => setContactModal(null)}
        onSubmit={(body) => void saveContact(body)}
      />
    </div>
  );
}
