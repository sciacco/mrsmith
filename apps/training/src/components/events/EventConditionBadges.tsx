// Badge di condizione operativa di un evento, derivati dai flags del
// backend (#156). Nessun ricalcolo: mostra solo cio che l'API dichiara.

import { StatusBadge, type StatusBadgeVariant } from '@mrsmith/ui';
import type { EventFlags } from '../../api/types';
import { EVENT_CONDITION_BADGE_LABELS } from '../../lib/labels';
import styles from './EventConditionBadges.module.css';

const VARIANTS: Record<keyof EventFlags, StatusBadgeVariant> = {
  cancelled: 'danger',
  withoutSessions: 'warning',
  unassignedEnrollments: 'warning',
  inProgress: 'accent',
  needsReconciliation: 'warning',
  concluded: 'success',
};

// Ordine di rilevanza: annullato prevale su tutto, poi le condizioni che
// richiedono intervento, poi gli stati di avanzamento.
const ORDER: (keyof EventFlags)[] = [
  'cancelled',
  'needsReconciliation',
  'withoutSessions',
  'unassignedEnrollments',
  'inProgress',
  'concluded',
];

export function EventConditionBadges({ flags }: { flags: EventFlags }) {
  const active = ORDER.filter((key) => flags[key]);
  if (active.length === 0) {
    return <StatusBadge value="planned" label="Regolare" variant="neutral" />;
  }
  return (
    <span className={styles.row}>
      {active.map((key) => (
        <StatusBadge key={key} value={key} label={EVENT_CONDITION_BADGE_LABELS[key]} variant={VARIANTS[key]} />
      ))}
    </span>
  );
}
