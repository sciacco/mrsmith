import styles from './DirtyBanner.module.css';

export function DirtyBanner() {
  return (
    <div className={styles.banner}>
      Hai modifiche non salvate
    </div>
  );
}
