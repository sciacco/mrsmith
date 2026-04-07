import { useState, type ReactNode } from 'react';
import styles from './TableToolbar.module.css';

interface TableToolbarProps {
  children: ReactNode;
  filters?: ReactNode;
  activeFilterCount?: number;
  className?: string;
}

export function TableToolbar({ children, filters, activeFilterCount = 0, className }: TableToolbarProps) {
  const [filtersOpen, setFiltersOpen] = useState(false);

  return (
    <div className={`${styles.wrapper} ${filtersOpen ? styles.wrapperOpen : ''} ${className ?? ''}`}>
      <div className={styles.toolbar}>
        {children}
        {filters && (
          <button
            type="button"
            className={`${styles.filterToggle} ${filtersOpen ? styles.filterToggleOpen : ''}`}
            onClick={() => setFiltersOpen(!filtersOpen)}
            aria-expanded={filtersOpen}
            aria-label="Mostra filtri"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M2 3h12M4 7h8M6 11h4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
            {activeFilterCount > 0 && (
              <span className={styles.filterBadge}>{activeFilterCount}</span>
            )}
          </button>
        )}
      </div>
      {filtersOpen && filters && (
        <div className={styles.filterRow}>{filters}</div>
      )}
    </div>
  );
}
