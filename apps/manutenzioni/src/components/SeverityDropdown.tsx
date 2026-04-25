import { useEffect, useRef, useState } from 'react';
import { Icon } from '@mrsmith/ui';
import type { SeverityValue } from '../api/types';
import styles from './SeverityDropdown.module.css';

type SeverityDropdownValue = SeverityValue | null;

const FALLBACK = { value: null as SeverityDropdownValue, label: 'Da definire', tone: 'undefined' };

const VALUES: Array<{ value: SeverityDropdownValue; label: string; tone: string }> = [
  { value: 'none', label: 'Nessun impatto', tone: 'none' },
  { value: 'degraded', label: 'Degradato', tone: 'degraded' },
  { value: 'unavailable', label: 'Non disponibile', tone: 'unavailable' },
  FALLBACK,
];

interface Props {
  value: SeverityDropdownValue;
  onChange: (value: SeverityDropdownValue) => void;
  isSuggested?: boolean;
  onConfirmSuggested?: () => void;
  disabled?: boolean;
}

export function SeverityDropdown({
  value,
  onChange,
  isSuggested,
  onConfirmSuggested,
  disabled,
}: Props) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDocClick(event: MouseEvent) {
      if (!ref.current) return;
      if (!ref.current.contains(event.target as Node)) setOpen(false);
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', onDocClick);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onDocClick);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const current = VALUES.find((entry) => entry.value === value) ?? FALLBACK;

  return (
    <div className={styles.wrapper} ref={ref}>
      <button
        type="button"
        className={`${styles.trigger} ${isSuggested ? styles.triggerSuggested : ''}`}
        onClick={() => !disabled && setOpen((current) => !current)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className={`${styles.dot} ${styles[`tone_${current.tone}`]}`} aria-hidden="true" />
        <span className={styles.label}>
          {current.label}
          {isSuggested ? <span className={styles.suggested}> (suggerito)</span> : null}
        </span>
        <Icon name="chevron-down" size={14} className={styles.chevron} />
      </button>
      {isSuggested && onConfirmSuggested ? (
        <button
          type="button"
          className={styles.confirm}
          onClick={onConfirmSuggested}
          title="Conferma il valore proposto"
          aria-label="Conferma il valore proposto"
        >
          <Icon name="check" size={14} />
        </button>
      ) : null}
      {open ? (
        <ul className={styles.menu} role="listbox">
          {VALUES.map((entry) => (
            <li key={entry.label}>
              <button
                type="button"
                className={`${styles.option} ${entry.value === value ? styles.optionSelected : ''}`}
                onClick={() => {
                  onChange(entry.value);
                  setOpen(false);
                }}
                role="option"
                aria-selected={entry.value === value}
              >
                <span className={`${styles.dot} ${styles[`tone_${entry.tone}`]}`} aria-hidden="true" />
                <span>{entry.label}</span>
                {entry.value === value ? (
                  <Icon name="check" size={14} className={styles.optionCheck} />
                ) : null}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
