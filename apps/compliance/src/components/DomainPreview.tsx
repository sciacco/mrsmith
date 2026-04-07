import styles from './Compliance.module.css';

interface DomainPreviewProps {
  valid: string[];
  invalid: string[];
}

export function DomainPreview({ valid, invalid }: DomainPreviewProps) {
  if (valid.length === 0 && invalid.length === 0) return null;

  return (
    <div className={styles.previewContainer}>
      <div className={styles.previewSummary}>
        {valid.length} validi
        {invalid.length > 0 && (
          <>, <span className={styles.previewInvalidCount}>{invalid.length} non validi</span></>
        )}
      </div>
      {invalid.map((d, i) => (
        <div key={`inv-${i}`} className={styles.previewRow}>
          <span className={`${styles.previewDot} ${styles.previewDotInvalid}`} />
          <span className={styles.previewDomainInvalid}>{d}</span>
        </div>
      ))}
      {valid.map((d, i) => (
        <div key={`val-${i}`} className={styles.previewRow}>
          <span className={`${styles.previewDot} ${styles.previewDotValid}`} />
          <span className={styles.previewDomainValid}>{d}</span>
        </div>
      ))}
    </div>
  );
}
