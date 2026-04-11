import type { Quote } from '../api/types';
import { ContactCard, type ContactFields } from './ContactCard';
import styles from './ContactsTab.module.css';

interface ContactsTabProps {
  quote: Quote;
  onChange: (field: string, value: string) => void;
}

interface ContactBinding {
  title: string;
  icon: 'settings' | 'mail' | 'user';
  fieldName: string;
  fieldTel: string;
  fieldEmail: string;
}

const bindings: ContactBinding[] = [
  {
    title: 'Tecnico',
    icon: 'settings',
    fieldName: 'rif_tech_nom',
    fieldTel: 'rif_tech_tel',
    fieldEmail: 'rif_tech_email',
  },
  {
    title: 'Altro tecnico',
    icon: 'settings',
    fieldName: 'rif_altro_tech_nom',
    fieldTel: 'rif_altro_tech_tel',
    fieldEmail: 'rif_altro_tech_email',
  },
  {
    title: 'Amministrativo',
    icon: 'mail',
    fieldName: 'rif_adm_nom',
    fieldTel: 'rif_adm_tech_tel',
    fieldEmail: 'rif_adm_tech_email',
  },
];

function readContact(quote: Quote, b: ContactBinding): ContactFields {
  return {
    name: (quote[b.fieldName as keyof Quote] as string | null) ?? '',
    tel: (quote[b.fieldTel as keyof Quote] as string | null) ?? '',
    email: (quote[b.fieldEmail as keyof Quote] as string | null) ?? '',
  };
}

export function ContactsTab({ quote, onChange }: ContactsTabProps) {
  return (
    <div className={styles.wrap}>
      <section className={styles.orderSection}>
        <label className={styles.label} htmlFor="rif-ordcli">
          Riferimento ordine cliente
        </label>
        <input
          id="rif-ordcli"
          className={styles.input}
          type="text"
          value={quote.rif_ordcli ?? ''}
          onChange={e => onChange('rif_ordcli', e.target.value)}
          placeholder="Es. PO 2026/0015"
        />
      </section>

      <div className={styles.contactGrid}>
        {bindings.map(b => {
          const value = readContact(quote, b);
          return (
            <ContactCard
              key={b.title}
              title={b.title}
              icon={b.icon}
              value={value}
              onChange={patch => {
                if (patch.name !== undefined) onChange(b.fieldName, patch.name);
                if (patch.tel !== undefined) onChange(b.fieldTel, patch.tel);
                if (patch.email !== undefined) onChange(b.fieldEmail, patch.email);
              }}
            />
          );
        })}
      </div>
    </div>
  );
}
