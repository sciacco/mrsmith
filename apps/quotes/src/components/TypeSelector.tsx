import { Icon, type IconName } from '@mrsmith/ui';
import styles from './TypeSelector.module.css';

type QuoteType = 'standard' | 'iaas';

interface TypeSelectorProps {
  value: QuoteType;
  onChange: (type: QuoteType) => void;
}

interface TypeOption {
  value: QuoteType;
  label: string;
  description: string;
  icon: IconName;
}

const options: TypeOption[] = [
  {
    value: 'standard',
    label: 'Standard',
    description: 'Proposte commerciali classiche con kit e prodotti configurabili.',
    icon: 'package',
  },
  {
    value: 'iaas',
    label: 'IaaS',
    description: 'Offerte cloud/VCloud con template a termini fissi e trial opzionale.',
    icon: 'cloud',
  },
];

export function TypeSelector({ value, onChange }: TypeSelectorProps) {
  return (
    <div className={styles.wrap} role="radiogroup" aria-label="Tipo di proposta">
      {options.map(opt => {
        const selected = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            role="radio"
            aria-checked={selected}
            className={`${styles.toggle} ${selected ? styles.selected : ''}`}
            onClick={() => onChange(opt.value)}
          >
            <span className={styles.iconWrap} aria-hidden="true">
              <Icon name={opt.icon} size={28} strokeWidth={1.75} />
            </span>
            <span className={styles.label}>{opt.label}</span>
            <span className={styles.description}>{opt.description}</span>
          </button>
        );
      })}
    </div>
  );
}
