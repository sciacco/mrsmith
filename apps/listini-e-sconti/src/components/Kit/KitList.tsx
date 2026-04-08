import { useState, useMemo } from 'react';
import { SearchInput } from '@mrsmith/ui';
import type { Kit } from '../../types';
import styles from './KitList.module.css';

interface KitListProps {
  kits: Kit[];
  selectedId: number | null;
  onSelect: (id: number) => void;
}

export function KitList({ kits, selectedId, onSelect }: KitListProps) {
  const [search, setSearch] = useState('');
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  const filtered = useMemo(() => {
    if (!search) return kits;
    const q = search.toLowerCase();
    return kits.filter((k) => k.internal_name.toLowerCase().includes(q));
  }, [kits, search]);

  // Group by category
  const grouped = useMemo(() => {
    const map = new Map<string, { name: string; color: string; kits: Kit[] }>();
    for (const kit of filtered) {
      let group = map.get(kit.category_name);
      if (!group) {
        group = { name: kit.category_name, color: kit.category_color, kits: [] };
        map.set(kit.category_name, group);
      }
      group.kits.push(kit);
    }
    return Array.from(map.values());
  }, [filtered]);

  function toggleCategory(name: string) {
    setCollapsed((prev) => ({ ...prev, [name]: !prev[name] }));
  }

  return (
    <div className={styles.list}>
      <div className={styles.searchBox}>
        <SearchInput value={search} onChange={setSearch} placeholder="Cerca kit..." />
      </div>
      <div className={styles.groups}>
        {grouped.map((group) => (
          <div key={group.name} className={styles.group}>
            <button
              className={styles.groupHeader}
              onClick={() => toggleCategory(group.name)}
              type="button"
            >
              <span
                className={styles.categoryDot}
                style={{ background: group.color || 'var(--color-text-muted)' }}
              />
              <span className={styles.categoryName}>{group.name}</span>
              <span className={styles.count}>{group.kits.length}</span>
              <svg
                className={`${styles.chevron} ${collapsed[group.name] ? styles.collapsed : ''}`}
                width="12" height="12" viewBox="0 0 12 12" fill="none"
              >
                <path d="M3 4.5L6 7.5L9 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
            {!collapsed[group.name] && (
              <div className={styles.items}>
                {group.kits.map((kit) => (
                  <button
                    key={kit.id}
                    className={`${styles.item} ${kit.id === selectedId ? styles.selected : ''}`}
                    onClick={() => onSelect(kit.id)}
                    type="button"
                  >
                    {kit.internal_name}
                  </button>
                ))}
              </div>
            )}
          </div>
        ))}
        {grouped.length === 0 && (
          <p className={styles.empty}>Nessun kit trovato</p>
        )}
      </div>
    </div>
  );
}
