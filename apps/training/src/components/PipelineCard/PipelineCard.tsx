import { Icon } from '@mrsmith/ui';
import type { PlanEnrollment } from '../../api/types';
import { ALERT_LEVEL_LABEL, classifyAlertLevel, type AlertLevel } from '../../lib/alertLevel';
import styles from './PipelineCard.module.css';

interface PipelineCardProps {
  enrollment: PlanEnrollment;
  selected: boolean;
  onToggle: () => void;
  onOpen: () => void;
  now?: Date;
}

const STATUS_VERB: Record<string, string> = {
  proposed: 'Approva',
  approved: 'Avvia',
  in_progress: 'Chiudi',
  completed: 'Rivedi',
  failed: 'Riapri',
  cancelled: 'Riapri',
  expired: 'Riapri',
};

const DAY_MS = 24 * 60 * 60 * 1000;

function formatProposalAge(enrollment: PlanEnrollment, now: Date): string | null {
  const ref = enrollment.plannedStart ?? enrollment.plannedEnd;
  if (!ref) return null;
  const stamped = ref.length > 10 ? ref : `${ref}T00:00:00`;
  const date = new Date(stamped);
  if (!Number.isFinite(date.getTime())) return null;
  const diff = Math.floor((date.getTime() - now.getTime()) / DAY_MS);
  if (diff >= 0) return `inizio in ${diff}gg`;
  return `${-diff}gg di ritardo`;
}

function formatMoney(value: number | undefined): string | undefined {
  if (value === undefined) return undefined;
  return new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 }).format(value);
}

function alertClass(level: AlertLevel) {
  return styles[`alert_${level}`] ?? '';
}

export function PipelineCard({ enrollment, selected, onToggle, onOpen, now = new Date() }: PipelineCardProps) {
  const level = classifyAlertLevel(enrollment, { now });
  const verb = STATUS_VERB[enrollment.status] ?? 'Apri';
  const proposalAge = formatProposalAge(enrollment, now);
  const budget = formatMoney(enrollment.costPlanned);

  function handleKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault();
      onOpen();
    }
    if (event.key === ' ') {
      event.preventDefault();
      onToggle();
    }
  }

  return (
    <article className={`${styles.card} ${selected ? styles.selected : ''}`} tabIndex={0} onKeyDown={handleKeyDown}>
      <label className={styles.checkboxWrap} onClick={(e) => e.stopPropagation()}>
        <input type="checkbox" checked={selected} onChange={onToggle} aria-label={`Seleziona ${enrollment.courseTitle}`} />
      </label>
      <button type="button" className={styles.body} onClick={onOpen}>
        <div className={styles.headline}>
          <span className={`${styles.alertBadge} ${alertClass(level)}`} title={ALERT_LEVEL_LABEL[level]} aria-label={ALERT_LEVEL_LABEL[level]}>
            ●
          </span>
          <strong className={styles.verb}>{verb}</strong>
          <span className={styles.title}>{enrollment.courseTitle}</span>
          <span className={styles.separator}>·</span>
          <span className={styles.person}>{enrollment.employeeName}</span>
          {enrollment.teamCode && <span className={styles.team}>{enrollment.teamCode}</span>}
          {enrollment.mandatory && <span className={styles.tag}>Obbligatoria</span>}
          <Icon name="chevron-right" size={14} className={styles.chev} />
        </div>
        <div className={styles.context}>
          {proposalAge && <span>{proposalAge}</span>}
          {budget && <span>{budget}</span>}
          {enrollment.hoursPlanned !== undefined && <span>{enrollment.hoursPlanned}h</span>}
          {enrollment.vendorName && <span>{enrollment.vendorName}</span>}
        </div>
      </button>
    </article>
  );
}
