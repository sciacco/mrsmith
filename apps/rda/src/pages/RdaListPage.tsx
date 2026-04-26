import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useBudgets, useDeletePO, useMyPOs, usePaymentMethodDefault, usePaymentMethods, useProviders } from '../api/queries';
import type { PoPreview } from '../api/types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { NewPoModal } from '../components/NewPoModal';
import { PoListTable } from '../components/PoListTable';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

function errorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status === 403) return 'Accesso non consentito.';
  return 'Le richieste non sono disponibili in questo momento.';
}

export function RdaListPage() {
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PoPreview | null>(null);
  const pos = useMyPOs();
  const budgets = useBudgets();
  const providers = useProviders();
  const methods = usePaymentMethods();
  const paymentDefault = usePaymentMethodDefault();
  const deletePO = useDeletePO();
  const { user } = useOptionalAuth();
  const { toast } = useToast();

  const loading = pos.isLoading || budgets.isLoading || providers.isLoading || methods.isLoading || paymentDefault.isLoading;
  const error = pos.error ?? budgets.error ?? providers.error ?? methods.error ?? paymentDefault.error;

  async function confirmDelete() {
    if (!deleteTarget) return;
    try {
      await deletePO.mutateAsync(deleteTarget.id);
      toast('Bozza eliminata');
      setDeleteTarget(null);
    } catch {
      toast('Eliminazione non riuscita', 'error');
    }
  }

  return (
    <main className="rdaPage">
      <header className="pageHeader">
        <div>
          <h1>Richieste di acquisto</h1>
          <p>Consulta le tue richieste e crea nuove bozze.</p>
        </div>
        <Button leftIcon={<Icon name="plus" />} onClick={() => setCreateOpen(true)}>
          Nuova richiesta
        </Button>
      </header>

      <section className="surface">
        <div className="surfaceHeader">
          <h2>Le mie RDA</h2>
        </div>
        {loading ? (
          <div className="stateCard"><Skeleton rows={8} /></div>
        ) : error ? (
          <div className="stateBlock">
            <div>
              <p className="stateTitle">{errorMessage(error)}</p>
              <p className="muted">Riprova piu tardi.</p>
            </div>
          </div>
        ) : (
          <PoListTable rows={pos.data ?? []} mode="requester" currentEmail={user?.email} onDelete={setDeleteTarget} />
        )}
      </section>

      <NewPoModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        budgets={budgets.data ?? []}
        providers={providers.data ?? []}
        methods={methods.data ?? []}
        cdlanDefault={paymentDefault.data?.code ?? ''}
      />

      <ConfirmDialog
        open={deleteTarget != null}
        title="Elimina bozza"
        message="Confermi eliminazione della bozza selezionata?"
        confirmLabel="Elimina"
        danger
        loading={deletePO.isPending}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void confirmDelete()}
      />
    </main>
  );
}
