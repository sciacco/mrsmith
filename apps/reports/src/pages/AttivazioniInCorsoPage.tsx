import { Fragment, useMemo, useState } from 'react';
import { Skeleton, Drawer, Icon, SearchInput } from '@mrsmith/ui';
import { usePendingActivations, usePendingActivationRows } from '../api/queries';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';
import styles from './AttivazioniInCorsoPage.module.css';
import type { PendingActivation } from '../types';

type SortKey = keyof PendingActivation;
type SortDir = 'asc' | 'desc';

function SortIcon({ active, dir }: { active: boolean; dir: SortDir }) {
  return (
    <svg
      className={`${styles.sortIcon} ${active ? styles.sortIconActive : ''}`}
      width="10"
      height="10"
      viewBox="0 0 10 10"
      fill="none"
      aria-hidden="true"
    >
      {dir === 'asc' ? (
        <path d="M5 2l4 5H1l4-5Z" fill="currentColor" />
      ) : (
        <path d="M5 8L1 3h8L5 8Z" fill="currentColor" />
      )}
    </svg>
  );
}

function SortableTh({
  label,
  sortKey,
  sort,
  onSort,
}: {
  label: string;
  sortKey: SortKey;
  sort: { key: SortKey; dir: SortDir };
  onSort: (key: SortKey) => void;
}) {
  const active = sort.key === sortKey;
  const nextDir = active && sort.dir === 'asc' ? 'desc' : 'asc';
  return (
    <th
      onClick={() => onSort(nextDir)}
      className={styles.sortableHeader}
      aria-sort={active ? (sort.dir === 'asc' ? 'ascending' : 'descending') : 'none'}
    >
      {label}
      <SortIcon active={active} dir={sort.dir} />
    </th>
  );
}

function formatDurationMonths(value: string | null | undefined): string {
  const normalized = value?.trim();
  if (!normalized) {
    return '—';
  }

  return /[a-z]/i.test(normalized) ? normalized : `${normalized}m`;
}

