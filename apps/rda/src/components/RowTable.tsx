import { Icon } from '@mrsmith/ui';
import type { PoRow } from '../api/types';
import { formatMoneyEUR } from '../lib/format';

export function RowTable({
  rows,
  editable,
  emptyLabel,
  onEdit,
  onDelete,
}: {
  rows: PoRow[];
  editable: boolean;
  emptyLabel: string;
  onEdit: (row: PoRow) => void;
  onDelete: (row: PoRow) => void;
}) {
  return (
    <div className="tableScroll">
      <table className="dataTable rowTable">
        <thead>
          <tr>
            <th>Riga</th>
            <th>Economia</th>
            <th>Q.ta</th>
            <th>Totale riga</th>
            <th className="actionsCell">Azioni</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id}>
              <td>
                <div className="rowTitleCell">
                  <strong>{row.description ?? row.product_description ?? '-'}</strong>
                  <span>{row.product_code ?? row.product_description ?? '-'}</span>
                </div>
              </td>
              <td>
                <div className="economicBreakdown">
                  <span className={`badge ${row.type === 'good' ? 'success' : 'info'}`}>{row.type === 'good' ? 'Bene' : 'Servizio'}</span>
                  <small>
                    {row.type === 'good'
                      ? `Unitario ${formatMoneyEUR(row.price)}`
                      : `NRC ${formatMoneyEUR(row.activation_fee ?? row.activation_price)} · MRC ${formatMoneyEUR(row.montly_fee ?? row.monthly_fee)}`}
                  </small>
                </div>
              </td>
              <td>{row.qty ?? '-'}</td>
              <td>{formatMoneyEUR(row.total_price)}</td>
              <td className="actionsCell">
                <span className="iconActions">
                  <button
                    className="iconButton"
                    type="button"
                    aria-label={editable ? 'Modifica riga' : 'Modifica riga non disponibile'}
                    title={editable ? 'Modifica' : 'Modifica disponibile solo in bozza'}
                    disabled={!editable}
                    onClick={() => onEdit(row)}
                  >
                    <Icon name="pencil" size={16} />
                  </button>
                  <button
                    className="iconButton dangerButton"
                    type="button"
                    aria-label="Elimina riga"
                    title="Elimina"
                    disabled={!editable}
                    onClick={() => onDelete(row)}
                  >
                    <Icon name="trash" size={16} />
                  </button>
                </span>
              </td>
            </tr>
          ))}
          {rows.length === 0 ? (
            <tr>
              <td colSpan={5} className="emptyInline">
                {emptyLabel}
              </td>
            </tr>
          ) : null}
        </tbody>
      </table>
    </div>
  );
}
