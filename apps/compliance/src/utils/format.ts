import { formatLocalDate } from '@mrsmith/format';

export function formatRequestDate(value: string): string {
  return formatLocalDate(value) ?? '—';
}
