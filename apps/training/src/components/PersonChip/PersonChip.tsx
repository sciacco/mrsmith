import styles from './PersonChip.module.css';
import type { PersonComplianceStatus } from '../../api/types';

const LABEL: Record<PersonComplianceStatus, string> = {
  a_norma: 'A norma',
  con_gap: 'Con gap',
  senza_piano: 'Senza piano',
  nuovo_assunto: 'Nuovo',
};

interface PersonChipProps {
  status: PersonComplianceStatus;
  gaps?: number;
}

export function PersonChip({ status, gaps }: PersonChipProps) {
  const label = LABEL[status] ?? status;
  return (
    <span className={`${styles.chip} ${styles[status]}`}>
      <span className={styles.dot} aria-hidden />
      {label}{gaps !== undefined && status === 'con_gap' && gaps > 0 ? ` · ${gaps}` : ''}
    </span>
  );
}
