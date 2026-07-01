import { useCallback, useMemo, useState } from 'react';

// ── Granularity & period model ──

export type Granularity = 'daily' | 'weekly' | 'monthly' | 'quarterly' | 'yearly';

export const GRANULARITIES: Granularity[] = ['daily', 'weekly', 'monthly', 'quarterly', 'yearly'];

const GRANULARITY_ORDER: Granularity[] = ['daily', 'weekly', 'monthly', 'quarterly', 'yearly'];

export const GROUP_LABELS: Record<Granularity, string> = {
  daily: 'Giornaliera',
  weekly: 'Settimanale',
  monthly: 'Mensile',
  quarterly: 'Trimestrale',
  yearly: 'Annuale',
};

export type PeriodPreset = '30d' | '90d' | '120d' | '6m' | '12m' | '24m';

export const PERIOD_OPTIONS: { value: PeriodPreset; label: string }[] = [
  { value: '30d', label: 'Ultimi 30 giorni' },
  { value: '90d', label: 'Ultimi 90 giorni' },
  { value: '120d', label: 'Ultimi 120 giorni' },
  { value: '6m', label: 'Ultimi 6 mesi' },
  { value: '12m', label: 'Ultimi 12 mesi' },
  { value: '24m', label: 'Ultimi 24 mesi' },
];

// Sensible default granularity for each window.
const PERIOD_DEFAULT_GROUP: Record<PeriodPreset, Granularity> = {
  '30d': 'daily',
  '90d': 'daily',
  '120d': 'daily',
  '6m': 'weekly',
  '12m': 'monthly',
  '24m': 'quarterly',
};

// ── Date helpers (all bucket strings are YYYY-MM-DD) ──

const MONTHS_SHORT = ['Gen', 'Feb', 'Mar', 'Apr', 'Mag', 'Giu', 'Lug', 'Ago', 'Set', 'Ott', 'Nov', 'Dic'];

export function toISODate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export function todayISO(): string {
  return toISODate(new Date());
}

export function periodToFrom(preset: PeriodPreset): string {
  const d = new Date();
  switch (preset) {
    case '30d': d.setDate(d.getDate() - 30); break;
    case '90d': d.setDate(d.getDate() - 90); break;
    case '120d': d.setDate(d.getDate() - 120); break;
    case '6m': d.setMonth(d.getMonth() - 6); break;
    case '12m': d.setMonth(d.getMonth() - 12); break;
    case '24m': d.setMonth(d.getMonth() - 24); break;
  }
  return toISODate(d);
}

// parseDate parses a YYYY-MM-DD bucket string into a local-midnight Date.
// Tolerates an optional time/timezone component (the MySQL driver can return
// DATE columns as RFC3339) by taking only the first 10 chars. Building the date
// from parts avoids UTC-vs-local day shifts.
function parseDate(s: string): Date {
  const [y, m, d] = s.slice(0, 10).split('-').map(Number);
  return new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1);
}

// Range [from, to] covered by a bucket anchor, given the bucket's granularity.
// from/to are always normalized to YYYY-MM-DD via toISODate.
function bucketRange(anchor: string, group: Granularity): { from: string; to: string } {
  const d = parseDate(anchor);
  const start = toISODate(d);
  switch (group) {
    case 'daily':
      return { from: start, to: start };
    case 'weekly': {
      const end = new Date(d);
      end.setDate(end.getDate() + 6);
      return { from: start, to: toISODate(end) };
    }
    case 'monthly':
      return { from: start, to: toISODate(new Date(d.getFullYear(), d.getMonth() + 1, 0)) };
    case 'quarterly':
      return { from: start, to: toISODate(new Date(d.getFullYear(), d.getMonth() + 3, 0)) };
    case 'yearly':
      return { from: start, to: toISODate(new Date(d.getFullYear(), 11, 31)) };
  }
}

// Human label for a bucket anchor, expressed in the bucket's own granularity.
export function bucketLabel(anchor: string, group: Granularity): string {
  const d = parseDate(anchor);
  if (Number.isNaN(d.getTime())) return anchor;
  switch (group) {
    case 'daily':
      return `${d.getDate()} ${MONTHS_SHORT[d.getMonth()]} ${d.getFullYear()}`;
    case 'weekly':
      return `Sett. del ${d.getDate()} ${MONTHS_SHORT[d.getMonth()]}`;
    case 'monthly':
      return `${MONTHS_SHORT[d.getMonth()]} ${d.getFullYear()}`;
    case 'quarterly':
      return `T${Math.floor(d.getMonth() / 3) + 1} ${d.getFullYear()}`;
    case 'yearly':
      return `${d.getFullYear()}`;
  }
}

