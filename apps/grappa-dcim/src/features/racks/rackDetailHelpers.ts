import type { RackPowerSummaryPoint, RackUnit } from '../../api/types';

export type SlotEntry =
  | { kind: 'occupied'; unitNum: number; unit: RackUnit }
  | { kind: 'empty-single'; unitNum: number }
  | { kind: 'empty-run'; from: number; to: number; count: number };

export function buildSlotEntries(units: RackUnit[], unitCount: number, collapseThreshold = 2): SlotEntry[] {
  const occupied = new Map<number, RackUnit>();
  for (const u of units) {
    if (u.num !== undefined) occupied.set(u.num, u);
  }

  const entries: SlotEntry[] = [];
  let i = 1;
  while (i <= unitCount) {
    if (occupied.has(i)) {
      entries.push({ kind: 'occupied', unitNum: i, unit: occupied.get(i)! });
      i++;
    } else {
      let j = i + 1;
      while (j <= unitCount && !occupied.has(j)) j++;
      const count = j - i;
      if (count >= collapseThreshold) {
        entries.push({ kind: 'empty-run', from: i, to: j - 1, count });
      } else {
        for (let k = i; k < j; k++) {
          entries.push({ kind: 'empty-single', unitNum: k });
        }
      }
      i = j;
    }
  }
  return entries;
}

export function buildSparklinePath(points: RackPowerSummaryPoint[], width: number, height: number): string {
  const valid = points.filter((p) => p.kilowatt !== undefined);
  if (valid.length < 2) return '';
  const values = valid.map((p) => p.kilowatt!);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  return valid
    .map((p, i) => {
      const x = ((i / (valid.length - 1)) * width).toFixed(1);
      const y = (height - ((p.kilowatt! - min) / range) * (height * 0.9)).toFixed(1);
      return `${x},${y}`;
    })
    .join(' ');
}

export function formatRelativeTime(isoDate?: string): string {
  if (!isoDate) return 'Mai';
  const diff = Date.now() - new Date(isoDate).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return 'Ora';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} min fa`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} ore fa`;
  const days = Math.floor(hours / 24);
  return `${days} gg fa`;
}

export function formatDate(isoDate?: string): string {
  if (!isoDate) return '—';
  return new Intl.DateTimeFormat('it-IT', { day: 'numeric', month: 'short', year: 'numeric' }).format(
    new Date(isoDate),
  );
}
