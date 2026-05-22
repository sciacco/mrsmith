import { Button, Icon } from '@mrsmith/ui';
import type { PlanningSummary } from '../../api/types';
import { PlanKebabMenu } from '../PlanKebabMenu';
import styles from './PlanHero.module.css';

interface PlanHeroProps {
  summary: PlanningSummary;
  pending: boolean;
  onLifecycleClick: () => void;
  onNewPlan: () => void;
  onNewEnrollment: () => void;
  onEditPlan: () => void;
  onShowHistory: () => void;
  onDeletePlan: () => void;
}

const STATUS_LABEL: Record<PlanningSummary['status'], string> = {
  draft: 'In preparazione',
  open: 'Aperto',
  frozen: 'Congelato',
  closed: 'Chiuso',
  missing: 'Nessun piano',
};

const ALIGNMENT_LABEL: Record<PlanningSummary['calendar_alignment'], string> = {
  in_linea: 'in linea con il calendario',
  in_ritardo: 'spesa in ritardo',
  in_anticipo: 'spesa in anticipo',
};

function lifecycleLabel(status: PlanningSummary['status']): string | null {
  switch (status) {
    case 'draft':
      return 'Apri piano';
    case 'open':
      return 'Chiudi piano';
    case 'frozen':
      return 'Apri piano';
    case 'closed':
      return 'Riapri';
    case 'missing':
      return null;
  }
}

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}

export function PlanHero({
  summary,
  pending,
  onLifecycleClick,
  onNewPlan,
  onNewEnrollment,
  onEditPlan,
  onShowHistory,
  onDeletePlan,
}: PlanHeroProps) {
  const isMissing = summary.status === 'missing';
  const lifecycle = lifecycleLabel(summary.status);
  const pct = Math.min(100, Math.max(0, summary.budget_pct));
  const canDelete = summary.status === 'draft' && summary.enrollments_planned === 0;
  const canCreateEnrollment = summary.status === 'draft' || summary.status === 'open';
  const showMenu = summary.status === 'draft' || summary.status === 'open';

  if (isMissing) {
    return (
      <section className={`${styles.hero} ${styles.heroMissing}`} aria-label={`Piano ${summary.year}`}>
        <div className={styles.heroBody}>
          <span className={styles.eyebrow}>Piano {summary.year}</span>
          <h1 className={styles.title}>Nessun piano per {summary.year}.</h1>
          <p className={styles.subtitle}>Crea un nuovo piano per iniziare la pianificazione.</p>
        </div>
        <div className={styles.actions}>
          <Button variant="primary" size="md" leftIcon={<Icon name="plus" size={16} />} onClick={onNewPlan}>
            Nuovo piano
          </Button>
        </div>
      </section>
    );
  }

  return (
    <section
      className={`${styles.hero} ${styles[`hero_${summary.status}`] ?? ''}`}
      aria-label={`Piano ${summary.year}`}
    >
      <div className={styles.heroHead}>
        <div className={styles.eyebrowRow}>
          <span className={styles.eyebrow}>Piano {summary.year}</span>
          <span className={`${styles.statusPill} ${styles[`pill_${summary.status}`] ?? ''}`}>
            {STATUS_LABEL[summary.status]}
          </span>
        </div>
        <div className={styles.actions}>
          {lifecycle && (
            <Button
              variant={summary.status === 'closed' ? 'secondary' : 'secondary'}
              size="sm"
              onClick={onLifecycleClick}
              loading={pending}
            >
              {lifecycle}
            </Button>
          )}
          {canCreateEnrollment && (
            <Button
              variant="primary"
              size="sm"
              leftIcon={<Icon name="plus" size={15} />}
              onClick={onNewEnrollment}
            >
              Nuova iscrizione
            </Button>
          )}
          {showMenu && (
            <PlanKebabMenu
              canDelete={canDelete}
              onEdit={onEditPlan}
              onHistory={onShowHistory}
              onDelete={onDeletePlan}
            />
          )}
        </div>
      </div>
      <div className={styles.heroBody}>
        <div className={styles.metricsRow}>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>Speso</span>
            <span className={styles.metricValue}>{formatEuro(summary.budget_spent)}</span>
          </div>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>Residuo</span>
            <span className={`${styles.metricValue} ${styles.metricValueResidual}`}>
              {formatEuro(summary.budget_residual)}
            </span>
          </div>
          <div className={styles.metric}>
            <span className={styles.metricLabel}>Totale</span>
            <span className={styles.metricValueMuted}>{formatEuro(summary.budget_total)}</span>
          </div>
        </div>
        <div className={styles.progress} aria-hidden="true">
          <div
            className={`${styles.progressFill} ${styles[`fill_${summary.calendar_alignment}`] ?? ''}`}
            style={{ width: `${pct}%` }}
          />
        </div>
        <p className={styles.alignment}>
          {pct.toFixed(0)}% del budget · {ALIGNMENT_LABEL[summary.calendar_alignment]}
          {summary.enrollments_planned > 0 && (
            <> · {summary.enrollments_planned} iscrizioni</>
          )}
        </p>
      </div>
    </section>
  );
}
