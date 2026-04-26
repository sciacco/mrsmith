import { Button, Icon, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useProviderMutations } from '../api/queries';
import type { PoDetail, ProviderReference, ProviderSummary } from '../api/types';
import { availableReferenceTypes, QUALIFICATION_REF, referenceTypeLabel } from '../lib/provider-refs';

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

export function ProviderRefTable({
  po,
  provider,
  editable,
  onSaveRecipients,
}: {
  po: PoDetail;
  provider?: ProviderSummary;
  editable: boolean;
  onSaveRecipients: (ids: number[]) => void;
}) {
  const [selected, setSelected] = useState<number[]>(() => recipientIDs(po));
  const [newType, setNewType] = useState<string>(availableReferenceTypes()[0]?.value ?? 'ADMINISTRATIVE_REF');
  const mutations = useProviderMutations();
  const { toast } = useToast();
  const refs = useMemo(() => providerRefs(provider), [provider]);

  useEffect(() => {
    setSelected(recipientIDs(po));
  }, [po]);

  function toggle(id: number, checked: boolean) {
    setSelected((current) => (checked ? [...new Set([...current, id])] : current.filter((item) => item !== id)));
  }

  async function update(ref: ProviderReference, form: HTMLFormElement) {
    if (!provider || !ref.id || ref.reference_type === QUALIFICATION_REF) return;
    try {
      await mutations.updateReference.mutateAsync({ providerId: provider.id, refId: ref.id, body: refFromForm(form, ref.reference_type ?? newType) });
      toast('Contatto aggiornato');
    } catch {
      toast('Salvataggio contatto non riuscito', 'error');
    }
  }

  async function add(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!provider) return;
    try {
      await mutations.createReference.mutateAsync({ providerId: provider.id, body: refFromForm(event.currentTarget, newType) });
      event.currentTarget.reset();
      toast('Contatto aggiunto');
    } catch {
      toast('Salvataggio contatto non riuscito', 'error');
    }
  }

  return (
    <div className="stack">
      <p className="muted">Seleziona i contatti a cui inviare l&apos;ordine. Se non viene spuntato alcun contatto, verra utilizzato il contatto di tipo qualifica.</p>
      <div className="tableScroll">
        <table className="dataTable">
          <thead>
            <tr><th>Email</th><th>Nome</th><th>Cognome</th><th>Telefono</th><th>Tipo</th><th>Destinatario</th><th className="actionsCell">Azioni</th></tr>
          </thead>
          <tbody>
            {refs.map((ref) => {
              const readonly = ref.reference_type === QUALIFICATION_REF || !editable;
              return (
                <tr key={ref.id ?? ref.email}>
                  <td colSpan={7}>
                    <form
                      className="inlineForm"
                      onSubmit={(event) => {
                        event.preventDefault();
                        void update(ref, event.currentTarget);
                      }}
                    >
                      <input name="email" defaultValue={ref.email} disabled={readonly} placeholder="Email" />
                      <input name="first_name" defaultValue={ref.first_name} disabled={readonly} placeholder="Nome" />
                      <input name="last_name" defaultValue={ref.last_name} disabled={readonly} placeholder="Cognome" />
                      <input name="phone" defaultValue={ref.phone} disabled={readonly} placeholder="Telefono" />
                      <span>{referenceTypeLabel(ref.reference_type)}</span>
                      <input
                        type="checkbox"
                        checked={Boolean(ref.id && selected.includes(ref.id))}
                        disabled={!editable || ref.reference_type === QUALIFICATION_REF || !ref.id}
                        aria-label="Seleziona destinatario"
                        onChange={(event) => ref.id && toggle(ref.id, event.target.checked)}
                      />
                      <Button size="sm" type="submit" disabled={readonly} loading={mutations.updateReference.isPending}>Salva</Button>
                    </form>
                  </td>
                </tr>
              );
            })}
            {refs.length === 0 ? <tr><td colSpan={7} className="emptyInline">Nessun contatto disponibile.</td></tr> : null}
          </tbody>
        </table>
      </div>
      <div className="actionRow">
        <Button leftIcon={<Icon name="check" />} disabled={!editable} onClick={() => onSaveRecipients(selected)}>Salva contatti selezionati</Button>
      </div>
      {editable ? (
        <form className="inlineForm" onSubmit={(event) => void add(event)}>
          <input name="email" placeholder="Email" />
          <input name="first_name" placeholder="Nome" />
          <input name="last_name" placeholder="Cognome" />
          <input name="phone" placeholder="Telefono" />
          <select value={newType} onChange={(event) => setNewType(event.target.value)}>
            {availableReferenceTypes().map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select>
          <span />
          <Button size="sm" type="submit" leftIcon={<Icon name="plus" />} loading={mutations.createReference.isPending}>Aggiungi</Button>
        </form>
      ) : null}
    </div>
  );
}
