import { Button, Icon, Modal } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { formatMoney, parseMistraMoney } from '../lib/format';

interface BudgetIncrementApproveDialogProps {
  open: boolean;
  poLabel: string;
  neededAmount?: number | string;
  currency?: string | null;
  loading?: boolean;
  onConfirm: (incrementPromise: string) => void;
  onClose: () => void;
}

function initialAmount(neededAmount?: number | string): string {
  if (neededAmount == null || neededAmount === '') return '';
  return String(neededAmount);
}

export function BudgetIncrementApproveDialog({
  open,
  poLabel,
  neededAmount,
  currency,
  loading,
  onConfirm,
  onClose,
}: BudgetIncrementApproveDialogProps) {
  const [amount, setAmount] = useState<string>(() => initialAmount(neededAmount));

  useEffect(() => {
    if (open) setAmount(initialAmount(neededAmount));
  }, [open, neededAmount]);

  const valid = parseMistraMoney(amount) > 0;
  const hasNeeded = neededAmount != null && neededAmount !== '';

  function confirm() {
    if (!valid || loading) return;
    onConfirm(String(parseMistraMoney(amount)));
  }

  return (
    <Modal open={open} onClose={onClose} title="Approva incremento budget" size="sm">
      <p className="modalText">{poLabel} attende copertura budget. Conferma l'importo dell'incremento da promettere.</p>
      <div className="field">
        <label htmlFor="budget-increment-amount">Importo incremento (€)</label>
        <input
          id="budget-increment-amount"
          type="text"
          inputMode="decimal"
          value={amount}
          autoFocus
          disabled={loading}
          onChange={(event) => setAmount(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') confirm();
          }}
        />
        {hasNeeded ? (
          <p className="muted">Richiesto dal richiedente: {formatMoney(neededAmount, currency)}</p>
        ) : null}
      </div>
      <div className="modalActions">
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button variant="primary" leftIcon={<Icon name="check" />} loading={loading} disabled={!valid} onClick={confirm}>
          Approva
        </Button>
      </div>
    </Modal>
  );
}
