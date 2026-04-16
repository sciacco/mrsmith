import { useState, useRef, useEffect, useLayoutEffect } from 'react';
import { createPortal } from 'react-dom';
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

const DROPDOWN_GAP = 6;
const VIEWPORT_PAD = 8;
const DROPDOWN_MAX_HEIGHT = 280;

export function SingleSelect<V extends string | number = string | number>({
  options,
  selected,
  onChange,
  placeholder = 'Seleziona...',
  allowClear,
}: SingleSelectProps<V>) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [coords, setCoords] = useState({ top: 0, left: 0, width: 0, placeTop: false });
  const triggerRef = useRef<HTMLDivElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClickOutside(e: MouseEvent) {
      const target = e.target as Node;
      if (
        !triggerRef.current?.contains(target) &&
        !dropdownRef.current?.contains(target)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [open]);

  useLayoutEffect(() => {
    if (!open) return;
    const trigger = triggerRef.current;
    if (!trigger) return;

    const update = () => {
      const rect = trigger.getBoundingClientRect();
      const spaceBelow = window.innerHeight - rect.bottom - VIEWPORT_PAD;
      const spaceAbove = rect.top - VIEWPORT_PAD;
      const placeTop = spaceBelow < DROPDOWN_MAX_HEIGHT && spaceAbove > spaceBelow;
      const top = placeTop
        ? rect.top - DROPDOWN_GAP
        : rect.bottom + DROPDOWN_GAP;
      setCoords({ top, left: rect.left, width: rect.width, placeTop });
    };

    update();
    window.addEventListener('scroll', update, true);
    window.addEventListener('resize', update);
    return () => {
      window.removeEventListener('scroll', update, true);
      window.removeEventListener('resize', update);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  const filtered = options.filter((o) =>
    o.label.toLowerCase().includes(search.toLowerCase()),
  );

  const selectedOption = options.find((o) => o.value === selected);
  const renderInline = Boolean(triggerRef.current?.closest('dialog[open]'));

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

  const dropdown = (
    <div
      ref={dropdownRef}
      className={`${styles.dropdown} ${renderInline ? styles.dropdownInline : ''}`}
      style={
        renderInline
          ? coords.placeTop
            ? { bottom: `calc(100% + ${DROPDOWN_GAP}px)` }
            : { top: `calc(100% + ${DROPDOWN_GAP}px)` }
          : {
              top: coords.top,
              left: coords.left,
              width: coords.width,
              transform: coords.placeTop ? 'translateY(-100%)' : undefined,
            }
      }
    >
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
  );

  return (
    <div className={styles.container}>
      <div
        ref={triggerRef}
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
      {open && (renderInline ? dropdown : createPortal(dropdown, document.body))}
    </div>
  );
}
