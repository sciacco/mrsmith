import { Icon } from '@mrsmith/ui';
import type { ReactNode } from 'react';
import styles from './LabeledDisclosure.module.css';

export function LabeledDisclosure({
  title,
  density,
  children,
}: {
  title: string;
  density?: string;
  children: ReactNode;
}) {
  return (
    <details className={styles.disclosure}>
      <summary>
        <span className={styles.label}>
          <span className={styles.title}>{title}</span>
          {density ? <span className={styles.density}>· {density}</span> : null}
        </span>
        <Icon name="chevron-down" size={16} />
      </summary>
      <div className={styles.body}>{children}</div>
    </details>
  );
}
