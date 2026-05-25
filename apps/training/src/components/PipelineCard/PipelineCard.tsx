import { Icon } from '@mrsmith/ui';
import type { PlanEnrollment } from '../../api/types';
import { ALERT_LEVEL_LABEL, classifyAlertLevel, type AlertLevel } from '../../lib/alertLevel';
import {
  enrollmentStatusLabel,
  enrollmentStatusTone,
  isActiveEnrollmentStatus,
} from '../../lib/enrollmentStatus.js';
import { formatBudget } from '../../lib/formatBudget';
import { daysUntilPipelineReference } from '../../lib/pipelineTiming';
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

interface DelayInfo {
  text: string;
  isPast: boolean;
}

function formatDelay(enrollment: PlanEnrollment, now: Date): DelayInfo | null {
  const daysUntil = daysUntilPipelineReference(enrollment, now);
  if (daysUntil === null) return null;
  if (daysUntil >= 0) return { text: `in ${daysUntil}gg`, isPast: false };
  return { text: `${-daysUntil}gg in ritardo`, isPast: true };
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
  const showStatusPill = isActiveEnrollmentStatus(enrollment.status);
  const statusTone = showStatusPill ? enrollmentStatusTone(enrollment.status) : null;
  const statusLabel = showStatusPill ? enrollmentStatusLabel(enrollment.status) : '';

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
          {enrollment.complianceRelated && (
            <span className={styles.complianceTag}>
              {enrollment.complianceFramework ? `Compliance ${enrollment.complianceFramework}` : 'Compliance'}
            </span>
          )}
          {enrollment.requiredByRule && <span className={styles.tag}>Obbligatoria</span>}
          {showStatusPill && statusTone && (
            <span className={`${styles.statusPill} ${styles[`status_${statusTone}`] ?? ''}`}>
              <span className={styles.statusDot} aria-hidden="true">●</span>
              {statusLabel}
            </span>
          )}
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
        <span className={styles.budget}>{budget ?? ''}</span>
      </div>
      <Icon name="chevron-right" size={14} className={styles.chev} />
    </article>
  );
}
