import { Icon } from '@mrsmith/ui';
import type { PoRow } from '../api/types';
import { formatMoney } from '../lib/format';

function parseContractualNumber(value: unknown): number | null {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null;
  }
  if (typeof value !== 'string') return null;

  const normalized = value.trim().replace(',', '.');
  if (!normalized) return null;
  const parsed = Number(normalized);
  return Number.isFinite(parsed) ? parsed : null;
}

function formatContractualNumber(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    useGrouping: false,
    maximumFractionDigits: 20,
  }).format(value);
}

function contractualSummary(row: PoRow): string | null {
  if (row.type !== 'service') return null;

  const nrc = parseContractualNumber(row.activation_fee ?? row.activation_price);
  const mrc = parseContractualNumber(row.montly_fee ?? row.monthly_fee);
  if (mrc === 0 && nrc !== null && nrc > 0) return 'Servizio una tantum';

  const fragments: string[] = [];
  const duration = parseContractualNumber(row.renew_detail?.initial_subscription_months);
  if (duration !== null && duration > 0) {
    fragments.push(`Durata ${formatContractualNumber(duration)} mesi`);
  }

  const nextDuration = parseContractualNumber(row.renew_detail?.next_subscription_months);
  if (nextDuration !== null && nextDuration > 0) {
    fragments.push(`Rinnovo ${formatContractualNumber(nextDuration)} mesi`);
  }

  const recurrence = parseContractualNumber(row.payment_detail?.month_recursion);
  if (recurrence !== null && recurrence > 0) {
    const monthLabel = recurrence === 1 ? 'mese' : 'mesi';
    fragments.push(`Ricorrenza ogni ${formatContractualNumber(recurrence)} ${monthLabel}`);
  }

  const automaticRenew = row.renew_detail?.automatic_renew;
  if (typeof automaticRenew === 'boolean') {
    fragments.push(automaticRenew ? 'Rinnovo automatico' : 'Senza rinnovo automatico');
    if (automaticRenew) {
      const cancellationAdvice = parseContractualNumber(row.renew_detail?.cancellation_advice);
      if (cancellationAdvice !== null && cancellationAdvice > 0) {
        fragments.push(`Preavviso disdetta ${formatContractualNumber(cancellationAdvice)} gg`);
      }
    }
  }

  return fragments.length > 0 ? fragments.join(' · ') : null;
}

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
              <td data-label="Riga">
                <div className="rowTitleCell">
                  <strong>{row.description ?? row.product_description ?? '-'}</strong>
                  <span>{row.product_code ?? row.product_description ?? '-'}</span>
                </div>
              </td>
              <td data-label="Economia">
                <div className="economicBreakdown">
                  <span className={`badge ${row.type === 'good' ? 'success' : 'info'}`}>{row.type === 'good' ? 'Bene' : 'Servizio'}</span>
                  <small>
                    {row.type === 'good'
                      ? `Unitario ${formatMoney(row.price, currency)}`
                      : `NRC ${formatMoney(row.activation_fee ?? row.activation_price, currency)} · MRC ${formatMoney(row.montly_fee ?? row.monthly_fee, currency)}`}
                  </small>
                  {(() => {
                    const summary = contractualSummary(row);
                    return summary ? <span className="contractualSummary">{summary}</span> : null;
                  })()}
                </div>
              </td>
              <td data-label="Q.ta">{row.qty ?? '-'}</td>
              <td data-label="Totale riga">{formatMoney(row.total_price, currency)}</td>
              <td data-label="Azioni" className="actionsCell">
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
