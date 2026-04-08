import type { Kit } from '../../types';
import styles from './KitCard.module.css';

interface KitMetadataProps {
  kit: Kit;
}

export function KitMetadata({ kit }: KitMetadataProps) {
  return (
    <div className={styles.metadataGrid}>
      <MetaItem label="Durata iniziale" value={`${kit.initial_subscription_months} mesi`} />
      <MetaItem label="Rinnovo" value={`${kit.next_subscription_months} mesi`} />
      <MetaItem label="Attivazione" value={`${kit.activation_time_days} gg`} />
      <MetaItem label="Fatturazione" value={kit.billing_period} />
      <MetaItem label="Sconto max" value={`${kit.sconto_massimo}%`} />
      <MetaItem label="Fatt. variabile" value={kit.variable_billing ? 'SI' : 'NO'} highlight={kit.variable_billing} />
      <MetaItem label="H24" value={kit.h24_assurance ? 'SI' : 'NO'} highlight={kit.h24_assurance} />
      <MetaItem label="SLA ore" value={`${kit.sla_resolution_hours}`} />
    </div>
  );
}

function MetaItem({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className={styles.metaItem}>
      <span className={styles.metaLabel}>{label}</span>
      <span className={`${styles.metaValue} ${highlight === true ? styles.metaPositive : ''} ${highlight === false ? styles.metaMuted : ''}`}>
        {value}
      </span>
    </div>
  );
}
