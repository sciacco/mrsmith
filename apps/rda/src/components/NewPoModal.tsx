import { Button, Icon, Modal, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useCreatePO } from '../api/queries';
import type { BudgetForUser, CreatePOPayload, PaymentMethod, ProviderSummary } from '../api/types';
import { coerceID } from '../lib/format';
import {
  buildPaymentMethodOptions,
  paymentCodeFromProvider,
  requiresPaymentMethodVerification,
} from '../lib/payment-options';
import { firstError, validateNewPO } from '../lib/validation';
import { BudgetSelect, findBudget } from './BudgetSelect';
import { NewProviderInlineForm } from './NewProviderInlineForm';
import { PaymentMethodSelect } from './PaymentMethodSelect';
import { ProviderSelect } from './ProviderSelect';

export function NewPoModal({
  open,
  budgets,
  providers,
  methods,
  cdlanDefault,
  onClose,
}: {
  open: boolean;
  budgets: BudgetForUser[];
  providers: ProviderSummary[];
  methods: PaymentMethod[];
  cdlanDefault: string;
  onClose: () => void;
}) {
  const [type, setType] = useState<'STANDARD' | 'ECOMMERCE'>('STANDARD');
  const [budgetId, setBudgetId] = useState<number | ''>('');
  const [providerId, setProviderId] = useState<number | ''>('');
  const [paymentMethod, setPaymentMethod] = useState('');
  const [project, setProject] = useState('');
  const [object, setObject] = useState('');
  const [description, setDescription] = useState('');
  const [note, setNote] = useState('');
  const [offerCode, setOfferCode] = useState('');
  const [offerDate, setOfferDate] = useState('');
  const createPO = useCreatePO();
  const { toast } = useToast();
  const navigate = useNavigate();

  const provider = providers.find((item) => item.id === providerId);
  const providerDefault = paymentCodeFromProvider(provider);
  const paymentOptions = useMemo(
    () =>
      buildPaymentMethodOptions({
        methods,
        providerDefault: provider?.default_payment_method,
        cdlanDefaultCode: cdlanDefault,
        currentCode: paymentMethod,
      }),
    [cdlanDefault, methods, paymentMethod, provider?.default_payment_method],
  );
  const paymentRequiresVerification = requiresPaymentMethodVerification(paymentMethod, providerDefault, cdlanDefault);

  useEffect(() => {
    if (!open) return;
    const next = providerDefault || cdlanDefault;
    if (next) setPaymentMethod(next);
  }, [cdlanDefault, open, providerDefault]);

  function reset() {
    setType('STANDARD');
    setBudgetId('');
    setProviderId('');
    setPaymentMethod('');
    setProject('');
    setObject('');
    setDescription('');
    setNote('');
    setOfferCode('');
    setOfferDate('');
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const budget = findBudget(budgets, budgetId);
    const body: CreatePOPayload = {
      type,
      budget_id: Number(budgetId),
      provider_id: Number(providerId),
      payment_method: paymentMethod,
      project: project.trim(),
      object: object.trim(),
      description: description.trim() || undefined,
      note: note.trim() || undefined,
      provider_offer_code: offerCode.trim() || undefined,
      provider_offer_date: offerDate || undefined,
      ...(budget?.cost_center ? { cost_center: budget.cost_center } : {}),
      ...(budget?.budget_user_id ? { budget_user_id: budget.budget_user_id } : {}),
    };
    const validation = validateNewPO(body);
    const message = firstError(validation);
    if (message) {
      toast(message, 'warning');
      return;
    }
    try {
      const created = await createPO.mutateAsync(body);
      const id = coerceID(created.id);
      toast('Bozza creata');
      onClose();
      reset();
      if (id) navigate(`/rda/po/${id}`);
    } catch {
      toast('Creazione non riuscita', 'error');
    }
  }

  function handleProviderCreated(providerCreated: ProviderSummary) {
    setProviderId(providerCreated.id);
  }

  return (
    <Modal open={open} onClose={onClose} title="Nuova richiesta" size="wide">
      <form className="formGrid" onSubmit={(event) => void submit(event)}>
        <h3 className="sectionTitle">RDA</h3>
        <div className="field">
          <label>Budget</label>
          <BudgetSelect budgets={budgets} value={budgetId} onChange={setBudgetId} />
        </div>
        <div className="field">
          <label>Tipo PO</label>
          <select value={type} onChange={(event) => setType(event.target.value as 'STANDARD' | 'ECOMMERCE')}>
            <option value="STANDARD">STANDARD</option>
            <option value="ECOMMERCE">ECOMMERCE</option>
          </select>
        </div>
        <div className="field">
          <label>Progetto</label>
          <input value={project} maxLength={50} onChange={(event) => setProject(event.target.value)} />
        </div>
        <div className="field">
          <label>Oggetto</label>
          <input value={object} onChange={(event) => setObject(event.target.value)} />
        </div>
        <h3 className="sectionTitle">Fornitore e pagamento</h3>
        <div className="providerPaymentGrid">
          <div className="field providerPaymentProvider">
            <label>Fornitore</label>
            <ProviderSelect providers={providers} value={providerId} onChange={setProviderId} />
          </div>
          <div className="field providerPaymentMethod">
            <label>Modalità di pagamento</label>
            <PaymentMethodSelect
              methods={paymentOptions}
              value={paymentMethod}
              requiresVerification={paymentRequiresVerification}
              onChange={setPaymentMethod}
            />
            {paymentRequiresVerification ? <p className="fieldWarning">Richiede approvazione metodo pagamento</p> : null}
          </div>
        </div>
        <div className="field">
          <label>Riferimento preventivo</label>
          <input value={offerCode} onChange={(event) => setOfferCode(event.target.value)} />
        </div>
        <div className="field">
          <label>Data preventivo</label>
          <input type="date" value={offerDate} onChange={(event) => setOfferDate(event.target.value)} />
        </div>
        <div className="field wide">
          <label>Descrizione interna</label>
          <textarea rows={3} value={description} onChange={(event) => setDescription(event.target.value)} />
        </div>
        <div className="field wide">
          <label>Note fornitore</label>
          <textarea rows={3} value={note} onChange={(event) => setNote(event.target.value)} />
        </div>
        <NewProviderInlineForm onCreated={handleProviderCreated} />
        <div className="modalActions fullWidth">
          <Button variant="secondary" onClick={onClose}>Annulla</Button>
          <Button type="submit" leftIcon={<Icon name="plus" />} loading={createPO.isPending}>Crea bozza</Button>
        </div>
      </form>
    </Modal>
  );
}
