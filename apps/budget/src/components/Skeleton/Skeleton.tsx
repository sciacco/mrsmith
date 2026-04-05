import styles from './Skeleton.module.css';

interface SkeletonProps {
  rows?: number;
}

export function Skeleton({ rows = 5 }: SkeletonProps) {
  return (
    <div className={styles.container}>
      {Array.from({ length: rows }, (_, i) => (
        <div
          key={i}
          className={styles.row}
          style={{
            animationDelay: `${i * 80}ms`,
            width: `${100 - (i % 3) * 12}%`,
          }}
        />
      ))}
    </div>
  );
}
