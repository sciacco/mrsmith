import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Button, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import { useBulkEnrollmentTransition, useTrainingWorkspace } from '../api/queries';
import type { BulkTargetState, PlanEnrollment } from '../api/types';
import { BulkActionBar } from '../components/BulkActionBar';
import { EnrollmentDrawer } from '../components/EnrollmentDrawer';
import { PipelineCard } from '../components/PipelineCard';
import { formatBudget } from '../lib/formatBudget';
import { priorityScore } from '../lib/priorityScore';
import {
  SEVERITY_BUCKET_DESCRIPTION,
  SEVERITY_BUCKET_LABEL,
  SEVERITY_BUCKET_ORDER,
  groupBySeverity,
  type SeverityBucket,
} from '../lib/severityGrouping';
import { buildTeamLabelMap } from '../lib/teamLabels';
import styles from './PipelinePage.module.css';

interface PipelinePageProps {
  isPeopleAdmin: boolean;
}

interface PendingBulk {
  targetState: BulkTargetState;
  label: string;
}

const CHIP_PRESETS: Array<{ value: string; label: string; status?: string }> = [
  { value: 'tutti', label: 'Tutti' },
  { value: 'proposed', label: 'Da approvare', status: 'proposed' },
  { value: 'approved', label: 'Da avviare', status: 'approved' },
  { value: 'in_progress', label: 'In corso', status: 'in_progress' },
];

const RITARDO_OPTIONS: Array<{ value: string; label: string }> = [
  { value: '', label: 'Qualsiasi' },
  { value: '>0', label: 'In ritardo' },
  { value: '>30', label: '> 30 giorni' },
];

const DAY_MS = 24 * 60 * 60 * 1000;

function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') return body.message;
  }
  return fallback;
}

function parseDate(value: string | undefined): Date | null {
  if (!value) return null;
  const stamped = value.length > 10 ? value : `${value}T00:00:00`;
  const date = new Date(stamped);
  return Number.isFinite(date.getTime()) ? date : null;
}

function delayDays(enrollment: PlanEnrollment, now: Date): number {
  const planned = parseDate(enrollment.plannedEnd) ?? parseDate(enrollment.plannedStart);
  if (!planned) return 0;
  return Math.floor((now.getTime() - planned.getTime()) / DAY_MS);
}

function buildYearOptions(enrollments: PlanEnrollment[]): Array<{ value: string; label: string }> {
  const years = new Set<number>();
  const current = new Date().getFullYear();
  years.add(current - 1);
  years.add(current);
  years.add(current + 1);
  for (const e of enrollments) if (Number.isFinite(e.year)) years.add(e.year);
  return Array.from(years)
    .sort((a, b) => b - a)
    .map((y) => ({ value: String(y), label: String(y) }));
}

