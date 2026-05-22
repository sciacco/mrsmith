import { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Icon } from '@mrsmith/ui';
import styles from './PlanKebabMenu.module.css';

interface PlanKebabMenuProps {
  canDelete: boolean;
  onEdit: () => void;
  onHistory: () => void;
  onDelete: () => void;
}

export function PlanKebabMenu({ canDelete, onEdit, onHistory, onDelete }: PlanKebabMenuProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const close = useCallback(() => setOpen(false), []);

  useEffect(() => {
    if (!open) return;
    const onClick = (event: MouseEvent) => {
      const target = event.target as Node;
      if (!rootRef.current?.contains(target) && !menuRef.current?.contains(target)) {
        close();
      }
    };
    document.addEventListener('mousedown', onClick);
    return () => document.removeEventListener('mousedown', onClick);
  }, [open, close]);

  useEffect(() => {
    if (!open) return;
    const onScroll = () => close();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') close();
    };
    window.addEventListener('scroll', onScroll, true);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('keydown', onKey);
    };
  }, [open, close]);

  function toggle(event: React.MouseEvent) {
    event.stopPropagation();
    if (open) {
      setOpen(false);
      return;
    }
    const rect = triggerRef.current?.getBoundingClientRect();
    if (rect) setPos({ top: rect.bottom + 4, left: rect.right });
    setOpen(true);
  }

  function invoke(action: () => void) {
    action();
    close();
  }

  const menu = open && pos ? createPortal(
    <div ref={menuRef} className={styles.menu} style={{ top: pos.top, left: pos.left }}>
      <button className={styles.menuItem} onClick={() => invoke(onEdit)}>
        Modifica piano
      </button>
      <button className={styles.menuItem} onClick={() => invoke(onHistory)}>
        Storico
      </button>
      <button
        className={`${styles.menuItem} ${styles.menuItemDanger}`}
        disabled={!canDelete}
        title={canDelete ? undefined : 'Solo bozze senza iscrizioni'}
        onClick={() => invoke(onDelete)}
      >
        Elimina piano
      </button>
    </div>,
    document.body,
  ) : null;

  return (
    <div className={styles.wrap} ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        className={styles.trigger}
        onClick={toggle}
        aria-label="Azioni piano"
        aria-expanded={open}
      >
        <Icon name="more-vertical" size={18} />
      </button>
      {menu}
    </div>
  );
}
