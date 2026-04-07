import { useState, useRef, useEffect } from 'react';
import styles from './SingleSelect.module.css';

interface Option<V extends string | number = string | number> {
  value: V;
  label: string;
}

interface SingleSelectProps<V extends string | number = string | number> {
  options: Option<V>[];
  selected: V | null;
  onChange: (value: V | null) => void;
  placeholder?: string;
  allowClear?: boolean;
}

export function SingleSelect<V extends string | number = string | number>({
  options,
  selected,
  onChange,
  placeholder = 'Seleziona...',
  allowClear,
}: SingleSelectProps<V>) {
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

  const selectedOption = options.find((o) => o.value === selected);

  function handleSelect(value: V) {
    onChange(value);
    setOpen(false);
    setSearch('');
  }

  function handleClear() {
    onChange(null);
    setOpen(false);
    setSearch('');
  }

  return (
    <div className={styles.container} ref={containerRef}>
      <div
        className={`${styles.trigger} ${open ? styles.triggerOpen : ''}`}
        onClick={() => setOpen(!open)}
      >
        {selectedOption ? (
          <span className={styles.selectedLabel}>{selectedOption.label}</span>
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
            placeholder="Cerca..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            autoFocus
          />
          <div className={styles.options}>
            {allowClear && !search && (
              <div
                className={`${styles.option} ${selected === null ? styles.optionSelected : ''}`}
                onClick={handleClear}
              >
                <span className={styles.radio}>
                  {selected === null && <span className={styles.radioDot} />}
                </span>
                <span className={styles.clearLabel}>Tutti</span>
              </div>
            )}
            {filtered.length === 0 ? (
              <div className={styles.empty}>Nessun risultato</div>
            ) : (
              filtered.map((o) => (
                <div
                  key={o.value}
                  className={`${styles.option} ${o.value === selected ? styles.optionSelected : ''}`}
                  onClick={() => handleSelect(o.value)}
                >
                  <span className={styles.radio}>
                    {o.value === selected && <span className={styles.radioDot} />}
                  </span>
                  <span>{o.label}</span>
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
