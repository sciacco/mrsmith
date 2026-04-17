import { useState, useRef, useEffect, useLayoutEffect } from 'react';
import { createPortal } from 'react-dom';
import styles from './MultiSelect.module.css';

interface Option<T extends number | string = number> {
  value: T;
  label: string;
}

interface MultiSelectProps<T extends number | string = number> {
  options: Option<T>[];
  selected: T[];
  onChange: (selected: T[]) => void;
  placeholder?: string;
}

const DROPDOWN_GAP = 6;
const VIEWPORT_PAD = 8;
const DROPDOWN_MAX_HEIGHT = 280;

export function MultiSelect<T extends number | string = number>({
  options,
  selected,
  onChange,
  placeholder = 'Seleziona...',
}: MultiSelectProps<T>) {
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
      // When portaled into a <dialog> (which has a transform), position:fixed
      // resolves against the dialog's containing block, not the viewport — so
      // we must subtract the dialog's viewport offset from the coordinates.
      const dialog = trigger.closest('dialog');
      const offset = dialog
        ? dialog.getBoundingClientRect()
        : { top: 0, left: 0 };
      const top = placeTop
        ? rect.top - DROPDOWN_GAP - offset.top
        : rect.bottom + DROPDOWN_GAP - offset.top;
      setCoords({
        top,
        left: rect.left - offset.left,
        width: rect.width,
        placeTop,
      });
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

  const selectedOptions = options.filter((o) => selected.includes(o.value));

  function toggle(value: T) {
    if (selected.includes(value)) {
      onChange(selected.filter((v) => v !== value));
    } else {
      onChange([...selected, value]);
    }
  }

  return (
    <div className={styles.container}>
      <div
        ref={triggerRef}
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
      {open &&
        createPortal(
          <div
            ref={dropdownRef}
            className={styles.dropdown}
            style={{
              top: coords.top,
              left: coords.left,
              width: coords.width,
              transform: coords.placeTop ? 'translateY(-100%)' : undefined,
            }}
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
          </div>,
          triggerRef.current?.closest('dialog') ?? document.body,
        )}
    </div>
  );
}
