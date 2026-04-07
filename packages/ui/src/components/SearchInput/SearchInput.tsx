import { type ChangeEvent } from 'react';
import styles from './SearchInput.module.css';

interface SearchInputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  autoFocus?: boolean;
}

export function SearchInput({
  value,
  onChange,
  placeholder = 'Cerca...',
  className,
  autoFocus,
}: SearchInputProps) {
  return (
    <div className={`${styles.wrap} ${className ?? ''}`}>
      <svg
        className={styles.icon}
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
      >
        <circle cx="7" cy="7" r="5.25" stroke="currentColor" strokeWidth="1.5" />
        <path d="M11 11l3.5 3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <input
        className={styles.input}
        type="text"
        placeholder={placeholder}
        value={value}
        onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        autoFocus={autoFocus}
      />
      {value && (
        <button
          className={styles.clearBtn}
          onClick={() => onChange('')}
          aria-label="Cancella ricerca"
          type="button"
        >
          ✕
        </button>
      )}
    </div>
  );
}