// Compact tick label for chart axes.
export function formatBucketTick(bucket: string, group: Granularity): string {
  const d = parseDate(bucket);
  if (Number.isNaN(d.getTime())) return bucket;
  const yy = String(d.getFullYear()).slice(2);
  switch (group) {
    case 'daily':
    case 'weekly':
      return `${d.getDate()} ${MONTHS_SHORT[d.getMonth()]}`;
    case 'monthly':
      return `${MONTHS_SHORT[d.getMonth()]} ${yy}`;
    case 'quarterly':
      return `T${Math.floor(d.getMonth() / 3) + 1} '${yy}`;
    case 'yearly':
      return `${d.getFullYear()}`;
  }
}

function finer(group: Granularity): Granularity | null {
  const i = GRANULARITY_ORDER.indexOf(group);
  if (i <= 0) return null;
  return GRANULARITY_ORDER[i - 1] ?? null;
}

// ── Drill state ──

export interface DrillSegment {
  from: string;
  to: string;
  group: Granularity;   // granularity of the drilled-into view
  label: string;        // breadcrumb label, e.g. "Mar 2026 · Giornaliera"
}

export interface BreadcrumbItem {
  label: string;
  index: number; // -1 = base view
}

export function useChargeDrill(initialPeriod: PeriodPreset = '12m') {
  const [period, setPeriodState] = useState<PeriodPreset>(initialPeriod);
  const [granularity, setGranularity] = useState<Granularity>(PERIOD_DEFAULT_GROUP[initialPeriod]);
  const [segments, setSegments] = useState<DrillSegment[]>([]);

  const atBase = segments.length === 0;

  // Changing the base window resets granularity to its default and clears drill.
  const setPeriod = useCallback((p: PeriodPreset) => {
    setPeriodState(p);
    setGranularity(PERIOD_DEFAULT_GROUP[p]);
    setSegments([]);
  }, []);

  // Manual granularity override is only allowed at base view.
  const setGranularitySafe = useCallback((g: Granularity) => {
    setSegments([]);
    setGranularity(g);
  }, []);

  const reset = useCallback(() => setSegments([]), []);

  // Effective query params: last drill segment if active, else the base window.
  const effective = useMemo(() => {
    const last = segments[segments.length - 1];
    if (last) {
      return { from: last.from, to: last.to, group: last.group };
    }
    return { from: periodToFrom(period), to: todayISO(), group: granularity };
  }, [segments, period, granularity]);

  const currentGroup = effective.group;
  const canDrill = finer(currentGroup) !== null;

  // Drill into a bucket anchor of the *current* view.
  const drillInto = useCallback((anchor: string) => {
    const newGroup = finer(currentGroup);
    if (!newGroup) return;
    const { from, to } = bucketRange(anchor, currentGroup);
    const label = `${bucketLabel(anchor, currentGroup)} · ${GROUP_LABELS[newGroup]}`;
    setSegments(prev => [...prev, { from, to, group: newGroup, label }]);
  }, [currentGroup]);

  const rollUpTo = useCallback((index: number) => {
    setSegments(prev => (index < 0 ? [] : prev.slice(0, index + 1)));
  }, []);

  // Breadcrumb: base crumb (-1) + one per segment.
  const breadcrumb = useMemo<BreadcrumbItem[]>(() => {
    const base = {
      label: `${PERIOD_OPTIONS.find(o => o.value === period)?.label ?? period} · ${GROUP_LABELS[granularity]}`,
      index: -1,
    };
    return [base, ...segments.map((seg, i) => ({ label: seg.label, index: i }))];
  }, [period, granularity, segments]);

  return {
    period,
    granularity,
    setPeriod,
    setGranularity: setGranularitySafe,
    segments,
    effective,
    atBase,
    canDrill,
    currentGroup,
    drillInto,
    rollUpTo,
    reset,
    breadcrumb,
  };
}
