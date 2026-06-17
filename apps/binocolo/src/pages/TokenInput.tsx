import { useState } from 'react';
import { Icon } from '@mrsmith/ui';
import styles from './TokenInput.module.css';

// TokenInput is a creatable chip/token field for a string[]: type and commit a chip
// with comma/Enter, Backspace on an empty input removes the last, paste of a
// comma-list explodes into chips. The committed values are chips (from props); the
// in-progress text is local, so there is no parse-on-keystroke snap-back.
export function TokenInput({
  value,
  onChange,
  placeholder,
  upper,
  disabled,
}: {
  value: string[];
  onChange: (next: string[]) => void;
  placeholder?: string;
  upper?: boolean;
  disabled?: boolean;
}) {
  const [draft, setDraft] = useState('');
  const normalize = (token: string) => (upper ? token.trim().toUpperCase() : token.trim());

  const commit = (raw: string) => {
    const tokens = raw
      .split(',')
      .map(normalize)
      .filter(Boolean);
    if (tokens.length === 0) {
      setDraft('');
      return;
    }
    const next = [...value];
    for (const token of tokens) {
      if (!next.includes(token)) next.push(token);
    }
    onChange(next);
    setDraft('');
  };

  const removeAt = (index: number) => onChange(value.filter((_, i) => i !== index));

  return (
    <div className={styles.wrap}>
      {value.map((token, index) => (
        <span key={`${token}-${index}`} className={styles.chip}>
          {token}
          <button
            type="button"
            className={styles.chipRemove}
            aria-label={`Rimuovi ${token}`}
            disabled={disabled}
            onClick={() => removeAt(index)}
          >
            <Icon name="x-circle" size={12} />
          </button>
        </span>
      ))}
      <input
        className={styles.input}
        value={draft}
        placeholder={value.length === 0 ? placeholder : ''}
        disabled={disabled}
        onChange={(event) => {
          const next = event.target.value;
          if (next.endsWith(',')) {
            commit(next);
          } else {
            setDraft(next);
          }
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            commit(draft);
          } else if (event.key === 'Backspace' && draft === '' && value.length > 0) {
            removeAt(value.length - 1);
          }
        }}
        onBlur={() => commit(draft)}
      />
    </div>
  );
}
