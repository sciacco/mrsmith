import { useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { SingleSelect } from '@mrsmith/ui';
import { useTrainingPlans } from '../../api/queries';
import type { TrainingPlanListRow } from '../../api/types';
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
  const plans = useTrainingPlans(!yearOptions);
  const defaultYear = useMemo(
    () => resolveDefaultYear(plans.data?.plans ?? []),
    [plans.data],
  );
  const year = params.get('year') ?? defaultYear;
  const team = params.get('team');

  const years = useMemo(() => yearOptions ?? buildDefaultYearOptions(), [yearOptions]);
  const teams = useMemo(() => teamOptions ?? [], [teamOptions]);
  const planRows = plans.data?.plans ?? [];
  const nonClosedPlans = planRows.filter((plan) => plan.status !== 'closed');
  const latestDraft = [...planRows]
    .filter((plan) => plan.status === 'draft')
    .sort((a, b) => b.created_at.localeCompare(a.created_at))[0];

  useEffect(() => {
    if (params.get('year') || yearOptions) return;
    if (defaultYear) {
      setParams((previous) => {
        const next = new URLSearchParams(previous);
        next.set('year', defaultYear);
        return next;
      }, { replace: true });
    }
  }, [defaultYear, params, setParams, yearOptions]);

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
        {yearOptions ? (
          <SingleSelect
            options={years}
            selected={year}
            onChange={(value) => update('year', value)}
            placeholder="Anno"
          />
        ) : (
          <YearSelector
            plans={planRows}
            selectedYear={year}
            onChange={(value) => update('year', value)}
          />
        )}
        {nonClosedPlans.length >= 2 && latestDraft && String(latestDraft.year) !== year && (
          <button
            type="button"
            className={styles.multiPlan}
            onClick={() => update('year', String(latestDraft.year))}
          >
            Bozza {latestDraft.year}
          </button>
        )}
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

function resolveDefaultYear(plans: TrainingPlanListRow[]): string {
  const open = plans.find((plan) => plan.status === 'open');
  if (open) return String(open.year);
  const current = new Date().getFullYear();
  if (plans.some((plan) => plan.year === current)) return String(current);
  const latest = [...plans].sort((a, b) => b.created_at.localeCompare(a.created_at))[0];
  return String(latest?.year ?? current);
}

function YearSelector({
  plans,
  selectedYear,
  onChange,
}: {
  plans: TrainingPlanListRow[];
  selectedYear: string;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const currentYear = new Date().getFullYear();
  const options = useMemo(() => {
    const byYear = new Map<number, TrainingPlanListRow | null>();
    plans.forEach((plan) => byYear.set(plan.year, plan));
    if (!byYear.has(currentYear)) byYear.set(currentYear, null);
    if (!byYear.has(Number(selectedYear))) byYear.set(Number(selectedYear), null);
    return [...byYear.entries()].sort((a, b) => b[0] - a[0]);
  }, [plans, currentYear, selectedYear]);
  const selectedPlan = plans.find((plan) => String(plan.year) === selectedYear);

  useEffect(() => {
    if (!open) return;
    const onClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onClick);
    window.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onClick);
      window.removeEventListener('keydown', onKey);
    };
  }, [open]);

  return (
    <div className={styles.yearSelector} ref={rootRef}>
      <button
        type="button"
        className={styles.yearTrigger}
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
      >
        <span>{selectedYear}</span>
        <StatusBadge status={selectedPlan?.status ?? 'missing'} />
      </button>
      {open && (
        <div className={styles.yearMenu}>
          {options.map(([year, plan]) => (
            <button
              key={year}
              type="button"
              className={`${styles.yearOption} ${String(year) === selectedYear ? styles.yearOptionActive : ''}`}
              onClick={() => {
                onChange(String(year));
                setOpen(false);
              }}
            >
              <span>{year}</span>
              <StatusBadge status={plan?.status ?? 'missing'} />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: TrainingPlanListRow['status'] | 'missing' }) {
  return <span className={`${styles.statusBadge} ${styles[`status_${status}`]}`}>{statusLabel(status)}</span>;
}

function statusLabel(status: TrainingPlanListRow['status'] | 'missing'): string {
  switch (status) {
    case 'draft':
      return 'Bozza';
    case 'open':
      return 'Aperto';
    case 'frozen':
      return 'Congelato';
    case 'closed':
      return 'Chiuso';
    case 'missing':
      return 'Da creare';
  }
}
