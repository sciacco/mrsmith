import { useState, useMemo, useRef } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { useVirtualizer } from '@tanstack/react-virtual';
import { isUpstreamAuthFailed } from '../../api/errors';
import { useHistory } from '../../api/queries';
import { ExportButtons } from '../../components/ExportButtons';
import styles from './HistoryPage.module.css';

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}/${d.getFullYear()}`;
}

export function HistoryPage() {
  const [searchQuery, setSearchQuery] = useState('');
  const { data: entries, isLoading, error } = useHistory();
  const scrollRef = useRef<HTMLDivElement>(null);

  const filteredEntries = useMemo(() => {
    if (!entries) return [];
    if (!searchQuery) return entries;
    const q = searchQuery.toLowerCase();
    return entries.filter((e) => e.domain.toLowerCase().includes(q));
  }, [entries, searchQuery]);

  const virtualizer = useVirtualizer({
    count: filteredEntries.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 44,
    overscan: 20,
  });

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div>
          <h1 className={styles.pageTitle}>Riepilogo</h1>
          <p className={styles.pageSubtitle}>Cronologia completa delle richieste di blocco e rilascio</p>
        </div>
      </div>

      <div className={styles.searchRow}>
        <div className={styles.searchWrap}>
          <input
            className={styles.searchInput}
            type="text"
            placeholder="Cerca dominio..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
          {searchQuery && (
            <button className={styles.clearBtn} onClick={() => setSearchQuery('')} aria-label="Cancella ricerca">
              ✕
            </button>
          )}
        </div>
        <ExportButtons basePath="/compliance/domains/history" params={{ search: searchQuery }} />
      </div>

      <div className={styles.tableCard}>
        {isLoading ? (
          <div className={styles.tableBody}><Skeleton rows={8} /></div>
        ) : isUpstreamAuthFailed(error) ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Servizio temporaneamente non disponibile</p>
            <p className={styles.emptyText}>La cronologia non puo essere caricata in questo momento.</p>
          </div>
        ) : filteredEntries.length === 0 ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Nessun risultato</p>
            <p className={styles.emptyText}>La cronologia e vuota</p>
          </div>
        ) : (
          <>
            <div className={styles.tableHeader}>
              <span>Dominio</span>
              <span>Data</span>
              <span>Riferimento</span>
              <span>Tipo</span>
            </div>
            <div
              ref={scrollRef}
              className={styles.tableBody}
              style={{ maxHeight: 'calc(100vh - 300px)', overflowY: 'auto' }}
            >
              <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
                {virtualizer.getVirtualItems().map((virtualRow) => {
                  const entry = filteredEntries[virtualRow.index]!;
                  return (
                    <div
                      key={virtualRow.key}
                      className={styles.historyRow}
                      style={{
                        position: 'absolute',
                        top: virtualRow.start,
                        height: virtualRow.size,
                        width: '100%',
                      }}
                    >
                      <span className={styles.domainName}>{entry.domain}</span>
                      <span className={styles.cellText}>{formatDate(entry.request_date)}</span>
                      <span className={styles.cellText}>{entry.reference}</span>
                      <span>
                        <span
                          className={`${styles.typeBadge} ${
                            entry.request_type === 'block' ? styles.typeBadgeBlock : styles.typeBadgeRelease
                          }`}
                        >
                          {entry.request_type === 'block' ? 'Blocco' : 'Rilascio'}
                        </span>
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
