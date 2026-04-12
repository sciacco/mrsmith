import { useMemo, useState, useRef, useEffect } from 'react';
import { Modal, SearchInput } from '@mrsmith/ui';
import { useKits } from '../api/queries';
import { billingPeriodLabel } from '../utils/kitLabels';
import styles from './KitPickerModal.module.css';

interface KitPickerModalProps {
  open: boolean;
  onSelect: (kitId: number) => void;
  onClose: () => void;
}

export function KitPickerModal({ open, onSelect, onClose }: KitPickerModalProps) {
  const { data: kits } = useKits();
  const [search, setSearch] = useState('');
  const searchWrapperRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open) {
      const id = window.setTimeout(() => {
        searchWrapperRef.current?.querySelector('input')?.focus();
      }, 60);
      return () => window.clearTimeout(id);
    }
    setSearch('');
  }, [open]);

  const filtered = useMemo(() => {
    if (!kits) return [];
    if (!search) return kits;
    const q = search.toLowerCase();
    return kits.filter(k =>
      k.internal_name.toLowerCase().includes(q) ||
      (k.category_name?.toLowerCase().includes(q) ?? false),
    );
  }, [kits, search]);

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
    <Modal open={open} onClose={onClose} title="Aggiungi kit" size="lg">
      <div className={styles.search} ref={searchWrapperRef}>
        <SearchInput value={search} onChange={setSearch} placeholder="Cerca kit..." />
      </div>
      <div className={styles.list}>
        {grouped.length === 0 && (
          <div className={styles.empty}>Nessun kit selezionabile disponibile.</div>
        )}
        {grouped.map(([cat, items]) => (
          <div key={cat}>
            <div className={styles.category}>{cat}</div>
            {items.map(k => (
              <button
                key={k.id}
                type="button"
                className={styles.kitItem}
                onClick={() => { onSelect(k.id); onClose(); }}
              >
                <span className={styles.kitName}>{k.internal_name}</span>
                <span className={styles.kitMeta}>
                  {billingPeriodLabel(k.billing_period)} · {k.activation_time_days}gg · {k.initial_subscription_months}/{k.next_subscription_months}m
                </span>
              </button>
            ))}
          </div>
        ))}
      </div>
    </Modal>
  );
}
