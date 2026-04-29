import { Icon } from '@mrsmith/ui';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react';
import { createPortal } from 'react-dom';
import type { Article } from '../api/types';
import styles from './ArticleCombobox.module.css';

const DROPDOWN_GAP = 6;
const VIEWPORT_PAD = 8;
const DROPDOWN_MAX_HEIGHT = 340;

function articleLabel(article: Article): string {
  return article.description?.trim() || article.code;
}

function articleSearchValue(article: Article): string {
  return [article.code, article.description, article.type === 'good' ? 'bene' : 'servizio'].filter(Boolean).join(' ').toLowerCase();
}

function ArticleTypeBadge({ type }: { type: Article['type'] }) {
  return <span className={`${styles.typeBadge} ${type === 'good' ? styles.good : styles.service}`}>{type === 'good' ? 'Bene' : 'Servizio'}</span>;
}

export function ArticleCombobox({
  articles,
  value,
  search,
  loading,
  disabled,
  onSearchChange,
  onChange,
}: {
  articles: Article[];
  value: Article | null;
  search: string;
  loading?: boolean;
  disabled?: boolean;
  onSearchChange: (value: string) => void;
  onChange: (value: Article | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const [coords, setCoords] = useState({ top: 0, left: 0, width: 0, placeTop: false });
  const triggerRef = useRef<HTMLButtonElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  const filtered = useMemo(() => {
    const normalized = search.trim().toLowerCase();
    if (!normalized) return articles;
    return articles.filter((article) => articleSearchValue(article).includes(normalized));
  }, [articles, search]);

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

  function selectArticle(article: Article) {
    onChange(article);
    setOpen(false);
    onSearchChange('');
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
      setActiveIndex((current) => Math.min(current + 1, Math.max(filtered.length - 1, 0)));
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((current) => Math.max(current - 1, 0));
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      const article = filtered[activeIndex];
      if (article) selectArticle(article);
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
          placeholder="Cerca per descrizione o codice"
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
        />
      </div>
      <div className={styles.options} role="listbox" aria-label="Articoli RDA">
        {loading ? <div className={styles.empty}>Caricamento articoli...</div> : null}
        {!loading && filtered.length === 0 ? <div className={styles.empty}>Nessun articolo trovato</div> : null}
        {!loading
          ? filtered.map((article, index) => {
              const selected = value != null && article.code === value.code && article.type === value.type;
              const active = index === activeIndex;
              return (
                <button
                  key={`${article.type}-${article.code}`}
                  type="button"
                  className={`${styles.option} ${selected ? styles.optionSelected : ''} ${active ? styles.optionActive : ''}`}
                  role="option"
                  aria-selected={selected}
                  onFocus={() => setActiveIndex(index)}
                  onMouseEnter={() => setActiveIndex(index)}
                  onClick={() => selectArticle(article)}
                >
                  <span className={styles.radio}>{selected ? <span className={styles.radioDot} /> : null}</span>
                  <span className={styles.optionText}>
                    <span>{articleLabel(article)}</span>
                    <small>{article.code}</small>
                  </span>
                  <ArticleTypeBadge type={article.type} />
                </button>
              );
            })
          : null}
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
        {value ? (
          <span className={styles.selectedContent}>
            <span className={styles.selectedText}>
              <span>{articleLabel(value)}</span>
              <small>{value.code}</small>
            </span>
            <ArticleTypeBadge type={value.type} />
          </span>
        ) : (
          <span className={styles.placeholder}>Seleziona articolo</span>
        )}
        <Icon name="chevron-down" size={16} className={open ? styles.arrowOpen : ''} />
      </button>
      {open && !disabled && (renderInline ? dropdown : createPortal(dropdown, document.body))}
    </div>
  );
}
