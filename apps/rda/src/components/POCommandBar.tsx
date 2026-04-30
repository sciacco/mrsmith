import { Button, Icon, Tooltip } from '@mrsmith/ui';
import { useMemo, useState } from 'react';
import { getRdaQuoteThreshold } from '../runtime-config';
import type { PoAction, PoActionMode, PoDetail } from '../api/types';
import type { TransitionAction } from '../api/queries';
import { countQuoteAttachments } from '../lib/attachments';
import { formatMoney, parseMistraMoney } from '../lib/format';
import { selectedModeID } from '../lib/po-detail-view-model';
import { PO_STATES, stateLabel } from '../lib/state-labels';
import { ConfirmDialog } from './ConfirmDialog';

const transitionActions: readonly TransitionAction[] = [
  'submit',
  'approve',
  'reject',
  'leasing/approve',
  'leasing/reject',
  'leasing/created',
  'no-leasing/approve',
  'payment-method/approve',
  'budget-increment/approve',
  'budget-increment/reject',
  'conformity/confirm',
  'conformity/reject',
  'send-to-provider',
];

interface POCommandBarProps {
  po: PoDetail;
  selectedMode: string;
  canSubmit: boolean;
  quoteRuleBlocked: boolean;
  saving: boolean;
  transitioning: boolean;
  onModeChange: (modeID: string) => void;
  onClose: () => void;
  onSubmit: () => void;
  onTransition: (action: TransitionAction) => void;
  onPDF: () => void;
}

function isTransitionAction(value: string): value is TransitionAction {
  return transitionActions.includes(value as TransitionAction);
}

function paymentLabel(po: PoDetail): string {
  const payment = po.payment_method;
  if (!payment) return '-';
  return typeof payment === 'string' ? payment : payment.description || payment.code || '-';
}

function actionIcon(action: PoAction): 'check' | 'check-circle' | 'arrow-right' | 'mail' | 'x' | 'x-circle' {
  if (action.tone === 'danger') return action.id.includes('conformity') ? 'x-circle' : 'x';
  if (action.id === 'send-to-provider') return 'mail';
  if (action.id === 'submit') return 'arrow-right';
  if (action.id === 'conformity/confirm') return 'check-circle';
  return 'check';
}

function summaryValue(value: string | number | undefined, fallback = '-'): string {
  if (value == null || value === '') return fallback;
  return String(value);
}

