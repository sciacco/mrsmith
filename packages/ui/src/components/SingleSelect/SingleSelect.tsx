import { useState, useRef, useEffect, useId, useLayoutEffect } from 'react';
import { createPortal } from 'react-dom';
import styles from './SingleSelect.module.css';

interface Option<V extends string | number = string | number> {
  value: V;
  label: string;
  secondaryLabel?: string;
}

interface SingleSelectProps<V extends string | number = string | number> {
  options: Option<V>[];
  selected: V | null;
  onChange: (value: V | null) => void;
  placeholder?: string;
  allowClear?: boolean;
  clearLabel?: string;
  disabled?: boolean;
  searchable?: boolean;
  ariaLabel?: string;
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
  clearLabel = 'Tutti',
  disabled = false,
  searchable,
  ariaLabel,
}: SingleSelectProps<V>) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [coords, setCoords] = useState({ top: 0, left: 0, width: 0, placeTop: false });
  const triggerRef = useRef<HTMLButtonElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const listboxId = useId();

  useEffect(() => {
    if (disabled) setOpen(false);
  }, [disabled]);

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
      const dropdownEl = dropdownRef.current;
      const renderedWidth = dropdownEl ? dropdownEl.offsetWidth : rect.width;
      const dropdownWidth = Math.max(rect.width, renderedWidth);
      const maxLeft = window.innerWidth - dropdownWidth - VIEWPORT_PAD;
      const left = Math.max(VIEWPORT_PAD, Math.min(rect.left, maxLeft));
      setCoords({ top, left, width: rect.width, placeTop });
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
      if (e.key === 'Escape') {
        setOpen(false);
        requestAnimationFrame(() => triggerRef.current?.focus());
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  const showSearch = searchable ?? options.length > 2;
  const activeSearch = showSearch ? search : '';
  const filtered = options.filter((o) =>
    o.label.toLowerCase().includes(activeSearch.toLowerCase()),
  );

  const selectedOption = options.find((o) => o.value === selected);
  const renderInline = Boolean(triggerRef.current?.closest('dialog[open]'));

  function handleSelect(value: V) {
    if (disabled) return;
    onChange(value);
    setOpen(false);
    setSearch('');
    requestAnimationFrame(() => triggerRef.current?.focus());
  }

  function handleClear() {
    if (disabled) return;
    onChange(null);
    setOpen(false);
    setSearch('');
    requestAnimationFrame(() => triggerRef.current?.focus());
  }

  function focusOption(position: 'first' | 'last' | 'next' | 'previous', current?: HTMLElement) {
    const optionElements = Array.from(dropdownRef.current?.querySelectorAll<HTMLElement>('[role="option"]') ?? []);
    if (optionElements.length === 0) return;
    const currentIndex = current ? optionElements.indexOf(current) : -1;
    const targetIndex = position === 'first' ? 0
      : position === 'last' ? optionElements.length - 1
      : position === 'next' ? Math.min(currentIndex + 1, optionElements.length - 1)
      : Math.max(currentIndex - 1, 0);
    optionElements[targetIndex]?.focus();
  }

  function handleOptionKeyDown(event: React.KeyboardEvent<HTMLElement>) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      focusOption('next', event.currentTarget);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      focusOption('previous', event.currentTarget);
    } else if (event.key === 'Home') {
      event.preventDefault();
      focusOption('first');
    } else if (event.key === 'End') {
      event.preventDefault();
      focusOption('last');
    } else if (event.key === 'Escape') {
      event.preventDefault();
      setOpen(false);
      triggerRef.current?.focus();
    }
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
              minWidth: coords.width,
              transform: coords.placeTop ? 'translateY(-100%)' : undefined,
            }
      }
    >
      {showSearch ? (
        <input
          className={styles.search}
          type="text"
          placeholder="Cerca..."
          aria-label="Cerca opzioni"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown') { event.preventDefault(); focusOption('first'); }
            if (event.key === 'ArrowUp') { event.preventDefault(); focusOption('last'); }
          }}
          autoFocus
        />
      ) : null}
      <div id={listboxId} className={styles.options} role="listbox">
        {allowClear && !activeSearch && (
          <button
            type="button"
            role="option"
            aria-selected={selected === null}
            className={`${styles.option} ${selected === null ? styles.optionSelected : ''}`}
            onClick={handleClear}
            onKeyDown={handleOptionKeyDown}
          >
            <span className={styles.radio}>
              {selected === null && <span className={styles.radioDot} />}
            </span>
            <span className={styles.clearLabel}>{clearLabel}</span>
          </button>
        )}
        {filtered.length === 0 ? (
          <div className={styles.empty}>Nessun risultato</div>
        ) : (
          filtered.map((o) => (
            <button
              type="button"
              role="option"
              aria-selected={o.value === selected}
              key={o.value}
              className={`${styles.option} ${o.value === selected ? styles.optionSelected : ''}`}
              onClick={() => handleSelect(o.value)}
              onKeyDown={handleOptionKeyDown}
            >
              <span className={styles.radio}>
                {o.value === selected && <span className={styles.radioDot} />}
              </span>
              {o.secondaryLabel ? (
                <span className={styles.optionLabels}>
                  <span className={styles.optionPrimary}>{o.label}</span>
                  <span className={styles.optionSecondary}>{o.secondaryLabel}</span>
                </span>
              ) : (
                <span>{o.label}</span>
              )}
            </button>
          ))
        )}
      </div>
    </div>
  );

  return (
    <div className={styles.container}>
      <button
        type="button"
        ref={triggerRef}
        className={`${styles.trigger} ${open && !disabled ? styles.triggerOpen : ''} ${disabled ? styles.triggerDisabled : ''}`}
        role="combobox"
        aria-expanded={open && !disabled}
        aria-haspopup="listbox"
        aria-controls={listboxId}
        aria-label={ariaLabel}
        disabled={disabled}
        onClick={() => {
          if (!disabled) setOpen(!open);
        }}
        onKeyDown={(event) => {
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault();
            if (!open) setOpen(true);
            requestAnimationFrame(() => focusOption(event.key === 'ArrowDown' ? 'first' : 'last'));
          }
        }}
      >
        {selectedOption ? (
          selectedOption.secondaryLabel ? (
            <span className={styles.selectedStack}>
              <span className={styles.selectedPrimary}>{selectedOption.label}</span>
              <span className={styles.selectedSecondary}>{selectedOption.secondaryLabel}</span>
            </span>
          ) : (
            <span className={styles.selectedLabel}>{selectedOption.label}</span>
          )
        ) : (
          <span className={styles.placeholder}>{placeholder}</span>
        )}
        <span className={`${styles.arrow} ${open ? styles.arrowOpen : ''}`}>
          &#9660;
        </span>
      </button>
      {open && !disabled && (renderInline ? dropdown : createPortal(dropdown, document.body))}
    </div>
  );
}
