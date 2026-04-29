import { Icon } from '@mrsmith/ui';
import type { PoRow } from '../api/types';
import { formatMoney } from '../lib/format';

export function RowTable({
  rows,
  currency,
  editable,
  emptyLabel,
  onEdit,
  onDelete,
}: {
  rows: PoRow[];
  currency?: string | null;
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
                      ? `Unitario ${formatMoney(row.price, currency)}`
                      : `NRC ${formatMoney(row.activation_fee ?? row.activation_price, currency)} · MRC ${formatMoney(row.montly_fee ?? row.monthly_fee, currency)}`}
                  </small>
                </div>
              </td>
              <td>{row.qty ?? '-'}</td>
              <td>{formatMoney(row.total_price, currency)}</td>
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
