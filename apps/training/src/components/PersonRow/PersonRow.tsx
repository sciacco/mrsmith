import { Icon } from '@mrsmith/ui';
import { Link } from 'react-router-dom';
import type { PersonSummary } from '../../api/types';
import { PersonChip } from '../PersonChip';
import styles from './PersonRow.module.css';

interface PersonRowProps {
  person: PersonSummary;
  selected: boolean;
  expanded: boolean;
  onToggleSelect: () => void;
  onToggleExpand: () => void;
}

function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part.charAt(0).toUpperCase())
    .join('');
}

function formatDate(value: string | undefined): string {
  if (!value) return '';
  return value.slice(0, 10);
}

export function PersonRow({ person, selected, expanded, onToggleSelect, onToggleExpand }: PersonRowProps) {
  return (
    <div className={`${styles.row} ${selected ? styles.selected : ''} ${expanded ? styles.expanded : ''}`}>
      <div className={styles.head}>
        <label className={styles.checkboxWrap} onClick={(e) => e.stopPropagation()}>
          <input type="checkbox" checked={selected} onChange={onToggleSelect} aria-label={`Seleziona ${person.name}`} />
        </label>
        <div className={styles.avatar} aria-hidden>{initialsOf(person.name)}</div>
        <Link to={`/persone/${person.id}`} className={styles.name}>
          {person.name}
        </Link>
        <span className={styles.team}>{person.team_code || '—'}</span>
        <PersonChip status={person.compliance_status} gaps={person.gaps_open} />
        <span className={styles.counter}>
          <Icon name="clipboard-check" size={14} /> {person.active_enrollments_count}
        </span>
        <span className={styles.deadline}>
          {person.next_deadline ? (
            <>
              <Icon name="clock" size={14} /> {person.next_deadline.label} {formatDate(person.next_deadline.date)}
            </>
          ) : (
            <span className={styles.muted}>—</span>
          )}
        </span>
        <button type="button" className={styles.expand} onClick={onToggleExpand} aria-label={expanded ? 'Chiudi' : 'Apri'} aria-expanded={expanded}>
          <Icon name={expanded ? 'chevron-down' : 'chevron-right'} size={16} />
        </button>
      </div>
      {expanded && (
        <div className={styles.peek}>
          <div className={styles.peekStat}>
            <strong>{person.active_enrollments_count}</strong>
            <span>iscrizioni attive</span>
          </div>
          <div className={styles.peekStat}>
            <strong>{person.gaps_open}</strong>
            <span>gap obbligatori</span>
          </div>
          <div className={styles.peekStat}>
            <strong>{person.expiring_certs_count}</strong>
            <span>cert in scadenza</span>
          </div>
          <div className={styles.peekStat}>
            <strong>{person.historical_enrollments}</strong>
            <span>storico iscrizioni</span>
          </div>
          <div className={styles.peekActions}>
            <Link to={`/persone/${person.id}`}>Apri scheda</Link>
          </div>
        </div>
      )}
    </div>
  );
}
