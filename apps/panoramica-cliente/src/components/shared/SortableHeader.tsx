import type { SortState } from '../../hooks/useSort';
import s from '../../pages/shared.module.css';

interface SortableHeaderProps<K> {
  label: string;
  sortKey: K;
  sort: SortState<K>;
  onToggle: (key: K) => void;
  className?: string;
}

export function SortableHeader<K>({ label, sortKey, sort, onToggle, className }: SortableHeaderProps<K>) {
  const active = sort.key === sortKey;
  return (
    <th className={className} onClick={() => onToggle(sortKey)}>
      {label}
      <span className={`${s.sortIndicator} ${active ? s.sortActive : ''}`}>
        {active ? (sort.dir === 'asc' ? '\u25B2' : '\u25BC') : '\u25B4'}
      </span>
    </th>
  );
}