export function PipelinePage({ isPeopleAdmin }: PipelinePageProps) {
  const [params, setParams] = useSearchParams();
  const { toast } = useToast();
  const workspace = useTrainingWorkspace(isPeopleAdmin);
  const bulkTransition = useBulkEnrollmentTransition(isPeopleAdmin);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [openId, setOpenId] = useState<string | null>(null);
  const [pendingBulk, setPendingBulk] = useState<PendingBulk | null>(null);
  const [collapsedBuckets, setCollapsedBuckets] = useState<Set<SeverityBucket>>(new Set(['info']));

  const now = useMemo(() => new Date(), []);

  if (!isPeopleAdmin) {
    return (
      <main className={styles.page}>
        <p className={styles.accessDenied}>Accesso riservato al team People.</p>
      </main>
    );
  }

  const q = params.get('q') ?? '';
  const teamFilter = params.get('team') ?? '';
  const yearFilter = params.get('year') ?? String(new Date().getFullYear());
  const statoFilter = params.get('stato') ?? 'tutti';
  const ritardoFilter = params.get('ritardo_gg') ?? '';

  function updateParam(key: string, value: string | null) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (value && value !== '') next.set(key, value);
      else next.delete(key);
      return next;
    }, { replace: true });
  }

  const allEnrollments = workspace.data?.plan ?? [];
  const teams = workspace.data?.masterData?.teams ?? [];
  const teamLabels = useMemo(() => buildTeamLabelMap(teams), [teams]);
  const teamOptions = useMemo(
    () => teams.map((t) => ({ value: t.code, label: t.name || t.code })),
    [teams],
  );
  const yearOptions = useMemo(() => buildYearOptions(allEnrollments), [allEnrollments]);

  const filtered = useMemo(() => {
    return allEnrollments
      .filter((enrollment) => {
        if (yearFilter && String(enrollment.year) !== yearFilter) return false;
        if (teamFilter && enrollment.teamCode !== teamFilter) return false;
        if (statoFilter && statoFilter !== 'tutti') {
          const preset = CHIP_PRESETS.find((p) => p.value === statoFilter);
          if (preset?.status && enrollment.status !== preset.status) return false;
        }
        if (ritardoFilter) {
          const days = delayDays(enrollment, now);
          if (ritardoFilter === '>0' && days <= 0) return false;
          if (ritardoFilter === '>30' && days <= 30) return false;
        }
        if (q.trim()) {
          const needle = q.trim().toLowerCase();
          const haystack = `${enrollment.employeeName} ${enrollment.employeeEmail} ${enrollment.courseTitle} ${enrollment.vendorName ?? ''}`.toLowerCase();
          if (!haystack.includes(needle)) return false;
        }
        return ['proposed', 'approved', 'in_progress'].includes(enrollment.status) || statoFilter !== 'tutti';
      })
      .sort((a, b) => priorityScore(b, { now }) - priorityScore(a, { now }));
  }, [allEnrollments, yearFilter, teamFilter, statoFilter, ritardoFilter, q, now]);

  const grouped = useMemo(() => groupBySeverity(filtered, now), [filtered, now]);

  useEffect(() => {
    const nonEmpty = SEVERITY_BUCKET_ORDER.filter((b) => (grouped.get(b)?.length ?? 0) > 0);
    if (nonEmpty.length !== 1) return;
    const only = nonEmpty[0];
    if (!only) return;
    setCollapsedBuckets((prev) => {
      if (!prev.has(only)) return prev;
      const next = new Set(prev);
      next.delete(only);
      return next;
    });
  }, [grouped]);

  const selectedEnrollments = useMemo(() => filtered.filter((e) => selectedIds.has(e.id)), [filtered, selectedIds]);
  const selectedBudget = useMemo(
    () => selectedEnrollments.reduce((sum, e) => sum + (e.costPlanned ?? 0), 0),
    [selectedEnrollments],
  );

  const openEnrollment = useMemo(
    () => (openId ? allEnrollments.find((e) => e.id === openId) ?? null : null),
    [allEnrollments, openId],
  );

  const criticalCount = grouped.get('critical')?.length ?? 0;
  const warningCount = grouped.get('warning')?.length ?? 0;
  const attentionCount = criticalCount + warningCount;

  function toggleSelection(id: string) {
    setSelectedIds((previous) => {
      const next = new Set(previous);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function clearSelection() {
    setSelectedIds(new Set());
  }

  function toggleBucket(bucket: SeverityBucket) {
    setCollapsedBuckets((previous) => {
      const next = new Set(previous);
      if (next.has(bucket)) next.delete(bucket);
      else next.add(bucket);
      return next;
    });
  }

  function performBulk(target: BulkTargetState, label: string) {
    setPendingBulk({ targetState: target, label });
  }

  function confirmBulk() {
    if (!pendingBulk) return;
    const ids = Array.from(selectedIds);
    bulkTransition.mutate(
      { enrollmentIds: ids, targetState: pendingBulk.targetState },
      {
        onSuccess: (response) => {
          if (response.failed > 0) {
            toast(`${response.succeeded} aggiornate, ${response.failed} fallite`, 'warning');
          } else {
            toast(`${response.succeeded} iscrizioni aggiornate`);
          }
          clearSelection();
          setPendingBulk(null);
        },
        onError: (error) => toast(apiErrorMessage(error, 'Operazione bulk non riuscita'), 'error'),
      },
    );
  }

  if (workspace.isLoading) {
    return (
      <main className={styles.page}>
        <Skeleton rows={6} />
      </main>
    );
  }

  const totalFiltered = filtered.length;

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <h1>Pipeline</h1>
        {attentionCount > 0 && (
          <span className={styles.attention}>
            <span className={styles.attentionDot} aria-hidden="true">●</span>
            {attentionCount} {attentionCount === 1 ? 'richiede attenzione' : 'richiedono attenzione'}
          </span>
        )}
      </header>

      <div className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <div className={styles.field}>
            <span className={styles.fieldLabel}>Anno</span>
            <SingleSelect
              options={yearOptions}
              selected={yearFilter}
              onChange={(value) => updateParam('year', value ? String(value) : null)}
              placeholder="Anno"
            />
          </div>
          <div className={styles.field}>
            <span className={styles.fieldLabel}>Team</span>
            <SingleSelect
              options={teamOptions}
              selected={teamFilter || null}
              onChange={(value) => updateParam('team', value ? String(value) : null)}
              placeholder="Tutti"
              allowClear
              clearLabel="Tutti"
            />
          </div>
          <div className={styles.chips} role="tablist" aria-label="Filtra per stato">
            {CHIP_PRESETS.map((preset) => (
              <button
                key={preset.value}
                type="button"
                role="tab"
                aria-selected={statoFilter === preset.value}
                className={`${styles.chip} ${statoFilter === preset.value ? styles.chipActive : ''}`}
                onClick={() => updateParam('stato', preset.value === 'tutti' ? null : preset.value)}
              >
                {preset.label}
              </button>
            ))}
          </div>
          <div className={styles.field}>
            <span className={styles.fieldLabel}>Ritardo</span>
            <SingleSelect
              options={RITARDO_OPTIONS}
              selected={ritardoFilter || ''}
              onChange={(value) => updateParam('ritardo_gg', value ? String(value) : null)}
              placeholder="Qualsiasi"
            />
          </div>
        </div>
        <div className={styles.searchWrap}>
          <SearchInput value={q} onChange={(value) => updateParam('q', value)} placeholder="Cerca persone o corsi" />
        </div>
      </div>

      {totalFiltered === 0 ? (
        <div className={styles.empty}>
          <strong>Inbox zero</strong>
          <span>Nessuna azione richiesta con i filtri correnti.</span>
        </div>
      ) : (
        <div className={styles.groups}>
          {SEVERITY_BUCKET_ORDER.map((bucket) => {
            const rows = grouped.get(bucket) ?? [];
            if (rows.length === 0) return null;
            const isCollapsed = collapsedBuckets.has(bucket);
            const bucketBudget = rows.reduce((sum, r) => sum + (r.costPlanned ?? 0), 0);
            return (
              <section key={bucket} className={`${styles.group} ${styles[`group_${bucket}`] ?? ''}`}>
                <button
                  type="button"
                  className={styles.groupHead}
                  onClick={() => toggleBucket(bucket)}
                  aria-expanded={!isCollapsed}
                >
                  <span className={`${styles.groupDot} ${styles[`dot_${bucket}`] ?? ''}`} aria-hidden="true">●</span>
                  <span className={styles.groupTitle}>{SEVERITY_BUCKET_LABEL[bucket]}</span>
                  <span className={styles.groupCount}>{rows.length}</span>
                  <span className={styles.groupDescription}>{SEVERITY_BUCKET_DESCRIPTION[bucket]}</span>
                  {bucketBudget > 0 && (
                    <span className={styles.groupBudget}>{formatBudget(bucketBudget)}</span>
                  )}
                  <span className={`${styles.chevron} ${isCollapsed ? '' : styles.chevronOpen}`} aria-hidden="true">▾</span>
                </button>
                {!isCollapsed && (
                  <div className={styles.cardList}>
                    {rows.map((enrollment) => (
                      <PipelineCard
                        key={enrollment.id}
                        enrollment={enrollment}
                        selected={selectedIds.has(enrollment.id)}
                        onToggle={() => toggleSelection(enrollment.id)}
                        onOpen={() => setOpenId(enrollment.id)}
                        teamLabels={teamLabels}
                        now={now}
                      />
                    ))}
                  </div>
                )}
              </section>
            );
          })}
        </div>
      )}

      <BulkActionBar
        selectedCount={selectedIds.size}
        onClear={clearSelection}
        summary={selectedBudget > 0 ? `Budget impatto ${formatBudget(selectedBudget)}` : undefined}
      >
        <Button variant="primary" size="sm" disabled={bulkTransition.isPending} onClick={() => performBulk('approved', 'Approva selezionati')}>
          Approva selezionati
        </Button>
        <Button variant="secondary" size="sm" disabled={bulkTransition.isPending} onClick={() => performBulk('in_progress', 'Avvia selezionati')}>
          Avvia selezionati
        </Button>
        <Button variant="secondary" size="sm" disabled={bulkTransition.isPending} onClick={() => performBulk('completed', 'Chiudi selezionati')}>
          Chiudi selezionati
        </Button>
      </BulkActionBar>

      <Modal open={pendingBulk !== null} onClose={() => setPendingBulk(null)} title={pendingBulk?.label ?? ''} size="sm">
        <div className={styles.modalBody}>
          <p>
            Confermi <strong>{pendingBulk?.label.toLowerCase()}</strong> per {selectedIds.size} iscrizioni
            {selectedBudget > 0 ? ` (impatto budget ${formatBudget(selectedBudget)})` : ''}?
          </p>
          <div className={styles.modalActions}>
            <Button variant="secondary" onClick={() => setPendingBulk(null)}>Annulla</Button>
            <Button variant="primary" disabled={bulkTransition.isPending} onClick={confirmBulk}>
              Conferma
            </Button>
          </div>
        </div>
      </Modal>

      <EnrollmentDrawer enrollment={openEnrollment} isPeopleAdmin={isPeopleAdmin} onClose={() => setOpenId(null)} />
    </main>
  );
}
