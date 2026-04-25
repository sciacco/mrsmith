import { useMemo, useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import type { ServiceDependency } from '../api/types';
import { dependencyTypeLabel, severityLabel } from '../lib/format';
import styles from './GraphSuggestionsBanner.module.css';

interface Props {
  suggestions: ServiceDependency[];
  ignoredIds: Set<number>;
  onIgnore: (dependencyId: number) => void;
  onAccept: (items: ServiceDependency[]) => void;
}

export function GraphSuggestionsBanner({ suggestions, ignoredIds, onIgnore, onAccept }: Props) {
  const visible = useMemo(
    () => suggestions.filter((item) => !ignoredIds.has(item.service_dependency_id)),
    [ignoredIds, suggestions],
  );
  const [checked, setChecked] = useState<Record<number, boolean>>({});

  if (visible.length === 0) return null;

  const selected = visible.filter((item) => checked[item.service_dependency_id]);

  function toggle(id: number, value: boolean) {
    setChecked((current) => ({ ...current, [id]: value }));
  }

  function handleAddSelected() {
    if (selected.length === 0) return;
    onAccept(selected);
    setChecked({});
  }

  function handleAddAll() {
    onAccept(visible);
    setChecked({});
  }

  return (
    <section className={styles.banner} aria-label="Suggerimenti dal grafo dipendenze">
      <header className={styles.header}>
        <Icon name="info" size={16} className={styles.headerIcon} />
        <h4 className={styles.title}>
          Il grafo propone {visible.length}{' '}
          {visible.length === 1 ? 'effetto su altri sistemi' : 'effetti su altri sistemi'}
        </h4>
      </header>
      <ul className={styles.list}>
        {visible.map((item) => {
          const id = item.service_dependency_id;
          const isChecked = checked[id] === true;
          return (
            <li key={id} className={styles.row}>
              <label className={styles.rowMain}>
                <input
                  type="checkbox"
                  checked={isChecked}
                  onChange={(event) => toggle(id, event.target.checked)}
                  className={styles.checkbox}
                />
                <span className={styles.rowName}>{item.downstream_service.name_it}</span>
                <span className={styles.rowReason}>
                  ← {item.upstream_service.name_it} · {dependencyTypeLabel(item.dependency_type)}
                </span>
                <span className={styles.rowSeverity}>
                  {severityLabel(item.default_severity)} (default)
                </span>
              </label>
              <button
                type="button"
                className={styles.ignore}
                onClick={() => onIgnore(id)}
                title="Ignora questa proposta"
                aria-label="Ignora questa proposta"
              >
                <Icon name="x" size={14} />
              </button>
            </li>
          );
        })}
      </ul>
      <div className={styles.actions}>
        <Button size="sm" variant="secondary" onClick={handleAddSelected} disabled={selected.length === 0}>
          Aggiungi selezionati ({selected.length})
        </Button>
        <Button size="sm" onClick={handleAddAll}>
          Aggiungi tutti
        </Button>
      </div>
    </section>
  );
}
