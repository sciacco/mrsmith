import { useEffect, useMemo, useState } from 'react';
import { Button, Drawer, SingleSelect, useToast } from '@mrsmith/ui';
import {
  useBulkPlanFromSuggestion,
  useBulkReviewEmployeeRequests,
  useTrainingLookups,
  useTrainingWorkspace,
} from '../../api/queries';
import type { PlanningSuggestion, TrainingRequest } from '../../api/types';
import styles from './ReviewActionDrawer.module.css';

export interface CreateFromSuggestionConfig {
  mode: 'create_from_suggestion';
  suggestion: PlanningSuggestion;
  year: number;
  /** Optional: override suggested course (used by Compliance section to pass rule_id-based defaults). */
  overrideCourseId?: string;
  overrideEmployeeIds?: string[];
  overrideTitle?: string;
}

export interface ReviewEmployeeRequestsConfig {
  mode: 'review_employee_requests';
  year: number;
  requestIds?: string[];
}

export type ReviewActionDrawerProps = {
  open: boolean;
  onClose: () => void;
  onCompleted?: (created: number) => void;
} & (CreateFromSuggestionConfig | ReviewEmployeeRequestsConfig);

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}

export function ReviewActionDrawer(props: ReviewActionDrawerProps) {
  if (!props.open) {
    return (
      <Drawer open={false} onClose={props.onClose} size="lg">
        {null}
      </Drawer>
    );
  }
  if (props.mode === 'create_from_suggestion') {
    return <CreateFromSuggestion {...props} />;
  }
  return <ReviewRequests {...props} />;
}

