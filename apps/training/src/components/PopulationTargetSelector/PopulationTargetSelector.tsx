import { SingleSelect } from '@mrsmith/ui';
import type { CustomGroup, LookupItem, PopulationKind, PopulationTarget } from '../../api/types';
import styles from './PopulationTargetSelector.module.css';

interface PopulationTargetSelectorProps {
  value: PopulationTarget;
  teams: LookupItem[];
  skillAreas: LookupItem[];
  groups: CustomGroup[];
  onChange: (target: PopulationTarget) => void;
}

const KIND_OPTIONS: Array<{ kind: PopulationKind; label: string }> = [
  { kind: 'all', label: 'Tutte' },
  { kind: 'team', label: 'Team' },
  { kind: 'skill_area', label: 'Skill area' },
  { kind: 'custom_group', label: 'Gruppo' },
];

export function PopulationTargetSelector({
  value,
  teams,
  skillAreas,
  groups,
  onChange,
}: PopulationTargetSelectorProps) {
  const kind = value.kind || 'all';
  const options =
    kind === 'team'
      ? teams.filter((team) => team.active).map((team) => ({ value: team.id, label: team.label }))
      : kind === 'skill_area'
      ? skillAreas.filter((area) => area.active).map((area) => ({ value: area.id, label: area.label }))
      : kind === 'custom_group'
      ? groups.filter((group) => group.active).map((group) => ({ value: group.id, label: `${group.name} (${group.member_count})` }))
      : [];

  return (
    <div className={styles.wrap}>
      <div className={styles.segmented} role="tablist" aria-label="Popolazione">
        {KIND_OPTIONS.map((option) => (
          <button
            key={option.kind}
            type="button"
            className={`${styles.segment} ${kind === option.kind ? styles.segmentActive : ''}`}
            onClick={() => onChange({ kind: option.kind })}
          >
            {option.label}
          </button>
        ))}
      </div>
      {kind !== 'all' && (
        <SingleSelect
          options={options}
          selected={value.id ?? null}
          onChange={(selected) => onChange({ kind, id: selected ?? undefined })}
          placeholder={
            kind === 'team'
              ? 'Seleziona team'
              : kind === 'skill_area'
              ? 'Seleziona skill area'
              : 'Seleziona gruppo'
          }
          searchable
        />
      )}
    </div>
  );
}
