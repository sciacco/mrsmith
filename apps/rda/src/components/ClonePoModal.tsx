import { Button, Icon, Modal, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { useClonePO } from '../api/queries';
import type { BudgetForUser, ClonePOPayload, ClonePOResponse, PoDetail } from '../api/types';
import { apiErrorMessage } from '../lib/api-error';
import { formatMoney } from '../lib/format';
import { selectedBudgetBinding } from '../lib/po-payload';
import { stateLabel } from '../lib/state-labels';
import { BudgetSelect, findBudget } from './BudgetSelect';

interface ClonePoModalProps {
  open: boolean;
  po: PoDetail;
  budgets: BudgetForUser[];
  onClose: () => void;
  onCloned: (response: ClonePOResponse) => void;
}

function availableSourceBudget(po: PoDetail, budgets: BudgetForUser[]): number | '' {
  const sourceID = po.budget?.budget_id ?? po.budget?.id;
  if (!sourceID) return '';
  return budgets.some((budget) => (budget.budget_id ?? budget.id ?? 0) === sourceID) ? sourceID : '';
}

function providerLabel(po: PoDetail): string {
  return po.provider?.company_name?.trim() || (po.provider?.id ? `Fornitore ${po.provider.id}` : '-');
}

export function ClonePoModal({ open, po, budgets, onClose, onCloned }: ClonePoModalProps) {
  const defaultBudgetID = useMemo(() => availableSourceBudget(po, budgets), [budgets, po]);
  const [budgetId, setBudgetId] = useState<number | ''>(defaultBudgetID);
  const [includeRows, setIncludeRows] = useState(true);
  const [includeRecipients, setIncludeRecipients] = useState(true);
  const clonePO = useClonePO();
  const { toast } = useToast();

  useEffect(() => {
    if (!open) return;
    setBudgetId(defaultBudgetID);
    setIncludeRows(true);
    setIncludeRecipients(true);
  }, [defaultBudgetID, open]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const budget = findBudget(budgets, budgetId);
    if (!budget) {
      toast('Seleziona un budget', 'warning');
      return;
    }
    const binding = selectedBudgetBinding(budgets, budgetId);
    if (Boolean(binding.cost_center) === Boolean(binding.budget_user_id)) {
      toast('Il budget deve indicare un solo centro di costo o utente.', 'warning');
      return;
    }
    const body: ClonePOPayload = {
      budget_id: Number(budgetId),
      include_rows: includeRows,
      include_recipients: includeRecipients,
      include_offer_fields: false,
      ...(binding.cost_center ? { cost_center: binding.cost_center } : {}),
      ...(binding.budget_user_id ? { budget_user_id: binding.budget_user_id } : {}),
    };
    try {
      const response = await clonePO.mutateAsync({ id: po.id, body });
      onClose();
      onCloned(response);
    } catch (error) {
      toast(apiErrorMessage(error, 'Duplicazione non riuscita'), 'error');
    }
  }

  const rows = po.rows?.length ?? 0;
  const recipients = po.recipients?.length ?? 0;

  return (
    <Modal open={open} onClose={onClose} title="Duplica RDA" size="wide" dismissible={!clonePO.isPending}>
      <form className="formGrid clonePoForm" onSubmit={(event) => void submit(event)}>
        <p className="modalText fullWidth">
          Crea una nuova bozza partendo da questa RDA. La nuova richiesta sara assegnata a te e potrai modificarla prima dell'invio.
        </p>

        <div className="clonePoSummary fullWidth" aria-label="Riepilogo RDA sorgente">
          <span>
            <small>RDA sorgente</small>
            <strong>{po.code ?? `PO ${po.id}`}</strong>
          </span>
          <span>
            <small>Fornitore</small>
            <strong>{providerLabel(po)}</strong>
          </span>
          <span>
            <small>Stato</small>
            <strong>{stateLabel(po.state)}</strong>
          </span>
          <span>
            <small>Totale</small>
            <strong>{formatMoney(po.total_price, po.currency)}</strong>
          </span>
        </div>

        <h3 className="sectionTitle">Nuova bozza</h3>
        <div className="field wide">
          <label>Budget</label>
          <BudgetSelect budgets={budgets} value={budgetId} onChange={setBudgetId} />
        </div>
        <label className="field checkboxField">
          <span>Copia righe ({rows})</span>
          <input type="checkbox" checked={includeRows} onChange={(event) => setIncludeRows(event.target.checked)} />
        </label>
        <label className="field checkboxField">
          <span>Copia destinatari ({recipients})</span>
          <input type="checkbox" checked={includeRecipients} onChange={(event) => setIncludeRecipients(event.target.checked)} />
        </label>
        <div className="clonePoNotice fullWidth">
          <Icon name="info" size={18} />
          <span>Allegati, commenti, approvazioni e riferimenti preventivo non verranno copiati.</span>
        </div>
        <div className="modalActions fullWidth">
          <Button variant="secondary" disabled={clonePO.isPending} onClick={onClose}>Annulla</Button>
          <Button type="submit" leftIcon={<Icon name="copy" />} loading={clonePO.isPending}>Crea bozza duplicata</Button>
        </div>
      </form>
    </Modal>
  );
}
