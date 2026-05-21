import { Icon } from '@mrsmith/ui';
import type { PlanEnrollment } from '../../api/types';
import { ALERT_LEVEL_LABEL, classifyAlertLevel, type AlertLevel } from '../../lib/alertLevel';
import { formatBudget } from '../../lib/formatBudget';
import { formatTeamLabel, type TeamLabelMap } from '../../lib/teamLabels';
import styles from './PipelineCard.module.css';

interface PipelineCardProps {
  enrollment: PlanEnrollment;
  selected: boolean;
  onToggle: () => void;
  onOpen: () => void;
  teamLabels: TeamLabelMap;
  now?: Date;
}

const DAY_MS = 24 * 60 * 60 * 1000;

interface DelayInfo {
  text: string;
  isPast: boolean;
}

function formatDelay(enrollment: PlanEnrollment, now: Date): DelayInfo | null {
  const ref = enrollment.plannedStart ?? enrollment.plannedEnd;
  if (!ref) return null;
  const stamped = ref.length > 10 ? ref : `${ref}T00:00:00`;
  const date = new Date(stamped);
  if (!Number.isFinite(date.getTime())) return null;
  const diff = Math.floor((date.getTime() - now.getTime()) / DAY_MS);
  if (diff >= 0) return { text: `in ${diff}gg`, isPast: false };
  return { text: `${-diff}gg in ritardo`, isPast: true };
}

function alertClass(level: AlertLevel) {
  return styles[`alert_${level}`] ?? '';
}

function delayClass(level: AlertLevel, isPast: boolean) {
  if (level === 'critical' && isPast) return styles.delayCritical;
  if (level === 'warning') return styles.delayWarning;
  return styles.delayMuted;
}

export function PipelineCard({
  enrollment,
  selected,
  onToggle,
  onOpen,
  teamLabels,
  now = new Date(),
}: PipelineCardProps) {
  const level = classifyAlertLevel(enrollment, { now });
  const delay = formatDelay(enrollment, now);
  const budget = formatBudget(enrollment.costPlanned);
  const teamLabel = formatTeamLabel(enrollment.teamCode, teamLabels);

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
    <article
      className={`${styles.card} ${selected ? styles.selected : ''}`}
      tabIndex={0}
      role="button"
      aria-label={`Apri ${enrollment.courseTitle}`}
      onClick={onOpen}
      onKeyDown={handleKeyDown}
    >
      <label className={styles.checkboxWrap} onClick={(e) => e.stopPropagation()}>
        <input type="checkbox" checked={selected} onChange={onToggle} aria-label={`Seleziona ${enrollment.courseTitle}`} />
      </label>
      <span
        className={`${styles.alertBadge} ${alertClass(level)}`}
        title={ALERT_LEVEL_LABEL[level]}
        aria-label={ALERT_LEVEL_LABEL[level]}
      >
        ●
      </span>
      <div className={styles.body}>
        <div className={styles.headline}>
          <span className={styles.title}>{enrollment.courseTitle}</span>
          <span className={styles.separator}>·</span>
          <span className={styles.person}>{enrollment.employeeName}</span>
          {teamLabel && <span className={styles.team}>{teamLabel}</span>}
          {enrollment.mandatory && <span className={styles.tag}>Obbligatoria</span>}
        </div>
        {(enrollment.vendorName || enrollment.hoursPlanned !== undefined) && (
          <div className={styles.subline}>
            {enrollment.vendorName && <span>{enrollment.vendorName}</span>}
            {enrollment.hoursPlanned !== undefined && (
              <>
                {enrollment.vendorName && <span className={styles.dotSep}>·</span>}
                <span>{enrollment.hoursPlanned}h</span>
              </>
            )}
          </div>
        )}
      </div>
      <div className={styles.meta}>
        {delay && <span className={`${styles.delay} ${delayClass(level, delay.isPast)}`}>{delay.text}</span>}
        {budget && <span className={styles.budget}>{budget}</span>}
      </div>
      <Icon name="chevron-right" size={14} className={styles.chev} />
    </article>
  );
}
