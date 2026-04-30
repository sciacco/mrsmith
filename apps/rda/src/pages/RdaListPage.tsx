import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useDeferredValue, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useDeletePO, useInbox, useMyPOs, usePermissions } from '../api/queries';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { RdaDashboardTable } from '../components/RdaDashboardTable';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { authorizedInboxKinds, type InboxKind } from '../lib/inbox';
import {
  buildRdaDashboardModel,
  filterRdaDashboardRows,
  parseRdaDashboardView,
  rdaQueueFilterOptions,
  rdaStateFilterOptions,
  type RdaDashboardRow,
  type RdaDashboardView,
} from '../lib/rda-dashboard';

function errorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status === 403) return 'Accesso non consentito.';
  if (error instanceof ApiError && error.status === 401) return 'Accesso richiesto.';
  return 'La coda RDA non e disponibile in questo momento.';
}

const viewLabels: Record<RdaDashboardView, string> = {
  todo: 'Da fare',
  mine: 'Le mie',
  all: 'Tutte',
};

export function RdaListPage() {
  const [deleteTarget, setDeleteTarget] = useState<RdaDashboardRow | null>(null);
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const { user } = useOptionalAuth();
  const { toast } = useToast();

  const permissions = usePermissions();
  const myPOs = useMyPOs();
  const deletePO = useDeletePO();
  const levelInbox = useInbox('level1-2', Boolean(permissions.data?.is_approver));
  const leasingInbox = useInbox('leasing', Boolean(permissions.data?.is_afc));
  const noLeasingInbox = useInbox('no-leasing', Boolean(permissions.data?.is_approver_no_leasing));
  const paymentInbox = useInbox('payment-method', Boolean(permissions.data?.is_afc));
  const budgetInbox = useInbox('budget-increment', Boolean(permissions.data?.is_approver_extra_budget));

  const view = parseRdaDashboardView(params.get('view'));
  const qFilter = params.get('q') ?? '';
  const stateFilter = params.get('state') ?? '';
  const queueFilter = params.get('queue') ?? '';
  const deferredQ = useDeferredValue(qFilter);

  const inboxQueries = useMemo(
    () => [
      { kind: 'level1-2' as const, query: levelInbox },
      { kind: 'leasing' as const, query: leasingInbox },
      { kind: 'no-leasing' as const, query: noLeasingInbox },
      { kind: 'payment-method' as const, query: paymentInbox },
      { kind: 'budget-increment' as const, query: budgetInbox },
    ],
    [budgetInbox, leasingInbox, levelInbox, noLeasingInbox, paymentInbox],
  );

  const authorizedKinds = useMemo(() => authorizedInboxKinds(permissions.data), [permissions.data]);
  const activeInboxQueries = inboxQueries.filter((item) => authorizedKinds.includes(item.kind));
  const loading = permissions.isLoading || myPOs.isLoading || activeInboxQueries.some((item) => item.query.isLoading);
  const error = permissions.error ?? myPOs.error ?? activeInboxQueries.find((item) => item.query.error)?.query.error;
  const refreshing = permissions.isFetching || myPOs.isFetching || activeInboxQueries.some((item) => item.query.isFetching);

  const dashboard = useMemo(() => {
    const byKind = new Map<InboxKind, typeof levelInbox>();
    for (const item of inboxQueries) byKind.set(item.kind, item.query);
    return buildRdaDashboardModel({
      myRows: myPOs.data ?? [],
      currentEmail: user?.email,
      permissions: permissions.data,
      inboxes: authorizedKinds.map((kind) => ({ kind, rows: byKind.get(kind)?.data ?? [] })),
    });
  }, [authorizedKinds, inboxQueries, myPOs.data, permissions.data, user?.email]);

  const visibleRows = useMemo(
    () =>
      filterRdaDashboardRows(dashboard.rows, {
        view,
        q: deferredQ,
        state: stateFilter,
        queue: queueFilter,
      }),
    [dashboard.rows, deferredQ, queueFilter, stateFilter, view],
  );

  const viewCounts = useMemo(
    () => ({
      todo: filterRdaDashboardRows(dashboard.rows, { view: 'todo' }).length,
      mine: filterRdaDashboardRows(dashboard.rows, { view: 'mine' }).length,
      all: dashboard.counts.totalAccessible,
    }),
    [dashboard.counts.totalAccessible, dashboard.rows],
  );

  const stateOptions = useMemo(
    () => rdaStateFilterOptions(dashboard.rows, view).map((option) => ({
      value: option.value,
      label: `${option.label} (${option.count})`,
    })),
    [dashboard.rows, view],
  );
  const queueOptions = useMemo(
    () => rdaQueueFilterOptions(dashboard.rows, view).map((option) => ({
      value: option.value,
      label: `${option.label} (${option.count})`,
    })),
    [dashboard.rows, view],
  );

  const hasFilters = qFilter !== '' || stateFilter !== '' || queueFilter !== '';

  function updateParam(key: 'q' | 'state' | 'queue', value: string) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (value) next.set(key, value);
      else next.delete(key);
      return next;
    });
  }

  function updateView(nextView: RdaDashboardView) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      next.set('view', nextView);
      return next;
    });
  }

  function clearFilters() {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      next.delete('q');
      next.delete('state');
      next.delete('queue');
      return next;
    });
  }

  function refreshDashboard() {
    void permissions.refetch();
    void myPOs.refetch();
    for (const item of activeInboxQueries) void item.query.refetch();
  }

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
          <h1>Cruscotto RDA</h1>
          <p>Gestisci le richieste in carico e segui le tue RDA aperte.</p>
        </div>
        <div className="headerActions">
          <Button
            variant="secondary"
            leftIcon={<Icon name="loader" />}
            loading={refreshing && !loading}
            onClick={refreshDashboard}
          >
            Aggiorna
          </Button>
          <Button leftIcon={<Icon name="plus" />} onClick={() => navigate('/rda/new')}>
            Nuova richiesta
          </Button>
        </div>
      </header>

      <section className="rdaCountStrip" aria-label="Riepilogo code RDA">
        <div>
          <strong>{dashboard.counts.toManage}</strong>
          <span>RDA da gestire</span>
        </div>
        <div>
          <strong>{dashboard.counts.ownDrafts}</strong>
          <span>Bozze proprie</span>
        </div>
        <div>
          <strong>{dashboard.counts.ownOpen}</strong>
          <span>Richieste proprie aperte</span>
        </div>
        <div>
          <strong>{dashboard.counts.totalAccessible}</strong>
          <span>Totale accessibile</span>
        </div>
      </section>

      <section className="surface rdaWorkspace">
        <div className="rdaWorkspaceHeader">
          <div className="rdaViewTabs" role="tablist" aria-label="Viste RDA">
            {(['todo', 'mine', 'all'] as RdaDashboardView[]).map((item) => (
              <button
                key={item}
                type="button"
                role="tab"
                aria-selected={view === item}
                className={`rdaViewTab ${view === item ? 'active' : ''}`}
                onClick={() => updateView(item)}
              >
                <span>{viewLabels[item]}</span>
                <strong>{viewCounts[item]}</strong>
              </button>
            ))}
          </div>
          <span className="rdaResultCount">{visibleRows.length} richieste</span>
        </div>

        <div className="rdaFilterBar">
          <SearchInput
            className="rdaFilterSearch"
            value={qFilter}
            onChange={(value) => updateParam('q', value)}
            placeholder="Cerca per PO, fornitore, progetto o richiedente..."
          />
          <div className="rdaFilterSelect">
            <SingleSelect
              options={stateOptions}
              selected={stateFilter || null}
              onChange={(value) => updateParam('state', value ?? '')}
              placeholder="Tutti gli stati"
              allowClear
              clearLabel="Tutti gli stati"
            />
          </div>
          <div className="rdaFilterSelect">
            <SingleSelect
              options={queueOptions}
              selected={queueFilter || null}
              onChange={(value) => updateParam('queue', value ?? '')}
              placeholder="Tutte le code"
              allowClear
              clearLabel="Tutte le code"
            />
          </div>
          {hasFilters ? (
            <button className="filterReset" type="button" onClick={clearFilters}>
              Cancella filtri
            </button>
          ) : null}
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
          <RdaDashboardTable rows={visibleRows} onDelete={setDeleteTarget} />
        )}
      </section>

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
