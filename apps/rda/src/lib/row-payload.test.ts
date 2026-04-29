import { buildRowPayload, draftFromPoRow } from './row-payload.js';
import type { Article } from '../api/types';

function assertEqual<T>(actual: T, expected: T, message: string) {
  if (actual !== expected) throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
}

function test(name: string, run: () => void) {
  try {
    run();
  } catch (error) {
    throw new Error(`${name}: ${error instanceof Error ? error.message : String(error)}`);
  }
}

const baseDraft = {
  description: 'Descrizione riga',
  qty: 2,
  price: 10,
  nrc: 5,
  mrc: 7,
  duration: 12,
  recurrence: 1,
  startAt: 'activation_date',
  startDate: '',
  automaticRenew: false,
  cancellationAdvice: '',
};

test('service article produces service row payload', () => {
  const article: Article = { code: 'SVC-1', description: 'Servizio gestito', type: 'service' };
  const payload = buildRowPayload({ ...baseDraft, article });

  assertEqual(payload.type, 'service', 'payload type should come from the selected article');
  assertEqual(payload.product_code, 'SVC-1', 'product code should come from the selected article');
  assertEqual(payload.monthly_fee, 7, 'service payload should include MRC');
  assertEqual(payload.activation_price, 5, 'service payload should include NRC');
  assertEqual(payload.price, undefined, 'service payload should not include good price');
});

test('good article produces good row payload', () => {
  const article: Article = { code: 'GOOD-1', description: 'Monitor', type: 'good' };
  const payload = buildRowPayload({ ...baseDraft, article });

  assertEqual(payload.type, 'good', 'payload type should come from the selected article');
  assertEqual(payload.product_code, 'GOOD-1', 'product code should come from the selected article');
  assertEqual(payload.price, 10, 'good payload should include unit price');
  assertEqual(payload.monthly_fee, undefined, 'good payload should not include MRC');
  assertEqual(payload.activation_price, undefined, 'good payload should not include NRC');
  assertEqual(payload.renew_detail, undefined, 'good payload should not include renewal detail');
});

test('existing service row produces editable draft from read shape', () => {
  const article: Article = { code: 'SVC-1', description: 'Servizio gestito', type: 'service' };
  const draft = draftFromPoRow(
    {
      id: 12,
      type: 'service',
      description: 'Canone aggiornato',
      product_code: 'SVC-1',
      product_description: 'Servizio gestito',
      qty: '2',
      montly_fee: '7,5',
      activation_fee: '5',
      payment_detail: { start_at: 'specific_date', start_at_date: '2026-05-01', month_recursion: '3' },
      renew_detail: { initial_subscription_months: '24', automatic_renew: true, cancellation_advice: 2 as unknown as string },
    },
    [article],
  );

  assertEqual(draft.article, article, 'draft should reuse catalog article when present');
  assertEqual(draft.description, 'Canone aggiornato', 'draft should keep row description');
  assertEqual(draft.qty, 2, 'draft should parse quantity');
  assertEqual(draft.mrc, 7.5, 'draft should parse legacy montly_fee');
  assertEqual(draft.nrc, 5, 'draft should parse activation fee');
  assertEqual(draft.duration, 24, 'draft should parse duration');
  assertEqual(draft.recurrence, 3, 'draft should parse recurrence');
  assertEqual(draft.startAt, 'specific_date', 'draft should keep specific date start');
  assertEqual(draft.startDate, '2026-05-01', 'draft should keep start date');
  assertEqual(draft.automaticRenew, true, 'draft should keep automatic renew');
  assertEqual(draft.cancellationAdvice, '2', 'draft should stringify cancellation advice');
});

test('existing good row uses fallback article when catalog misses code', () => {
  const draft = draftFromPoRow({
    id: 13,
    type: 'good',
    description: 'Monitor 27',
    product_code: 'MON-27',
    product_description: 'Monitor',
    qty: 3,
    price: '125.5',
    payment_detail: { start_at: 'advance_payment' },
  });

  assertEqual(draft.article?.code, 'MON-27', 'fallback article should keep product code');
  assertEqual(draft.article?.description, 'Monitor', 'fallback article should keep product description');
  assertEqual(draft.article?.type, 'good', 'fallback article should keep row type');
  assertEqual(draft.price, 125.5, 'draft should parse good price');
  assertEqual(draft.startAt, 'advance_payment', 'draft should keep good advance payment start');
});

console.log('row-payload tests passed');
