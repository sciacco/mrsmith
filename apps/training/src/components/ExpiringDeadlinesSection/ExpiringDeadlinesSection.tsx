import { Link } from 'react-router-dom';
import type { ComplianceExpiringRow } from '../../api/types';
import styles from './ExpiringDeadlinesSection.module.css';

interface ExpiringDeadlinesSectionProps {
  rows: ComplianceExpiringRow[];
  deadlineDays: number;
  onDeadlineDaysChange: (days: number) => void;
}

const PRESETS = [30, 60, 90];

export function ExpiringDeadlinesSection({
  rows,
  deadlineDays,
  onDeadlineDaysChange,
}: ExpiringDeadlinesSectionProps) {
  return (
    <section className={styles.section} aria-label="Scadenze imminenti">
      <header className={styles.header}>
        <div>
          <h2 className={styles.title}>Scadenze imminenti</h2>
          <p className={styles.subtitle}>Certificazioni in scadenza nei prossimi {deadlineDays} giorni.</p>
        </div>
        <div className={styles.chips} role="group">
          {PRESETS.map((preset) => (
            <button
              key={preset}
              type="button"
              className={`${styles.chip} ${preset === deadlineDays ? styles.chipActive : ''}`}
              onClick={() => onDeadlineDaysChange(preset)}
            >
              {preset}gg
            </button>
          ))}
        </div>
      </header>

      {rows.length === 0 ? (
        <p className={styles.empty}>Nessuna scadenza nei prossimi {deadlineDays} giorni.</p>
      ) : (
        <ul className={styles.list}>
          {rows.map((row) => (
            <li key={`${row.employee_id}:${row.rule_id}`} className={styles.row}>
              <span className={`${styles.dot} ${styles[`dot_${row.severity}`]}`} aria-hidden="true" />
              <Link to={`/persone/${row.employee_id}`} className={styles.personLink}>
                {row.employee_name}
              </Link>
              <span className={styles.ruleLabel}>{row.rule_title}</span>
              <span className={styles.daysLabel}>
                {row.expires_in_days === 0
                  ? 'oggi'
                  : row.expires_in_days < 0
                  ? `scaduta da ${Math.abs(row.expires_in_days)}gg`
                  : `${row.expires_in_days}gg`}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
