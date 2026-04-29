import { Button, Icon, useToast } from '@mrsmith/ui';
import { useState } from 'react';
import { useDeleteRow } from '../api/queries';
import type { PoDetail, PoRow } from '../api/types';
import { formatMoneyEUR } from '../lib/format';
import { ConfirmDialog } from './ConfirmDialog';
import { RowModal } from './RowModal';

export function RowsTab({ po, editable }: { po: PoDetail; editable: boolean }) {
  const [modalOpen, setModalOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PoRow | null>(null);
  const remove = useDeleteRow();
  const { toast } = useToast();

  async function confirmDelete() {
    if (!deleteTarget) return;
    try {
      await remove.mutateAsync({ id: po.id, rowId: deleteTarget.id });
      toast('Riga eliminata');
      setDeleteTarget(null);
    } catch {
      toast('Eliminazione non riuscita', 'error');
    }
  }

  return (
    <div className="stack">
      <div className="surfaceHeader">
        <h2>Totale PO: {formatMoneyEUR(po.total_price)}</h2>
        <Button size="sm" leftIcon={<Icon name="plus" />} disabled={!editable} onClick={() => setModalOpen(true)}>Nuova riga</Button>
      </div>
      <div className="tableScroll">
        <table className="dataTable rowTable">
          <thead>
            <tr>
              <th>Riga</th><th>Economia</th><th>Q.ta</th><th>Totale riga</th><th className="actionsCell">Azioni</th>
            </tr>
          </thead>
          <tbody>
            {(po.rows ?? []).map((row) => (
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
                    <small>{row.type === 'good' ? `Unitario ${formatMoneyEUR(row.price)}` : `NRC ${formatMoneyEUR(row.activation_fee ?? row.activation_price)} · MRC ${formatMoneyEUR(row.montly_fee ?? row.monthly_fee)}`}</small>
                  </div>
                </td>
                <td>{row.qty ?? '-'}</td>
                <td>{formatMoneyEUR(row.total_price)}</td>
                <td className="actionsCell">
                  <span className="iconActions">
                    <button className="iconButton" type="button" aria-label="Modifica riga non disponibile" title="Elimina e ricrea la riga per modificarla." disabled>
                      <Icon name="pencil" size={16} />
                    </button>
                    <button
                      className="iconButton dangerButton"
                      type="button"
                      aria-label="Elimina riga"
                      title="Elimina"
                      disabled={!editable}
                      onClick={() => setDeleteTarget(row)}
                    >
                      <Icon name="trash" size={16} />
                    </button>
                  </span>
                </td>
              </tr>
            ))}
            {(po.rows ?? []).length === 0 ? <tr><td colSpan={5} className="emptyInline">Nessuna riga PO presente.</td></tr> : null}
          </tbody>
        </table>
      </div>
      <RowModal poId={po.id} open={modalOpen} onClose={() => setModalOpen(false)} />
      <ConfirmDialog
        open={deleteTarget != null}
        title="Elimina riga"
        message="Confermi eliminazione della riga selezionata?"
        confirmLabel="Elimina"
        danger
        loading={remove.isPending}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
