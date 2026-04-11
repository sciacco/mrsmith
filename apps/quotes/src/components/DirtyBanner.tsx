import { Button, Icon } from '@mrsmith/ui';
import styles from './DirtyBanner.module.css';

interface DirtyBannerProps {
  onSave: () => void;
  saving?: boolean;
}

export function DirtyBanner({ onSave, saving = false }: DirtyBannerProps) {
  return (
    <div className={styles.banner} role="status" aria-live="polite">
      <span className={styles.icon} aria-hidden="true">
        <Icon name="triangle-alert" size={16} />
      </span>
      <span className={styles.text}>Hai modifiche non salvate</span>
      <div className={styles.action}>
        <Button variant="primary" size="sm" onClick={onSave} loading={saving}>
          Salva
        </Button>
      </div>
    </div>
  );
}
