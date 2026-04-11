import styles from './TemplateHeroCard.module.css';

interface TemplateHeroCardProps {
  kitName: string | null;
  category?: string | null;
  services: string;
  initialTermMonths: number;
  renewalTermMonths: number;
}

export function TemplateHeroCard({
  kitName,
  category,
  services,
  initialTermMonths,
  renewalTermMonths,
}: TemplateHeroCardProps) {
  const isEmpty = !kitName;

  return (
    <div
      className={`${styles.root} ${isEmpty ? styles.empty : styles.filled}`}
      aria-live="polite"
    >
      <div className={styles.eyebrow}>Kit derivato</div>

      {isEmpty ? (
        <>
          <div className={`${styles.nameSkeleton} ${styles.shimmer}`} aria-hidden="true" />
          <div className={styles.tileGrid}>
            <div className={`${styles.tileSkeleton} ${styles.shimmer}`} aria-hidden="true" />
            <div className={`${styles.tileSkeleton} ${styles.shimmer}`} aria-hidden="true" />
            <div className={`${styles.tileSkeleton} ${styles.shimmer}`} aria-hidden="true" />
          </div>
          <div className={styles.emptyHint}>
            Seleziona un template IaaS per derivare kit, servizi e termini.
          </div>
        </>
      ) : (
        <>
          <div className={styles.name} title={kitName ?? undefined}>
            {kitName}
          </div>
          {category && <div className={styles.category}>{category}</div>}
          <div className={styles.tileGrid}>
            <div className={styles.tile}>
              <div className={styles.tileLabel}>Servizi</div>
              <div className={styles.tileValue}>{services || '—'}</div>
            </div>
            <div className={styles.tile}>
              <div className={styles.tileLabel}>Termine iniziale</div>
              <div className={styles.tileValue}>{initialTermMonths}m</div>
            </div>
            <div className={styles.tile}>
              <div className={styles.tileLabel}>Rinnovo</div>
              <div className={styles.tileValue}>{renewalTermMonths}m</div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
