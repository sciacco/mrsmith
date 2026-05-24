import { Button, Icon, SearchInput, SingleSelect, Skeleton } from '@mrsmith/ui';
import { useDeferredValue, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useOrders } from '../api/queries';
import type { OrderSummary } from '../api/types';
import { OrdersTable, type OrderSortKey, type SortDirection } from '../components/OrdersTable';
import { formatTipoDoc, formatTipoProposta, orderCode } from '../lib/formatters';
import { apiErrorMessage } from '../lib/errors';
import styles from './OrderListPage.module.css';

const PAGE_SIZE = 50;

export function OrderListPage() {
  const navigate = useNavigate();
  const orders = useOrders();
  const [search, setSearch] = useState('');
  const [stateFilter, setStateFilter] = useState<string | null>(null);
  const [docFilter, setDocFilter] = useState<string | null>(null);
  const [proposalFilter, setProposalFilter] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<OrderSortKey>('id');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const [page, setPage] = useState(1);
  const deferredSearch = useDeferredValue(search);

  const raw = orders.data ?? [];
  const stateOptions = useMemo(() => toOptions(raw.map((order) => order.cdlan_stato)), [raw]);
  const docOptions = useMemo(() => toOptions(raw.map((order) => order.cdlan_tipodoc), formatTipoDoc), [raw]);
  const proposalOptions = useMemo(() => toOptions(raw.map((order) => order.cdlan_tipo_ord), formatTipoProposta), [raw]);

  const filtered = useMemo(() => {
    const needle = deferredSearch.trim().toLowerCase();
    return raw.filter((order) => {
      if (stateFilter && order.cdlan_stato !== stateFilter) return false;
      if (docFilter && order.cdlan_tipodoc !== docFilter) return false;
      if (proposalFilter && order.cdlan_tipo_ord !== proposalFilter) return false;
      if (!needle) return true;
      return [
        orderCode(order.cdlan_ndoc, order.cdlan_anno),
        order.cdlan_cliente,
        order.cdlan_sost_ord,
        order.cdlan_systemodv,
        order.service_type,
      ]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(needle));
    });
  }, [deferredSearch, docFilter, proposalFilter, raw, stateFilter]);

  const sorted = useMemo(() => sortOrders(filtered, sortKey, sortDirection), [filtered, sortDirection, sortKey]);
  const totalPages = Math.max(1, Math.ceil(sorted.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const pageRows = sorted.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);
  const hasFilters = Boolean(search || stateFilter || docFilter || proposalFilter);

  function updateSort(key: OrderSortKey) {
    if (sortKey === key) {
      setSortDirection((direction) => (direction === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDirection(key === 'id' || key === 'date' ? 'desc' : 'asc');
    }
  }

  function clearFilters() {
    setSearch('');
    setStateFilter(null);
    setDocFilter(null);
    setProposalFilter(null);
    setPage(1);
  }

  return (
    <main className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1>Ordini</h1>
        </div>
        <Button variant="secondary" leftIcon={<Icon name="loader" />} loading={orders.isFetching && !orders.isLoading} onClick={() => void orders.refetch()}>
          Aggiorna
        </Button>
      </header>

      <section className={styles.surface}>
        <div className={styles.toolbar}>
          <SearchInput value={search} onChange={(value) => { setSearch(value); setPage(1); }} placeholder="Cerca ordine, cliente, ODV" className={styles.search} />
          <div className={styles.selectWrap}>
            <SingleSelect options={stateOptions} selected={stateFilter} onChange={(value) => { setStateFilter(value); setPage(1); }} placeholder="Stato" allowClear clearLabel="Tutti gli stati" />
          </div>
          <div className={styles.selectWrap}>
            <SingleSelect options={docOptions} selected={docFilter} onChange={(value) => { setDocFilter(value); setPage(1); }} placeholder="Tipo documento" allowClear clearLabel="Tutti i documenti" />
          </div>
          <div className={styles.selectWrap}>
            <SingleSelect options={proposalOptions} selected={proposalFilter} onChange={(value) => { setProposalFilter(value); setPage(1); }} placeholder="Tipo proposta" allowClear clearLabel="Tutte le proposte" />
          </div>
          {hasFilters ? <button type="button" className={styles.resetButton} onClick={clearFilters}>Cancella filtri</button> : null}
        </div>

        {orders.isLoading ? (
          <div className={styles.stateBox}><Skeleton rows={10} /></div>
        ) : orders.error ? (
          <div className={styles.stateBox}>
            <div className={styles.errorIcon}><Icon name="triangle-alert" size={28} /></div>
            <strong>{apiErrorMessage(orders.error, 'Ordini non disponibili.')}</strong>
            <p>Riprova più tardi o riapri l'app dal portale.</p>
          </div>
        ) : sorted.length === 0 ? (
          <div className={styles.stateBox}>
            <div className={styles.emptyIcon}><Icon name="package" size={30} /></div>
            <strong>{hasFilters ? 'Nessun ordine corrisponde ai filtri' : 'Nessun ordine disponibile'}</strong>
            <p>{hasFilters ? 'Modifica la ricerca o cancella i filtri.' : 'Gli ordini verranno mostrati qui quando disponibili.'}</p>
          </div>
        ) : (
          <>
            <div className={styles.tableMeta}>{sorted.length} ordini</div>
            <OrdersTable rows={pageRows} sortKey={sortKey} sortDirection={sortDirection} onSort={updateSort} onOpen={(id) => navigate(`/ordini/${id}`)} />
            <div className={styles.pager}>
              <span>Pagina {safePage} di {totalPages}</span>
              <Button variant="secondary" size="sm" disabled={safePage <= 1} onClick={() => setPage((current) => Math.max(1, current - 1))}>Precedente</Button>
              <Button variant="secondary" size="sm" disabled={safePage >= totalPages} onClick={() => setPage((current) => Math.min(totalPages, current + 1))}>Successivo</Button>
            </div>
          </>
        )}
      </section>
    </main>
  );
}

function toOptions(values: Array<string | null | undefined>, formatter?: (value: string | null | undefined) => string) {
  return Array.from(new Set(values.filter((value): value is string => Boolean(value))))
    .sort((a, b) => (formatter ? formatter(a).localeCompare(formatter(b)) : a.localeCompare(b)))
    .map((value) => ({ value, label: formatter ? formatter(value) : value }));
}

function sortOrders(rows: OrderSummary[], key: OrderSortKey, direction: SortDirection): OrderSummary[] {
  const multiplier = direction === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => compareOrder(a, b, key) * multiplier);
}

function compareOrder(a: OrderSummary, b: OrderSummary, key: OrderSortKey): number {
  switch (key) {
    case 'id':
      return a.id - b.id;
    case 'code':
      return orderCode(a.cdlan_ndoc, a.cdlan_anno).localeCompare(orderCode(b.cdlan_ndoc, b.cdlan_anno));
    case 'customer':
      return (a.cdlan_cliente ?? '').localeCompare(b.cdlan_cliente ?? '');
    case 'date':
      return dateValue(a.cdlan_datadoc) - dateValue(b.cdlan_datadoc);
    case 'state':
      return (a.cdlan_stato ?? '').localeCompare(b.cdlan_stato ?? '');
  }
}

function dateValue(value: string | null): number {
  if (!value) return 0;
  const parsed = new Date(value).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
}
