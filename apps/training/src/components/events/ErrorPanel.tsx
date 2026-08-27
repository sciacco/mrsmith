// Pannello di errore inline persistente (docs/UI-UX.md §14.3): gli errori
// 4xx del backend che richiedono attenzione dell'operatore si mostrano qui
// per intero, mai solo in un toast che scompare in 4 secondi.

import { Icon } from '@mrsmith/ui';
import styles from './ErrorPanel.module.css';

export function ErrorPanel({ message, onDismiss }: { message: string | null; onDismiss?: () => void }) {
  if (!message) return null;
  return (
    <div className={styles.panel} role="alert">
      <Icon name="triangle-alert" size={18} />
      <p className={styles.message}>{message}</p>
      {onDismiss && (
        <button type="button" className={styles.dismiss} onClick={onDismiss} aria-label="Chiudi errore">
          <Icon name="x" size={16} />
        </button>
      )}
    </div>
  );
}
