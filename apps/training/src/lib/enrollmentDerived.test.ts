import assert from 'node:assert/strict';
import test from 'node:test';
import { formatEuro2, formatEuroCompact } from './enrollmentDerived.ts';

test('formatEuroCompact rounds before rendering, including negative half values', () => {
  assert.equal(formatEuroCompact(12.56), '13');
  assert.equal(formatEuroCompact(1234.56), '1235');
  // Math.round(-12.5) is -12; native zero-decimal Intl rendering alone would be -13.
  assert.equal(formatEuroCompact(-12.5), '-12');
});

test('formatEuro2 renders from zero through two fraction digits and rounds extra precision', () => {
  assert.equal(formatEuro2(12), '12');
  assert.equal(formatEuro2(12.5), '12,5');
  assert.equal(formatEuro2(12.34), '12,34');
  assert.equal(formatEuro2(12.345), '12,35');
});
