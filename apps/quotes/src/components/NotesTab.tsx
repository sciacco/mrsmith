import { Icon } from '@mrsmith/ui';
import type { Quote } from '../api/types';
import { RichTextEditor } from './RichTextEditor';
import styles from './NotesTab.module.css';

interface NotesTabProps {
  quote: Quote;
  onChange: (field: string, value: string) => void;
}

export function NotesTab({ quote, onChange }: NotesTabProps) {
  const hasNotes = (quote.notes ?? '').trim().length > 0;

  return (
    <div className={styles.wrap}>
      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <span className={styles.label}>Descrizione</span>
          <span className={styles.hint}>
            Testo introduttivo visibile nell&apos;offerta PDF.
          </span>
        </div>
        <RichTextEditor
          value={quote.description ?? ''}
          onChange={html => onChange('description', html)}
          placeholder="Scrivi una breve descrizione della proposta..."
          standalone
        />
      </section>

      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <span className={styles.label}>Pattuizioni speciali</span>
          <span className={styles.hint}>
            Condizioni fuori standard. La loro presenza richiede approvazione commerciale.
          </span>
        </div>
        <RichTextEditor
          value={quote.notes ?? ''}
          onChange={html => onChange('notes', html)}
          placeholder="Specifica eventuali pattuizioni fuori standard..."
          standalone
        />
        <div className={`${styles.banner} ${hasNotes ? styles.bannerAmber : styles.bannerMuted}`}>
          <Icon name={hasNotes ? 'triangle-alert' : 'info'} size={16} />
          <span>
            {hasNotes
              ? 'Questa proposta contiene pattuizioni speciali e richiederà approvazione commerciale.'
              : 'Se inserisci pattuizioni speciali, la proposta richiederà approvazione commerciale.'}
          </span>
        </div>
      </section>
    </div>
  );
}
