import { useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Skeleton } from '@mrsmith/ui';
import { useComplianceOverview } from '../api/queries';
import type { ComplianceRule, PlanningSuggestion } from '../api/types';
import { ComplianceRuleSection } from '../components/ComplianceRuleSection';
import { ExpiringDeadlinesSection } from '../components/ExpiringDeadlinesSection';
import { ReviewActionDrawer } from '../components/ReviewActionDrawer';
import styles from './CompliancePage.module.css';

interface CompliancePageProps {
  isPeopleAdmin: boolean;
}

function ruleToSuggestion(rule: ComplianceRule): PlanningSuggestion {
  return {
    id: `rule-${rule.id}`,
    severity: rule.severity === 'ok' ? 'info' : rule.severity,
    origin: 'compliance',
    title: rule.title,
    description: rule.population_target,
    affected_count: rule.target_count - rule.covered_count,
    affected_employee_ids: rule.gaps.map((g) => g.employee_id),
    suggested_course_id: rule.suggested_course_ids[0],
    alternative_course_ids: rule.suggested_course_ids.slice(1),
    estimated_cost: 0,
    dismissed: false,
    rule_id: rule.id,
  };
}

export function CompliancePage({ isPeopleAdmin }: CompliancePageProps) {
  const [params] = useSearchParams();
  const year = params.get('year') ?? String(new Date().getFullYear());
  const team = params.get('team') ?? '';
  const [deadlineDays, setDeadlineDays] = useState(30);
  const [drawer, setDrawer] = useState<{ rule: ComplianceRule } | null>(null);

  const compliance = useComplianceOverview(year, team, deadlineDays, isPeopleAdmin);

  if (!isPeopleAdmin) {
    return (
      <main className={styles.page}>
        <p className={styles.accessDenied}>Accesso riservato al team People.</p>
      </main>
    );
  }

  if (compliance.isLoading || !compliance.data) {
    return (
      <main className={styles.page}>
        <Skeleton rows={6} />
      </main>
    );
  }

  const data = compliance.data;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Compliance</h1>
          <p className={styles.subtitle}>Copertura mandatory rules e scadenze imminenti.</p>
        </div>
        <Link to="/compliance/regole" className={styles.manageLink}>
          Gestisci regole →
        </Link>
      </header>

      <ExpiringDeadlinesSection
        rows={data.expiring_deadlines}
        deadlineDays={deadlineDays}
        onDeadlineDaysChange={setDeadlineDays}
      />

      <div className={styles.rulesHeader}>
        <h2 className={styles.rulesTitle}>Regole</h2>
        <span className={styles.rulesCount}>{data.rules.length} attive</span>
      </div>

      {data.rules.length === 0 ? (
        <p className={styles.empty}>100% copertura su tutte le rule applicate al team.</p>
      ) : (
        <div className={styles.rules}>
          {data.rules.map((rule) => (
            <ComplianceRuleSection
              key={rule.id}
              rule={rule}
              onPlanGap={(r) => setDrawer({ rule: r })}
            />
          ))}
        </div>
      )}

      {drawer && drawer.rule.suggested_course_ids.length > 0 && (
        <ReviewActionDrawer
          open
          mode="create_from_suggestion"
          year={Number(year) || data.year}
          suggestion={ruleToSuggestion(drawer.rule)}
          overrideCourseId={drawer.rule.suggested_course_ids[0]}
          overrideEmployeeIds={drawer.rule.gaps.map((g) => g.employee_id)}
          overrideTitle={`Pianifica ${drawer.rule.title}`}
          onClose={() => setDrawer(null)}
        />
      )}
    </main>
  );
}
