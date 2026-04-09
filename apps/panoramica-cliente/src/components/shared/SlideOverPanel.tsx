import { useEffect, useRef, type ReactNode } from 'react';
import styles from './SlideOverPanel.module.css';

interface SlideOverPanelProps {
  open: boolean;
  onClose: () => void;
  width?: number;
  title: ReactNode;
  children: ReactNode;
}

export function SlideOverPanel({ open, onClose, width = 480, title, children }: SlideOverPanelProps) {
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [open, onClose]);

  useEffect(() => {
    if (open && panelRef.current) {
      panelRef.current.focus();
    }
  }, [open]);

  if (!open) return null;

  return (
    <div className={styles.backdrop} onClick={onClose}>
      <div
        ref={panelRef}
        className={styles.panel}
        style={{ width }}
        onClick={e => e.stopPropagation()}
        tabIndex={-1}
      >
        <div className={styles.header}>
          <div className={styles.title}>{title}</div>
          <button className={styles.closeBtn} onClick={onClose} aria-label="Chiudi">
            &#x2715;
          </button>
        </div>
        <div className={styles.content}>{children}</div>
      </div>
    </div>
  );
}
