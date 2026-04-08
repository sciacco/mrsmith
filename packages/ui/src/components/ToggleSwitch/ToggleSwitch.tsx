import type { ChangeEvent, InputHTMLAttributes } from 'react';
import styles from './ToggleSwitch.module.css';

export interface ToggleSwitchProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'type'> {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: string;
  id: string;
}

export function ToggleSwitch({
  checked,
  onChange,
  label,
  id,
  disabled,
  className,
  ...rest
}: ToggleSwitchProps) {
  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    onChange(e.target.checked);
  }

  return (
    <label
      htmlFor={id}
      className={[
        styles.wrapper,
        disabled ? styles.disabled : '',
        className ?? '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <span className={styles.switchRoot}>
        <input
          {...rest}
          id={id}
          type="checkbox"
          role="switch"
          aria-checked={checked}
          checked={checked}
          disabled={disabled}
          onChange={handleChange}
          className={styles.input}
        />
        <span className={styles.track} aria-hidden="true">
          <span className={styles.thumb} />
        </span>
      </span>
      {label ? <span className={styles.label}>{label}</span> : null}
    </label>
  );
}
