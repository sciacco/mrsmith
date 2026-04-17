import styles from './DealCardSkeleton.module.css';

interface DealCardSkeletonProps {
  rows?: number;
}

export function DealCardSkeleton({ rows = 4 }: DealCardSkeletonProps) {
  return (
    <div className={styles.wrap} aria-busy="true" aria-live="polite">
      {Array.from({ length: rows }, (_, index) => (
        <div key={index} className={styles.card} style={{ animationDelay: `${index * 70}ms` }}>
          <div className={styles.row}>
            <span className={`${styles.bar} ${styles.eyebrow}`} />
            <span className={`${styles.bar} ${styles.stage}`} />
          </div>
          <span className={`${styles.bar} ${styles.heading}`} />
          <span className={`${styles.bar} ${styles.subtitle}`} />
          <span className={`${styles.bar} ${styles.meta}`} />
        </div>
      ))}
    </div>
  );
}
