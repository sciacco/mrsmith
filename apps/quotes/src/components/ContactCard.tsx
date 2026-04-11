import { Icon, type IconName } from '@mrsmith/ui';
import styles from './ContactCard.module.css';

export interface ContactFields {
  name: string;
  tel: string;
  email: string;
}

interface ContactCardProps {
  title: string;
  icon?: IconName;
  value: ContactFields;
  onChange: (patch: Partial<ContactFields>) => void;
  disabled?: boolean;
}

export function ContactCard({ title, icon = 'user', value, onChange, disabled }: ContactCardProps) {
  return (
    <div className={`${styles.card} ${disabled ? styles.disabled : ''}`}>
      <div className={styles.header}>
        <span className={styles.iconWrap} aria-hidden="true">
          <Icon name={icon} size={16} />
        </span>
        <span className={styles.title}>{title}</span>
      </div>
      <div className={styles.fields}>
        <label className={styles.field}>
          <span className={styles.label}>Nome</span>
          <input
            className={styles.input}
            type="text"
            value={value.name}
            disabled={disabled}
            onChange={e => onChange({ name: e.target.value })}
            placeholder="Nome e cognome"
          />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Telefono</span>
          <input
            className={styles.input}
            type="tel"
            value={value.tel}
            disabled={disabled}
            onChange={e => onChange({ tel: e.target.value })}
            placeholder="+39..."
          />
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Email</span>
          <input
            className={styles.input}
            type="email"
            value={value.email}
            disabled={disabled}
            onChange={e => onChange({ email: e.target.value })}
            placeholder="nome@azienda.it"
          />
        </label>
      </div>
    </div>
  );
}
