import { useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { BlockRequest } from '../../api/types';
import styles from './BlocksPage.module.css';

interface BlocksTableProps {
  blocks: BlockRequest[];
  selectedId: number | null;
  onSelect: (id: number) => void;
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}/${d.getFullYear()}`;
}

export function BlocksTable({ blocks, selectedId, onSelect }: BlocksTableProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: blocks.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 52,
    overscan: 10,
  });

  return (
    <>
      <div className={styles.tableHeader}>
        <span>Data</span>
        <span>Provenienza</span>
        <span>Riferimento</span>
        <span />
      </div>
      <div
        ref={scrollRef}
        className={styles.tableBody}
        style={{ maxHeight: 'calc(100vh - 300px)', overflowY: 'auto' }}
      >
        <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const block = blocks[virtualRow.index]!;
            return (
              <div
                key={block.id}
                className={`${styles.row} ${selectedId === block.id ? styles.rowSelected : ''}`}
                onClick={() => onSelect(block.id)}
                style={{
                  position: 'absolute',
                  top: virtualRow.start,
                  height: virtualRow.size,
                  width: '100%',
                }}
              >
                <div className={styles.rowAccent} />
                <div className={styles.rowIcon}>
                  <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
                    <rect x="2" y="2" width="14" height="14" rx="2" stroke="currentColor" strokeWidth="1.5" />
                    <path d="M6 6l6 6M12 6l-6 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                  </svg>
                </div>
                <span className={styles.rowText}>{formatDate(block.request_date)}</span>
                <span className={styles.rowText}>{block.method_description}</span>
                <span className={styles.rowText}>{block.reference}</span>
                <svg className={styles.rowChevron} width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                  <path d="M6 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}
