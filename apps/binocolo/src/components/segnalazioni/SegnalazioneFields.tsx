import type { SegnalazioneWrite } from '../../api/types';
import styles from './segnalazioni.module.css';

type FieldKey = keyof SegnalazioneWrite;

interface FieldMeta {
  key: FieldKey;
  label: string;
  multiline?: boolean;
  span2?: boolean;
  inputMode?: 'url' | 'text';
  autoComplete?: string;
}

/** I sei campi del form, tutti a testo libero e individualmente facoltativi. */
export const SEGNALAZIONE_FIELDS: FieldMeta[] = [
  { key: 'name', label: 'Nome azienda o opportunità', span2: true, autoComplete: 'organization' },
  { key: 'website', label: 'Sito web', inputMode: 'url', autoComplete: 'url' },
  { key: 'location', label: 'Località' },
  { key: 'fiscalId', label: 'Identificativo fiscale', span2: true },
  { key: 'contacts', label: 'Interlocutori o recapiti', multiline: true, span2: true },
  { key: 'notes', label: 'Note addizionali', multiline: true, span2: true },
];

/** Campi modificabili. `invalid` marca tutti i campi qualificanti quando la
 *  regola minima non è rispettata; il messaggio vive nel contenitore
 *  (`describedBy`) perché la regola riguarda l'insieme, non un campo solo. */
export function SegnalazioneFields({ idPrefix, value, onChange, disabled, invalid, describedBy }: {
  idPrefix: string;
  value: SegnalazioneWrite;
  onChange: (next: SegnalazioneWrite) => void;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
}) {
  return (
    <div className={styles.grid}>
      {SEGNALAZIONE_FIELDS.map((field) => {
        const id = `${idPrefix}-${field.key}`;
        const qualifying = field.key === 'name' || field.key === 'website' || field.key === 'fiscalId' || field.key === 'notes';
        const flag = Boolean(invalid && qualifying);
        const common = {
          id,
          value: value[field.key],
          disabled,
          'aria-invalid': flag || undefined,
          'aria-describedby': describedBy,
          onChange: (event: { target: { value: string } }) => onChange({ ...value, [field.key]: event.target.value }),
        };
        return (
          <div key={field.key} className={`${styles.field} ${field.span2 ? styles.span2 : ''}`}>
            <label htmlFor={id} className={styles.label}>{field.label}</label>
            {field.multiline ? (
              <textarea {...common} className={`${styles.textarea} ${flag ? styles.inputError : ''}`} rows={2} />
            ) : (
              <input {...common} className={`${styles.input} ${flag ? styles.inputError : ''}`} type="text" inputMode={field.inputMode} autoComplete={field.autoComplete ?? 'off'} />
            )}
          </div>
        );
      })}
    </div>
  );
}

/** Stessi sei campi in sola lettura (stato chiusa). */
export function SegnalazioneReadonly({ value }: { value: SegnalazioneWrite }) {
  return (
    <div className={styles.readonly}>
      {SEGNALAZIONE_FIELDS.map((field) => {
        const text = value[field.key].trim();
        return (
          <div key={field.key} className={styles.readField}>
            <span className={styles.label}>{field.label}</span>
            {text ? <p className={styles.readValue}>{text}</p> : <p className={`${styles.readValue} ${styles.readEmpty}`}>Non indicato</p>}
          </div>
        );
      })}
    </div>
  );
}
