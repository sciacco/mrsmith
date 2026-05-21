import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Button } from '@mrsmith/ui';
import type { ComplianceRule, ComplianceRuleGap } from '../../api/types';
import styles from './ComplianceRuleSection.module.css';

interface ComplianceRuleSectionProps {
  rule: ComplianceRule;
  onPlanGap: (rule: ComplianceRule) => void;
}

const SEVERITY_LABEL: Record<ComplianceRule['severity'], string> = {
  critical: 'CRITICO',
  warning: 'ATTENZIONE',
  ok: 'OK',
};

const STATUS_LABEL: Record<ComplianceRuleGap['status'], string> = {
  never_covered: 'mai coperta',
  expired: 'scaduta',
  expiring_soon: 'in scadenza',
};

export function ComplianceRuleSection({ rule, onPlanGap }: ComplianceRuleSectionProps) {
  const [expanded, setExpanded] = useState(rule.severity === 'critical');

  const coveragePct = Math.round(rule.coverage_pct);
  const gapCount = rule.target_count - rule.covered_count;

  return (
    <section
      className={`${styles.section} ${styles[`severity_${rule.severity}`]}`}
      aria-label={rule.title}
    >
      <button type="button" className={styles.head} onClick={() => setExpanded((v) => !v)}>
        <div className={styles.headBody}>
          <div className={styles.titleRow}>
            <span
              className={`${styles.dot} ${styles[`dot_${rule.severity}`]}`}
              aria-hidden="true"
            />
            <h2 className={styles.title}>{rule.title}</h2>
          </div>
          <p className={styles.metadata}>
            {rule.cadence_label} · {rule.population_target}
          </p>
        </div>
        <div className={styles.headRight}>
          <span className={styles.coverage}>
            {coveragePct}%{' '}
            <span className={styles.coverageMeta}>
              ({rule.covered_count}/{rule.target_count})
            </span>
          </span>
          <span className={`${styles.badge} ${styles[`badge_${rule.severity}`]}`}>
            {SEVERITY_LABEL[rule.severity]}
            {gapCount > 0 && ` · ${gapCount}`}
          </span>
          <span className={styles.chevron} aria-hidden="true">
            {expanded ? '▾' : '▸'}
          </span>
        </div>
      </button>

      {expanded && (
        <div className={styles.expanded}>
          {rule.gaps.length === 0 ? (
            <p className={styles.empty}>Nessun gap aperto.</p>
          ) : (
            <>
              <ul className={styles.gapList}>
                {rule.gaps.map((gap) => (
                  <li key={gap.employee_id} className={styles.gapRow}>
                    <span className={styles.personName}>{gap.employee_name}</span>
                    <span className={styles.gapStatus}>{STATUS_LABEL[gap.status]}</span>
                    <Link
                      to={`/persone/${gap.employee_id}`}
                      className={styles.gapLink}
                      aria-label={`Apri scheda di ${gap.employee_name}`}
                    >
                      ›
                    </Link>
                  </li>
                ))}
              </ul>
              {gapCount > rule.gaps.length && (
                <p className={styles.moreHint}>
                  Mostrati {rule.gaps.length} di {gapCount} gap.
                </p>
              )}
              <div className={styles.actions}>
                <Button variant="secondary" size="sm" onClick={() => onPlanGap(rule)}>
                  Pianifica per tutti i {gapCount}
                </Button>
              </div>
            </>
          )}
        </div>
      )}
    </section>
  );
}
