import type { LookupItem } from '../../api/types';
import styles from './MemberMultiSelect.module.css';

interface MemberMultiSelectProps {
  people: LookupItem[];
  selectedIds: string[];
  query: string;
  onQueryChange: (query: string) => void;
  onChange: (ids: string[]) => void;
}

export function MemberMultiSelect({
  people,
  selectedIds,
  query,
  onQueryChange,
  onChange,
}: MemberMultiSelectProps) {
  const selected = new Set(selectedIds);
  const needle = query.trim().toLowerCase();
  const visible = people
    .filter((person) => person.active)
    .filter((person) => !needle || person.label.toLowerCase().includes(needle))
    .slice(0, 80);

  function toggle(id: string) {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onChange(Array.from(next));
  }

  return (
    <div className={styles.wrap}>
      <input
        className={styles.search}
        value={query}
        onChange={(event) => onQueryChange(event.target.value)}
        placeholder="Cerca persona"
        type="search"
      />
      <div className={styles.count}>{selected.size} persone selezionate</div>
      <ul className={styles.list}>
        {visible.map((person) => (
          <li key={person.id} className={styles.row}>
            <label>
              <input
                type="checkbox"
                checked={selected.has(person.id)}
                onChange={() => toggle(person.id)}
              />
              <span>{person.label}</span>
            </label>
          </li>
        ))}
      </ul>
    </div>
  );
}
