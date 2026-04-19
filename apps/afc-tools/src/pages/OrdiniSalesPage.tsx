import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Icon,
  SearchInput,
  SingleSelect,
  Skeleton,
  StatusBadge,
  TableToolbar,
  Tooltip,
  useTableFilter,
} from '@mrsmith/ui';
import { useNavigate } from 'react-router-dom';
import { useSalesOrders } from '../api/queries';
import { usePagedData } from '../hooks/usePagedData';
import type { SalesOrderSummary } from '../types';
import { formatDate } from '../utils/format';
import shared from './shared.module.css';
import styles from './OrdiniSalesPage.module.css';

type SortKey = 'codice' | 'cliente' | 'data';
type SortDir = 'asc' | 'desc';

const PAGE_SIZE = 50;

function compareStr(a: string | null, b: string | null): number {
  const av = a ?? '';
  const bv = b ?? '';
  return av.localeCompare(bv);
}

function compareDate(a: string | null, b: string | null): number {
  const av = a ? new Date(a).getTime() : NaN;
  const bv = b ? new Date(b).getTime() : NaN;
  const aNull = Number.isNaN(av);
  const bNull = Number.isNaN(bv);
  if (aNull && bNull) return 0;
  if (aNull) return 1;
  if (bNull) return -1;
  return av - bv;
}

