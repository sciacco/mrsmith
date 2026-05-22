import styles from './PersonChip.module.css';
import type { PersonFlagKey, PersonFlags } from '../../api/types';

const LABEL: Record<PersonFlagKey, string> = {
  da_pianificare: 'Obbligo da gestire',
  compliance_gap: 'Obbligo da gestire',
  scadenze_imminenti: 'Scadenza entro 60 giorni',
  failed_recente: 'Esito negativo',
  senza_formazione_attiva: 'Senza corsi attivi',
};

interface PersonChipProps {
  flags: PersonFlags;
  gaps?: number;
}

const DOMINANT_FLAGS: PersonFlagKey[] = [
  'da_pianificare',
  'compliance_gap',
  'scadenze_imminenti',
  'failed_recente',
  'senza_formazione_attiva',
];

export function dominantPersonFlag(flags: PersonFlags): PersonFlagKey | null {
  return DOMINANT_FLAGS.find((flag) => flags[flag]) ?? null;
}

export function PersonChip({ flags, gaps }: PersonChipProps) {
  const flag = dominantPersonFlag(flags);
  if (!flag) return null;

  const label = LABEL[flag];
  return (
    <span className={`${styles.chip} ${styles[flag]}`}>
      <span className={styles.dot} aria-hidden />
      {label}{gaps !== undefined && flag === 'compliance_gap' && gaps > 0 ? ` · ${gaps}` : ''}
    </span>
  );
}
