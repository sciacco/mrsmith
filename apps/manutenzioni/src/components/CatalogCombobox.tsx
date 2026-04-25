import { useEffect, useMemo, useRef, useState } from 'react';
import { Icon } from '@mrsmith/ui';
import type { ReferenceItem } from '../api/types';
import styles from './CatalogCombobox.module.css';

interface Props {
  options: ReferenceItem[];
  excludedIds: Set<number>;
  onSelect: (item: ReferenceItem) => void;
  onCreateRequest: (initialName: string) => void;
  placeholder?: string;
  domainHintId?: number | null;
}

export function CatalogCombobox({
  options,
  excludedIds,
  onSelect,
  onCreateRequest,
  placeholder = 'Cerca o crea voce di catalogo…',
  domainHintId,
}: Props) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const wrapperRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDocClick(event: MouseEvent) {
      if (!wrapperRef.current) return;
      if (!wrapperRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', onDocClick);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onDocClick);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const trimmed = query.trim();
  const lower = trimmed.toLowerCase();

  const { exactMatches, hasExactMatch, similarMatches } = useMemo(() => {
    const visibleOptions = options.filter((option) => !excludedIds.has(option.id));
    if (!lower) {
      const items = domainHintId
        ? [...visibleOptions].sort((a, b) => {
            const aDomain = a.technical_domain_id === domainHintId ? 0 : 1;
            const bDomain = b.technical_domain_id === domainHintId ? 0 : 1;
            return aDomain - bDomain || a.name_it.localeCompare(b.name_it);
          })
        : visibleOptions;
      return { exactMatches: items.slice(0, 50), hasExactMatch: false, similarMatches: [] };
    }
    const exactMatches: ReferenceItem[] = [];
    const similarMatches: ReferenceItem[] = [];
    let hasExact = false;
    for (const option of visibleOptions) {
      const name = option.name_it.toLowerCase();
      if (name === lower) hasExact = true;
      if (name.includes(lower)) {
        exactMatches.push(option);
      } else if (similarity(name, lower) >= 0.55) {
        similarMatches.push(option);
      }
    }
    return {
      exactMatches: exactMatches.slice(0, 30),
      hasExactMatch: hasExact,
      similarMatches: similarMatches.slice(0, 5),
    };
  }, [domainHintId, excludedIds, lower, options]);

  const showCreateOption = trimmed.length > 0 && !hasExactMatch;

  return (
    <div className={styles.wrapper} ref={wrapperRef}>
      <div className={styles.field} onClick={() => setOpen(true)}>
        <Icon name="search" size={16} className={styles.fieldIcon} />
        <input
          type="text"
          className={styles.input}
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          placeholder={placeholder}
        />
      </div>
      {open ? (
        <div className={styles.menu} role="listbox">
          {exactMatches.length === 0 && similarMatches.length === 0 && !showCreateOption ? (
            <p className={styles.empty}>Nessuna voce corrisponde.</p>
          ) : null}
          {exactMatches.length > 0 ? (
            <ul className={styles.list}>
              {exactMatches.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    className={styles.option}
                    onClick={() => {
                      onSelect(item);
                      setQuery('');
                      setOpen(false);
                    }}
                  >
                    <span className={styles.optionName}>{item.name_it}</span>
                    {item.target_type_name ? (
                      <span className={styles.optionMeta}>{item.target_type_name}</span>
                    ) : null}
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
          {similarMatches.length > 0 ? (
            <>
              <p className={styles.sectionLabel}>Voci simili</p>
              <ul className={styles.list}>
                {similarMatches.map((item) => (
                  <li key={item.id}>
                    <button
                      type="button"
                      className={`${styles.option} ${styles.optionSimilar}`}
                      onClick={() => {
                        onSelect(item);
                        setQuery('');
                        setOpen(false);
                      }}
                    >
                      <span className={styles.optionName}>{item.name_it}</span>
                      {item.target_type_name ? (
                        <span className={styles.optionMeta}>{item.target_type_name}</span>
                      ) : null}
                    </button>
                  </li>
                ))}
              </ul>
            </>
          ) : null}
          {showCreateOption ? (
            <>
              {exactMatches.length > 0 || similarMatches.length > 0 ? (
                <div className={styles.divider} aria-hidden="true" />
              ) : null}
              <button
                type="button"
                className={styles.createOption}
                onClick={() => {
                  onCreateRequest(trimmed);
                  setQuery('');
                  setOpen(false);
                }}
              >
                <Icon name="plus" size={14} />
                <span>
                  Crea nuova voce: <strong>"{trimmed}"</strong>
                </span>
              </button>
            </>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function similarity(a: string, b: string): number {
  if (a === b) return 1;
  if (a.length === 0 || b.length === 0) return 0;
  const longer = a.length >= b.length ? a : b;
  const shorter = a.length >= b.length ? b : a;
  const distance = levenshtein(longer, shorter);
  return (longer.length - distance) / longer.length;
}

function levenshtein(a: string, b: string): number {
  const m = a.length;
  const n = b.length;
  if (m === 0) return n;
  if (n === 0) return m;
  let previous = Array.from({ length: n + 1 }, (_, j) => j);
  let current = new Array<number>(n + 1).fill(0);
  for (let i = 1; i <= m; i += 1) {
    current[0] = i;
    for (let j = 1; j <= n; j += 1) {
      const cost = a.charCodeAt(i - 1) === b.charCodeAt(j - 1) ? 0 : 1;
      current[j] = Math.min(
        (current[j - 1] ?? 0) + 1,
        (previous[j] ?? 0) + 1,
        (previous[j - 1] ?? 0) + cost,
      );
    }
    [previous, current] = [current, previous];
  }
  return previous[n] ?? 0;
}
