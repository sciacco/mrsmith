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

// Show the prefill as a whole Euro amount. Mistra serializes the upstream
// numeric(20,5) as a 3-decimal string (e.g. "1250.000"); if any fractional digit
// is non-zero, round UP to the next integer so the promise never falls below the
// live needed amount (rda.trigger_subtract_budget enforces
// increment_promise >= compute_budget_increment_needed and raises T0WZ3 otherwise).
// Integer arithmetic keeps this exact for any value (no float artifacts).
function initialAmount(neededAmount?: number | string): string {
  if (neededAmount == null || neededAmount === '') return '';
  const raw = String(neededAmount).trim().replace(',', '.');
  if (!raw) return '';
  const [intPart = '0', fracPart = ''] = raw.split('.');
  const intN = parseInt(intPart, 10) || 0;
  return /[1-9]/.test(fracPart) ? String(intN + 1) : String(intN);
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
