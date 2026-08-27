// Editor regola formativa (#157, §Regole 2): platea XOR posizioni, ricorrenza
// in mesi + àncora sempre in coppia (validazione inline), natura del bisogno
// derivata dal corso mostrata accanto alla scelta. Attiva/disattiva restano
// gesti dedicati, non campi del form (idioma PUT del backend).

import { useState } from 'react';
import { Button, Modal, MultiSelect, SingleSelect, ToggleSwitch, VisuallyHidden } from '@mrsmith/ui';
import {
  useCreateRule,
  useTrainingCourses,
  useTrainingGroups,
  useTrainingLookups,
  useTrainingSkillAreas,
  useTrainingTeams,
  useUpdateRule,
} from '../../api/queries';
import type { PopulationKind, RuleDetail, RuleInput } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { NEED_LABELS, POPULATION_KIND_LABELS } from '../../lib/labels';
import styles from '../requests/requestShared.module.css';
import ruleStyles from './ruleShared.module.css';

const POPULATION_KINDS: PopulationKind[] = ['all', 'team', 'skill_area', 'custom_group', 'people'];

interface RuleEditorModalProps {
  open: boolean;
  mode: 'create' | 'edit';
  ruleId?: string;
  initial?: RuleDetail;
  onClose: () => void;
  onSaved: (id: string) => void;
}

