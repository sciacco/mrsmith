import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Button, Modal, SearchInput, Skeleton, useToast } from '@mrsmith/ui';
import { useBulkEnrollmentTransition, useTrainingWorkspace } from '../api/queries';
import type { BulkTargetState, PlanEnrollment } from '../api/types';
import { BulkActionBar } from '../components/BulkActionBar';
import { EnrollmentDrawer } from '../components/EnrollmentDrawer';
import { PipelineCard } from '../components/PipelineCard';
import { classifyAlertLevel } from '../lib/alertLevel';
import { priorityScore } from '../lib/priorityScore';
import {
  TEMPORAL_BUCKET_LABEL,
  TEMPORAL_BUCKET_ORDER,
  bucketForEnrollment,
  type TemporalBucket,
} from '../lib/temporalGrouping';
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

const RITARDO_OPTIONS = [
  { value: '>0', label: 'In ritardo' },
  { value: '>30', label: 'In ritardo > 30gg' },
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

function isAccessDenied(): boolean {
  return false;
}

export function PipelinePage({ isPeopleAdmin }: PipelinePageProps) {
  const [params, setParams] = useSearchParams();
  const { toast } = useToast();
  const workspace = useTrainingWorkspace(isPeopleAdmin);
  const bulkTransition = useBulkEnrollmentTransition(isPeopleAdmin);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [openId, setOpenId] = useState<string | null>(null);
  const [pendingBulk, setPendingBulk] = useState<PendingBulk | null>(null);

  const now = useMemo(() => new Date(), []);

  if (!isPeopleAdmin || isAccessDenied()) {
    return (
      <main className={styles.page}>
        <p className={styles.accessDenied}>Accesso riservato al team People.</p>
      </main>
    );
  }

  const queryString = params;
  const q = queryString.get('q') ?? '';
  const teamFilter = queryString.get('team') ?? '';
  const yearFilter = queryString.get('year') ?? String(new Date().getFullYear());
  const statoFilter = queryString.get('stato') ?? 'tutti';
  const ritardoFilter = queryString.get('ritardo_gg') ?? '';

  function updateParam(key: string, value: string | null) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (value && value !== '') next.set(key, value);
      else next.delete(key);
      return next;
    }, { replace: true });
  }

  const allEnrollments = workspace.data?.plan ?? [];

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

  const grouped = useMemo(() => {
    const map = new Map<TemporalBucket, PlanEnrollment[]>();
    for (const bucket of TEMPORAL_BUCKET_ORDER) map.set(bucket, []);
    for (const enrollment of filtered) {
      const bucket = bucketForEnrollment(enrollment, now);
      map.get(bucket)!.push(enrollment);
    }
    return map;
  }, [filtered, now]);

  const selectedEnrollments = useMemo(() => filtered.filter((e) => selectedIds.has(e.id)), [filtered, selectedIds]);
  const selectedBudget = useMemo(
    () => selectedEnrollments.reduce((sum, e) => sum + (e.costPlanned ?? 0), 0),
    [selectedEnrollments],
  );

  const openEnrollment = useMemo(
    () => (openId ? allEnrollments.find((e) => e.id === openId) ?? null : null),
    [allEnrollments, openId],
  );

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

  const ritardoBanner = ritardoFilter ? RITARDO_OPTIONS.find((o) => o.value === ritardoFilter)?.label : null;

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
        <div>
          <h1>Pipeline</h1>
          <p className={styles.subtitle}>Inbox unificata: cose da approvare, avviare, chiudere.</p>
        </div>
        <SearchInput value={q} onChange={(value) => updateParam('q', value)} placeholder="Cerca persone o corsi" />
      </header>

      <div className={styles.filters}>
        <div className={styles.chips}>
          {CHIP_PRESETS.map((preset) => (
            <button
              key={preset.value}
              type="button"
              className={`${styles.chip} ${statoFilter === preset.value ? styles.chipActive : ''}`}
              onClick={() => updateParam('stato', preset.value === 'tutti' ? null : preset.value)}
            >
              {preset.label}
            </button>
          ))}
          {RITARDO_OPTIONS.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`${styles.chip} ${ritardoFilter === option.value ? styles.chipActive : ''}`}
              onClick={() => updateParam('ritardo_gg', ritardoFilter === option.value ? null : option.value)}
            >
              {option.label}
            </button>
          ))}
        </div>
        {ritardoBanner && (
          <div className={styles.banner}>
            <span>Filtro: {ritardoBanner}</span>
            <button type="button" onClick={() => updateParam('ritardo_gg', null)}>Rimuovi</button>
          </div>
        )}
      </div>

      {totalFiltered === 0 ? (
        <div className={styles.empty}>
          <strong>Inbox zero</strong>
          <span>Nessuna azione richiesta con i filtri correnti.</span>
        </div>
      ) : (
        <div className={styles.groups}>
          {TEMPORAL_BUCKET_ORDER.map((bucket) => {
            const rows = grouped.get(bucket) ?? [];
            if (rows.length === 0) return null;
            const criticals = rows.filter((r) => classifyAlertLevel(r, { now }) === 'critical').length;
            return (
              <section key={bucket} className={styles.group}>
                <header className={styles.groupHead}>
                  <h2>{TEMPORAL_BUCKET_LABEL[bucket]}</h2>
                  <span className={styles.count}>
                    {rows.length} iscrizioni{criticals > 0 ? ` · ${criticals} critiche` : ''}
                  </span>
                </header>
                <div className={styles.cardList}>
                  {rows.map((enrollment) => (
                    <PipelineCard
                      key={enrollment.id}
                      enrollment={enrollment}
                      selected={selectedIds.has(enrollment.id)}
                      onToggle={() => toggleSelection(enrollment.id)}
                      onOpen={() => setOpenId(enrollment.id)}
                      now={now}
                    />
                  ))}
                </div>
              </section>
            );
          })}
        </div>
      )}

      <BulkActionBar
        selectedCount={selectedIds.size}
        onClear={clearSelection}
        summary={selectedBudget > 0 ? `Budget impatto ${new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 }).format(selectedBudget)}` : undefined}
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
            {selectedBudget > 0 ? ` (impatto budget ${new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 }).format(selectedBudget)})` : ''}?
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
