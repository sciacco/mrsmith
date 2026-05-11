import { Icon, Tooltip, type IconName } from '@mrsmith/ui';
import { useNavigate } from 'react-router-dom';
import type { RdaDashboardRow } from '../lib/rda-dashboard';
import { formatDateIT, formatMoney } from '../lib/format';
import { StateBadge } from './StateBadge';

interface RdaDashboardTableProps {
  rows: RdaDashboardRow[];
  onDelete?: (po: RdaDashboardRow) => void;
}

function rowDate(po: RdaDashboardRow): string {
  return formatDateIT(po.created ?? po.creation_date ?? po.updated);
}

function requesterLabel(po: RdaDashboardRow): string {
  const requester = po.requester;
  const name = [requester?.first_name, requester?.last_name].filter(Boolean).join(' ');
  if (requester?.name) return requester.name;
  if (name) return name;
  return requester?.email ?? '-';
}

function requestTitle(po: RdaDashboardRow): string {
  return po.object || po.project || 'Richiesta senza oggetto';
}

function openLabel(po: RdaDashboardRow): string {
  if (po.isOwnDraft) return 'Modifica richiesta';
  if (po.isActionable) return 'Gestisci richiesta';
  return 'Visualizza richiesta';
}

interface RowAction {
  iconName: IconName;
  label: string;
}

function actionLabel(po: RdaDashboardRow, fallback: string): string {
  return po.actionLabel || fallback;
}

function rowAction(po: RdaDashboardRow): RowAction {
  if (po.isOwnDraft) {
    return { iconName: 'pencil', label: actionLabel(po, 'Modifica bozza') };
  }

  switch (po.primaryQueue.key) {
    case 'level1-2':
      return { iconName: 'file-check', label: actionLabel(po, 'Valuta approvazione') };
    case 'leasing':
      return { iconName: 'landmark', label: actionLabel(po, 'Valuta leasing') };
    case 'no-leasing':
      return { iconName: 'git-branch', label: actionLabel(po, 'Valuta no leasing') };
    case 'payment-method':
      return { iconName: 'credit-card', label: actionLabel(po, 'Conferma metodo pagamento') };
    case 'budget-increment':
      return { iconName: 'circle-dollar-sign', label: actionLabel(po, 'Valuta incremento budget') };
    default:
      break;
  }

  switch (po.state) {
    case 'PENDING_SEND':
      return { iconName: 'mail', label: actionLabel(po, 'Invia al fornitore') };
    case 'PENDING_VERIFICATION':
      return { iconName: 'clipboard-check', label: actionLabel(po, 'Verifica fornitura') };
    default:
      break;
  }

  if (po.isActionable) {
    return { iconName: 'clipboard-check', label: openLabel(po) };
  }

  return { iconName: 'eye', label: openLabel(po) };
}

export function RdaDashboardTable({ rows, onDelete }: RdaDashboardTableProps) {
  const navigate = useNavigate();

  if (rows.length === 0) {
    return (
      <div className="stateBlock">
        <div>
          <p className="stateTitle">Nessuna richiesta trovata.</p>
          <p className="muted">Non ci sono richieste che corrispondono ai filtri selezionati.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="tableScroll">
      <table className="dataTable rdaDashboardTable">
        <thead>
          <tr>
            <th>Richiesta</th>
            <th>Stato</th>
            <th>Fornitore</th>
            <th>Richiedente</th>
            <th>Creata</th>
            <th className="moneyCell">Totale</th>
            <th className="actionsCell">Azione</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((po) => {
            const action = rowAction(po);
            const code = po.code ?? `PO ${po.id}`;
            const visibleContexts = po.contexts.filter((context) => context.type !== 'visibility');
            const contexts = visibleContexts.slice(0, 2);
            const extraContexts = visibleContexts.length - contexts.length;
            const contextTooltip = visibleContexts.map((context) => context.label).join(', ');
            const hasContexts = contexts.length > 0;

            return (
              <tr key={po.id} onDoubleClick={() => navigate(`/rda/po/${po.id}`)}>
                <td>
                  <div className="requestCell">
                    <span className="requestCode">{code}</span>
                    <strong>{requestTitle(po)}</strong>
                    {po.project && po.project !== po.object ? <small>{po.project}</small> : null}
                  </div>
                </td>
                <td>
                  <div className="statusCell">
                    <StateBadge state={po.state} />
                    {hasContexts ? (
                      <Tooltip content={contextTooltip}>
                        <span className="queuePills" aria-label={`Aree: ${contextTooltip}`}>
                          {contexts.map((context) => (
                            <span className="queuePill" key={context.key}>{context.label}</span>
                          ))}
                          {extraContexts > 0 ? <span className="queuePill mutedPill">+{extraContexts}</span> : null}
                        </span>
                      </Tooltip>
                    ) : null}
                  </div>
                </td>
                <td>
                  <span className="textCell">{po.provider?.company_name ?? '-'}</span>
                </td>
                <td>
                  <span className="textCell">{requesterLabel(po)}</span>
                </td>
                <td className="dateCell">{rowDate(po)}</td>
                <td className="moneyCell">{formatMoney(po.total_price, po.currency)}</td>
                <td className="actionsCell" onDoubleClick={(event) => event.stopPropagation()}>
                  <span className="iconActions">
                    <Tooltip content={action.label}>
                      <button
                        className="iconButton"
                        type="button"
                        aria-label={action.label}
                        title={action.label}
                        onClick={() => navigate(`/rda/po/${po.id}`)}
                      >
                        <Icon name={action.iconName} size={16} />
                      </button>
                    </Tooltip>
                    {po.isOwnDraft ? (
                      <Tooltip content="Elimina richiesta">
                        <button
                          className="iconButton dangerButton"
                          type="button"
                          aria-label="Elimina richiesta"
                          title="Elimina richiesta"
                          onClick={() => onDelete?.(po)}
                        >
                          <Icon name="trash" size={16} />
                        </button>
                      </Tooltip>
                    ) : null}
                  </span>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
