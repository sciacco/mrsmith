import type { WindowBody } from '../api/types';

export const WINDOW_RANGE_ERROR_MESSAGE = 'La fine della finestra deve essere successiva all’inizio.';

export function validateWindowTiming(body: Pick<WindowBody, 'scheduled_start_at' | 'scheduled_end_at'>): string | null {
  const start = Date.parse(body.scheduled_start_at);
  const end = Date.parse(body.scheduled_end_at);
  if (Number.isNaN(start) || Number.isNaN(end)) return WINDOW_RANGE_ERROR_MESSAGE;
  return end > start ? null : WINDOW_RANGE_ERROR_MESSAGE;
}

export function windowDurationMinutes(body: Pick<WindowBody, 'scheduled_start_at' | 'scheduled_end_at'>): number | null {
  const start = Date.parse(body.scheduled_start_at);
  const end = Date.parse(body.scheduled_end_at);
  if (Number.isNaN(start) || Number.isNaN(end)) return null;
  const minutes = Math.round((end - start) / 60000);
  return minutes > 0 ? minutes : null;
}
