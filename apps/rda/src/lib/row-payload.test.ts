import { buildRowPayload } from './row-payload.js';
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

console.log('row-payload tests passed');
