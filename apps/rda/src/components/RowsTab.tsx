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
        <table className="dataTable">
          <thead>
            <tr>
              <th>Descrizione</th><th>Costo unitario / NRC</th><th>MRC</th><th>Q.ta</th><th>Tipo</th><th>Totale riga</th><th className="actionsCell">Azioni</th>
            </tr>
          </thead>
          <tbody>
            {(po.rows ?? []).map((row) => (
              <tr key={row.id}>
                <td>{row.description ?? row.product_description ?? '-'}</td>
                <td>{formatMoneyEUR(row.type === 'good' ? row.price : row.activation_fee ?? row.activation_price)}</td>
                <td>{formatMoneyEUR(row.montly_fee ?? row.monthly_fee)}</td>
                <td>{row.qty ?? '-'}</td>
                <td>{row.type === 'good' ? 'Bene' : 'Servizio'}</td>
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
            {(po.rows ?? []).length === 0 ? <tr><td colSpan={7} className="emptyInline">Nessuna riga PO presente.</td></tr> : null}
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
