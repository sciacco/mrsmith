import type { OrderDetail } from '../api/types';
import { formatEmpty } from '../lib/formatters';
import styles from '../pages/OrderDetailPage.module.css';

export function AltriDatiTab({ order }: { order: OrderDetail }) {
  return (
    <section className={styles.cardSection}>
      <div className={styles.sectionHeader}>
        <h2>Altri dati</h2>
      </div>
      <div className={styles.factGrid}>
        <div className={styles.factItem}>
          <span>ODV</span>
          <strong className={styles.mono}>{formatEmpty(order.cdlan_systemodv)}</strong>
        </div>
        <div className={styles.factItem}>
          <span>Commerciale</span>
          <strong>{formatEmpty(order.cdlan_commerciale)}</strong>
        </div>
      </div>
    </section>
  );
}
