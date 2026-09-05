// Registrazione di una richiesta formativa per conto della persona (#157,
// §Richieste 3). Dati original-only: nessuna modifica dopo la creazione, il
// backend non la espone. Il team si precompila quando l'appartenenza attiva
// e unica, altrimenti si sceglie tra quelle della persona.

import { useMemo, useState } from 'react';
import { Button, Modal, MultiSelect, SingleSelect, VisuallyHidden } from '@mrsmith/ui';
import { useCreateRequest, useTrainingLookups, useTrainingPeople, useTrainingSkillAreas } from '../../api/queries';
import { LEVEL_OPTIONS } from '../../lib/levels';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import styles from './requestShared.module.css';

interface RequestCreateModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: (id: string) => void;
}

export function RequestCreateModal({ open, onClose, onCreated }: RequestCreateModalProps) {
  const people = useTrainingPeople();
  const lookups = useTrainingLookups();
  const skillAreas = useTrainingSkillAreas();
  const createRequest = useCreateRequest();

  const [employeeId, setEmployeeId] = useState('');
  const [teamId, setTeamId] = useState('');
  const [courseMode, setCourseMode] = useState<'catalog' | 'new'>('catalog');
  const [courseId, setCourseId] = useState('');
  const [newCourseTitle, setNewCourseTitle] = useState('');
  const [skillAreaIds, setSkillAreaIds] = useState<string[]>([]);
  const [areaLevels, setAreaLevels] = useState<Record<string, { current: string; target: string }>>({});
  const [priority, setPriority] = useState('');
  const [motivation, setMotivation] = useState('');
  const [desiredStart, setDesiredStart] = useState('');
  const [desiredEnd, setDesiredEnd] = useState('');
  const [error, setError] = useState<string | null>(null);

  const person = (people.data ?? []).find((p) => p.id === employeeId);
  const activeTeams = person?.teams ?? [];

  function handleEmployeeChange(id: string | null) {
    setEmployeeId(id ?? '');
    const teams = (people.data ?? []).find((p) => p.id === id)?.teams ?? [];
    setTeamId(teams.length === 1 ? (teams[0]?.id ?? '') : '');
  }

  const canSubmit =
    employeeId !== '' &&
    teamId !== '' &&
    motivation.trim() !== '' &&
    (courseMode === 'catalog' ? courseId !== '' : newCourseTitle.trim() !== '');

  const employeeOptions = useMemo(
    () =>
      (people.data ?? [])
        .filter((p) => p.status === 'active')
        .map((p) => ({ value: p.id, label: `${p.lastName} ${p.firstName}` })),
    [people.data],
  );

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    try {
      const response = await createRequest.mutateAsync({
        employeeId,
        selectedTeamId: teamId,
        courseId: courseMode === 'catalog' ? courseId : undefined,
        newCourseTitle: courseMode === 'new' ? newCourseTitle.trim() : undefined,
        skillAreas: skillAreaIds.map((areaId) => ({
          id: areaId,
          levelCurrent: areaLevels[areaId]?.current !== undefined && areaLevels[areaId]?.current !== ''
            ? Number(areaLevels[areaId]?.current)
            : undefined,
          levelTarget: areaLevels[areaId]?.target !== undefined && areaLevels[areaId]?.target !== ''
            ? Number(areaLevels[areaId]?.target)
            : undefined,
        })),
        priority: priority !== '' ? Number(priority) : undefined,
        motivation: motivation.trim(),
        desiredStart: desiredStart || undefined,
        desiredEnd: desiredEnd || undefined,
      });
      if (response.id) onCreated(response.id);
    } catch (e) {
      setError(describeApiError(e, 'Registrazione non riuscita'));
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Registra richiesta" size="md">
      <div className={`${styles.body} ${styles.bodyModal}`}>
        <label className={styles.field}>
          <span className={styles.labelHead}>
            Persona
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <SingleSelect
            options={employeeOptions}
            selected={employeeId || null}
            onChange={handleEmployeeChange}
            placeholder="Seleziona persona..."
            searchable
          />
        </label>
        <label className={styles.field}>
          <span className={styles.labelHead}>
            Team
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <SingleSelect
            options={activeTeams.map((t) => ({ value: t.id, label: t.name }))}
            selected={teamId || null}
            onChange={(v) => setTeamId(v ?? '')}
            placeholder={employeeId === '' ? 'Seleziona prima la persona' : 'Seleziona team...'}
            disabled={activeTeams.length <= 1}
          />
          {employeeId !== '' && activeTeams.length === 0 && (
            <span className={styles.hint}>La persona non ha appartenenze attive a nessun team.</span>
          )}
        </label>

        <div className={styles.toggleGroup}>
          <Button
            type="button"
            variant={courseMode === 'catalog' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setCourseMode('catalog')}
          >
            Corso a catalogo
          </Button>
          <Button
            type="button"
            variant={courseMode === 'new' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setCourseMode('new')}
          >
            Nuovo corso
          </Button>
        </div>
        {courseMode === 'catalog' ? (
          <label className={styles.field}>
            Corso
            <SingleSelect
              options={(lookups.data?.courses ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.label }))}
              selected={courseId || null}
              onChange={(v) => setCourseId(v ?? '')}
              placeholder="Seleziona corso..."
              searchable
            />
          </label>
        ) : (
          <label className={styles.field}>
            Titolo
            <input
              className={styles.input}
              value={newCourseTitle}
              onChange={(e) => setNewCourseTitle(e.target.value)}
              placeholder="Titolo della formazione desiderata"
            />
            <span className={styles.hint}>
              Il titolo entra a catalogo come corso da completare; se esiste già un corso con lo stesso nome, la richiesta si aggancia a quello.
            </span>
          </label>
        )}

        <label className={styles.field}>
          Aree di competenza
          <MultiSelect<string>
            options={(skillAreas.data ?? []).map((a) => ({ value: a.id, label: a.name }))}
            selected={skillAreaIds}
            onChange={setSkillAreaIds}
            placeholder="Nessuna area"
          />
        </label>
        {skillAreaIds.map((areaId) => {
          const area = (skillAreas.data ?? []).find((a) => a.id === areaId);
          const areaName = area?.name ?? 'Area';
          const levels = areaLevels[areaId] ?? { current: '', target: '' };
          return (
            <div className={styles.field} key={areaId}>
              <span className={styles.labelHead}>{areaName}</span>
              <div className={styles.row}>
                <label className={styles.field}>
                  <span className={styles.labelHead}>Attuale</span>
                  <SingleSelect<number>
                    options={LEVEL_OPTIONS}
                    selected={levels.current !== '' ? Number(levels.current) : null}
                    onChange={(v) => setAreaLevels({ ...areaLevels, [areaId]: { ...levels, current: v !== null ? String(v) : '' } })}
                    placeholder="Non indicato"
                    allowClear
                    clearLabel="Non indicato"
                    ariaLabel={`${areaName} — livello attuale`}
                  />
                </label>
                <label className={styles.field}>
                  <span className={styles.labelHead}>Atteso</span>
                  <SingleSelect<number>
                    options={LEVEL_OPTIONS}
                    selected={levels.target !== '' ? Number(levels.target) : null}
                    onChange={(v) => setAreaLevels({ ...areaLevels, [areaId]: { ...levels, target: v !== null ? String(v) : '' } })}
                    placeholder="Non indicato"
                    allowClear
                    clearLabel="Non indicato"
                    ariaLabel={`${areaName} — livello atteso`}
                  />
                </label>
              </div>
            </div>
          );
        })}

        <label className={styles.field}>
          Priorità (1 = più importante)
          <input
            type="number"
            min={1}
            className={styles.input}
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
          />
          <span className={styles.hint}>Facoltativa: lascia vuoto se non è stata indicata.</span>
        </label>

        <label className={styles.field}>
          <span className={styles.labelHead}>
            Motivazione
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <textarea
            className={styles.textarea}
            value={motivation}
            onChange={(e) => setMotivation(e.target.value)}
            rows={3}
          />
        </label>

        <div className={styles.row}>
          <label className={styles.field}>
            Inizio desiderato
            <input
              type="date"
              className={styles.input}
              value={desiredStart}
              onChange={(e) => setDesiredStart(e.target.value)}
            />
          </label>
          <label className={styles.field}>
            Fine desiderata
            <input
              type="date"
              className={styles.input}
              value={desiredEnd}
              onChange={(e) => setDesiredEnd(e.target.value)}
            />
          </label>
        </div>

        <ErrorPanel message={error} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={createRequest.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={createRequest.isPending} disabled={!canSubmit} onClick={submit}>
            Registra richiesta
          </Button>
        </div>
      </div>
    </Modal>
  );
}
