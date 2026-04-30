import { Icon, Tooltip } from '@mrsmith/ui';
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
            <th>Prossimo passo</th>
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
            const label = openLabel(po);
            const iconName = po.isOwnDraft ? 'pencil' : po.isActionable ? 'chevron-right' : 'eye';
            const code = po.code ?? `PO ${po.id}`;
            const contexts = po.contexts.slice(0, 2);
            const extraContexts = po.contexts.length - contexts.length;
            const contextTooltip = po.contexts.map((context) => context.label).join(', ');

            return (
              <tr key={po.id}>
                <td>
                  <div className="requestCell">
                    <span className="requestCode">{code}</span>
                    <strong>{requestTitle(po)}</strong>
                    {po.project && po.project !== po.object ? <small>{po.project}</small> : null}
                  </div>
                </td>
                <td>
                  <div className="nextStepCell">
                    <strong>{po.nextStepLabel}</strong>
                    <Tooltip content={contextTooltip}>
                      <span className="queuePills" aria-label={`Code: ${contextTooltip}`}>
                        {contexts.map((context) => (
                          <span className="queuePill" key={context.key}>{context.label}</span>
                        ))}
                        {extraContexts > 0 ? <span className="queuePill mutedPill">+{extraContexts}</span> : null}
                      </span>
                    </Tooltip>
                  </div>
                </td>
                <td><StateBadge state={po.state} /></td>
                <td>
                  <span className="textCell">{po.provider?.company_name ?? '-'}</span>
                </td>
                <td>
                  <span className="textCell">{requesterLabel(po)}</span>
                </td>
                <td className="dateCell">{rowDate(po)}</td>
                <td className="moneyCell">{formatMoney(po.total_price, po.currency)}</td>
                <td className="actionsCell">
                  <span className="iconActions">
                    <Tooltip content={label}>
                      <button
                        className="iconButton"
                        type="button"
                        aria-label={label}
                        title={label}
                        onClick={() => navigate(`/rda/po/${po.id}`)}
                      >
                        <Icon name={iconName} size={16} />
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
