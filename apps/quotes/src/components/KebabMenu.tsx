import { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import { Icon } from '@mrsmith/ui';
import styles from './KebabMenu.module.css';

interface KebabMenuProps {
  quoteId: number;
  canDelete: boolean;
  onDelete?: () => void;
  deleteDisabled?: boolean;
  deleteLabel?: string;
}

export function KebabMenu({ quoteId, canDelete, onDelete, deleteDisabled = false, deleteLabel = 'Elimina' }: KebabMenuProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  const close = useCallback(() => setOpen(false), []);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (
        ref.current && !ref.current.contains(e.target as Node) &&
        menuRef.current && !menuRef.current.contains(e.target as Node)
      ) close();
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open, close]);

  // Close on scroll so the menu doesn't drift
  useEffect(() => {
    if (!open) return;
    const onScroll = () => close();
    window.addEventListener('scroll', onScroll, true);
    return () => window.removeEventListener('scroll', onScroll, true);
  }, [open, close]);

  const handleToggle = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (open) {
      setOpen(false);
      return;
    }
    if (triggerRef.current) {
      const rect = triggerRef.current.getBoundingClientRect();
      setPos({ top: rect.bottom + 4, left: rect.right });
    }
    setOpen(true);
  }, [open]);

  const menu = open && pos && createPortal(
    <div
      ref={menuRef}
      className={styles.menu}
      style={{ top: pos.top, left: pos.left }}
    >
      <button
        className={styles.menuItem}
        onClick={e => { e.stopPropagation(); navigate(`/quotes/${quoteId}`); }}
      >
        Apri
      </button>
      {canDelete && (
        <button
          className={`${styles.menuItem} ${styles.menuItemDanger}`}
          disabled={deleteDisabled}
          title={deleteDisabled ? 'Attendi il completamento della cancellazione corrente.' : undefined}
          onClick={e => { e.stopPropagation(); onDelete?.(); close(); }}
        >
          {deleteLabel}
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
    </div>,
    document.body,
  );

  return (
    <div className={styles.wrap} ref={ref}>
      <button
        ref={triggerRef}
        className={styles.trigger}
        onClick={handleToggle}
        aria-label="Menu azioni"
      >
        <Icon name="more-vertical" size={18} />
      </button>
      {menu}
    </div>
  );
}
