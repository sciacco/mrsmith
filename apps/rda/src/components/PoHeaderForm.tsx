import { Button, Icon, Modal, Tooltip } from '@mrsmith/ui';
import { useEffect, useState, type FormEvent } from 'react';
import type { BudgetForUser, PoDetail, ProviderSummary } from '../api/types';
import { formatDateIT, normalizeCurrency, RDA_CURRENCIES } from '../lib/format';
import type { PaymentMethodOption } from '../lib/payment-options';
import { stateLabel } from '../lib/state-labels';
import { BudgetSelect } from './BudgetSelect';
import { PaymentMethodSelect } from './PaymentMethodSelect';
import { PoNotesDisclosures } from './PoNotesDisclosures';
import { ProviderCombobox } from './ProviderCombobox';

export interface HeaderFormState {
  budget_id: number | '';
  object: string;
  project: string;
  provider_id: number | '';
  payment_method: string;
  currency: string;
  provider_offer_code: string;
  provider_offer_date: string;
  description: string;
  note: string;
}

function paymentCode(po: PoDetail): string {
  const value = po.payment_method;
  if (!value) return '';
  return typeof value === 'string' ? value.trim() : value.code.trim();
}

export function headerStateFromPO(po: PoDetail): HeaderFormState {
  return {
    budget_id: po.budget?.budget_id ?? po.budget?.id ?? '',
    object: po.object ?? '',
    project: po.project ?? '',
    provider_id: po.provider?.id ?? '',
    payment_method: paymentCode(po),
    currency: normalizeCurrency(po.currency),
    provider_offer_code: po.provider_offer_code ?? '',
    provider_offer_date: po.provider_offer_date?.slice(0, 10) ?? '',
    description: po.description ?? '',
    note: po.note ?? '',
  };
}

function providerLabel(provider?: ProviderSummary): string {
  return provider?.company_name?.trim() || (provider?.id ? `Fornitore ${provider.id}` : '-');
}

function budgetLabel(budgets: BudgetForUser[], value: number | '', fallback?: BudgetForUser): string {
  const selected = budgets.find((budget) => (budget.budget_id ?? budget.id ?? 0) === value) ?? fallback;
  if (!selected) return '-';
  return selected.name ?? `Budget ${selected.budget_id ?? selected.id ?? value}`;
}

function paymentLabel(value: string, methods: PaymentMethodOption[], po: PoDetail): string {
  const selected = methods.find((method) => method.code === value);
  if (selected) return selected.label;
  const current = po.payment_method;
  if (!current) return value || '-';
  if (typeof current === 'string') return current;
  return current.description || current.code || value || '-';
}

function optionalText(value?: string | null): string {
  return value?.trim() || '-';
}

