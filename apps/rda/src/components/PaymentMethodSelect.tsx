import { Icon } from '@mrsmith/ui';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react';
import { createPortal } from 'react-dom';
import type { PaymentMethodOption, PaymentOptionBadge } from '../lib/payment-options';
import styles from './PaymentMethodSelect.module.css';

const DROPDOWN_GAP = 6;
const VIEWPORT_PAD = 8;
const DROPDOWN_MAX_HEIGHT = 320;

export function PaymentMethodSelect({
  methods,
  value,
  disabled,
  onChange,
}: {
  methods: PaymentMethodOption[];
  value: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const [coords, setCoords] = useState({ top: 0, left: 0, width: 0, placeTop: false });
  const triggerRef = useRef<HTMLButtonElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  const filtered = useMemo(() => {
    const normalized = search.trim().toLowerCase();
    if (!normalized) return methods;
    return methods.filter(
      (m) =>
        m.label.toLowerCase().includes(normalized) ||
        m.code.toLowerCase().includes(normalized) ||
        m.badges.some((badge) => paymentBadgeLabel(badge).toLowerCase().includes(normalized)) ||
        (m.isNotPreapproved && 'non preapprovata'.includes(normalized)),
    );
  }, [methods, search]);

  const selected = methods.find((m) => m.code === value);
  const renderInline = Boolean(triggerRef.current?.closest('dialog[open]'));

  useEffect(() => {
    if (disabled) setOpen(false);
  }, [disabled]);

  useEffect(() => {
    if (!open) return;
    function handleClickOutside(event: MouseEvent) {
      const target = event.target as Node;
      if (!triggerRef.current?.contains(target) && !dropdownRef.current?.contains(target)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const frame = window.requestAnimationFrame(() => searchRef.current?.focus());
    return () => window.cancelAnimationFrame(frame);
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
      const top = placeTop ? rect.top - DROPDOWN_GAP : rect.bottom + DROPDOWN_GAP;
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
    function handleEscape(event: globalThis.KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpen(false);
        triggerRef.current?.focus();
      }
    }
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [open]);

  useEffect(() => {
    setActiveIndex(0);
  }, [search]);

  useEffect(() => {
    setActiveIndex((current) => Math.min(current, Math.max(filtered.length - 1, 0)));
  }, [filtered.length]);

  function selectMethod(method: PaymentMethodOption) {
    onChange(method.code);
    setOpen(false);
    setSearch('');
    setActiveIndex(0);
    triggerRef.current?.focus();
  }

  function handleTriggerKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    if (disabled) return;
    if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      setOpen(true);
    }
  }

  function handleListKeyDown(event: KeyboardEvent<HTMLInputElement | HTMLDivElement>) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActiveIndex((current) => Math.min(current + 1, filtered.length - 1));
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((current) => Math.max(current - 1, 0));
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      const method = filtered[activeIndex];
      if (method) selectMethod(method);
    }
  }

  const dropdownStyle: CSSProperties = renderInline
    ? coords.placeTop
      ? { bottom: `calc(100% + ${DROPDOWN_GAP}px)` }
      : { top: `calc(100% + ${DROPDOWN_GAP}px)` }
    : {
        top: coords.top,
        left: coords.left,
        width: coords.width,
        transform: coords.placeTop ? 'translateY(-100%)' : undefined,
      };

  const dropdown = (
    <div
      ref={dropdownRef}
      className={`${styles.dropdown} ${renderInline ? styles.dropdownInline : ''}`}
      style={dropdownStyle}
      onKeyDown={handleListKeyDown}
    >
      <div className={styles.searchShell}>
        <Icon name="search" size={16} />
        <input
          ref={searchRef}
          className={styles.search}
          type="text"
          placeholder="Cerca metodo pagamento"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <div className={styles.options} role="listbox" aria-label="Metodi pagamento">
        {filtered.length === 0 ? (
          <div className={styles.empty}>Nessun metodo trovato</div>
        ) : (
          filtered.map((method, index) => {
            const isSelected = method.code === value;
            const isActive = index === activeIndex;
            return (
              <button
                key={method.code}
                type="button"
                className={`${styles.option} ${isSelected ? styles.optionSelected : ''} ${isActive ? styles.optionActive : ''}`}
                role="option"
                aria-selected={isSelected}
                onFocus={() => setActiveIndex(index)}
                onMouseEnter={() => setActiveIndex(index)}
                onClick={() => selectMethod(method)}
              >
                <span className={styles.radio}>{isSelected ? <span className={styles.radioDot} /> : null}</span>
                <PaymentMethodOptionContent method={method} />
              </button>
            );
          })
        )}
      </div>
    </div>
  );

  return (
    <div className={styles.container}>
      <button
        ref={triggerRef}
        type="button"
        className={`${styles.trigger} ${open && !disabled ? styles.triggerOpen : ''}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={handleTriggerKeyDown}
      >
        {selected ? (
          <span className={styles.selectedLabel}>
            {selected.badges.map((badge) => (
              <PaymentBadge key={badge} badge={badge} />
            ))}
            <span className={styles.optionLabel}>{selected.label}</span>
            {selected.isNotPreapproved ? <NonPreapprovedBadge /> : null}
            {selected.code !== selected.label ? <span className={styles.selectedCode}>{selected.code}</span> : null}
          </span>
        ) : (
          <span className={styles.placeholder}>Seleziona pagamento</span>
        )}
        <Icon name={open ? 'chevron-up' : 'chevron-down'} size={16} />
      </button>
      {open && !disabled && (renderInline ? dropdown : createPortal(dropdown, document.body))}
    </div>
  );
}

function paymentBadgeLabel(badge: PaymentOptionBadge): string {
  return badge === 'provider-default' ? 'Default' : 'CDLAN';
}

function PaymentBadge({ badge }: { badge: PaymentOptionBadge }) {
  return (
    <span className={`${styles.pill} ${badge === 'provider-default' ? styles.pillDefault : styles.pillCdlan}`}>
      {paymentBadgeLabel(badge)}
    </span>
  );
}

function PaymentMethodOptionContent({ method }: { method: PaymentMethodOption }) {
  return (
    <span className={styles.optionText}>
      <span className={styles.paymentLine}>
        {method.badges.map((badge) => (
          <PaymentBadge key={badge} badge={badge} />
        ))}
        <span className={styles.optionLabel}>{method.label}</span>
        {method.isNotPreapproved ? <NonPreapprovedBadge /> : null}
      </span>
      {method.code !== method.label ? <span className={styles.optionCode}>{method.code}</span> : null}
    </span>
  );
}

function NonPreapprovedBadge() {
  return <span className={`${styles.pill} ${styles.pillNotPreapproved}`}>NON PREAPPROVATA</span>;
}
