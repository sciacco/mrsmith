import type { OrderDetail } from '../api/types';
import { formatEmpty, formatIsColo } from '../lib/formatters';
import styles from '../pages/OrderDetailPage.module.css';

export function AziendaTab({ order }: { order: OrderDetail }) {
  return (
    <section className={styles.cardSection}>
      <div className={styles.sectionHeader}>
        <h2>Azienda</h2>
      </div>
      <div className={styles.factGrid}>
        <Field label="Ragione sociale" value={order.cdlan_cliente} />
        <Field label="ID cliente" value={order.cdlan_cliente_id} mono />
        <Field label="Partita IVA" value={order.profile_iva} mono />
        <Field label="Codice fiscale" value={order.profile_cf} mono />
        <Field label="Indirizzo" value={order.profile_address} />
        <Field label="Città" value={order.profile_city} />
        <Field label="CAP" value={order.profile_cap} mono />
        <Field label="Provincia" value={order.profile_pv} />
        <Field label="SDI" value={order.profile_sdi} mono />
        <Field label="Profilo lingua" value={order.profile_lang} />
        <Field label="Soluzione" value={formatIsColo(order.is_colo)} />
      </div>
    </section>
  );
}

function Field({ label, value, mono }: { label: string; value: string | number | null | undefined; mono?: boolean }) {
  return (
    <div className={styles.factItem}>
      <span>{label}</span>
      <strong className={mono ? styles.mono : undefined}>{formatEmpty(value)}</strong>
    </div>
  );
}
