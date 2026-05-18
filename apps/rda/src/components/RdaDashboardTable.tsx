import { Icon, Tooltip, type IconName } from '@mrsmith/ui';
import { useNavigate } from 'react-router-dom';
import {
  defaultRdaDashboardSortDirection,
  rdaDashboardApproverSummary,
  rdaDashboardRequesterLabel,
  rdaDashboardRequestTitle,
  type RdaDashboardRow,
  type RdaDashboardSort,
  type RdaDashboardSortKey,
} from '../lib/rda-dashboard';
import { formatDateIT, formatMoney } from '../lib/format';
import { StateBadge } from './StateBadge';

interface RdaDashboardTableProps {
  rows: RdaDashboardRow[];
  sort?: RdaDashboardSort | null;
  onSortChange?: (key: RdaDashboardSortKey) => void;
  onDelete?: (po: RdaDashboardRow) => void;
}

function rowDate(po: RdaDashboardRow): string {
  return formatDateIT(po.created ?? po.creation_date ?? po.updated);
}

function requesterDisplayLabel(po: RdaDashboardRow): string {
  return po.requester?.email?.trim() || rdaDashboardRequesterLabel(po);
}

function approverTooltipContent(summary: ReturnType<typeof rdaDashboardApproverSummary>) {
  if (!summary) return null;

  return (
    <div className="approverTooltip">
      <strong>Elenco approvatori</strong>
      <div className="approverTooltipRows">
        {summary.groups.map((group) => (
          <div className="approverTooltipRow" key={group.label}>
            <span>{group.label}</span>
            <div>
              {group.emails.map((email) => (
                <span key={`${group.label}-${email}`}>{email}</span>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
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

  if (po.isActionable) {
    switch (po.state) {
      case 'PENDING_SEND':
        return { iconName: 'mail', label: actionLabel(po, 'Invia al fornitore') };
      case 'PENDING_VERIFICATION':
        return { iconName: 'clipboard-check', label: actionLabel(po, 'Verifica fornitura') };
      default:
        break;
    }

    return { iconName: 'clipboard-check', label: openLabel(po) };
  }

  return { iconName: 'eye', label: openLabel(po) };
}

function isEcommercePO(po: RdaDashboardRow): boolean {
  return po.type === 'ECOMMERCE';
}

function ariaSort(sort: RdaDashboardSort | null | undefined, key: RdaDashboardSortKey): 'ascending' | 'descending' | 'none' {
  if (sort?.key !== key) return 'none';
  return sort.direction === 'asc' ? 'ascending' : 'descending';
}

function nextSortLabel(sort: RdaDashboardSort | null | undefined, key: RdaDashboardSortKey): string {
  const nextDirection = sort?.key === key
    ? (sort.direction === 'asc' ? 'desc' : 'asc')
    : defaultRdaDashboardSortDirection(key);
  return nextDirection === 'asc' ? 'Ordina crescente' : 'Ordina decrescente';
}

function SortableHeader({
  label,
  sortKey,
  sort,
  onSortChange,
  className,
}: {
  label: string;
  sortKey: RdaDashboardSortKey;
  sort?: RdaDashboardSort | null;
  onSortChange?: (key: RdaDashboardSortKey) => void;
  className?: string;
}) {
  const active = sort?.key === sortKey;
  const direction = active ? sort.direction : defaultRdaDashboardSortDirection(sortKey);
  const labelText = `${label}: ${nextSortLabel(sort, sortKey)}`;

  return (
    <th className={className} aria-sort={ariaSort(sort, sortKey)}>
      <button
        className={`rdaSortHeaderButton ${active ? 'active' : ''}`}
        type="button"
        aria-label={labelText}
        title={labelText}
        onClick={() => onSortChange?.(sortKey)}
      >
        <span>{label}</span>
        <span className="rdaSortIndicator" aria-hidden="true">
          <Icon name={direction === 'asc' ? 'chevron-up' : 'chevron-down'} size={13} strokeWidth={2.2} />
        </span>
      </button>
    </th>
  );
}

export function RdaDashboardTable({ rows, sort, onSortChange, onDelete }: RdaDashboardTableProps) {
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
            <SortableHeader label="Richiesta" sortKey="request" sort={sort} onSortChange={onSortChange} />
            <SortableHeader label="Stato" sortKey="state" sort={sort} onSortChange={onSortChange} />
            <SortableHeader label="Fornitore" sortKey="provider" sort={sort} onSortChange={onSortChange} />
            <SortableHeader label="Richiedente / Approvatori" sortKey="requester" sort={sort} onSortChange={onSortChange} />
            <SortableHeader label="Creata" sortKey="created" sort={sort} onSortChange={onSortChange} />
            <SortableHeader label="Totale" sortKey="total" sort={sort} onSortChange={onSortChange} className="moneyCell" />
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
            const requesterLabel = requesterDisplayLabel(po);
            const approverSummary = rdaDashboardApproverSummary(po);

            return (
              <tr key={po.id} onDoubleClick={() => navigate(`/rda/po/${po.id}`)}>
                <td>
                  <div className="requestCell">
                    <span className="requestCodeLine">
                      <span className="requestCode">{code}</span>
                      {isEcommercePO(po) ? (
                        <Tooltip content="PO e-commerce">
                          <span className="requestCodeIcon" aria-label="PO e-commerce">
                            <Icon name="shopping-cart" size={14} strokeWidth={2} />
                          </span>
                        </Tooltip>
                      ) : null}
                    </span>
                    <strong>{rdaDashboardRequestTitle(po)}</strong>
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
                  <div className="peopleCell">
                    <span className="peoplePrimary" aria-label={`Richiedente: ${requesterLabel}`} title={requesterLabel}>
                      {requesterLabel}
                    </span>
                    {approverSummary ? (
                      <Tooltip content={approverTooltipContent(approverSummary)} maxWidth={360}>
                        <span className="peopleSecondary" aria-label={approverSummary.ariaLabel}>
                          {approverSummary.visible}
                        </span>
                      </Tooltip>
                    ) : null}
                  </div>
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
                    {po.canDelete ? (
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
