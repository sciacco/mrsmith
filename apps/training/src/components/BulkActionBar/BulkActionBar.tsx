import { useEffect, type ReactNode } from 'react';
import { Button } from '@mrsmith/ui';
import styles from './BulkActionBar.module.css';

interface BulkActionBarProps {
  selectedCount: number;
  onClear: () => void;
  children?: ReactNode;
  summary?: ReactNode;
}

export function BulkActionBar({ selectedCount, onClear, children, summary }: BulkActionBarProps) {
  useEffect(() => {
    if (selectedCount === 0) return;
    function handler(event: KeyboardEvent) {
      if (event.key === 'Escape') onClear();
    }
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [selectedCount, onClear]);

  if (selectedCount === 0) return null;

  return (
    <div className={styles.bar} role="region" aria-label="Azioni multiple">
      <div className={styles.left}>
        <span className={styles.count}>{selectedCount} selezionati</span>
        {summary && <span className={styles.summary}>{summary}</span>}
      </div>
      <div className={styles.actions}>
        {children}
        <Button variant="ghost" size="sm" onClick={onClear}>
          Annulla selezione
        </Button>
      </div>
    </div>
  );
}
