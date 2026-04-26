import { Button, Icon, useToast } from '@mrsmith/ui';
import { useState } from 'react';
import { useProviderMutations } from '../api/queries';
import type { ProviderPayload, ProviderSummary } from '../api/types';

export function NewProviderInlineForm({ onCreated }: { onCreated: (provider: ProviderSummary) => void }) {
  const { createProvider } = useProviderMutations();
  const { toast } = useToast();
  const [open, setOpen] = useState(false);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const companyName = String(form.get('company_name') ?? '').trim();
    const email = String(form.get('email') ?? '').trim();
    if (!companyName) {
      toast('Inserisci la ragione sociale', 'warning');
      return;
    }
    if (!email) {
      toast('Inserisci il contatto qualifica', 'warning');
      return;
    }
    const payload: ProviderPayload = {
      company_name: companyName,
      state: 'DRAFT',
      country: 'IT',
      language: 'it',
      vat_number: String(form.get('vat_number') ?? '').trim() || undefined,
      cf: String(form.get('cf') ?? '').trim() || undefined,
      postal_code: String(form.get('postal_code') ?? '').trim() || undefined,
      province: String(form.get('province') ?? '').trim() || undefined,
      city: String(form.get('city') ?? '').trim() || undefined,
      address: String(form.get('address') ?? '').trim() || undefined,
      ref: {
        first_name: String(form.get('first_name') ?? '').trim(),
        last_name: String(form.get('last_name') ?? '').trim(),
        email,
        phone: String(form.get('phone') ?? '').trim(),
        reference_type: 'QUALIFICATION_REF',
      },
    };
    try {
      const provider = await createProvider.mutateAsync(payload);
      toast('Fornitore creato');
      onCreated(provider);
      setOpen(false);
      event.currentTarget.reset();
    } catch {
      toast('Dati fornitore non disponibili in questo momento', 'error');
    }
  }

  return (
    <div className="fullWidth stack">
      <Button variant="secondary" leftIcon={<Icon name={open ? 'chevron-up' : 'chevron-down'} />} onClick={() => setOpen((value) => !value)}>
        Nuovo fornitore
      </Button>
      {open ? (
        <form className="formGrid three" onSubmit={(event) => void submit(event)}>
          <div className="field"><label>Ragione sociale</label><input name="company_name" /></div>
          <div className="field"><label>P.IVA</label><input name="vat_number" /></div>
          <div className="field"><label>CF</label><input name="cf" /></div>
          <div className="field"><label>Indirizzo</label><input name="address" /></div>
          <div className="field"><label>Citta</label><input name="city" /></div>
          <div className="field"><label>CAP</label><input name="postal_code" /></div>
          <div className="field"><label>Provincia</label><input name="province" maxLength={2} /></div>
          <div className="field"><label>Nome qualifica</label><input name="first_name" /></div>
          <div className="field"><label>Cognome qualifica</label><input name="last_name" /></div>
          <div className="field"><label>Email qualifica</label><input name="email" type="email" /></div>
          <div className="field"><label>Telefono qualifica</label><input name="phone" /></div>
          <div className="actionRow fullWidth">
            <Button type="submit" leftIcon={<Icon name="check" />} loading={createProvider.isPending}>Crea fornitore</Button>
          </div>
        </form>
      ) : null}
    </div>
  );
}
