// Promemoria nelle liste richieste/catalogo/eventi (#170.D). Lo StatoBadge
// warning segnala solo i richiami datati <= oggi (civile Europe/Rome); i
// promemoria senza data o futuri restano neutri (nessuna attività operativa
// suggerita). Riutilizza StatusBadge ed evita la tripla duplicazione del
// rendering del promemoria nelle tre liste.

import { StatusBadge } from '@mrsmith/ui';
import { formatDateOnly } from '../events/eventFormat';
import styles from './ReminderBadge.module.css';

const ROME_TIME_ZONE = 'Europe/Rome';

function todayRomeDate(): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: ROME_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date());
  const at = Object.fromEntries(parts.map((p) => [p.type, p.value]));
  return `${at.year}-${at.month}-${at.day}`;
}

export function isOverdueReminder(date?: string): boolean {
  if (!date) return false;
  // Confronto lessicografico su stringa ISO YYYY-MM-DD.
  return date <= todayRomeDate();
}

interface ReminderBadgeProps {
  text?: string;
  date?: string;
  /** Reso quando non c'è alcun promemoria; se assente non viene renderizzato nulla. */
  fallback?: React.ReactNode;
  /** Forza la resa neutra anche se datato: usato per le istruttorie sospese che
      non devono suggerire attività operativa. */
  neutral?: boolean;
}

export function ReminderBadge({ text, date, fallback, neutral = false }: ReminderBadgeProps) {
  const reminderText = text?.trim();
  // Una data senza testo non identifica un promemoria operativo.
  if (!reminderText) return <>{fallback ?? null}</>;

  if (!date) return <span className={styles.text}>{reminderText}</span>;

  return (
    <span className={styles.reminder}>
      <span className={styles.text}>{reminderText}</span>
      <StatusBadge
        value={isOverdueReminder(date) && !neutral ? 'overdue' : 'reminder'}
        label={`Richiamo ${formatDateOnly(date)}`}
        variant={isOverdueReminder(date) && !neutral ? 'warning' : 'neutral'}
      />
    </span>
  );
}
