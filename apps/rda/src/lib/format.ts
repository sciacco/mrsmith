import type { CurrencyCode, PoApprover, PoDetail, PoPreview } from '../api/types';

export const DEFAULT_RDA_CURRENCY: CurrencyCode = 'EUR';
export const RDA_CURRENCIES: CurrencyCode[] = ['EUR', 'USD', 'GBP'];

export function normalizeCurrency(value?: string | null): CurrencyCode {
  const normalized = value?.trim().toUpperCase();
  return RDA_CURRENCIES.includes(normalized as CurrencyCode) ? (normalized as CurrencyCode) : DEFAULT_RDA_CURRENCY;
}

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

export function formatMoney(value?: string | number | null, currency?: string | null): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: normalizeCurrency(currency),
    currencyDisplay: 'narrowSymbol',
  }).format(parseMistraMoney(value));
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

function approvalLevel(value: unknown): string {
  return String(value ?? '').trim();
}

export function isApprover(po: Pick<PoPreview, 'approvers' | 'current_approval_level'> | undefined, userEmail?: string | null): boolean {
  const normalizedEmail = userEmail?.trim().toLowerCase();
  if (!po || !normalizedEmail) return false;
  const currentLevel = approvalLevel(po.current_approval_level);
  return (po.approvers ?? []).some((approver) => {
    if (approver.user?.email?.trim().toLowerCase() !== normalizedEmail) return false;
    const approverLevel = approvalLevel(approver.level);
    return currentLevel === '' || approverLevel === '' || approverLevel === currentLevel;
  });
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
