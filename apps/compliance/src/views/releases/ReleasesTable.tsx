import type { ReleaseRequest } from '../../api/types';
import styles from './ReleasesPage.module.css';

interface ReleasesTableProps {
  releases: ReleaseRequest[];
  selectedId: number | null;
  onSelect: (id: number) => void;
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}/${d.getFullYear()}`;
}

export function ReleasesTable({ releases, selectedId, onSelect }: ReleasesTableProps) {
  return (
    <>
      <div className={styles.tableHeader}>
        <span>Data</span>
        <span>Riferimento</span>
        <span />
      </div>
      <div className={styles.tableBody}>
        {releases.map((rel, i) => (
          <div
            key={rel.id}
            className={`${styles.row} ${selectedId === rel.id ? styles.rowSelected : ''}`}
            onClick={() => onSelect(rel.id)}
            style={{ animationDelay: `${i * 50}ms` }}
          >
            <div className={styles.rowAccent} />
            <div className={styles.rowIcon}>
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
                <rect x="2" y="2" width="14" height="14" rx="2" stroke="currentColor" strokeWidth="1.5" />
                <path d="M6 9l2 2 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </div>
            <span className={styles.rowText}>{formatDate(rel.request_date)}</span>
            <span className={styles.rowText}>{rel.reference}</span>
            <svg className={styles.rowChevron} width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M6 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
        ))}
      </div>
    </>
  );
}
