import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Icon, type IconName } from '@mrsmith/ui';
import type { Template } from '../api/types';
import styles from './TemplatePicker.module.css';

interface TemplatePickerProps {
  templates: Template[] | undefined;
  selectedId: string;
  onChange: (templateId: string) => void;
  isLoading?: boolean;
}

type SectionKey = 'standard' | 'colocation' | 'iaas';

interface Section {
  key: SectionKey;
  label: string;
  icon: IconName;
  items: Template[];
}

const POPOVER_GAP = 6;
const VIEWPORT_PAD = 8;
const POPOVER_MAX_HEIGHT = 520;

const SECTION_ORDER: SectionKey[] = ['standard', 'colocation', 'iaas'];

function classifySection(t: Template): SectionKey | null {
  if (t.template_type === 'iaas') return 'iaas';
  if (t.template_type === 'standard') return t.is_colo ? 'colocation' : 'standard';
  return null;
}

function sectionLabel(key: SectionKey): string {
  switch (key) {
    case 'standard':
      return 'Standard';
    case 'colocation':
      return 'Colocation';
    case 'iaas':
      return 'IaaS';
  }
}

function sectionIcon(key: SectionKey): IconName {
  switch (key) {
    case 'standard':
      return 'package';
    case 'colocation':
      return 'server';
    case 'iaas':
      return 'cloud';
  }
}

function langLabel(lang: string | null | undefined): string {
  const normalized = (lang ?? '').trim().toLowerCase();
  if (normalized.startsWith('en')) return 'English';
  if (normalized.startsWith('it')) return 'Italiano';
  return normalized.toUpperCase() || '—';
}

