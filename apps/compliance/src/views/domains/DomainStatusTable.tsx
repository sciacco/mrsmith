import { useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { DomainStatus } from '../../api/types';
import styles from './DomainsPage.module.css';

interface DomainStatusTableProps {
  domains: DomainStatus[];
}

export function DomainStatusTable({ domains }: DomainStatusTableProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: domains.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 44,
    overscan: 20,
  });

  if (domains.length === 0) {
    return (
      <div className={styles.emptyState}>
        <p className={styles.emptyTitle}>Nessun dominio trovato</p>
      </div>
    );
  }

  return (
    <>
      <div className={styles.tableHeader}>
        <span>Dominio</span>
        <span>Blocchi</span>
        <span>Rilasci</span>
      </div>
      <div
        ref={scrollRef}
        className={styles.tableBody}
        style={{ maxHeight: 'calc(100vh - 350px)', overflowY: 'auto' }}
      >
        <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const domain = domains[virtualRow.index]!;
            return (
              <div
                key={virtualRow.key}
                className={styles.domainRow}
                style={{
                  position: 'absolute',
                  top: virtualRow.start,
                  height: virtualRow.size,
                  width: '100%',
                }}
              >
                <span className={styles.domainName}>{domain.domain}</span>
                <span><span className={styles.badge}>{domain.block_count}</span></span>
                <span><span className={styles.badge}>{domain.release_count}</span></span>
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}
