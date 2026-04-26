import type { BudgetForUser, PaymentMethod, PoDetail, ProviderSummary } from '../api/types';
import { formatDateIT } from '../lib/format';
import { stateLabel } from '../lib/state-labels';
import { BudgetSelect } from './BudgetSelect';
import { PaymentMethodSelect } from './PaymentMethodSelect';
import { ProviderSelect } from './ProviderSelect';
import { RecipientsList } from './RecipientsList';

export interface HeaderFormState {
  budget_id: number | '';
  object: string;
  project: string;
  provider_id: number | '';
  payment_method: string;
  provider_offer_code: string;
  provider_offer_date: string;
  description: string;
  note: string;
}

function paymentCode(po: PoDetail): string {
  const value = po.payment_method;
  if (!value) return '';
  return typeof value === 'string' ? value : value.code;
}

export function headerStateFromPO(po: PoDetail): HeaderFormState {
  return {
    budget_id: po.budget?.budget_id ?? po.budget?.id ?? '',
    object: po.object ?? '',
    project: po.project ?? '',
    provider_id: po.provider?.id ?? '',
    payment_method: paymentCode(po),
    provider_offer_code: po.provider_offer_code ?? '',
    provider_offer_date: po.provider_offer_date?.slice(0, 10) ?? '',
    description: po.description ?? '',
    note: po.note ?? '',
  };
}

export function PoHeaderForm({
  po,
  value,
  budgets,
  providers,
  paymentMethods,
  draftEditable,
  paymentEditable,
  onChange,
}: {
  po: PoDetail;
  value: HeaderFormState;
  budgets: BudgetForUser[];
  providers: ProviderSummary[];
  paymentMethods: PaymentMethod[];
  draftEditable: boolean;
  paymentEditable: boolean;
  onChange: (value: HeaderFormState) => void;
}) {
  const update = <K extends keyof HeaderFormState>(key: K, next: HeaderFormState[K]) => onChange({ ...value, [key]: next });
  const headerDate = po.created ?? po.creation_date ?? po.updated;

  return (
    <section className="surface">
      <div className="surfaceHeader">
        <div>
          <h2>Ordine Numero: {po.code ?? po.id} del {formatDateIT(headerDate)}</h2>
          <p className="muted">Stato Attuale: {stateLabel(po.state)}</p>
        </div>
      </div>
      <div className="tabBody formGrid three">
        <div className="field">
          <label>Budget</label>
          <BudgetSelect budgets={budgets} value={value.budget_id} disabled={!draftEditable} onChange={(next) => update('budget_id', next)} />
        </div>
        <div className="field">
          <label>Oggetto</label>
          <input value={value.object} disabled={!draftEditable} onChange={(event) => update('object', event.target.value)} />
        </div>
        <div className="field">
          <label>Progetto</label>
          <input value={value.project} disabled={!draftEditable} maxLength={50} onChange={(event) => update('project', event.target.value)} />
        </div>
        <div className="field">
          <label>Fornitore</label>
          <ProviderSelect providers={providers} value={value.provider_id} disabled={!draftEditable} onChange={(next) => update('provider_id', next)} />
        </div>
        <div className="field">
          <label>Metodo pagamento</label>
          <PaymentMethodSelect
            methods={paymentMethods}
            value={value.payment_method}
            disabled={!draftEditable && !paymentEditable}
            onChange={(next) => update('payment_method', next)}
          />
        </div>
        <div className="field">
          <label>Riferimento preventivo</label>
          <input value={value.provider_offer_code} disabled={!draftEditable} onChange={(event) => update('provider_offer_code', event.target.value)} />
        </div>
        <div className="field">
          <label>Data preventivo</label>
          <input type="date" value={value.provider_offer_date} disabled={!draftEditable} onChange={(event) => update('provider_offer_date', event.target.value)} />
        </div>
        <div className="field wide">
          <label>Contatti selezionati</label>
          <RecipientsList recipients={po.recipients} />
        </div>
      </div>
    </section>
  );
}
