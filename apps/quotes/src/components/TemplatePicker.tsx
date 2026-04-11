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
const POPOVER_MAX_HEIGHT = 480;

function classifySection(t: Template): SectionKey {
  if (t.template_type === 'iaas') return 'iaas';
  if (t.is_colo) return 'colocation';
  return 'standard';
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
      return 'box';
    case 'iaas':
      return 'cloud';
  }
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
      ? list.filter(t => t.description.toLowerCase().includes(search.toLowerCase()))
      : list;
    const order: SectionKey[] = ['standard', 'colocation', 'iaas'];
    return order
      .map<Section>(key => ({
        key,
        label: sectionLabel(key),
        icon: sectionIcon(key),
        items: filtered.filter(t => classifySection(t) === key),
      }))
      .filter(s => s.items.length > 0);
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

  return (
    <div className={styles.root}>
      {selected && selectedSection ? (
        <button
          ref={triggerRef}
          type="button"
          className={`${styles.trigger} ${open ? styles.triggerOpen : ''}`}
          onClick={() => setOpen(o => !o)}
          aria-haspopup="dialog"
          aria-expanded={open}
        >
          <span className={`${styles.triggerIcon} ${styles[`icon_${selectedSection}`]}`}>
            <Icon name={sectionIcon(selectedSection)} size={20} />
          </span>
          <span className={styles.triggerBody}>
            <span className={styles.triggerName}>{selected.description}</span>
            <span className={styles.triggerSub}>{sectionLabel(selectedSection)}</span>
          </span>
          <span className={styles.triggerCta}>
            Cambia
            <Icon name="chevron-down" size={14} />
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
          <Icon name="package" size={18} />
          <span>
            {isLoading ? 'Caricamento template...' : 'Scegli un template per iniziare'}
          </span>
          <Icon name="chevron-down" size={14} />
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
              width: Math.max(coords.width, 480),
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
            <div className={styles.sectionsWrap}>
              {sections.length === 0 ? (
                <div className={styles.empty}>
                  {templates === undefined
                    ? 'Caricamento template...'
                    : 'Nessun template trovato.'}
                </div>
              ) : (
                sections.map(section => (
                  <div key={section.key} className={styles.section}>
                    <div className={styles.sectionHeader}>{section.label.toUpperCase()}</div>
                    <div className={styles.cardGrid}>
                      {section.items.map(t => {
                        const isSelected = t.template_id === selectedId;
                        return (
                          <button
                            key={t.template_id}
                            type="button"
                            className={`${styles.card} ${isSelected ? styles.cardSelected : ''}`}
                            onClick={() => {
                              onChange(t.template_id);
                              setOpen(false);
                            }}
                            title={t.description}
                          >
                            <span
                              className={`${styles.cardIcon} ${styles[`icon_${section.key}`]}`}
                            >
                              <Icon name={section.icon} size={18} />
                            </span>
                            <span className={styles.cardName}>{t.description}</span>
                            {isSelected && (
                              <span className={styles.cardCheck} aria-hidden="true">
                                <Icon name="check" size={14} />
                              </span>
                            )}
                          </button>
                        );
                      })}
                    </div>
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
