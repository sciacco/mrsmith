import { useEffect, useMemo, useRef, useState } from 'react';
import type { FilterOption } from '../api/types';
import s from './AutocompleteFilter.module.css';

interface AutocompleteFilterProps {
  id: string;
  label: string;
  placeholder: string;
  /** Current selected value (exact), or empty string when cleared. */
  value: string;
  onValueChange: (value: string) => void;
  /** Loads suggestions for the given query string. */
  loadSuggestions: (q: string) => Promise<FilterOption[]>;
  /** Minimum characters before triggering a search. */
  minLength?: number;
}

export function AutocompleteFilter({
  id,
  label,
  placeholder,
  value,
  onValueChange,
  loadSuggestions,
  minLength = 2,
}: AutocompleteFilterProps) {
  const [query, setQuery] = useState(value);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [options, setOptions] = useState<FilterOption[]>([]);
  const [activeIndex, setActiveIndex] = useState(-1);
  const boxRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Keep the textbox in sync when the parent clears the value.
  useEffect(() => {
    setQuery(value);
  }, [value]);

  // Close on outside click.
  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, []);

  // Debounced suggestion loading.
  useEffect(() => {
    const q = query.trim();
    if (q.length < minLength) {
      setOptions([]);
      setLoading(false);
      return;
    }
    if (q === value) {
      // Already the selected value; don't re-fetch.
      return;
    }
    setLoading(true);
    const handle = setTimeout(() => {
      let cancelled = false;
      loadSuggestions(q)
        .then((items) => {
          if (!cancelled) {
            setOptions(items);
            setOpen(true);
            setActiveIndex(-1);
          }
        })
        .catch(() => {
          if (!cancelled) setOptions([]);
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
      return () => {
        cancelled = true;
      };
    }, 250);
    return () => clearTimeout(handle);
  }, [query, loadSuggestions, minLength, value]);

  function selectOption(opt: FilterOption) {
    onValueChange(opt.value);
    setQuery(opt.value);
    setOpen(false);
    setActiveIndex(-1);
  }

  function clearSelection() {
    onValueChange('');
    setQuery('');
    setOptions([]);
    setOpen(false);
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (!open || options.length === 0) {
      if (e.key === 'ArrowDown' && options.length > 0) {
        setOpen(true);
        setActiveIndex(0);
        e.preventDefault();
      }
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, options.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const opt = options[activeIndex] ?? options[0];
      if (opt) selectOption(opt);
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  }

  const showDropdown = open && (loading || options.length > 0 || query.trim().length >= minLength);

  const visibleOptions = useMemo(() => options.slice(0, 50), [options]);

  return (
    <div className={s.field}>
      {label && <label htmlFor={id}>{label}</label>}
      <div className={s.input} ref={boxRef}>
        <input
          id={id}
          className={s.textbox}
          type="text"
          value={query}
          placeholder={placeholder}
          autoComplete="off"
          onChange={(e) => {
            setQuery(e.target.value);
            if (e.target.value === '') clearSelection();
          }}
          onFocus={() => {
            if (options.length > 0) setOpen(true);
          }}
          onKeyDown={onKeyDown}
        />
        {query && (
          <button type="button" className={s.clearBtn} aria-label="Cancella filtro" onClick={clearSelection}>
            ×
          </button>
        )}
        {showDropdown && (
          <div className={s.dropdown} ref={listRef} role="listbox">
            {loading && <div className={s.hint}>Caricamento…</div>}
            {!loading && visibleOptions.length === 0 && (
              <div className={s.hint}>Nessun suggerimento</div>
            )}
            {!loading &&
              visibleOptions.map((opt, idx) => (
                <button
                  key={opt.value}
                  type="button"
                  role="option"
                  aria-selected={idx === activeIndex}
                  className={`${s.option} ${idx === activeIndex ? s.active : ''}`}
                  onClick={() => selectOption(opt)}
                  onMouseEnter={() => setActiveIndex(idx)}
                >
                  <span className={s.optionValue}>{opt.value}</span>
                  <span className={s.optionCount}>{opt.count}</span>
                </button>
              ))}
          </div>
        )}
      </div>
    </div>
  );
}