export default function AttivazioniInCorsoPage() {
  const [selectedOrder, setSelectedOrder] = useState<string | null>(null);
  const [noteOpen, setNoteOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [sort, setSort] = useState<{ key: SortKey; dir: SortDir }>({
    key: 'ragione_sociale',
    dir: 'asc',
  });

  const activationsQ = usePendingActivations();
  const rowsQ = usePendingActivationRows(selectedOrder);

  const displayData = useMemo(() => {
    const q = search.trim().toLowerCase();
    let rows = activationsQ.data ?? [];

    if (q) {
      rows = rows.filter(
        (r) =>
          r.ragione_sociale.toLowerCase().includes(q) ||
          r.numero_ordine.toLowerCase().includes(q),
      );
    }

    return [...rows].sort((a, b) => {
      const av = a[sort.key] ?? '';
      const bv = b[sort.key] ?? '';
      const cmp = String(av).localeCompare(String(bv), 'it', { numeric: true });
      return sort.dir === 'asc' ? cmp : -cmp;
    });
  }, [activationsQ.data, search, sort]);

  const selectedSummary = useMemo(
    () => activationsQ.data?.find((r) => r.numero_ordine === selectedOrder),
    [activationsQ.data, selectedOrder],
  );

  const totalMrc = useMemo(
    () => rowsQ.data?.reduce((acc, r) => acc + (r.totale_mrc ?? 0), 0) ?? 0,
    [rowsQ.data],
  );

  const orderNote = useMemo(
    () => rowsQ.data?.find((r) => r.note_legali)?.note_legali ?? null,
    [rowsQ.data],
  );

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Attivazioni in corso</h1>
      <p className={styles.subtitle}>
        Elenco ordini in stato confermato con righe da attivare. Seleziona una riga per i dettagli
      </p>

      {activationsQ.isLoading && <Skeleton rows={8} />}

      {activationsQ.error && (
        <p>Errore nel caricamento dei dati.</p>
      )}

      {activationsQ.data && (
        <>
          <div className={styles.toolbar}>
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder="Cerca per cliente o numero ordine…"
              className={styles.searchInput}
            />
          </div>
          <div className={shared.info}>{displayData.length} ordini</div>
          <div className={shared.tableWrap}>
            <table className={`${shared.table} ${styles.table}`}>
              <thead>
                <tr>
                  <th></th>
                  <SortableTh label="Cliente" sortKey="ragione_sociale" sort={sort} onSort={(k) => setSort((p) => (p.key === k && p.dir === 'asc' ? { key: k, dir: 'desc' } : { key: k, dir: 'asc' }))} />
                  <SortableTh label="N. Ordine" sortKey="numero_ordine" sort={sort} onSort={(k) => setSort((p) => (p.key === k && p.dir === 'asc' ? { key: k, dir: 'desc' } : { key: k, dir: 'asc' }))} />
                  <SortableTh label="Data documento" sortKey="data_documento" sort={sort} onSort={(k) => setSort((p) => (p.key === k && p.dir === 'asc' ? { key: k, dir: 'desc' } : { key: k, dir: 'asc' }))} />
                  <SortableTh label="Durata servizio" sortKey="durata_servizio" sort={sort} onSort={(k) => setSort((p) => (p.key === k && p.dir === 'asc' ? { key: k, dir: 'desc' } : { key: k, dir: 'asc' }))} />
                  <SortableTh label="Durata rinnovo" sortKey="durata_rinnovo" sort={sort} onSort={(k) => setSort((p) => (p.key === k && p.dir === 'asc' ? { key: k, dir: 'desc' } : { key: k, dir: 'asc' }))} />
                  <SortableTh label="Sost. ord." sortKey="sost_ord" sort={sort} onSort={(k) => setSort((p) => (p.key === k && p.dir === 'asc' ? { key: k, dir: 'desc' } : { key: k, dir: 'asc' }))} />
                  <SortableTh label="Sostituito da" sortKey="sostituito_da" sort={sort} onSort={(k) => setSort((p) => (p.key === k && p.dir === 'asc' ? { key: k, dir: 'desc' } : { key: k, dir: 'asc' }))} />
                  <SortableTh label="Storico" sortKey="storico" sort={sort} onSort={(k) => setSort((p) => (p.key === k && p.dir === 'asc' ? { key: k, dir: 'desc' } : { key: k, dir: 'asc' }))} />
                </tr>
              </thead>
              <tbody>
                {displayData.map((row, i) => (
                  <tr
                    key={row.numero_ordine}
                    className={selectedOrder === row.numero_ordine ? styles.selectedRow : undefined}
                    onClick={() => setSelectedOrder(
                      selectedOrder === row.numero_ordine ? null : row.numero_ordine,
                    )}
                    style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
                  >
                    <td><div className={styles.accentBar} /></td>
                    <td>{row.ragione_sociale}</td>
                    <td className={shared.mono}>{row.numero_ordine}</td>
                    <td>{row.data_documento?.slice(0, 10) ?? ''}</td>
                    <td>{row.durata_servizio ?? ''}</td>
                    <td>{row.durata_rinnovo ?? ''}</td>
                    <td>{row.sost_ord ?? ''}</td>
                    <td>{row.sostituito_da ?? ''}</td>
                    <td>{row.storico ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      <Drawer
        open={!!selectedOrder}
        onClose={() => {
          setSelectedOrder(null);
          setNoteOpen(false);
        }}
        size="xl"
        title={selectedSummary?.ragione_sociale ?? 'Dettaglio ordine'}
        subtitle={
          selectedSummary && (
            <div className={styles.drawerAggregates}>
              <span className={shared.mono}>{selectedSummary.numero_ordine}</span>
              <span className={styles.sep}>·</span>
              <span>{selectedSummary.data_documento?.slice(0, 10) ?? ''}</span>
              <span className={styles.sep}>·</span>
              <span>
                Durata Iniziale: {formatDurationMonths(selectedSummary.durata_servizio)}
                {' / '}
                Rinnovi {formatDurationMonths(selectedSummary.durata_rinnovo)}
              </span>
              <span className={styles.sep}>·</span>
              <span>
                Totale MRC <strong>{formatMoneyEUR(totalMrc)}</strong>
              </span>
              {rowsQ.data && (
                <>
                  <span className={styles.sep}>·</span>
                  <span>{rowsQ.data.length} righe</span>
                </>
              )}
              {orderNote && (
                <button
                  type="button"
                  className={`${styles.noteToggle} ${noteOpen ? styles.noteToggleActive : ''}`}
                  onClick={() => setNoteOpen((v) => !v)}
                  title="Note legali"
                >
                  <Icon name="file-text" size={16} />
                  <span>Note</span>
                </button>
              )}
            </div>
          )
        }
      >
        {orderNote && noteOpen && (
          <div
            className={styles.notePanel}
            dangerouslySetInnerHTML={{ __html: orderNote }}
          />
        )}

        {rowsQ.isLoading && <Skeleton rows={4} />}

        {rowsQ.error && <p>Errore nel caricamento delle righe.</p>}

        {rowsQ.data && (
          <div className={shared.tableWrap}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th className={shared.numCol}>Quantita</th>
                  <th className={shared.numCol}>NRC</th>
                  <th className={shared.numCol}>MRC</th>
                  <th className={shared.numCol}>Totale MRC</th>
                  <th>Stato riga</th>
                  <th>Serial number</th>
                </tr>
              </thead>
              <tbody>
                {rowsQ.data.map((row, i) => (
                  <Fragment key={i}>
                    <tr
                      className={styles.detailRow}
                      style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
                    >
                      <td className={shared.numCol}>{row.quantita ?? ''}</td>
                      <td className={shared.numCol}>{row.nrc != null ? formatMoneyEUR(row.nrc) : ''}</td>
                      <td className={shared.numCol}>{row.mrc != null ? formatMoneyEUR(row.mrc) : ''}</td>
                      <td className={shared.numCol}>{row.totale_mrc != null ? formatMoneyEUR(row.totale_mrc) : ''}</td>
                      <td>{row.stato_riga ?? ''}</td>
                      <td className={shared.mono}>{row.serialnumber ?? ''}</td>
                    </tr>
                    <tr className={styles.descRow}>
                      <td colSpan={6} className={styles.descCell}>
                        {row.descrizione_long ? (
                          <span dangerouslySetInnerHTML={{ __html: row.descrizione_long }} />
                        ) : (
                          '—'
                        )}
                      </td>
                    </tr>
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Drawer>
    </div>
  );
}