function SummaryChip({ label, value, tone }: { label: string; value: string; tone?: 'warning' | 'success' | 'info' }) {
  return (
    <span className={`commandChip ${tone ?? ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </span>
  );
}

function ModePicker({
  modes,
  selected,
  onChange,
}: {
  modes: PoActionMode[];
  selected: string;
  onChange: (modeID: string) => void;
}) {
  const current = modes.find((mode) => mode.id === selected) ?? modes[0];
  if (modes.length <= 1) {
    return (
      <div className="commandModeSingle">
        <span>Operi come</span>
        <strong>{current?.label ?? 'Consultazione'}</strong>
      </div>
    );
  }

  return (
    <label className="commandModeSelect">
      <span>Operi come</span>
      <select value={selected} onChange={(event) => onChange(event.target.value)}>
        {modes.map((mode) => (
          <option key={mode.id} value={mode.id}>{mode.label}</option>
        ))}
      </select>
    </label>
  );
}

function ActionButton({
  action,
  busy,
  onRun,
}: {
  action: PoAction;
  busy: boolean;
  onRun: (action: PoAction) => void;
}) {
  const button = (
    <Button
      variant={action.tone === 'danger' ? 'danger' : action.primary ? 'primary' : 'secondary'}
      leftIcon={<Icon name={actionIcon(action)} />}
      loading={busy}
      disabled={action.disabled}
      onClick={() => onRun(action)}
    >
      {action.label}
    </Button>
  );

  if (!action.disabled || !action.disabled_reason) return button;
  return (
    <Tooltip content={action.disabled_reason}>
      <span>{button}</span>
    </Tooltip>
  );
}

export function POCommandBar({
  po,
  selectedMode,
  canSubmit,
  quoteRuleBlocked,
  saving,
  transitioning,
  onModeChange,
  onClose,
  onSubmit,
  onTransition,
  onPDF,
}: POCommandBarProps) {
  const [confirmAction, setConfirmAction] = useState<PoAction | null>(null);
  const actionModel = po.action_model;
  const modes = actionModel?.modes ?? [];
  const modeID = selectedModeID(actionModel, selectedMode);
  const mode = modes.find((item) => item.id === modeID);
  const actions = useMemo(
    () => (actionModel?.actions ?? []).filter((action) => action.mode_id === modeID),
    [actionModel?.actions, modeID],
  );
  const summary = actionModel?.summary;
  const quoteCount = summary?.quote_count ?? countQuoteAttachments(po.attachments);
  const attachmentCount = summary?.attachment_count ?? po.attachments?.length ?? 0;
  const quoteRequired = parseMistraMoney(summary?.total_price ?? po.total_price) >= getRdaQuoteThreshold();
  const busy = saving || transitioning;
  const submitAction = actions.find((action) => action.id === 'submit');
  const serverSubmitBlocked = Boolean(submitAction?.disabled);
  const submitBlockedReason = submitAction?.disabled_reason ?? 'Completa righe e allegati richiesti.';
  const primaryActions = actions.filter((action) => action.primary && action.tone !== 'danger');
  const secondaryActions = actions.filter((action) => !action.primary && action.tone !== 'danger');
  const dangerActions = actions.filter((action) => action.tone === 'danger');
  const permissionUnavailable = actionModel?.permission_status === 'unavailable';

  function runAction(action: PoAction) {
    if (action.disabled || !isTransitionAction(action.id)) return;
    if (action.tone === 'danger') {
      setConfirmAction(action);
      return;
    }
    onTransition(action.id);
  }

  function confirmDanger() {
    if (!confirmAction || !isTransitionAction(confirmAction.id)) return;
    onTransition(confirmAction.id);
    setConfirmAction(null);
  }

  return (
    <section className="surface poCommandBar">
      <div className="commandTop">
        <Button variant="secondary" leftIcon={<Icon name="arrow-left" />} onClick={onClose}>Chiudi</Button>
        <div className="commandTitle">
          <span className="eyebrow">Cruscotto PO</span>
          <strong>{po.code ?? `PO ${po.id}`}</strong>
        </div>
        <ModePicker modes={modes} selected={modeID} onChange={onModeChange} />
      </div>

      <div className="commandSummary" aria-label="Sintesi PO">
        <SummaryChip label="Stato" value={stateLabel(summary?.state ?? po.state)} tone="warning" />
        <SummaryChip label="Totale" value={formatMoney(summary?.total_price ?? po.total_price, summary?.currency ?? po.currency)} tone="info" />
        <SummaryChip label="Righe" value={summaryValue(summary?.row_count ?? po.rows?.length, '0')} />
        <SummaryChip
          label="Preventivi"
          value={quoteRequired ? `${quoteCount}/2` : `${attachmentCount}`}
          tone={quoteRequired && quoteCount < 2 ? 'warning' : 'success'}
        />
        <SummaryChip label="Destinatari" value={summaryValue(summary?.recipient_count ?? po.recipients?.length, '0')} />
        <SummaryChip label="Pagamento" value={summary?.payment_method || paymentLabel(po)} />
      </div>

      <div className="commandBottom">
        <div className="commandReason">
          <strong>{mode?.description ?? 'Segui lo stato della richiesta.'}</strong>
          <span>{permissionUnavailable ? 'Le autorizzazioni sono temporaneamente non disponibili.' : mode?.reason ?? 'Nessuna azione richiesta ora.'}</span>
          {quoteRuleBlocked ? <span className="warningText">Carica almeno 2 preventivi prima di inviare.</span> : null}
        </div>

        <div className="commandActions">
          {po.state !== PO_STATES.DRAFT ? (
            <Button variant="secondary" leftIcon={<Icon name="download" />} onClick={onPDF} loading={transitioning}>Scarica PDF</Button>
          ) : null}

          {modeID === 'requester_draft' ? (
            <Tooltip content={!canSubmit || serverSubmitBlocked ? submitBlockedReason : 'La richiesta passa agli approvatori.'}>
              <span>
                <Button leftIcon={<Icon name="arrow-right" />} onClick={onSubmit} loading={busy} disabled={!canSubmit || serverSubmitBlocked}>
                  Manda in approvazione
                </Button>
              </span>
            </Tooltip>
          ) : null}

          {modeID !== 'requester_draft' && modeID !== 'requester_payment_update'
            ? [...secondaryActions, ...primaryActions].map((action) => (
                <ActionButton key={`${action.mode_id}-${action.id}`} action={action} busy={busy} onRun={runAction} />
              ))
            : null}

          {dangerActions.length > 0 ? (
            <span className="commandDangerActions">
              {dangerActions.map((action) => (
                <ActionButton key={`${action.mode_id}-${action.id}`} action={action} busy={busy} onRun={runAction} />
              ))}
            </span>
          ) : null}
        </div>
      </div>

      <ConfirmDialog
        open={confirmAction != null}
        title={confirmAction?.label ?? ''}
        message={confirmAction?.description ? `${confirmAction.description} Confermi di procedere?` : 'Confermi di procedere?'}
        confirmLabel={confirmAction?.label ?? 'Conferma'}
        danger
        loading={transitioning}
        onClose={() => setConfirmAction(null)}
        onConfirm={confirmDanger}
      />
    </section>
  );
}