function CreateFromSuggestion({
  open,
  onClose,
  onCompleted,
  suggestion,
  year,
  overrideCourseId,
  overrideEmployeeIds,
  overrideTitle,
}: { open: boolean; onClose: () => void; onCompleted?: (created: number) => void } & CreateFromSuggestionConfig) {
  const { toast } = useToast();
  const lookups = useTrainingLookups(true);
  const bulkPlan = useBulkPlanFromSuggestion();

  const employeeIds = useMemo(
    () => overrideEmployeeIds ?? suggestion.affected_employee_ids,
    [overrideEmployeeIds, suggestion.affected_employee_ids],
  );

  const employeeLookup = useMemo(() => {
    const map = new Map<string, string>();
    (lookups.data?.employees ?? []).forEach((e) => map.set(e.id, e.label));
    return map;
  }, [lookups.data]);

  const courseOptions = useMemo(
    () =>
      (lookups.data?.courses ?? [])
        .filter((c) => c.active)
        .map((c) => ({ value: c.id, label: c.label })),
    [lookups.data],
  );

  const [courseId, setCourseId] = useState<string>(overrideCourseId ?? suggestion.suggested_course_id ?? '');
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set(employeeIds));
  const [plannedStart, setPlannedStart] = useState('');
  const [plannedEnd, setPlannedEnd] = useState('');

  useEffect(() => {
    if (open) {
      setCourseId(overrideCourseId ?? suggestion.suggested_course_id ?? '');
      setSelectedIds(new Set(employeeIds));
      const now = new Date();
      setPlannedStart('');
      setPlannedEnd('');
      // Provide a default end ~90 days out
      const end = new Date(now);
      end.setDate(end.getDate() + 90);
      setPlannedEnd(end.toISOString().slice(0, 10));
      setPlannedStart(now.toISOString().slice(0, 10));
    }
  }, [open, suggestion.id, employeeIds, overrideCourseId, suggestion.suggested_course_id]);

  const courseCost = suggestion.suggested_course_cost ?? 0;
  const totalCost = courseCost * selectedIds.size;
  const totalHours = (suggestion.suggested_course_hours ?? 0) * selectedIds.size;
  const canSubmit = courseId !== '' && selectedIds.size > 0 && !bulkPlan.isPending;

  function toggle(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    setSelectedIds((prev) => (prev.size === employeeIds.length ? new Set() : new Set(employeeIds)));
  }

  async function handleSubmit() {
    if (!canSubmit) return;
    try {
      const res = await bulkPlan.mutateAsync({
        suggestion_id: suggestion.id,
        employee_ids: Array.from(selectedIds),
        course_id: courseId,
        plan_params: {
          year,
          planned_start: plannedStart || undefined,
          planned_end: plannedEnd || undefined,
          hours_planned: suggestion.suggested_course_hours,
          cost_planned: suggestion.suggested_course_cost,
          mandatory: suggestion.origin === 'compliance',
        },
      });
      toast(`Create ${res.created} iscrizioni`);
      onCompleted?.(res.created);
      onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Errore nella pianificazione';
      toast(message, 'error');
    }
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size="lg"
      title={overrideTitle ?? 'Rivedi suggerimento'}
      subtitle={
        <span className={styles.subtitle}>
          {suggestion.severity === 'critical'
            ? 'Compliance gap'
            : suggestion.origin === 'expiring'
            ? 'Scadenza imminente'
            : suggestion.origin === 'skill_gap'
            ? 'Skill gap'
            : 'Suggerimento'} · {suggestion.title}
        </span>
      }
      footer={
        <div className={styles.footer}>
          <div className={styles.totals}>
            <span className={styles.totalsValue}>
              {formatEuro(totalCost)}{' '}
              <span className={styles.totalsLabel}>
                ({selectedIds.size} × {formatEuro(courseCost)})
              </span>
            </span>
            {totalHours > 0 && <span className={styles.totalsHours}>{totalHours} ore totali</span>}
          </div>
          <div className={styles.footerActions}>
            <Button variant="ghost" size="md" onClick={onClose}>Annulla</Button>
            <Button
              variant="primary"
              size="md"
              onClick={handleSubmit}
              loading={bulkPlan.isPending}
              disabled={!canSubmit}
            >
              Pianifica {selectedIds.size} selezionati
            </Button>
          </div>
        </div>
      }
    >
      <div className={styles.body}>
        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Corso</h3>
          <SingleSelect
            options={courseOptions}
            selected={courseId}
            onChange={(value) => setCourseId(value ?? '')}
            placeholder="Seleziona corso"
          />
          {suggestion.suggested_course_hours !== undefined && (
            <p className={styles.sectionHint}>
              Default: {suggestion.suggested_course_hours}h ·{' '}
              {suggestion.suggested_course_cost !== undefined && formatEuro(suggestion.suggested_course_cost)}/persona
            </p>
          )}
        </section>

        <section className={styles.section}>
          <header className={styles.sectionHeader}>
            <h3 className={styles.sectionTitle}>Persone</h3>
            <button type="button" className={styles.linkBtn} onClick={toggleAll}>
              {selectedIds.size === employeeIds.length ? 'Deseleziona tutte' : 'Seleziona tutte'}
            </button>
          </header>
          <ul className={styles.personList}>
            {employeeIds.map((id) => {
              const label = employeeLookup.get(id) ?? id;
              return (
                <li key={id} className={styles.personRow}>
                  <label className={styles.personLabel}>
                    <input
                      type="checkbox"
                      checked={selectedIds.has(id)}
                      onChange={() => toggle(id)}
                    />
                    <span>{label}</span>
                  </label>
                </li>
              );
            })}
          </ul>
        </section>

        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Tempistica</h3>
          <div className={styles.dateRow}>
            <label className={styles.dateField}>
              <span className={styles.dateLabel}>Inizio</span>
              <input
                type="date"
                value={plannedStart}
                onChange={(e) => setPlannedStart(e.target.value)}
                className={styles.dateInput}
              />
            </label>
            <label className={styles.dateField}>
              <span className={styles.dateLabel}>Fine</span>
              <input
                type="date"
                value={plannedEnd}
                onChange={(e) => setPlannedEnd(e.target.value)}
                className={styles.dateInput}
              />
            </label>
          </div>
        </section>
      </div>
    </Drawer>
  );
}

