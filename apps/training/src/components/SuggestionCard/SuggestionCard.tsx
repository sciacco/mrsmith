import { Button } from '@mrsmith/ui';
import type { PlanningSuggestion } from '../../api/types';
import styles from './SuggestionCard.module.css';

interface SuggestionCardProps {
  suggestion: PlanningSuggestion;
  budgetResidual: number;
  pending?: boolean;
  onPlanAll: (suggestion: PlanningSuggestion) => void;
  onReview: (suggestion: PlanningSuggestion) => void;
  onSkip: (suggestion: PlanningSuggestion) => void;
}

const ORIGIN_LABEL: Record<PlanningSuggestion['origin'], string> = {
  compliance: 'COMPLIANCE GAP',
  expiring: 'SCADENZA IMMINENTE',
  skill_gap: 'SKILL GAP',
  employee_request: 'RICHIESTE EMPLOYEE',
};

const SEVERITY_LABEL: Record<PlanningSuggestion['severity'], string> = {
  critical: 'CRITICO',
  warning: 'ATTENZIONE',
  info: 'INFO',
};

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}

export function SuggestionCard({
  suggestion,
  budgetResidual,
  pending,
  onPlanAll,
  onReview,
  onSkip,
}: SuggestionCardProps) {
  const residualAfter = budgetResidual - (suggestion.estimated_cost ?? 0);
  const isEmployeeRequest = suggestion.origin === 'employee_request';

  return (
    <article
      className={`${styles.card} ${styles[`severity_${suggestion.severity}`]}`}
      aria-label={suggestion.title}
    >
      <header className={styles.head}>
        <span className={`${styles.dot} ${styles[`dot_${suggestion.severity}`]}`} aria-hidden="true" />
        <span className={styles.eyebrow}>
          {SEVERITY_LABEL[suggestion.severity]} · {ORIGIN_LABEL[suggestion.origin]}
        </span>
      </header>

      <h2 className={styles.title}>{suggestion.title}</h2>

      {suggestion.suggested_course_name && (
        <p className={styles.courseLine}>
          Corso suggerito:{' '}
          <span className={styles.courseName}>{suggestion.suggested_course_name}</span>
          {suggestion.suggested_course_hours !== undefined && (
            <> · {suggestion.suggested_course_hours}h</>
          )}
          {suggestion.suggested_course_cost !== undefined && (
            <> · {formatEuro(suggestion.suggested_course_cost)}/persona</>
          )}
        </p>
      )}

      {!suggestion.suggested_course_name && suggestion.description && (
        <p className={styles.description}>{suggestion.description}</p>
      )}

      {!isEmployeeRequest && suggestion.estimated_cost > 0 && (
        <div className={styles.budgetRow}>
          <div className={styles.budgetMetric}>
            <span className={styles.budgetLabel}>Costo stimato</span>
            <span className={styles.budgetValue}>{formatEuro(suggestion.estimated_cost)}</span>
          </div>
          <span className={styles.budgetSeparator} aria-hidden="true">·</span>
          <div className={styles.budgetMetric}>
            <span className={styles.budgetLabel}>Residuo dopo</span>
            <span
              className={`${styles.budgetValue} ${
                residualAfter < 0 ? styles.budgetValueNegative : styles.budgetValueResidual
              }`}
            >
              {formatEuro(residualAfter)}
            </span>
          </div>
        </div>
      )}

      <footer className={styles.actions}>
        {!isEmployeeRequest && suggestion.suggested_course_id && (
          <Button
            variant="primary"
            size="sm"
            onClick={() => onPlanAll(suggestion)}
            disabled={pending}
          >
            Pianifica tutti
          </Button>
        )}
        <Button
          variant="secondary"
          size="sm"
          onClick={() => onReview(suggestion)}
          disabled={pending}
        >
          {isEmployeeRequest ? 'Rivedi richieste' : 'Rivedi'}
        </Button>
        {!isEmployeeRequest && (
          <Button variant="ghost" size="sm" onClick={() => onSkip(suggestion)} disabled={pending}>
            Salta
          </Button>
        )}
      </footer>
    </article>
  );
}
