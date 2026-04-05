import { useState, useRef, useEffect } from 'react';
import styles from './MultiSelect.module.css';

interface Option {
  value: number;
  label: string;
}

interface MultiSelectProps {
  options: Option[];
  selected: number[];
  onChange: (selected: number[]) => void;
  placeholder?: string;
}

export function MultiSelect({
  options,
  selected,
  onChange,
  placeholder = 'Seleziona...',
}: MultiSelectProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const filtered = options.filter((o) =>
    o.label.toLowerCase().includes(search.toLowerCase()),
  );

  const selectedOptions = options.filter((o) => selected.includes(o.value));

  function toggle(value: number) {
    if (selected.includes(value)) {
      onChange(selected.filter((v) => v !== value));
    } else {
      onChange([...selected, value]);
    }
  }

  return (
    <div className={styles.container} ref={containerRef}>
      <div
        className={`${styles.trigger} ${open ? styles.triggerOpen : ''}`}
        onClick={() => setOpen(!open)}
      >
        {selectedOptions.length > 0 ? (
          <div className={styles.chips}>
            {selectedOptions.map((o) => (
              <span key={o.value} className={styles.chip}>
                {o.label}
                <button
                  className={styles.chipRemove}
                  onClick={(e) => {
                    e.stopPropagation();
                    toggle(o.value);
                  }}
                  aria-label={`Rimuovi ${o.label}`}
                >
                  &times;
                </button>
              </span>
            ))}
          </div>
        ) : (
          <span className={styles.placeholder}>{placeholder}</span>
        )}
        <span className={`${styles.arrow} ${open ? styles.arrowOpen : ''}`}>
          &#9660;
        </span>
      </div>
      {open && (
        <div className={styles.dropdown}>
          <input
            className={styles.search}
            type="text"
            placeholder="Cerca utenti..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            autoFocus
          />
          <div className={styles.options}>
            {filtered.length === 0 ? (
              <div className={styles.empty}>Nessun risultato</div>
            ) : (
              filtered.map((o) => (
                <label key={o.value} className={styles.option}>
                  <input
                    type="checkbox"
                    checked={selected.includes(o.value)}
                    onChange={() => toggle(o.value)}
                  />
                  <span>{o.label}</span>
                </label>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