function ReviewRequests({
  open,
  onClose,
  onCompleted,
  year,
  requestIds,
}: { open: boolean; onClose: () => void; onCompleted?: (handled: number) => void } & ReviewEmployeeRequestsConfig) {
  const { toast } = useToast();
  const workspace = useTrainingWorkspace(true);
  const bulkReview = useBulkReviewEmployeeRequests();

  const candidates: TrainingRequest[] = useMemo(() => {
    const all = (workspace.data?.requests ?? []).filter((r) =>
      r.status === 'submitted' || r.status === 'under_review',
    );
    if (!requestIds || requestIds.length === 0) return all;
    const filter = new Set(requestIds);
    return all.filter((r) => filter.has(r.id));
  }, [workspace.data, requestIds]);

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [motivation, setMotivation] = useState('');

  useEffect(() => {
    if (open) {
      setSelectedIds(new Set(candidates.map((c) => c.id)));
      setMotivation('');
    }
  }, [open, candidates.length]);

  function toggle(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleApprove() {
    if (selectedIds.size === 0) return;
    try {
      const res = await bulkReview.mutateAsync({
        request_ids: Array.from(selectedIds),
        target: 'approved',
      });
      toast(`Approvate ${res.succeeded} richieste`);
      onCompleted?.(res.succeeded);
      onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Errore nella revisione';
      toast(message, 'error');
    }
  }

  async function handleReject() {
    if (selectedIds.size === 0) return;
    if (motivation.trim().length < 3) {
      toast('Motivazione obbligatoria (min. 3 caratteri)', 'error');
      return;
    }
    try {
      const res = await bulkReview.mutateAsync({
        request_ids: Array.from(selectedIds),
        target: 'rejected',
        motivation: motivation.trim(),
      });
      toast(`Rifiutate ${res.succeeded} richieste`);
      onCompleted?.(res.succeeded);
      onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Errore nella revisione';
      toast(message, 'error');
    }
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size="lg"
      title="Richieste employee"
      subtitle={<span className={styles.subtitle}>{candidates.length} in attesa di revisione</span>}
      footer={
        <div className={styles.footer}>
          <div className={styles.totals}>
            <span className={styles.totalsLabel}>{selectedIds.size} selezionate</span>
          </div>
          <div className={styles.footerActions}>
            <Button variant="ghost" size="md" onClick={onClose}>Annulla</Button>
            <Button
              variant="secondary"
              size="md"
              onClick={handleReject}
              loading={bulkReview.isPending}
              disabled={selectedIds.size === 0}
            >
              Rifiuta selezionate
            </Button>
            <Button
              variant="primary"
              size="md"
              onClick={handleApprove}
              loading={bulkReview.isPending}
              disabled={selectedIds.size === 0}
            >
              Approva selezionate
            </Button>
          </div>
        </div>
      }
    >
      <div className={styles.body}>
        {candidates.length === 0 ? (
          <p className={styles.empty}>Nessuna richiesta in attesa.</p>
        ) : (
          <>
            <ul className={styles.requestList}>
              {candidates.map((req) => (
                <li key={req.id} className={styles.requestRow}>
                  <label className={styles.requestLabel}>
                    <input
                      type="checkbox"
                      checked={selectedIds.has(req.id)}
                      onChange={() => toggle(req.id)}
                    />
                    <div className={styles.requestBody}>
                      <span className={styles.requestPerson}>{req.employeeName}</span>
                      <span className={styles.requestCourse}>
                        {req.courseTitle ?? req.freeTextTitle ?? '—'}
                        {req.desiredYear && ` · ${req.desiredYear}`}
                      </span>
                      {req.motivation && (
                        <p className={styles.requestMotivation}>“{req.motivation}”</p>
                      )}
                    </div>
                  </label>
                </li>
              ))}
            </ul>

            <section className={styles.section}>
              <h3 className={styles.sectionTitle}>Motivazione (solo per rifiuto)</h3>
              <textarea
                className={styles.textarea}
                rows={3}
                value={motivation}
                onChange={(e) => setMotivation(e.target.value)}
                placeholder="Es. fuori scope formativo 2026, già coperta dal piano del team."
              />
              <p className={styles.sectionHint}>Obbligatoria solo se rifiuti. Min. 3 caratteri.</p>
            </section>
          </>
        )}
      </div>
      <input type="hidden" value={year} readOnly />
    </Drawer>
  );
}