function SummaryItem({
  label,
  value,
  detail,
  wide,
  warning,
}: {
  label: string;
  value: string;
  detail?: string;
  wide?: boolean;
  warning?: boolean;
}) {
  return (
    <div className={`summaryItem ${wide ? 'wide' : ''} ${warning ? 'warning' : ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      {detail ? <p>{detail}</p> : null}
    </div>
  );
}

export function PoHeaderSummary({
  po,
  value,
  budgets,
  providers,
  paymentMethods,
  paymentRequiresVerification,
  canEdit,
  editDisabledReason,
  onEdit,
}: {
  po: PoDetail;
  value: HeaderFormState;
  budgets: BudgetForUser[];
  providers: ProviderSummary[];
  paymentMethods: PaymentMethodOption[];
  paymentRequiresVerification?: boolean;
  canEdit: boolean;
  editDisabledReason: string;
  onEdit: () => void;
}) {
  const headerDate = po.created ?? po.creation_date ?? po.updated;
  const provider = providers.find((item) => item.id === value.provider_id) ?? po.provider;
  const payment = paymentLabel(value.payment_method, paymentMethods, po);
  const editButton = (
    <span className="headerEditAction">
      <Button variant="secondary" leftIcon={<Icon name="pencil" />} disabled={!canEdit} onClick={onEdit}>
        Modifica dati
      </Button>
    </span>
  );

  return (
    <section className="surface">
      <div className="surfaceHeader">
        <div>
          <h2>Ordine Numero: {po.code ?? po.id} del {formatDateIT(headerDate)}</h2>
          <p className="muted">Stato Attuale: {stateLabel(po.state)}</p>
        </div>
        {canEdit ? editButton : <Tooltip content={editDisabledReason}>{editButton}</Tooltip>}
      </div>
      <div className="poSummaryBody">
        <div className="poSummaryGrid">
          <SummaryItem label="Budget" value={budgetLabel(budgets, value.budget_id, po.budget)} />
          <SummaryItem label="Progetto" value={optionalText(value.project)} />
          <SummaryItem label="Fornitore" value={providerLabel(provider)} />
          <SummaryItem label="Pagamento" value={payment} warning={paymentRequiresVerification} detail={paymentRequiresVerification ? 'Richiede approvazione metodo pagamento' : undefined} />
          <SummaryItem label="Riferimento preventivo" value={optionalText(value.provider_offer_code)} />
          <SummaryItem label="Data preventivo" value={value.provider_offer_date ? formatDateIT(value.provider_offer_date) : '-'} />
          <SummaryItem label="Valuta" value={normalizeCurrency(value.currency)} />
        </div>

        <PoNotesDisclosures note={value.note} description={value.description} />
      </div>
    </section>
  );
}

export function PoHeaderEditModal({
  open,
  po,
  value,
  budgets,
  providers,
  paymentMethods,
  paymentRequiresVerification,
  draftEditable,
  paymentEditable,
  saving,
  onChange,
  onSave,
  onClose,
  onProviderChange,
  onRequestNewProvider,
}: {
  open: boolean;
  po: PoDetail;
  value: HeaderFormState;
  budgets: BudgetForUser[];
  providers: ProviderSummary[];
  paymentMethods: PaymentMethodOption[];
  paymentRequiresVerification?: boolean;
  draftEditable: boolean;
  paymentEditable: boolean;
  saving: boolean;
  onChange: (value: HeaderFormState) => void;
  onSave: (value: HeaderFormState) => void;
  onClose: () => void;
  onProviderChange: (value: number | '') => void;
  onRequestNewProvider: (search: string) => void;
}) {
  const [draft, setDraft] = useState(value);
  const canEditPayment = draftEditable || paymentEditable;

  useEffect(() => {
    if (open) setDraft(value);
  }, [open, value]);

  const update = <K extends keyof HeaderFormState>(key: K, next: HeaderFormState[K]) => {
    const updated = { ...draft, [key]: next };
    setDraft(updated);
    onChange(updated);
  };

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSave(draft);
  }

  const title = `Stai modificano il Purchase Order ${po.code ?? po.id}`;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      size="fluid"
    >
      <form className="formGrid poEditForm" onSubmit={submit}>
        <div className="field">
          <label>Budget</label>
          <BudgetSelect budgets={budgets} value={draft.budget_id} disabled={!draftEditable} onChange={(next) => update('budget_id', next)} />
        </div>
        <div className="field">
          <label>Progetto</label>
          <input value={draft.project} disabled={!draftEditable} maxLength={50} onChange={(event) => update('project', event.target.value)} />
        </div>
        <div className="field wide">
          <label>Oggetto</label>
          <input value={draft.object} disabled={!draftEditable} onChange={(event) => update('object', event.target.value)} />
        </div>
        <div className="providerPaymentGrid">
          <div className="field providerPaymentProvider">
            <label>Fornitore</label>
            <ProviderCombobox
              providers={providers}
              value={draft.provider_id}
              disabled={!draftEditable}
              onChange={onProviderChange}
              onRequestNewProvider={onRequestNewProvider}
            />
          </div>
          <div className="field providerPaymentMethod">
            <label>Modalità di pagamento</label>
            <PaymentMethodSelect
              methods={paymentMethods}
              value={draft.payment_method}
              disabled={!canEditPayment}
              requiresVerification={paymentRequiresVerification}
              onChange={(next) => update('payment_method', next)}
            />
            {paymentRequiresVerification ? <p className="fieldWarning">Richiede approvazione metodo pagamento</p> : null}
          </div>
        </div>
        <div className="poOfferGrid">
          <div className="field">
            <label>Riferimento preventivo</label>
            <input value={draft.provider_offer_code} disabled={!draftEditable} onChange={(event) => update('provider_offer_code', event.target.value)} />
          </div>
          <div className="field">
            <label>Data preventivo</label>
            <input type="date" value={draft.provider_offer_date} disabled={!draftEditable} onChange={(event) => update('provider_offer_date', event.target.value)} />
          </div>
          <div className="field currencyField">
            <label>Valuta</label>
            <select value={draft.currency} disabled={!draftEditable} onChange={(event) => update('currency', normalizeCurrency(event.target.value))}>
              {RDA_CURRENCIES.map((currency) => (
                <option key={currency} value={currency}>
                  {currency}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="field wide">
          <label>Descrizione ad uso interno</label>
          <textarea rows={4} value={draft.description} disabled={!draftEditable} onChange={(event) => update('description', event.target.value)} />
        </div>
        <div className="field wide">
          <label>Note fornitore</label>
          <textarea rows={4} value={draft.note} disabled={!draftEditable} onChange={(event) => update('note', event.target.value)} />
        </div>
        <div className="modalActions fullWidth">
          <Button variant="secondary" onClick={onClose}>Annulla</Button>
          <Button type="submit" leftIcon={<Icon name="check" />} loading={saving}>
            Salva modifiche
          </Button>
        </div>
      </form>
    </Modal>
  );
}
