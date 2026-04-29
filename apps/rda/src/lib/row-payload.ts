import type { Article, RowPayload } from '../api/types';

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

export function rowTypeFromArticle(article: Article | null): RowPayload['type'] {
  return article?.type ?? 'service';
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
