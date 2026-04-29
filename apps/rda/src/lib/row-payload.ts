import type { Article, PoRow, RowPayload } from '../api/types';
import { parseMistraMoney } from './format.js';

export interface RowPayloadDraft {
  article: Article | null;
  description: string;
  qty: number;
  price: number;
  nrc: number;
  mrc: number;
  duration: number;
  recurrence: number;
  startAt: string;
  startDate: string;
  automaticRenew: boolean;
  cancellationAdvice: string;
}

export function emptyRowDraft(): RowPayloadDraft {
  return {
    article: null,
    description: '',
    qty: 1,
    price: 0,
    nrc: 0,
    mrc: 0,
    duration: 12,
    recurrence: 1,
    startAt: 'activation_date',
    startDate: '',
    automaticRenew: false,
    cancellationAdvice: '',
  };
}

export function rowTypeFromArticle(article: Article | null): RowPayload['type'] {
  return article?.type ?? 'service';
}

export function draftFromPoRow(row: PoRow, catalog: Article[] = []): RowPayloadDraft {
  const type: Article['type'] = row.type === 'good' ? 'good' : 'service';
  const code = row.product_code ?? '';
  const fallbackArticle = code
    ? {
        code,
        description: row.product_description ?? row.description ?? code,
        type,
      }
    : null;
  const article = catalog.find((item) => item.code === code && item.type === type) ?? fallbackArticle;
  const startAt = normalizeStartAt(row.payment_detail?.start_at ?? row.payment_detail?.start_pay_at_activation_date, type);
  const duration = parseMistraMoney(row.renew_detail?.initial_subscription_months);
  const recurrence = parseMistraMoney(row.payment_detail?.month_recursion);

  return {
    article,
    description: row.description ?? row.product_description ?? '',
    qty: nonZero(parseMistraMoney(row.qty), 1),
    price: parseMistraMoney(row.price),
    nrc: parseMistraMoney(row.activation_fee ?? row.activation_price),
    mrc: parseMistraMoney(row.montly_fee ?? row.monthly_fee),
    duration: nonZero(duration, 12),
    recurrence: nonZero(recurrence, 1),
    startAt,
    startDate: row.payment_detail?.start_at_date ?? '',
    automaticRenew: Boolean(row.renew_detail?.automatic_renew),
    cancellationAdvice: row.renew_detail?.cancellation_advice == null ? '' : String(row.renew_detail.cancellation_advice),
  };
}

function nonZero(value: number, fallback: number): number {
  return value > 0 ? value : fallback;
}

function normalizeStartAt(value: unknown, type: Article['type']): string {
  const startAt = typeof value === 'string' ? value : '';
  if (type === 'good' && (startAt === 'activation_date' || startAt === 'advance_payment' || startAt === 'specific_date')) return startAt;
  if (type === 'service' && (startAt === 'activation_date' || startAt === 'specific_date')) return startAt;
  return 'activation_date';
}

export function buildRowPayload(draft: RowPayloadDraft): RowPayload {
  const type = rowTypeFromArticle(draft.article);
  return {
    type,
    description: draft.description.trim(),
    qty: draft.qty,
    product_code: draft.article?.code ?? '',
    product_description: draft.article?.description ?? draft.article?.code ?? '',
    ...(type === 'good' ? { price: draft.price } : { monthly_fee: draft.mrc, activation_price: draft.nrc }),
    payment_detail: {
      start_at: draft.startAt,
      ...(draft.startAt === 'specific_date' ? { start_at_date: draft.startDate } : {}),
      ...(type === 'service' ? { month_recursion: draft.recurrence } : {}),
    },
    ...(type === 'service'
      ? {
          renew_detail: {
            initial_subscription_months: draft.duration,
            automatic_renew: draft.automaticRenew,
            cancellation_advice: draft.cancellationAdvice,
          },
        }
      : {}),
  };
}

export function rowPreviewTotal(draft: RowPayloadDraft): number {
  const type = rowTypeFromArticle(draft.article);
  if (type === 'good') return draft.price * draft.qty;
  return draft.mrc * draft.qty * draft.duration + draft.nrc * draft.qty;
}