export function TemplatePicker({
  templates,
  selectedId,
  onChange,
  isLoading,
}: TemplatePickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [coords, setCoords] = useState({ top: 0, left: 0, width: 0, placeTop: false });
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const popoverRef = useRef<HTMLDivElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);

  const selected = useMemo(
    () => templates?.find(t => t.template_id === selectedId) ?? null,
    [templates, selectedId],
  );

  const sections: Section[] = useMemo(() => {
    const list = templates ?? [];
    const filtered = search.trim()
      ? list.filter(t =>
          t.description.toLowerCase().includes(search.toLowerCase()),
        )
      : list;
    return SECTION_ORDER.map<Section>(key => ({
      key,
      label: sectionLabel(key),
      icon: sectionIcon(key),
      items: filtered.filter(t => classifySection(t) === key),
    }));
  }, [templates, search]);

  useEffect(() => {
    if (!open) return;
    function handleClickOutside(e: MouseEvent) {
      const target = e.target as Node;
      if (
        !triggerRef.current?.contains(target) &&
        !popoverRef.current?.contains(target)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  useLayoutEffect(() => {
    if (!open) return;
    const trigger = triggerRef.current;
    if (!trigger) return;

    const update = () => {
      const rect = trigger.getBoundingClientRect();
      const spaceBelow = window.innerHeight - rect.bottom - VIEWPORT_PAD;
      const spaceAbove = rect.top - VIEWPORT_PAD;
      const placeTop = spaceBelow < POPOVER_MAX_HEIGHT && spaceAbove > spaceBelow;
      const top = placeTop
        ? rect.top - POPOVER_GAP
        : rect.bottom + POPOVER_GAP;
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
    if (open) {
      requestAnimationFrame(() => searchRef.current?.focus());
    } else {
      setSearch('');
    }
  }, [open]);

  const selectedSection: SectionKey | null = selected ? classifySection(selected) : null;
  const totalCount = templates?.length ?? 0;

  return (
    <div className={styles.root}>
      {selected && selectedSection ? (
        <button
          ref={triggerRef}
          type="button"
          className={`${styles.trigger} ${styles[`trigger_${selectedSection}`]} ${open ? styles.triggerOpen : ''}`}
          onClick={() => setOpen(o => !o)}
          aria-haspopup="dialog"
          aria-expanded={open}
        >
          <span className={`${styles.triggerIcon} ${styles[`tint_${selectedSection}`]}`}>
            <Icon name={sectionIcon(selectedSection)} size={22} />
          </span>
          <span className={styles.triggerBody}>
            <span className={styles.triggerName}>{selected.description}</span>
            <span className={styles.triggerMeta}>
              Lingua {langLabel(selected.lang)}
            </span>
          </span>
          <span className={`${styles.familyPill} ${styles[`pill_${selectedSection}`]}`}>
            {sectionLabel(selectedSection)}
          </span>
          <span className={styles.triggerChevron}>
            <Icon name="chevron-down" size={16} />
          </span>
        </button>
      ) : (
        <button
          ref={triggerRef}
          type="button"
          className={`${styles.triggerEmpty} ${open ? styles.triggerOpen : ''}`}
          onClick={() => setOpen(o => !o)}
          aria-haspopup="dialog"
          aria-expanded={open}
          disabled={isLoading}
        >
          <span className={styles.triggerEmptyIcon}>
            <Icon name="package" size={22} />
          </span>
          <span className={styles.triggerEmptyBody}>
            <span className={styles.triggerEmptyTitle}>
              {isLoading ? 'Caricamento template...' : 'Scegli un template per iniziare'}
            </span>
            <span className={styles.triggerEmptySub}>
              Standard · Colocation · IaaS
            </span>
          </span>
          <span className={styles.triggerChevron}>
            <Icon name="chevron-down" size={16} />
          </span>
        </button>
      )}

      {open &&
        createPortal(
          <div
            ref={popoverRef}
            className={styles.popover}
            style={{
              top: coords.top,
              left: coords.left,
              width: Math.max(coords.width, 520),
              transform: coords.placeTop ? 'translateY(-100%)' : undefined,
            }}
            role="dialog"
            aria-label="Seleziona template"
          >
            <div className={styles.searchWrap}>
              <Icon name="search" size={16} />
              <input
                ref={searchRef}
                type="text"
                className={styles.searchInput}
                placeholder="Cerca template..."
                value={search}
                onChange={e => setSearch(e.target.value)}
              />
            </div>
            <div className={styles.bandsWrap}>
              {totalCount === 0 ? (
                <div className={styles.empty}>
                  {templates === undefined
                    ? 'Caricamento template...'
                    : 'Nessun template disponibile.'}
                </div>
              ) : (
                sections.map(section => (
                  <div
                    key={section.key}
                    className={`${styles.band} ${styles[`band_${section.key}`]}`}
                  >
                    <div className={styles.bandHeader}>
                      <span
                        className={`${styles.bandHeaderIcon} ${styles[`tint_${section.key}`]}`}
                      >
                        <Icon name={section.icon} size={14} />
                      </span>
                      <span className={styles.bandHeaderLabel}>
                        {section.label.toUpperCase()}
                      </span>
                    </div>
                    {section.items.length === 0 ? (
                      <div className={styles.bandEmpty}>
                        {search.trim()
                          ? 'Nessun risultato in questa famiglia.'
                          : `Nessun template ${section.label} per la lingua selezionata.`}
                      </div>
                    ) : (
                      <ul className={styles.rowList}>
                        {section.items.map(t => {
                          const isSelected = t.template_id === selectedId;
                          return (
                            <li key={t.template_id}>
                              <button
                                type="button"
                                className={`${styles.row} ${isSelected ? styles.rowSelected : ''} ${styles[`rowHover_${section.key}`]}`}
                                onClick={() => {
                                  onChange(t.template_id);
                                  setOpen(false);
                                }}
                              >
                                <span
                                  className={`${styles.rowIcon} ${styles[`tint_${section.key}`]}`}
                                >
                                  <Icon name={section.icon} size={16} />
                                </span>
                                <span className={styles.rowName}>{t.description}</span>
                                <span className={styles.rowMeta}>
                                  {langLabel(t.lang)}
                                </span>
                                {isSelected ? (
                                  <span
                                    className={`${styles.rowCheck} ${styles[`tint_${section.key}`]}`}
                                  >
                                    <Icon name="check" size={14} />
                                  </span>
                                ) : (
                                  <span className={styles.rowChevron}>
                                    <Icon name="chevron-right" size={14} />
                                  </span>
                                )}
                              </button>
                            </li>
                          );
                        })}
                      </ul>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>,
          document.body,
        )}
    </div>
  );
}
