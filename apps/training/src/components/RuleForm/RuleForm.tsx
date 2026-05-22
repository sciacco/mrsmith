import { SingleSelect } from '@mrsmith/ui';
import type { CustomGroup, LookupItem, MandatoryRuleInput } from '../../api/types';
import { PopulationTargetSelector } from '../PopulationTargetSelector';
import styles from './RuleForm.module.css';

interface RuleFormProps {
  value: MandatoryRuleInput;
  courses: LookupItem[];
  teams: LookupItem[];
  skillAreas: LookupItem[];
  groups: CustomGroup[];
  onChange: (value: MandatoryRuleInput) => void;
}

export function RuleForm({ value, courses, teams, skillAreas, groups, onChange }: RuleFormProps) {
  const courseOptions = courses
    .filter((course) => course.active)
    .map((course) => ({ value: course.id, label: course.label }));

  return (
    <div className={styles.form}>
      <label className={styles.field}>
        <span>Nome regola</span>
        <input
          value={value.name}
          onChange={(event) => onChange({ ...value, name: event.target.value })}
          placeholder="Es. Sicurezza base"
        />
      </label>

      <label className={styles.field}>
        <span>Corso</span>
        <SingleSelect
          options={courseOptions}
          selected={value.course_id || null}
          onChange={(selected) => onChange({ ...value, course_id: selected ?? '' })}
          placeholder="Seleziona corso"
          searchable
        />
      </label>

      <div className={styles.field}>
        <span>Popolazione</span>
        <PopulationTargetSelector
          value={value.population_target}
          teams={teams}
          skillAreas={skillAreas}
          groups={groups}
          onChange={(target) => onChange({ ...value, population_target: target })}
        />
      </div>

      <label className={styles.toggle}>
        <input
          type="checkbox"
          checked={value.active ?? true}
          onChange={(event) => onChange({ ...value, active: event.target.checked })}
        />
        <span>Regola attiva</span>
      </label>

      <label className={styles.field}>
        <span>Note</span>
        <textarea
          rows={3}
          value={value.notes ?? ''}
          onChange={(event) => onChange({ ...value, notes: event.target.value })}
          placeholder="Indicazioni per il team People"
        />
      </label>
    </div>
  );
}
