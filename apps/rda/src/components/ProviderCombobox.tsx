import { Icon } from '@mrsmith/ui';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react';
import { createPortal } from 'react-dom';
import type { ProviderSummary } from '../api/types';
import styles from './ProviderCombobox.module.css';

const DROPDOWN_GAP = 6;
const VIEWPORT_PAD = 8;
const DROPDOWN_MAX_HEIGHT = 320;

function providerLabel(provider: ProviderSummary): string {
  return provider.company_name?.trim() || `Fornitore ${provider.id}`;
}

function providerSearchValue(provider: ProviderSummary): string {
  return [provider.company_name, provider.vat_number, provider.id].filter(Boolean).join(' ').toLowerCase();
}

export function ProviderCombobox({
  providers,
  value,
  disabled,
  onChange,
  onRequestNewProvider,
}: {
  providers: ProviderSummary[];
  value: number | '';
  disabled?: boolean;
  onChange: (value: number | '') => void;
  onRequestNewProvider: (search: string) => void;
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
    if (!normalized) return providers;
    return providers.filter((provider) => providerSearchValue(provider).includes(normalized));
  }, [providers, search]);

  const selectedProvider = providers.find((provider) => provider.id === value);
  const actionIndex = filtered.length;
  const renderInline = Boolean(triggerRef.current?.closest('dialog[open]'));
  const requestLabel = search.trim()
    ? `Richiedi censimento per "${search.trim()}"`
    : 'Richiedi il censimento di un nuovo fornitore';

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
    setActiveIndex((current) => Math.min(current, actionIndex));
  }, [actionIndex]);

  function selectProvider(provider: ProviderSummary) {
    onChange(provider.id);
    setOpen(false);
    setSearch('');
    setActiveIndex(0);
    triggerRef.current?.focus();
  }

  function requestProvider() {
    const nextSearch = search.trim();
    setOpen(false);
    setSearch('');
    setActiveIndex(0);
    onRequestNewProvider(nextSearch);
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
      setActiveIndex((current) => Math.min(current + 1, actionIndex));
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((current) => Math.max(current - 1, 0));
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      if (activeIndex === actionIndex) {
        requestProvider();
        return;
      }
      const provider = filtered[activeIndex];
      if (provider) selectProvider(provider);
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
          placeholder="Cerca fornitore"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <div className={styles.options} role="listbox" aria-label="Fornitori">
        {filtered.length === 0 ? (
          <div className={styles.empty}>Nessun fornitore trovato</div>
        ) : (
          filtered.map((provider, index) => {
            const selected = provider.id === value;
            const active = index === activeIndex;
            return (
              <button
                key={provider.id}
                type="button"
                className={`${styles.option} ${selected ? styles.optionSelected : ''} ${active ? styles.optionActive : ''}`}
                role="option"
                aria-selected={selected}
                onFocus={() => setActiveIndex(index)}
                onMouseEnter={() => setActiveIndex(index)}
                onClick={() => selectProvider(provider)}
              >
                <span className={styles.radio}>{selected ? <span className={styles.radioDot} /> : null}</span>
                <span className={styles.optionText}>
                  <span>{providerLabel(provider)}</span>
                  {provider.vat_number ? <small>P.IVA {provider.vat_number}</small> : null}
                </span>
              </button>
            );
          })
        )}
      </div>
      <div className={styles.actionPanel}>
        <button
          type="button"
          className={`${styles.requestButton} ${activeIndex === actionIndex ? styles.requestButtonActive : ''}`}
          onFocus={() => setActiveIndex(actionIndex)}
          onMouseEnter={() => setActiveIndex(actionIndex)}
          onClick={requestProvider}
        >
          <Icon name="plus" size={16} />
          <span>{requestLabel}</span>
        </button>
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
        {selectedProvider ? (
          <span className={styles.selectedLabel}>{providerLabel(selectedProvider)}</span>
        ) : (
          <span className={styles.placeholder}>Seleziona fornitore</span>
        )}
        <Icon name={open ? 'chevron-up' : 'chevron-down'} size={16} />
      </button>
      {open && !disabled && (renderInline ? dropdown : createPortal(dropdown, document.body))}
    </div>
  );
}
