import type { Quote } from '../api/types';
import styles from './ContactsTab.module.css';

interface ContactsTabProps {
  quote: Quote;
  onChange: (field: string, value: string) => void;
}

function ContactField({ label, value, field, onChange }: {
  label: string; value: string | null; field: string;
  onChange: (field: string, value: string) => void;
}) {
  return (
    <div className={styles.field}>
      <label className={styles.label}>{label}</label>
      <input
        className={styles.input}
        type="text"
        value={value ?? ''}
        onChange={e => onChange(field, e.target.value)}
      />
    </div>
  );
}

export function ContactsTab({ quote, onChange }: ContactsTabProps) {
  return (
    <div className={styles.grid}>
      <div className={styles.card}>
        <div className={styles.cardTitle}>Ordine cliente</div>
        <ContactField label="Riferimento" value={quote.rif_ordcli} field="rif_ordcli" onChange={onChange} />
      </div>

      <div className={styles.card}>
        <div className={styles.cardTitle}>Tecnico</div>
        <ContactField label="Nome" value={quote.rif_tech_nom} field="rif_tech_nom" onChange={onChange} />
        <ContactField label="Telefono" value={quote.rif_tech_tel} field="rif_tech_tel" onChange={onChange} />
        <ContactField label="Email" value={quote.rif_tech_email} field="rif_tech_email" onChange={onChange} />
      </div>

      <div className={styles.card}>
        <div className={styles.cardTitle}>Altro Tecnico</div>
        <ContactField label="Nome" value={quote.rif_altro_tech_nom} field="rif_altro_tech_nom" onChange={onChange} />
        <ContactField label="Telefono" value={quote.rif_altro_tech_tel} field="rif_altro_tech_tel" onChange={onChange} />
        <ContactField label="Email" value={quote.rif_altro_tech_email} field="rif_altro_tech_email" onChange={onChange} />
      </div>

      <div className={styles.card}>
        <div className={styles.cardTitle}>Amministrativo</div>
        <ContactField label="Nome" value={quote.rif_adm_nom} field="rif_adm_nom" onChange={onChange} />
        <ContactField label="Telefono" value={quote.rif_adm_tech_tel} field="rif_adm_tech_tel" onChange={onChange} />
        <ContactField label="Email" value={quote.rif_adm_tech_email} field="rif_adm_tech_email" onChange={onChange} />
      </div>
    </div>
  );
}