export function RuleEditorModal({ open, mode, ruleId, initial, onClose, onSaved }: RuleEditorModalProps) {
  const courses = useTrainingCourses();
  const teams = useTrainingTeams();
  const skillAreas = useTrainingSkillAreas();
  const groups = useTrainingGroups();
  const lookups = useTrainingLookups();
  const createRule = useCreateRule();
  const updateRule = useUpdateRule();

  const [name, setName] = useState(initial?.name ?? '');
  const [courseId, setCourseId] = useState(initial?.courseId ?? '');
  const [populationMode, setPopulationMode] = useState<'population' | 'seats'>(initial?.seats ? 'seats' : 'population');
  const [populationKind, setPopulationKind] = useState<PopulationKind>(initial?.population?.kind ?? 'all');
  const [targetId, setTargetId] = useState(initial?.population?.targetId ?? '');
  const [personIds, setPersonIds] = useState<string[]>(initial?.population?.personIds ?? []);
  const [seatCount, setSeatCount] = useState(initial?.seats ? String(initial.seats.requested) : '');
  const [isMandatory, setIsMandatory] = useState(initial?.isMandatory ?? false);
  const [deadline, setDeadline] = useState(initial?.deadline ?? '');
  const [recurrenceMonths, setRecurrenceMonths] = useState(
    initial?.recurrenceMonths !== undefined ? String(initial.recurrenceMonths) : '',
  );
  const [recurrenceAnchor, setRecurrenceAnchor] = useState<'calendar' | 'completion' | ''>(
    (initial?.recurrenceAnchor as 'calendar' | 'completion' | undefined) ?? '',
  );
  const [notes, setNotes] = useState(initial?.notes ?? '');
  const [error, setError] = useState<string | null>(null);

  if (!open) return null;

  const pending = createRule.isPending || updateRule.isPending;
  const selectedCourse = (courses.data ?? []).find((c) => c.id === courseId);
  const needLabel = initial
    ? NEED_LABELS[initial.need]
    : selectedCourse
      ? NEED_LABELS[selectedCourse.leadsToCertId ? 'certification' : 'attendance']
      : undefined;

  const recurrencePairError =
    (recurrenceMonths !== '') !== (recurrenceAnchor !== '')
      ? 'Ricorrenza in mesi e àncora vanno indicate insieme.'
      : null;

  const populationValid =
    populationMode === 'seats'
      ? Number(seatCount) > 0
      : populationKind === 'all' || (populationKind === 'people' ? personIds.length > 0 : targetId !== '');

  const canSubmit =
    name.trim() !== '' && courseId !== '' && deadline !== '' && populationValid && recurrencePairError === null;

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    const input: RuleInput = {
      name: name.trim(),
      courseId,
      isMandatory,
      deadline,
      recurrenceMonths: recurrenceMonths !== '' ? Number(recurrenceMonths) : undefined,
      recurrenceAnchor: recurrenceAnchor || undefined,
      notes: notes.trim() || undefined,
      population:
        populationMode === 'population'
          ? {
              kind: populationKind,
              id: populationKind === 'all' || populationKind === 'people' ? undefined : targetId,
              personIds: populationKind === 'people' ? personIds : undefined,
            }
          : undefined,
      seatCount: populationMode === 'seats' ? Number(seatCount) : undefined,
    };
    try {
      const response =
        mode === 'edit' && ruleId
          ? await updateRule.mutateAsync({ id: ruleId, input })
          : await createRule.mutateAsync(input);
      if (response.id) onSaved(response.id);
      else if (ruleId) onSaved(ruleId);
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={mode === 'create' ? 'Nuova regola' : 'Modifica regola'} size="wide">
      <div className={styles.body}>
        <label className={styles.field}>
          <span className={styles.labelHead}>
            Nome
            <span className={styles.requiredMarker} aria-hidden="true" />
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </span>
          <input className={styles.input} value={name} onChange={(e) => setName(e.target.value)} />
        </label>

        <div className={styles.row}>
          <label className={styles.field}>
            <span className={styles.labelHead}>
              Corso
              <span className={styles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <SingleSelect
              options={(courses.data ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.title }))}
              selected={courseId || null}
              onChange={(v) => setCourseId(v ?? '')}
              placeholder="Seleziona corso..."
              searchable
            />
          </label>
          {needLabel && (
            <label className={styles.field}>
              Natura del bisogno
              <span className={styles.hint}>{needLabel} — derivata dal corso</span>
            </label>
          )}
        </div>

        <div className={styles.toggleGroup}>
          <Button
            variant={populationMode === 'population' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setPopulationMode('population')}
          >
            Platea
          </Button>
          <Button
            variant={populationMode === 'seats' ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setPopulationMode('seats')}
          >
            Posizioni
          </Button>
        </div>

        {populationMode === 'population' ? (
          <>
            <label className={styles.field}>
              Tipo di platea
              <SingleSelect
                options={POPULATION_KINDS.map((k) => ({ value: k, label: POPULATION_KIND_LABELS[k] ?? k }))}
                selected={populationKind}
                onChange={(v) => {
                  setPopulationKind((v as PopulationKind) ?? 'all');
                  setTargetId('');
                  setPersonIds([]);
                }}
              />
            </label>
            {populationKind === 'team' && (
              <label className={styles.field}>
                Team
                <SingleSelect
                  options={(teams.data ?? []).map((t) => ({ value: t.id, label: t.name }))}
                  selected={targetId || null}
                  onChange={(v) => setTargetId(v ?? '')}
                  placeholder="Seleziona team..."
                />
              </label>
            )}
            {populationKind === 'skill_area' && (
              <label className={styles.field}>
                Area di competenza
                <SingleSelect
                  options={(skillAreas.data ?? []).map((a) => ({ value: a.id, label: a.name }))}
                  selected={targetId || null}
                  onChange={(v) => setTargetId(v ?? '')}
                  placeholder="Seleziona area..."
                />
                <span className={styles.hint}>
                  Un'area senza gruppo locale collegato non è utilizzabile come platea: il collegamento si fa dal
                  catalogo.
                </span>
              </label>
            )}
            {populationKind === 'custom_group' && (
              <label className={styles.field}>
                Gruppo locale
                <SingleSelect
                  options={(groups.data ?? []).map((g) => ({ value: g.id, label: g.name }))}
                  selected={targetId || null}
                  onChange={(v) => setTargetId(v ?? '')}
                  placeholder="Seleziona gruppo..."
                />
              </label>
            )}
            {populationKind === 'people' && (
              <label className={styles.field}>
                <span className={styles.labelHead}>
                  Persone
                  <span className={styles.requiredMarker} aria-hidden="true" />
                  <VisuallyHidden>obbligatorio</VisuallyHidden>
                </span>
                <MultiSelect<string>
                  options={(lookups.data?.employees ?? []).filter((e) => e.active).map((e) => ({ value: e.id, label: e.label }))}
                  selected={personIds}
                  onChange={setPersonIds}
                  placeholder="Seleziona persone..."
                />
              </label>
            )}
          </>
        ) : (
          <label className={styles.field}>
            <span className={styles.labelHead}>
              Numero di posizioni
              <span className={styles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input
              type="number"
              min={1}
              className={styles.input}
              value={seatCount}
              onChange={(e) => setSeatCount(e.target.value)}
            />
          </label>
        )}

        <div className={ruleStyles.toggleRow}>
          Obbligatoria
          <ToggleSwitch id="rule-mandatory" checked={isMandatory} onChange={setIsMandatory} />
        </div>

        <div className={styles.row}>
          <label className={styles.field}>
            <span className={styles.labelHead}>
              Scadenza
              <span className={styles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input type="date" className={styles.input} value={deadline} onChange={(e) => setDeadline(e.target.value)} />
          </label>
        </div>

        <div className={styles.row}>
          <label className={styles.field}>
            Ricorrenza — mesi
            <input
              type="number"
              min={1}
              className={styles.input}
              value={recurrenceMonths}
              onChange={(e) => setRecurrenceMonths(e.target.value)}
              aria-invalid={recurrencePairError ? 'true' : undefined}
            />
          </label>
          <label className={styles.field}>
            Ricorrenza — àncora
            <SingleSelect
              options={[
                { value: 'calendar', label: 'Da calendario' },
                { value: 'completion', label: 'Da completamento' },
              ]}
              selected={recurrenceAnchor || null}
              onChange={(v) => setRecurrenceAnchor((v as 'calendar' | 'completion') ?? '')}
              placeholder="Nessuna"
              allowClear
            />
          </label>
        </div>
        {recurrencePairError && <span className={ruleStyles.errorText}>{recurrencePairError}</span>}

        <label className={styles.field}>
          Note
          <textarea className={styles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
        </label>

        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={pending} disabled={!canSubmit} onClick={submit}>
            {mode === 'create' ? 'Crea regola' : 'Salva modifiche'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
