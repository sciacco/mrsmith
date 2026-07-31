import assert from 'node:assert/strict';
import test from 'node:test';
import { formatBudget, formatRequiredBudget } from './formatBudget.ts';

test('formatBudget preserves missing values and formats zero as zero-decimal EUR', () => {
  assert.equal(formatBudget(null), undefined);
  assert.equal(formatBudget(undefined), undefined);
  assert.equal(formatBudget(0), '0\u00a0€');
});

test('formatBudget delegates cent rounding and preserves negative finite values', () => {
  // These are the native it-IT Intl zero-decimal results, including half-up rounding.
  assert.equal(formatBudget(1234.56), '1235\u00a0€');
  assert.equal(formatBudget(-12.5), '-13\u00a0€');
});

test('formatRequiredBudget matches valid budgets and falls back for non-finite values', () => {
  assert.equal(formatRequiredBudget(1234.56), formatBudget(1234.56));
  assert.equal(formatRequiredBudget(0), formatBudget(0));
  assert.equal(formatRequiredBudget(Number.NaN), '—');
  assert.equal(formatRequiredBudget(Number.POSITIVE_INFINITY), '—');
  assert.equal(formatRequiredBudget(Number.NEGATIVE_INFINITY), '—');
});
