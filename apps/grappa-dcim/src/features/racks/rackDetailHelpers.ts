import type { RackPowerSummaryPoint } from '../../api/types';

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
