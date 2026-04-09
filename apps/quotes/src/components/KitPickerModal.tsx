import { useMemo, useState, useRef, useEffect } from 'react';
import { useKits } from '../api/queries';
import styles from './KitPickerModal.module.css';

interface KitPickerModalProps {
  onSelect: (kitId: number) => void;
  onClose: () => void;
}

export function KitPickerModal({ onSelect, onClose }: KitPickerModalProps) {
  const { data: kits } = useKits();
  const [search, setSearch] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => { inputRef.current?.focus(); }, []);

  const filtered = useMemo(() => {
    if (!kits) return [];
    if (!search) return kits;
    const q = search.toLowerCase();
    return kits.filter(k => k.internal_name.toLowerCase().includes(q));
  }, [kits, search]);

  // Group by category
  const grouped = useMemo(() => {
    const map = new Map<string, typeof filtered>();
    for (const k of filtered) {
      const cat = k.category_name ?? 'Altro';
      const list = map.get(cat) ?? [];
      list.push(k);
      map.set(cat, list);
    }
    return Array.from(map.entries());
  }, [filtered]);

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={e => e.stopPropagation()}>
        <div className={styles.header}>
          <span className={styles.title}>Aggiungi kit</span>
          <button className={styles.closeBtn} onClick={onClose}>&times;</button>
        </div>
        <div className={styles.search}>
          <input
            ref={inputRef}
            className={styles.searchInput}
            placeholder="Cerca kit..."
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
        </div>
        <div className={styles.list}>
          {grouped.map(([cat, items]) => (
            <div key={cat}>
              <div className={styles.category}>{cat}</div>
              {items.map(k => (
                <div key={k.id} className={styles.kitItem} onClick={() => { onSelect(k.id); onClose(); }}>
                  <span>{k.internal_name}</span>
                  <span className={styles.kitPrice}>
                    NRC {k.nrc.toFixed(2)} / MRC {k.mrc.toFixed(2)}
                  </span>
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
