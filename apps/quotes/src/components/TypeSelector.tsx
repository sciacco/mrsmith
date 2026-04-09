import styles from './TypeSelector.module.css';

interface TypeSelectorProps {
  value: 'standard' | 'iaas';
  onChange: (type: 'standard' | 'iaas') => void;
}

export function TypeSelector({ value, onChange }: TypeSelectorProps) {
  return (
    <div className={styles.wrap}>
      <button
        className={`${styles.toggle} ${value === 'standard' ? styles.selected : ''}`}
        onClick={() => onChange('standard')}
      >
        Standard
      </button>
      <button
        className={`${styles.toggle} ${value === 'iaas' ? styles.selected : ''}`}
        onClick={() => onChange('iaas')}
      >
        IaaS
      </button>
    </div>
  );
}
