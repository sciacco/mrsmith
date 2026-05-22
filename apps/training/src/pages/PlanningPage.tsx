import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Skeleton, useToast } from '@mrsmith/ui';
import {
  useBulkPlanFromSuggestion,
  useDismissSuggestion,
  usePlanningSuggestions,
  useTransitionPlan,
} from '../api/queries';
import type { PlanningSuggestion, SuggestionSeverity } from '../api/types';
import { NewPlanModal } from '../components/NewPlanModal';
import { PlanHero } from '../components/PlanHero';
import { ReviewActionDrawer } from '../components/ReviewActionDrawer';
import { SuggestionCard } from '../components/SuggestionCard';
import styles from './PlanningPage.module.css';

interface PlanningPageProps {
  isPeopleAdmin: boolean;
}

const SEVERITIES: SuggestionSeverity[] = ['critical', 'warning', 'info'];

const SEVERITY_LABEL: Record<SuggestionSeverity, string> = {
  critical: 'Critici',
  warning: 'Attenzione',
  info: 'Info',
};

type DrawerState =
  | { kind: 'none' }
  | { kind: 'create_from_suggestion'; suggestion: PlanningSuggestion }
  | { kind: 'review_requests'; suggestion: PlanningSuggestion };

export function PlanningPage({ isPeopleAdmin }: PlanningPageProps) {
  const [params] = useSearchParams();
  const { toast } = useToast();
  const year = params.get('year') ?? String(new Date().getFullYear());
  const team = params.get('team') ?? '';
  const planning = usePlanningSuggestions(year, team, isPeopleAdmin);
  const dismiss = useDismissSuggestion();
  const bulkPlan = useBulkPlanFromSuggestion();
  const transition = useTransitionPlan();

  const [severityFilter, setSeverityFilter] = useState<'all' | SuggestionSeverity>('all');
  const [drawer, setDrawer] = useState<DrawerState>({ kind: 'none' });
  const [newPlanOpen, setNewPlanOpen] = useState(false);

  const suggestions = useMemo(() => {
    if (!planning.data) return [];
    if (severityFilter === 'all') return planning.data.suggestions;
    return planning.data.suggestions.filter((s) => s.severity === severityFilter);
  }, [planning.data, severityFilter]);

  const severityCounts = useMemo(() => {
    const counts: Record<SuggestionSeverity, number> = { critical: 0, warning: 0, info: 0 };
    (planning.data?.suggestions ?? []).forEach((s) => {
      counts[s.severity] = (counts[s.severity] ?? 0) + 1;
    });
    return counts;
  }, [planning.data]);

  const total = planning.data?.suggestions.length ?? 0;
  const budgetResidual = planning.data?.plan?.budget_residual ?? 0;
  const plan = planning.data?.plan ?? null;

  if (!isPeopleAdmin) {
    return (
      <main className={styles.page}>
        <p className={styles.accessDenied}>Accesso riservato al team People.</p>
      </main>
    );
  }

  if (planning.isLoading || !planning.data) {
    return (
      <main className={styles.page}>
        <Skeleton rows={6} />
      </main>
    );
  }

  function handlePlanAll(suggestion: PlanningSuggestion) {
    if (!suggestion.suggested_course_id || !plan) return;
    bulkPlan.mutate(
      {
        suggestion_id: suggestion.id,
        employee_ids: suggestion.affected_employee_ids,
        course_id: suggestion.suggested_course_id,
        plan_params: {
          year: plan.year,
          hours_planned: suggestion.suggested_course_hours,
          cost_planned: suggestion.suggested_course_cost,
          mandatory: suggestion.origin === 'compliance',
        },
      },
      {
        onSuccess: (res) => toast(`Pianificate ${res.created} iscrizioni`),
        onError: (err) => toast(err instanceof Error ? err.message : 'Errore', 'error'),
      },
    );
  }

  function handleReview(suggestion: PlanningSuggestion) {
    if (suggestion.origin === 'employee_request') {
      setDrawer({ kind: 'review_requests', suggestion });
    } else {
      setDrawer({ kind: 'create_from_suggestion', suggestion });
    }
  }

  function handleSkip(suggestion: PlanningSuggestion) {
    if (!plan?.plan_id) return;
    dismiss.mutate(
      { suggestionId: suggestion.id, planId: plan.plan_id },
      {
        onSuccess: () => toast('Suggerimento rimosso dalla coda'),
        onError: (err) => toast(err instanceof Error ? err.message : 'Errore', 'error'),
      },
    );
  }

  function handleLifecycle() {
    if (!plan) return;
    const target =
      plan.status === 'draft'
        ? 'open'
        : plan.status === 'open'
        ? 'closed'
        : plan.status === 'frozen'
        ? 'open'
        : 'reopened';
    if (plan.status === 'open' || plan.status === 'closed') {
      const message =
        plan.status === 'open'
          ? `Chiudere il piano ${plan.year}? Le iscrizioni non avviate saranno scadute.`
          : `Riaprire il piano ${plan.year}?`;
      if (!window.confirm(message)) return;
    }
    transition.mutate(
      { planId: plan.plan_id, target },
      {
        onSuccess: (res) => {
          if ((res.expired_enrollments_count ?? 0) > 0) {
            toast(`${res.expired_enrollments_count} iscrizioni scadute`);
          } else {
            toast(`Piano ${res.status === 'open' ? 'aperto' : res.status === 'closed' ? 'chiuso' : 'aggiornato'}`);
          }
        },
        onError: (err) => toast(err instanceof Error ? err.message : 'Errore', 'error'),
      },
    );
  }

  const hasSuggestions = total > 0;
  const showEmpty = !hasSuggestions && plan?.status === 'open';
  const showClosedBanner = plan?.status === 'closed';
  const showDraftBanner = plan?.status === 'draft';
  const newPlanDefaultYear = plan && plan.status !== 'missing' ? plan.year + 1 : Number(year);
  const newPlanPrevYearAvailable = plan
    ? plan.status !== 'missing' || plan.has_prev_year_plan
    : false;

  return (
    <main className={styles.page}>
      {plan && (
        <PlanHero
          summary={plan}
          pending={transition.isPending}
          onLifecycleClick={handleLifecycle}
          onNewPlan={() => setNewPlanOpen(true)}
        />
      )}

      {showDraftBanner && (
        <div className={`${styles.banner} ${styles.bannerDraft}`}>
          Piano in preparazione: le iscrizioni saranno effettive all'apertura.
        </div>
      )}

      {showClosedBanner && (
        <div className={`${styles.banner} ${styles.bannerClosed}`}>
          Piano chiuso · Sola lettura. Per riaprirlo usa il pulsante in alto.
        </div>
      )}

      {!showClosedBanner && plan && plan.status !== 'missing' && (
        <>
          <div className={styles.toolbar}>
            <div className={styles.toolbarLeft}>
              <h2 className={styles.toolbarTitle}>Suggerimenti</h2>
              <span className={styles.toolbarCount}>{total} attivi</span>
            </div>
            <div className={styles.chips} role="tablist">
              <SeverityChip
                label={`Tutti ${total}`}
                active={severityFilter === 'all'}
                onClick={() => setSeverityFilter('all')}
              />
              {SEVERITIES.map((severity) => {
                const count = severityCounts[severity] ?? 0;
                if (count === 0) return null;
                return (
                  <SeverityChip
                    key={severity}
                    label={`${SEVERITY_LABEL[severity]} ${count}`}
                    active={severityFilter === severity}
                    onClick={() => setSeverityFilter(severity)}
                    severity={severity}
                  />
                );
              })}
            </div>
          </div>

          {showEmpty ? (
            <div className={styles.empty}>
              <p className={styles.emptyTitle}>Piano {plan.year} in linea.</p>
              <p className={styles.emptySubtitle}>Nessuna azione richiesta.</p>
              {plan.enrollments_planned > 0 && (
                <p className={styles.emptyFooter}>
                  {plan.enrollments_planned} iscrizioni pianificate · {formatEuro(plan.budget_spent)} allocati
                </p>
              )}
            </div>
          ) : (
            <div className={styles.queue}>
              {suggestions.map((suggestion) => (
                <SuggestionCard
                  key={suggestion.id}
                  suggestion={suggestion}
                  budgetResidual={budgetResidual}
                  pending={bulkPlan.isPending || dismiss.isPending}
                  onPlanAll={handlePlanAll}
                  onReview={handleReview}
                  onSkip={handleSkip}
                />
              ))}
            </div>
          )}
        </>
      )}

      <NewPlanModal
        open={newPlanOpen}
        defaultYear={newPlanDefaultYear}
        prevYearAvailable={newPlanPrevYearAvailable}
        onClose={() => setNewPlanOpen(false)}
      />

      {plan && drawer.kind === 'create_from_suggestion' && (
        <ReviewActionDrawer
          open
          mode="create_from_suggestion"
          year={plan.year}
          suggestion={drawer.suggestion}
          onClose={() => setDrawer({ kind: 'none' })}
        />
      )}
      {plan && drawer.kind === 'review_requests' && (
        <ReviewActionDrawer
          open
          mode="review_employee_requests"
          year={plan.year}
          requestIds={drawer.suggestion.affected_employee_ids}
          onClose={() => setDrawer({ kind: 'none' })}
        />
      )}
    </main>
  );
}

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}

function SeverityChip({
  label,
  active,
  severity,
  onClick,
}: {
  label: string;
  active: boolean;
  severity?: SuggestionSeverity;
  onClick: () => void;
}) {
  const classes = [styles.chip, active ? styles.chipActive : '', severity ? styles[`chip_${severity}`] : '']
    .filter(Boolean)
    .join(' ');
  return (
    <button type="button" className={classes} onClick={onClick} role="tab" aria-selected={active}>
      {label}
    </button>
  );
}
