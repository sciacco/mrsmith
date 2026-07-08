import type { ReactNode } from 'react';
import styles from './SpineNav.module.css';

export interface SpineItem {
  id: string;
  index: number;
  title: string;
  meta?: ReactNode;
}

interface SpineNavProps {
  items: SpineItem[];
  activeId?: string;
  onNavigate: (id: string) => void;
}

export function SpineNav({ items, activeId, onNavigate }: SpineNavProps) {
  return (
    <nav className={styles.spine} aria-label="Navigazione della scheda" data-scheda-sticky="spine">
      {items.map((item) => {
        const active = item.id === activeId;
        return (
          <button
            key={item.id}
            type="button"
            className={`${styles.item} ${active ? styles.itemActive : ''}`}
            aria-current={active ? 'true' : undefined}
            onClick={() => onNavigate(item.id)}
          >
            <span className={styles.accent} aria-hidden="true" />
            <span className={styles.index}>{item.index}</span>
            <span className={styles.body}>
              <span className={styles.title}>{item.title}</span>
              {item.meta != null ? <span className={styles.meta}>{item.meta}</span> : null}
            </span>
          </button>
        );
      })}
    </nav>
  );
}
