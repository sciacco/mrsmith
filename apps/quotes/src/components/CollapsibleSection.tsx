import { useId, type ReactNode } from 'react';
import { Icon } from '@mrsmith/ui';
import styles from './CollapsibleSection.module.css';

interface CollapsibleSectionProps {
  title: string;
  summary?: string;
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
  trailing?: ReactNode;
}

export function CollapsibleSection({
  title,
  summary,
  open,
  onToggle,
  children,
  trailing,
}: CollapsibleSectionProps) {
  const contentId = useId();

  return (
    <section className={`${styles.section} ${open ? styles.open : ''}`}>
      <button
        type="button"
        className={styles.header}
        onClick={onToggle}
        aria-expanded={open}
        aria-controls={contentId}
      >
        <span className={`${styles.chevron} ${open ? styles.chevronOpen : ''}`} aria-hidden="true">
          <Icon name="chevron-right" size={16} />
        </span>
        <span className={styles.title}>{title}</span>
        {!open && summary && <span className={styles.summary}>{summary}</span>}
        {trailing && <span className={styles.trailing}>{trailing}</span>}
      </button>
      {open && (
        <div id={contentId} className={styles.body}>
          {children}
        </div>
      )}
    </section>
  );
}
