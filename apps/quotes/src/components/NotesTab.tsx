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
      <div className={styles.field}>
        <label className={styles.label}>Descrizione</label>
        <RichTextEditor
          value={quote.description ?? ''}
          onChange={html => onChange('description', html)}
        />
      </div>

      <div className={styles.field}>
        <label className={styles.label}>Pattuizioni speciali</label>
        <RichTextEditor
          value={quote.notes ?? ''}
          onChange={html => onChange('notes', html)}
        />
      </div>

      <div className={`${styles.warning} ${hasNotes ? styles.warningAmber : styles.warningMuted}`}>
        {hasNotes
          ? 'Questa proposta richiederà approvazione'
          : 'Le pattuizioni speciali richiedono approvazione'}
      </div>
    </div>
  );
}
