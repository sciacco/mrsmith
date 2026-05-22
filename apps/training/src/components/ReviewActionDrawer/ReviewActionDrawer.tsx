import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, Drawer, SingleSelect, StatusBadge, useToast } from '@mrsmith/ui';
import {
  useBulkPlanFromSuggestion,
  useBulkReviewEmployeeRequests,
  useCustomGroups,
  usePeopleDirectory,
  useTrainingLookups,
  useTrainingWorkspace,
} from '../../api/queries';
import type { CatalogCourse, PersonSummary, PlanningSuggestion, TrainingRequest } from '../../api/types';
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

export interface CreateFromScratchConfig {
  mode: 'create_from_scratch';
  year: number;
  team?: string;
  budgetResidual: number;
}

export type ReviewActionDrawerProps = {
  open: boolean;
  onClose: () => void;
  onCompleted?: (created: number) => void;
} & (CreateFromSuggestionConfig | ReviewEmployeeRequestsConfig | CreateFromScratchConfig);

function personRank(p: PersonSummary): number {
  if (p.flags.compliance_gap) return 0;
  if (p.flags.scadenze_imminenti) return 1;
  if (p.flags.da_pianificare || p.flags.senza_formazione_attiva) return 2;
  return 3;
}

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}

function formatCourseEstimate(hours: number | undefined, cost: number | undefined, vendorName?: string): string {
  return [
    hours !== undefined ? `${hours}h` : 'ore da definire',
    cost !== undefined ? `${formatEuro(cost)}/persona` : 'costo da definire',
    vendorName || null,
  ].filter(Boolean).join(' · ');
}

function formatTotalHours(hoursPerPerson: number | undefined, peopleCount: number): string {
  if (hoursPerPerson === undefined) return 'ore da definire';
  return `${hoursPerPerson * peopleCount}h`;
}