export default function OrdiniSalesPage() {
  const q = useSalesOrders();
  const navigate = useNavigate();

  const [search, setSearch] = useState('');
  const [stato, setStato] = useState<string | null>(null);
  const [tipo, setTipo] = useState<string | null>(null);
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [sortKey, setSortKey] = useState<SortKey>('data');
  const [sortDir, setSortDir] = useState<SortDir>('desc');

  const rawData = q.data ?? [];

  const data = useMemo(() => {
    if (!dateFrom && !dateTo) return rawData;
    const fromT = dateFrom ? new Date(dateFrom).getTime() : null;
    const toT = dateTo ? new Date(dateTo).getTime() + 24 * 60 * 60 * 1000 - 1 : null;
    return rawData.filter((o) => {
      if (!o.cdlan_datadoc) return false;
      const t = new Date(o.cdlan_datadoc).getTime();
      if (fromT !== null && t < fromT) return false;
      if (toT !== null && t > toT) return false;
      return true;
    });
  }, [rawData, dateFrom, dateTo]);

  const statoOptions = useMemo(() => {
    const set = new Set<string>();
    rawData.forEach((o) => o.cdlan_stato && set.add(o.cdlan_stato));
    return Array.from(set).sort().map((v) => ({ value: v, label: v }));
  }, [rawData]);

  const tipoOptions = useMemo(() => {
    const set = new Set<string>();
    rawData.forEach((o) => o.tipo_di_servizi && set.add(o.tipo_di_servizi));
    return Array.from(set).sort().map((v) => ({ value: v, label: v }));
  }, [rawData]);

  const { filtered } = useTableFilter<SalesOrderSummary>({
    data,
    searchQuery: search,
    searchFields: ['codice_ordine', 'cdlan_cliente', 'cdlan_sost_ord'],
    filters: {
      stato: { field: 'cdlan_stato', value: stato },
      tipo: { field: 'tipo_di_servizi', value: tipo },
    },
  });

  const sorted = useMemo(() => {
    const arr = [...filtered];
    const dir = sortDir === 'asc' ? 1 : -1;
    arr.sort((a, b) => {
      switch (sortKey) {
        case 'codice':
          return compareStr(a.codice_ordine, b.codice_ordine) * dir;
        case 'cliente':
          return compareStr(a.cdlan_cliente, b.cdlan_cliente) * dir;
        case 'data':
          return compareDate(a.cdlan_datadoc, b.cdlan_datadoc) * dir;
      }
    });
    return arr;
  }, [filtered, sortKey, sortDir]);

  const { page, setPage, totalPages, pageData, rangeLabel } = usePagedData(sorted, PAGE_SIZE);

  const activeFilterCount =
    (stato ? 1 : 0) + (tipo ? 1 : 0) + (dateFrom || dateTo ? 1 : 0);
  const hasActiveFilters = Boolean(search) || activeFilterCount > 0;

  const [focusedIdx, setFocusedIdx] = useState(0);
  const rowRefs = useRef<(HTMLTableRowElement | null)[]>([]);

  useEffect(() => {
    setFocusedIdx(0);
  }, [page, sortKey, sortDir, search, stato, tipo, dateFrom, dateTo]);

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir(key === 'data' ? 'desc' : 'asc');
    }
  }

  function handleRowKey(e: React.KeyboardEvent<HTMLTableRowElement>, i: number, id: number) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      navigate(`/ordini-sales/${id}`);
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      const next = Math.min(i + 1, pageData.length - 1);
      setFocusedIdx(next);
      rowRefs.current[next]?.focus();
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      const prev = Math.max(i - 1, 0);
      setFocusedIdx(prev);
      rowRefs.current[prev]?.focus();
    }
  }

  function sortHeader(key: SortKey, label: string) {
    const active = sortKey === key;
    const arrow = active ? (sortDir === 'asc' ? '▲' : '▼') : '↕';
    return (
      <button
        type="button"
        className={`${styles.sortBtn} ${active ? styles.sortBtnActive : ''}`}
        onClick={() => toggleSort(key)}
      >
        {label}
        <span className={styles.sortArrow}>{arrow}</span>
      </button>
    );
  }

  function ariaSortFor(key: SortKey): 'ascending' | 'descending' | 'none' {
    if (sortKey !== key) return 'none';
    return sortDir === 'asc' ? 'ascending' : 'descending';
  }

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Ordini Sales</h1>

      <TableToolbar
        activeFilterCount={activeFilterCount}
        filters={
          <div className={styles.dateRange}>
            <label className={styles.dateLabel}>
              Dal
              <input
                type="date"
                value={dateFrom}
                onChange={(e) => {
                  setDateFrom(e.target.value);
                  setPage(1);
                }}
                className={styles.dateInput}
              />
            </label>
            <label className={styles.dateLabel}>
              Al
              <input
                type="date"
                value={dateTo}
                onChange={(e) => {
                  setDateTo(e.target.value);
                  setPage(1);
                }}
                className={styles.dateInput}
              />
            </label>
            {(dateFrom || dateTo) && (
              <button
                type="button"
                className={shared.btnLink}
                onClick={() => {
                  setDateFrom('');
                  setDateTo('');
                  setPage(1);
                }}
              >
                Azzera date
              </button>
            )}
          </div>
        }
      >
        <div className={styles.search}>
          <SearchInput
            value={search}
            onChange={(v) => {
              setSearch(v);
              setPage(1);
            }}
            placeholder="Cerca per codice, cliente o sostituisce…"
          />
        </div>
        <div className={styles.selectWrap}>
          <SingleSelect
            options={statoOptions}
            selected={stato}
            onChange={(v) => {
              setStato(v as string | null);
              setPage(1);
            }}
            placeholder="Stato"
            allowClear
          />
        </div>
        <div className={styles.selectWrap}>
          <SingleSelect
            options={tipoOptions}
            selected={tipo}
            onChange={(v) => {
              setTipo(v as string | null);
              setPage(1);
            }}
            placeholder="Tipo servizi"
            allowClear
          />
        </div>
      </TableToolbar>

      {q.isLoading && <Skeleton rows={10} />}
      {q.isError && <div className={shared.error}>Errore nel caricamento degli ordini.</div>}

      {q.data && sorted.length > 0 && (
        <>
          <div className={shared.tableWrap}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th aria-sort={ariaSortFor('codice')}>{sortHeader('codice', 'Codice ordine')}</th>
                  <th aria-sort={ariaSortFor('cliente')}>{sortHeader('cliente', 'Cliente')}</th>
                  <th>Stato</th>
                  <th aria-sort={ariaSortFor('data')}>{sortHeader('data', 'Data documento')}</th>
                  <th className={styles.colHideMd}>Tipo servizi</th>
                  <th className={styles.colHideMd}>Data conferma</th>
                </tr>
              </thead>
              <tbody>
                {pageData.map((o, i) => (
                  <tr
                    key={o.id}
                    ref={(el) => {
                      rowRefs.current[i] = el;
                    }}
                    className={styles.row}
                    tabIndex={i === focusedIdx ? 0 : -1}
                    role="link"
                    onClick={() => navigate(`/ordini-sales/${o.id}`)}
                    onKeyDown={(e) => handleRowKey(e, i, o.id)}
                    onFocus={() => setFocusedIdx(i)}
                    style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}
                  >
                    <td>
                      <span className={`${styles.codeCell} ${shared.mono}`}>
                        {o.codice_ordine ?? ''}
                        {o.dal_cp === 'Sì' && (
                          <Tooltip content="Creato dal Customer Portal" placement="top">
                            <span className={styles.cpIcon} aria-label="Dal Customer Portal">
                              <Icon name="link" size={12} />
                            </span>
                          </Tooltip>
                        )}
                      </span>
                    </td>
                    <td>
                      <span className={styles.clientCell} title={o.cdlan_cliente ?? ''}>
                        {o.cdlan_cliente ?? ''}
                      </span>
                    </td>
                    <td>
                      {o.cdlan_stato && <StatusBadge value={o.cdlan_stato} />}
                    </td>
                    <td>{formatDate(o.cdlan_datadoc)}</td>
                    <td className={styles.colHideMd}>{o.tipo_di_servizi ?? ''}</td>
                    <td className={styles.colHideMd}>{formatDate(o.cdlan_dataconferma)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className={styles.pager}>
            <span className={styles.pagerLabel}>{rangeLabel}</span>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setPage(page - 1)}
              disabled={page <= 1}
            >
              Precedente
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setPage(page + 1)}
              disabled={page >= totalPages}
            >
              Successivo
            </Button>
          </div>
          <div className={styles.visuallyHidden} aria-live="polite" aria-atomic="true">
            Pagina {page} di {totalPages}
          </div>
        </>
      )}

      {q.data && sorted.length === 0 && (
        <div className={styles.emptyState}>
          <div className={styles.emptyIcon}>
            <Icon name="package" size={28} />
          </div>
          <div className={styles.emptyTitle}>
            {hasActiveFilters ? 'Nessun ordine corrisponde ai filtri' : 'Nessun ordine attivo o inviato'}
          </div>
          <div className={styles.emptyDesc}>
            {hasActiveFilters
              ? 'Modifica i filtri o svuota la ricerca per vedere altri risultati.'
              : 'Gli ordini con stato ATTIVO o INVIATO verranno mostrati qui.'}
          </div>
        </div>
      )}
    </div>
  );
}
