import { Icon, Tooltip } from '@mrsmith/ui';
import { useNavigate } from 'react-router-dom';
import type { PoPreview } from '../api/types';
import { extractApproverList, formatDateIT, formatMoneyEUR } from '../lib/format';
import { StateBadge } from './StateBadge';

interface PoListTableProps {
  rows: PoPreview[];
  mode: 'requester' | 'inbox';
  currentEmail?: string | null;
  onDelete?: (po: PoPreview) => void;
}

function canEdit(po: PoPreview, currentEmail?: string | null): boolean {
  return Boolean(po.state === 'DRAFT' && po.requester?.email && currentEmail && po.requester.email.toLowerCase() === currentEmail.toLowerCase());
}

function rowDate(po: PoPreview): string {
  return formatDateIT(po.created ?? po.creation_date ?? po.updated);
}

export function PoListTable({ rows, mode, currentEmail, onDelete }: PoListTableProps) {
  const navigate = useNavigate();

  if (rows.length === 0) {
    return (
      <div className="stateBlock">
        <div>
          <p className="stateTitle">Nessuna richiesta trovata.</p>
          <p className="muted">Non ci sono elementi da mostrare.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="tableScroll">
      <table className="dataTable">
        <thead>
          <tr>
            <th className="actionsCell">{mode === 'requester' ? 'Azioni' : 'Gestisci'}</th>
            <th>Stato</th>
            {mode === 'requester' ? <th>Approvatori</th> : null}
            <th>Richiedente</th>
            <th>Data creazione</th>
            <th>Numero PO</th>
            <th>Fornitore</th>
            <th>Progetto</th>
            <th>Prezzo totale</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((po) => {
            const editable = canEdit(po, currentEmail);
            const path = `/rda/po/${po.id}`;
            return (
              <tr key={po.id}>
                <td className="actionsCell">
                  <span className="iconActions">
                    {mode === 'requester' ? (
                      <>
                        <Tooltip content={editable ? 'Modifica' : 'Modifica disponibile solo in bozza'}>
                          <button
                            className="iconButton"
                            type="button"
                            aria-label="Modifica richiesta"
                            title="Modifica richiesta"
                            disabled={!editable}
                            onClick={() => navigate(path)}
                          >
                            <Icon name="pencil" size={16} />
                          </button>
                        </Tooltip>
                        <Tooltip content={editable ? 'Elimina' : 'Elimina disponibile solo in bozza'}>
                          <button
                            className="iconButton dangerButton"
                            type="button"
                            aria-label="Elimina richiesta"
                            title="Elimina richiesta"
                            disabled={!editable}
                            onClick={() => onDelete?.(po)}
                          >
                            <Icon name="trash" size={16} />
                          </button>
                        </Tooltip>
                        <Tooltip content={po.state === 'DRAFT' ? 'Apri bozza' : 'Visualizza'}>
                          <button
                            className="iconButton"
                            type="button"
                            aria-label="Visualizza richiesta"
                            title="Visualizza richiesta"
                            onClick={() => navigate(path)}
                          >
                            <Icon name="eye" size={16} />
                          </button>
                        </Tooltip>
                      </>
                    ) : (
                      <Tooltip content="Gestisci">
                        <button
                          className="iconButton"
                          type="button"
                          aria-label="Gestisci richiesta"
                          title="Gestisci richiesta"
                          onClick={() => navigate(path)}
                        >
                          <Icon name="chevron-right" size={16} />
                        </button>
                      </Tooltip>
                    )}
                  </span>
                </td>
                <td><StateBadge state={po.state} /></td>
                {mode === 'requester' ? <td>{extractApproverList(po.approvers)}</td> : null}
                <td>{po.requester?.email ?? '-'}</td>
                <td>{rowDate(po)}</td>
                <td>{po.code ?? po.id}</td>
                <td>{po.provider?.company_name ?? '-'}</td>
                <td>{po.project ?? '-'}</td>
                <td>{formatMoneyEUR(po.total_price)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
