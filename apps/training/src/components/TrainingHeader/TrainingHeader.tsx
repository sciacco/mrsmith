import { useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { SingleSelect } from '@mrsmith/ui';
import styles from './TrainingHeader.module.css';

interface TrainingHeaderProps {
  yearOptions?: Array<{ value: string; label: string }>;
  teamOptions?: Array<{ value: string; label: string }>;
}

function buildDefaultYearOptions(): Array<{ value: string; label: string }> {
  const current = new Date().getFullYear();
  return [current - 1, current, current + 1].map((y) => ({ value: String(y), label: String(y) }));
}

export function TrainingHeader({ yearOptions, teamOptions }: TrainingHeaderProps) {
  const [params, setParams] = useSearchParams();
  const year = params.get('year') ?? String(new Date().getFullYear());
  const team = params.get('team');

  const years = useMemo(() => yearOptions ?? buildDefaultYearOptions(), [yearOptions]);
  const teams = useMemo(() => teamOptions ?? [], [teamOptions]);

  function update(key: string, value: string | null) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (value && value !== '') next.set(key, value);
      else next.delete(key);
      return next;
    }, { replace: true });
  }

  return (
    <div className={styles.header} role="region" aria-label="Filtri formazione">
      <div className={styles.group}>
        <span className={styles.label}>Anno</span>
        <SingleSelect
          options={years}
          selected={year}
          onChange={(value) => update('year', value)}
          placeholder="Anno"
        />
      </div>
      <div className={styles.group}>
        <span className={styles.label}>Team</span>
        <SingleSelect
          options={teams}
          selected={team}
          onChange={(value) => update('team', value)}
          placeholder="Tutti i team"
          allowClear
        />
      </div>
    </div>
  );
}