function formatTotalCost(costPerPerson: number | undefined, peopleCount: number): string {
  if (costPerPerson === undefined) return 'costo da definire';
  return formatEuro(costPerPerson * peopleCount);
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body as { message?: string } | undefined;
    return body?.message ?? fallback;
  }
  return error instanceof Error && !error.message.startsWith('API ')
    ? error.message
    : fallback;
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
  if (props.mode === 'create_from_scratch') {
    return <CreateFromScratch {...props} />;
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

  const courseCost = suggestion.suggested_course_cost;
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
        mandatory_rule_id: suggestion.rule_id,
        source_custom_group_id: suggestion.source_custom_group_id,
      });
      toast(`Create ${res.created} iscrizioni`);
      onCompleted?.(res.created);
      onClose();
    } catch (err) {
      const message = errorMessage(err, 'Errore nella pianificazione');
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
              {selectedIds.size > 0
                ? formatTotalCost(courseCost, selectedIds.size)
                : 'Nessuna persona selezionata'}
              {selectedIds.size > 0 && courseCost !== undefined && (
                <span className={styles.totalsLabel}>
                  {' '}({selectedIds.size} × {formatEuro(courseCost)})
                </span>
              )}
            </span>
            {selectedIds.size > 0 && (
              <span className={styles.totalsHours}>
                {formatTotalHours(suggestion.suggested_course_hours, selectedIds.size)}
              </span>
            )}
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
          {courseId && (
            <p className={styles.sectionHint}>
              Stima: {formatCourseEstimate(suggestion.suggested_course_hours, suggestion.suggested_course_cost)}
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
        year,
      });
      toast(`Approvate ${res.succeeded} richieste`);
      onCompleted?.(res.succeeded);
      onClose();
    } catch (err) {
      const message = errorMessage(err, 'Errore nella revisione');
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
        year,
      });
      toast(`Rifiutate ${res.succeeded} richieste`);
      onCompleted?.(res.succeeded);
      onClose();
    } catch (err) {
      const message = errorMessage(err, 'Errore nella revisione');
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

function CreateFromScratch({
  open,
  onClose,
  onCompleted,
  year,
  team,
  budgetResidual,
}: { open: boolean; onClose: () => void; onCompleted?: (created: number) => void } & CreateFromScratchConfig) {
  const { toast } = useToast();
  const workspace = useTrainingWorkspace(true);
  const bulkPlan = useBulkPlanFromSuggestion();
  const [search, setSearch] = useState('');
  const people = usePeopleDirectory({ year: String(year), team, q: search }, open);
  const groups = useCustomGroups({ status: 'attivo' }, open);
  const [courseId, setCourseId] = useState('');
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [plannedStart, setPlannedStart] = useState('');
  const [plannedEnd, setPlannedEnd] = useState('');
  const [filterTeam, setFilterTeam] = useState<string>('all');
  const [filterOnlyGap, setFilterOnlyGap] = useState(false);
  const [filterHideEnrolled, setFilterHideEnrolled] = useState(true);
  const [selectionMode, setSelectionMode] = useState<'people' | 'group'>('people');
  const [selectedGroupId, setSelectedGroupId] = useState('');

  useEffect(() => {
    if (!open) return;
    setSearch('');
    setCourseId('');
    setSelectedIds(new Set());
    setSelectionMode('people');
    setSelectedGroupId('');
    setFilterTeam('all');
    setFilterOnlyGap(false);
    setFilterHideEnrolled(true);
    const now = new Date();
    const end = new Date(now);
    end.setDate(end.getDate() + 90);
    setPlannedStart(now.toISOString().slice(0, 10));
    setPlannedEnd(end.toISOString().slice(0, 10));
  }, [open, year, team]);

  const activeCourses = useMemo(
    () => (workspace.data?.catalog ?? []).filter((course) => course.active),
    [workspace.data],
  );

  const courseOptions = useMemo(
    () => activeCourses.map((course) => ({ value: course.id, label: course.title })),
    [activeCourses],
  );
  const teamLabel = useMemo(
    () => workspace.data?.masterData?.teams.find((item) => item.code === team)?.name,
    [workspace.data, team],
  );
  const groupOptions = useMemo(
    () =>
      (groups.data?.groups ?? [])
        .filter((group) => group.active)
        .map((group) => ({ value: group.id, label: `${group.name} (${group.member_count})` })),
    [groups.data],
  );

  const selectedCourse = activeCourses.find((course) => course.id === courseId);
  const selectedGroup = (groups.data?.groups ?? []).find((group) => group.id === selectedGroupId);
  const directoryPeople = people.data ?? [];

  const alreadyEnrolledEmails = useMemo(() => {
    if (!selectedCourse) return new Set<string>();
    const planEnrollments = workspace.data?.plan ?? [];
    const courseTitle = selectedCourse.title;
    return new Set(
      planEnrollments
        .filter(
          (e) =>
            e.year === year &&
            e.courseTitle === courseTitle &&
            (e.status ?? '').toUpperCase() !== 'ANNULLATO',
        )
        .map((e) => e.employeeEmail.toLowerCase()),
    );
  }, [workspace.data, selectedCourse, year]);

  const teamOptions = useMemo(() => {
    const teams = new Map<string, string>();
    directoryPeople.forEach((p) => {
      if (p.team_code) teams.set(p.team_code, p.team_name || 'Team senza nome');
    });
    return Array.from(teams, ([value, label]) => ({ value, label }))
      .sort((a, b) => a.label.localeCompare(b.label, 'it'));
  }, [directoryPeople]);

  const displayedPeople = useMemo(() => {
    let list = directoryPeople.slice();
    if (filterTeam !== 'all') {
      list = list.filter((p) => p.team_code === filterTeam);
    }
    if (filterOnlyGap) {
      list = list.filter((p) => p.flags.compliance_gap);
    }
    if (filterHideEnrolled && selectedCourse) {
      list = list.filter((p) => !alreadyEnrolledEmails.has(p.email.toLowerCase()));
    }
    if (selectedCourse) {
      list.sort((a, b) => {
        const ra = personRank(a);
        const rb = personRank(b);
        if (ra !== rb) return ra - rb;
        if (b.priority_score !== a.priority_score) return b.priority_score - a.priority_score;
        return a.name.localeCompare(b.name);
      });
    }
    return list;
  }, [directoryPeople, filterTeam, filterOnlyGap, filterHideEnrolled, selectedCourse, alreadyEnrolledEmails]);

  const gapCountForCurrentFilter = useMemo(() => {
    let list = directoryPeople;
    if (filterTeam !== 'all') list = list.filter((p) => p.team_code === filterTeam);
    return list.filter((p) => p.flags.compliance_gap).length;
  }, [directoryPeople, filterTeam]);

  const selectedPeopleArray = useMemo(
    () => directoryPeople.filter((p) => selectedIds.has(p.id)),
    [directoryPeople, selectedIds],
  );

  const courseCost = selectedCourse?.defaultCost;
  const courseHours = selectedCourse?.defaultHours;
  const totalCost = courseCost !== undefined ? courseCost * selectedIds.size : undefined;
  const totalHours = courseHours !== undefined ? courseHours * selectedIds.size : undefined;
  const residualAfter = totalCost !== undefined ? budgetResidual - totalCost : undefined;
  const residualPct = residualAfter !== undefined && budgetResidual > 0
    ? Math.round((residualAfter / budgetResidual) * 100)
    : null;
  const canSubmit = courseId !== '' && selectedIds.size > 0 && !bulkPlan.isPending;

  function toggle(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function removeSelected(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  }

  function selectAllDisplayed() {
    const ids = displayedPeople
      .filter((p) => !alreadyEnrolledEmails.has(p.email.toLowerCase()))
      .map((p) => p.id);
    const allSelected = ids.length > 0 && ids.every((id) => selectedIds.has(id));
    setSelectedIds((prev) => {
      const next = new Set(prev);
      ids.forEach((id) => {
        if (allSelected) next.delete(id);
        else next.add(id);
      });
      return next;
    });
  }

  function chooseGroup(groupId: string) {
    setSelectedGroupId(groupId);
    const group = (groups.data?.groups ?? []).find((item) => item.id === groupId);
    setSelectedIds(new Set((group?.members ?? []).map((member) => member.id)));
  }

  const selectableDisplayedCount = displayedPeople.filter(
    (p) => !alreadyEnrolledEmails.has(p.email.toLowerCase()),
  ).length;
  const allDisplayedSelected =
    selectableDisplayedCount > 0 &&
    displayedPeople
      .filter((p) => !alreadyEnrolledEmails.has(p.email.toLowerCase()))
      .every((p) => selectedIds.has(p.id));

  async function handleSubmit() {
    if (!canSubmit || !selectedCourse) return;
    try {
      const res = await bulkPlan.mutateAsync({
        suggestion_id: null,
        employee_ids: Array.from(selectedIds),
        course_id: courseId,
        plan_params: {
          year,
          planned_start: plannedStart || undefined,
          planned_end: plannedEnd || undefined,
          hours_planned: selectedCourse.defaultHours,
          cost_planned: selectedCourse.defaultCost,
          mandatory: selectedCourse.mandatory,
        },
        source_custom_group_id: selectionMode === 'group' ? selectedGroupId || undefined : undefined,
      });
      toast(`Create ${res.created} iscrizioni`);
      onCompleted?.(res.created);
      onClose();
    } catch (err) {
      const message = errorMessage(err, 'Errore nella pianificazione');
      toast(message, 'error');
    }
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size="xl"
      title="Nuova iscrizione"
      subtitle={<span className={styles.subtitle}>Piano {year}{teamLabel ? ` · ${teamLabel}` : ''}</span>}
      footer={
        <div className={styles.footer}>
          <div className={styles.summaryFooter}>
            <span className={styles.summaryHeadline}>
              {selectedIds.size > 0
                ? [
                    `${selectedIds.size} ${selectedIds.size === 1 ? 'persona' : 'persone'}`,
                    totalHours !== undefined ? `${totalHours}h` : 'ore da definire',
                    totalCost !== undefined ? formatEuro(totalCost) : 'costo da definire',
                  ].join(' · ')
                : 'Nessuna persona selezionata'}
            </span>
            {selectedCourse && (
              <span className={`${styles.summaryLine} ${residualAfter !== undefined && residualAfter < 0 ? styles.totalsWarning : ''}`}>
                {residualAfter !== undefined
                  ? `Residuo dopo: ${formatEuro(residualAfter)}${residualPct !== null ? ` (${residualPct}%)` : ''}`
                  : 'Residuo dopo: costo da definire'}
              </span>
            )}
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
              {selectedIds.size > 0 ? `Pianifica ${selectedIds.size}` : 'Pianifica'}
            </Button>
          </div>
        </div>
      }
    >
      <div className={styles.bodySplit}>
        <div className={styles.topRow}>
          <div className={styles.column}>
            <h3 className={styles.sectionTitle}>Corso</h3>
            <SingleSelect
              options={courseOptions}
              selected={courseId}
              onChange={(value) => setCourseId(value ?? '')}
              placeholder="Seleziona corso"
              searchable
            />
            {selectedCourse ? (
              <>
                <div className={styles.courseBadges}>
                  {selectedCourse.mandatory && (
                    <StatusBadge value="" label="Obbligatorio" variant="warning" dot />
                  )}
                  {selectedCourse.complianceFramework && (
                    <StatusBadge
                      value=""
                      label={selectedCourse.complianceFramework}
                      variant="neutral"
                      dot={false}
                    />
                  )}
                </div>
                <CourseHint course={selectedCourse} />
              </>
            ) : (
              <small className={styles.fieldCaption}>&nbsp;</small>
            )}
          </div>

          <label className={`${styles.column} ${styles.dateField}`}>
            <h3 className={styles.sectionTitle}>Inizio</h3>
            <input
              type="date"
              value={plannedStart}
              onChange={(e) => setPlannedStart(e.target.value)}
              className={styles.dateInput}
            />
            <small className={styles.fieldCaption}>Finestra di completamento</small>
          </label>

          <label className={`${styles.column} ${styles.dateField}`}>
            <h3 className={styles.sectionTitle}>Fine</h3>
            <input
              type="date"
              value={plannedEnd}
              onChange={(e) => setPlannedEnd(e.target.value)}
              className={styles.dateInput}
            />
            <small className={styles.fieldCaption}>&nbsp;</small>
          </label>
        </div>

        <section className={styles.column}>
          <header className={styles.sectionHeader}>
            <h3 className={styles.sectionTitle}>Persone</h3>
            {selectableDisplayedCount > 0 && (
              <button type="button" className={styles.linkBtn} onClick={selectAllDisplayed}>
                {allDisplayedSelected
                  ? 'Deseleziona tutti'
                  : `Seleziona tutti (${selectableDisplayedCount})`}
              </button>
            )}
          </header>

          <div className={styles.segmented} role="tablist" aria-label="Origine selezione">
            <button
              type="button"
              className={`${styles.segmentedButton} ${selectionMode === 'people' ? styles.segmentedActive : ''}`}
              onClick={() => {
                setSelectionMode('people');
                setSelectedGroupId('');
                setSelectedIds(new Set());
              }}
            >
              Scegli persone
            </button>
            <button
              type="button"
              className={`${styles.segmentedButton} ${selectionMode === 'group' ? styles.segmentedActive : ''}`}
              onClick={() => {
                setSelectionMode('group');
                setSelectedIds(new Set());
              }}
            >
              Scegli gruppo
            </button>
          </div>

          {selectionMode === 'group' && (
            <div className={styles.groupPicker}>
              <SingleSelect
                options={groupOptions}
                selected={selectedGroupId || null}
                onChange={(value) => chooseGroup(value ?? '')}
                placeholder="Seleziona gruppo"
                searchable
              />
              {selectedGroup && (
                <p className={styles.sectionHint}>
                  {selectedGroup.member_count} persone · {selectedGroup.description || 'Popolazione ad-hoc'}
                </p>
              )}
            </div>
          )}

          {selectionMode === 'people' && (
          <div className={styles.peopleFilters}>
            <div className={styles.peopleFiltersRow}>
              <input
                className={styles.searchInput}
                type="search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Cerca per nome o email"
              />
              {teamOptions.length > 0 && (
                <div className={styles.peopleFiltersTeam}>
                  <SingleSelect
                    options={[
                      { value: 'all', label: 'Tutti i team' },
                      ...teamOptions,
                    ]}
                    selected={filterTeam}
                    onChange={(value) => setFilterTeam(value ?? 'all')}
                    placeholder="Team"
                  />
                </div>
              )}
            </div>
            <div className={styles.peopleFiltersToggles}>
              <label className={styles.peopleFiltersToggle}>
                <input
                  type="checkbox"
                  checked={filterOnlyGap}
                  onChange={(e) => setFilterOnlyGap(e.target.checked)}
                />
                Solo con gap{gapCountForCurrentFilter > 0 && ` (${gapCountForCurrentFilter})`}
              </label>
              {selectedCourse && (
                <label className={styles.peopleFiltersToggle}>
                  <input
                    type="checkbox"
                    checked={filterHideEnrolled}
                    onChange={(e) => setFilterHideEnrolled(e.target.checked)}
                  />
                  Nascondi già iscritti
                </label>
              )}
            </div>
          </div>
          )}

          {selectedPeopleArray.length > 0 && (
            <div className={styles.selectedStack}>
              <div className={styles.selectedStackHeader}>
                Selezionati ({selectedPeopleArray.length})
              </div>
              <ul className={styles.selectedStackList}>
                {selectedPeopleArray.map((p) => (
                  <li key={p.id} className={styles.selectedChip}>
                    <span className={styles.selectedChipName}>{p.name}</span>
                    <button
                      type="button"
                      className={styles.selectedChipRemove}
                      onClick={() => removeSelected(p.id)}
                      aria-label={`Rimuovi ${p.name}`}
                    >
                      ×
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {!selectedCourse && (
            <div className={styles.peopleEmptyHint}>
              Scegli un corso per ordinare le persone per fabbisogno.
            </div>
          )}

          {selectionMode === 'group' && !selectedGroupId ? (
            <p className={styles.empty}>Nessun gruppo selezionato.</p>
          ) : selectionMode === 'group' && selectedGroup && selectedGroup.member_count === 0 ? (
            <p className={styles.empty}>Il gruppo non contiene persone.</p>
          ) : selectionMode === 'group' ? (
            <ul className={styles.personList}>
              {(selectedGroup?.members ?? []).map((member) => (
                <li key={member.id} className={styles.personRow}>
                  <label className={styles.personLabel}>
                    <input
                      type="checkbox"
                      checked={selectedIds.has(member.id)}
                      onChange={() => toggle(member.id)}
                    />
                    <span className={styles.personBody}>
                      <span className={styles.personHeader}>
                        <span className={styles.personName}>{member.name}</span>
                        {member.team_name && <span className={styles.personTeam}>{member.team_name}</span>}
                      </span>
                      <span className={styles.personMeta}>{member.email}</span>
                    </span>
                  </label>
                </li>
              ))}
            </ul>
          ) : people.isLoading ? (
            <p className={styles.empty}>Caricamento persone...</p>
          ) : displayedPeople.length === 0 ? (
            <p className={styles.empty}>Nessuna persona trovata.</p>
          ) : (
            <ul className={styles.personList}>
              {displayedPeople.map((person) => {
                const isAlreadyEnrolled = alreadyEnrolledEmails.has(person.email.toLowerCase());
                return (
                  <PersonOption
                    key={person.id}
                    person={person}
                    checked={selectedIds.has(person.id)}
                    onToggle={() => toggle(person.id)}
                    isAlreadyEnrolled={isAlreadyEnrolled}
                    courseSelected={Boolean(selectedCourse)}
                    courseTitle={selectedCourse?.title}
                  />
                );
              })}
            </ul>
          )}
        </section>
      </div>
    </Drawer>
  );
}

function CourseHint({ course }: { course: CatalogCourse }) {
  return <p className={styles.sectionHint}>{formatCourseEstimate(course.defaultHours, course.defaultCost, course.vendorName)}</p>;
}

function PersonOption({
  person,
  checked,
  onToggle,
  isAlreadyEnrolled,
  courseSelected,
  courseTitle,
}: {
  person: PersonSummary;
  checked: boolean;
  onToggle: () => void;
  isAlreadyEnrolled: boolean;
  courseSelected: boolean;
  courseTitle?: string;
}) {
  const badges: ReactNode[] = [];
  if (isAlreadyEnrolled) {
    badges.push(
      <StatusBadge
        key="enrolled"
        value=""
        label="Già iscritta"
        variant="neutral"
        dot
        tooltip={courseTitle ? `Già iscritta a ${courseTitle}` : 'Già iscritta'}
      />,
    );
  } else {
    if (person.flags.compliance_gap && person.gaps_open > 0) {
      badges.push(
        <StatusBadge
          key="gap"
          value=""
          label={person.gaps_open === 1 ? 'Gap' : `Gap ×${person.gaps_open}`}
          variant="danger"
          dot={false}
        />,
      );
    }
    if (person.expiring_certs_count > 0 && badges.length < 2) {
      badges.push(
        <StatusBadge
          key="cert"
          value=""
          label={
            person.expiring_certs_count === 1
              ? 'Cert in scadenza'
              : `Cert ×${person.expiring_certs_count}`
          }
          variant="warning"
          dot={false}
          tooltip={person.next_deadline?.label}
        />,
      );
    }
    if (person.flags.da_pianificare && badges.length < 2) {
      badges.push(
        <StatusBadge key="todo" value="" label="Da pianificare" variant="accent" dot={false} />,
      );
    }
  }

  const showActiveHint =
    !isAlreadyEnrolled && courseSelected && person.active_enrollments_count > 0;

  const rowClass = `${styles.personRow} ${isAlreadyEnrolled ? styles.personRowDisabled : ''}`;

  return (
    <li className={rowClass}>
      <label className={styles.personLabel}>
        <input
          type="checkbox"
          checked={checked}
          onChange={onToggle}
          disabled={isAlreadyEnrolled}
        />
        <span className={styles.personBody}>
          <span className={styles.personHeader}>
            <span className={styles.personName}>{person.name}</span>
            {person.team_name && <span className={styles.personTeam}>{person.team_name}</span>}
          </span>
          <span className={styles.personMeta}>{person.email}</span>
          {badges.length > 0 && <span className={styles.personBadges}>{badges}</span>}
          {showActiveHint && (
            <span className={styles.personSubMeta}>
              {person.active_enrollments_count}{' '}
              {person.active_enrollments_count === 1
                ? 'iscrizione attiva nel piano'
                : 'iscrizioni attive nel piano'}
            </span>
          )}
        </span>
      </label>
    </li>
  );
}
