import { Button, Icon, Tooltip } from '@mrsmith/ui';
import type { RdaPermissions, PoDetail } from '../api/types';
import type { TransitionAction } from '../api/queries';
import { formatMoney, isApprover, isRequester } from '../lib/format';
import { PO_STATES } from '../lib/state-labels';

interface ActionBarProps {
  po: PoDetail;
  permissions?: RdaPermissions;
  currentEmail?: string | null;
  dirty: boolean;
  canSubmit: boolean;
  quoteRuleBlocked: boolean;
  saving: boolean;
  transitioning: boolean;
  onClose: () => void;
  onSave: () => void;
  onSubmit: () => void;
  onTransition: (action: TransitionAction) => void;
  onPDF: () => void;
}

function levelLabel(value: unknown): string {
  return String(value ?? '1') === '2' ? 'Liv 2' : 'Liv 1';
}

export function ActionBar({
  po,
  permissions,
  currentEmail,
  dirty,
  canSubmit,
  quoteRuleBlocked,
  saving,
  transitioning,
  onClose,
  onSave,
  onSubmit,
  onTransition,
  onPDF,
}: ActionBarProps) {
  const requester = isRequester(po, currentEmail);
  const approverForPO = isApprover(po, currentEmail);
  const draftRequester = po.state === PO_STATES.DRAFT && requester;
  const l1l2 = po.state === PO_STATES.PENDING_APPROVAL && permissions?.is_approver && approverForPO;
  const afcPayment = po.state === PO_STATES.PENDING_APPROVAL_PAYMENT_METHOD && permissions?.is_afc;
  const afcLeasing = po.state === PO_STATES.PENDING_LEASING && permissions?.is_afc;
  const afcLeasingCreated = po.state === PO_STATES.PENDING_LEASING_ORDER_CREATION && permissions?.is_afc;
  const noLeasing = po.state === PO_STATES.PENDING_APPROVAL_NO_LEASING && permissions?.is_approver_no_leasing;
  const budgetIncrement = po.state === PO_STATES.PENDING_BUDGET_INCREMENT && permissions?.is_approver_extra_budget;
  const pendingSend = po.state === PO_STATES.PENDING_SEND;
  const pendingVerification = po.state === PO_STATES.PENDING_VERIFICATION;
  const busy = saving || transitioning;

  return (
    <section className="surface actionBar">
      <div className="actionBarGroup">
        <Button variant="secondary" leftIcon={<Icon name="arrow-left" />} onClick={onClose}>Chiudi</Button>
        {po.state !== PO_STATES.DRAFT ? (
          <Button variant="secondary" leftIcon={<Icon name="download" />} onClick={onPDF} loading={transitioning}>Genera PDF</Button>
        ) : null}
        {quoteRuleBlocked ? <span className="warningText">Attenzione: importo superiore a {formatMoney(3000, po.currency)}. Aggiungi 2 preventivi.</span> : null}
      </div>
      <div className="actionBarGroup">
        {draftRequester ? (
          <>
            <Button variant="secondary" leftIcon={<Icon name="check" />} onClick={onSave} loading={saving} disabled={!dirty}>
              Aggiorna bozza PO
            </Button>
            <Tooltip content={canSubmit ? 'Manda in approvazione' : 'Completa righe e allegati richiesti'}>
              <span>
                <Button leftIcon={<Icon name="arrow-right" />} onClick={onSubmit} loading={busy} disabled={!canSubmit}>
                  Manda PO in approvazione
                </Button>
              </span>
            </Tooltip>
          </>
        ) : null}
        {l1l2 ? (
          <>
            <Button leftIcon={<Icon name="check" />} loading={busy} onClick={() => onTransition('approve')}>Approva ({levelLabel(po.current_approval_level)})</Button>
            <Button variant="danger" leftIcon={<Icon name="x" />} loading={busy} onClick={() => onTransition('reject')}>Rifiuta ({levelLabel(po.current_approval_level)})</Button>
          </>
        ) : null}
        {afcPayment ? (
          <>
            <Button leftIcon={<Icon name="check" />} loading={busy} onClick={() => onTransition('payment-method/approve')}>Approva metodo pagamento</Button>
            <Button variant="danger" leftIcon={<Icon name="x" />} loading={busy} onClick={() => onTransition('reject')}>Rifiuta metodo pagamento</Button>
          </>
        ) : null}
        {afcLeasing ? (
          <>
            <Button leftIcon={<Icon name="check" />} loading={busy} onClick={() => onTransition('leasing/approve')}>Approva leasing</Button>
            <Button variant="danger" leftIcon={<Icon name="x" />} loading={busy} onClick={() => onTransition('leasing/reject')}>Rifiuta leasing</Button>
          </>
        ) : null}
        {afcLeasingCreated ? (
          <Button leftIcon={<Icon name="check" />} loading={busy} onClick={() => onTransition('leasing/created')}>Leasing creato</Button>
        ) : null}
        {noLeasing ? (
          <>
            <Button leftIcon={<Icon name="check" />} loading={busy} onClick={() => onTransition('no-leasing/approve')}>Approva no leasing</Button>
            <Button variant="danger" leftIcon={<Icon name="x" />} loading={busy} onClick={() => onTransition('reject')}>Rifiuta</Button>
          </>
        ) : null}
        {budgetIncrement ? (
          <>
            <Button leftIcon={<Icon name="check" />} loading={busy} onClick={() => onTransition('budget-increment/approve')}>Approva incremento budget</Button>
            <Button variant="danger" leftIcon={<Icon name="x" />} loading={busy} onClick={() => onTransition('budget-increment/reject')}>Rifiuta incremento budget</Button>
          </>
        ) : null}
        {pendingSend ? (
          <Button leftIcon={<Icon name="mail" />} loading={busy} onClick={() => onTransition('send-to-provider')}>Invia ordine al fornitore</Button>
        ) : null}
        {pendingVerification ? (
          <>
            <Button leftIcon={<Icon name="check-circle" />} loading={busy} onClick={() => onTransition('conformity/confirm')}>Erogato e conforme</Button>
            <Button variant="danger" leftIcon={<Icon name="x-circle" />} loading={busy} onClick={() => onTransition('conformity/reject')}>In contestazione</Button>
          </>
        ) : null}
      </div>
    </section>
  );
}
