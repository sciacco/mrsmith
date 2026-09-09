import { Icon } from '@mrsmith/ui';
import type { MATag } from '../../api/types';
import styles from './MATagChips.module.css';

interface MATagChipsProps {
  tags: MATag[];
  /** Se presente, ogni chip offre il comando che rimuove la SOLA associazione
   *  (il tag resta nel catalogo e sulle altre aziende): scheda azienda. */
  onRemove?: (tagId: string) => void;
  /** Comandi di rimozione disabilitati mentre una mutazione è in corso. */
  removeDisabled?: boolean;
  /** Chip con rimozione in corso: il pulsante mostra lo spinner. */
  removingTagId?: string | null;
  /** Card kanban (soprattutto compatte): scala ridotta, coerente ai chip di stato. */
  size?: 'md' | 'sm';
}

/**
 * Chip neutre per i tag aziendali condivisi (issue #198). Volontariamente
 * monocrome (surface + bordo sottile + testo secondario): su card e scheda i
 * colori accesi segnalano STATO/esiti — i tag devono restare distinguibili a
 * colpo d'occhio da ogni chip colorata. Usato dalla testata della scheda e
 * dalle BoardCard (anche compatte).
 */
export function MATagChips({ tags, onRemove, removeDisabled, removingTagId, size = 'md' }: MATagChipsProps) {
  const list = tags ?? [];
  if (list.length === 0) return null;
  return (
    <ul className={`${styles.list} ${size === 'sm' ? styles.sm : ''}`.trim()}>
      {list.map((tag) => (
        <li key={tag.id} className={styles.chip}>
          <span className={styles.name}>{tag.name}</span>
          {onRemove ? (
            <button
              type="button"
              className={styles.remove}
              aria-label={`Rimuovi tag ${tag.name}`}
              data-tag-remove={tag.id}
              disabled={removeDisabled}
              onClick={() => onRemove(tag.id)}
            >
              {removingTagId === tag.id ? <Icon name="loader" size={11} className={styles.spin} /> : <Icon name="x" size={11} aria-hidden="true" />}
            </button>
          ) : null}
        </li>
      ))}
    </ul>
  );
}
