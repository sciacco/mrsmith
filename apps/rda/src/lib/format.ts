import type { PoApprover, PoDetail } from '../api/types';

export function formatDateIT(raw?: string | null): string {
  if (!raw) return '-';
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return raw.slice(0, 10);
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'medium' }).format(parsed);
}

export function formatDateTimeIT(raw?: string | null): string {
  if (!raw) return '-';
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return raw;
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'medium', timeStyle: 'short' }).format(parsed);
}

export function parseMistraMoney(value?: string | number | null): number {
  if (typeof value === 'number') return Number.isFinite(value) ? value : 0;
  if (!value) return 0;
  let cleaned = String(value).replace(/[^\d,.-]/g, '');
  if (cleaned.includes(',') && cleaned.includes('.')) {
    cleaned = cleaned.replaceAll('.', '').replace(',', '.');
  } else {
    cleaned = cleaned.replace(',', '.');
  }
  const parsed = Number(cleaned);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function formatMoneyEUR(value?: string | number | null): string {
  return new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR' }).format(parseMistraMoney(value));
}

export function extractApproverList(approvers?: PoApprover[]): string {
  const emails = (approvers ?? [])
    .map((approver) => approver.user?.email)
    .filter((email): email is string => Boolean(email));
  return emails.length ? emails.join(', ') : '-';
}

export function isRequester(po: PoDetail | undefined, userEmail?: string | null): boolean {
  return Boolean(po?.requester?.email && userEmail && po.requester.email.toLowerCase() === userEmail.toLowerCase());
}

export function isApprover(po: PoDetail | undefined, userEmail?: string | null): boolean {
  if (!po || !userEmail) return false;
  return (po.approvers ?? []).some((approver) => approver.user?.email?.toLowerCase() === userEmail.toLowerCase());
}

export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export function coerceID(value: unknown): number | null {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}
