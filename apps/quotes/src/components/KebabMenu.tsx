import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import styles from './KebabMenu.module.css';

interface KebabMenuProps {
  quoteId: number;
  canDelete: boolean;
  onDelete?: () => void;
}

export function KebabMenu({ quoteId, canDelete, onDelete }: KebabMenuProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  const close = useCallback(() => setOpen(false), []);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) close();
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open, close]);

  return (
    <div className={styles.wrap} ref={ref}>
      <button
        className={styles.trigger}
        onClick={e => { e.stopPropagation(); setOpen(!open); }}
        aria-label="Menu azioni"
      >
        &#x22EE;
      </button>
      {open && (
        <div className={styles.menu}>
          <button
            className={styles.menuItem}
            onClick={e => { e.stopPropagation(); navigate(`/quotes/${quoteId}`); }}
          >
            Apri
          </button>
          {canDelete && (
            <button
              className={`${styles.menuItem} ${styles.menuItemDanger}`}
              onClick={e => { e.stopPropagation(); onDelete?.(); close(); }}
            >
              Elimina
            </button>
          )}
          <button
            className={`${styles.menuItem} ${styles.menuItemDisabled}`}
            disabled
            title="Prossimamente"
            onClick={e => e.stopPropagation()}
          >
            Duplica
          </button>
        </div>
      )}
    </div>
  );
}
