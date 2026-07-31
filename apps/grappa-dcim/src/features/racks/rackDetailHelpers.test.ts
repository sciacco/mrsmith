import assert from 'node:assert/strict';
import test from 'node:test';
import { createRackDetailFormatterAdapter } from './rackDetailFormatters.ts';
import { formatRackPowerLabel } from './rackSearchHelpers.ts';

test('formatDate delegates the instant and requested date format to the canonical formatter', () => {
  const calls: Array<{ value: string | null | undefined; options: unknown }> = [];
  const adapter = createRackDetailFormatterAdapter({
    formatInstant(value, options) {
      calls.push({ value, options });
      return 'formatted date';
    },
    formatNumber: () => null,
  });

  assert.equal(adapter.formatDate('2026-07-16T12:30:00Z'), 'formatted date');
  assert.deepEqual(calls, [
    {
      value: '2026-07-16T12:30:00Z',
      options: { format: { day: 'numeric', month: 'short', year: 'numeric' } },
    },
  ]);
});

test('formatRackNumber delegates the value and precision option to the canonical formatter', () => {
  const calls: Array<{ value: number | null | undefined; options: unknown }> = [];
  const adapter = createRackDetailFormatterAdapter({
    formatInstant: () => null,
    formatNumber(value, options) {
      calls.push({ value, options });
      return 'formatted number';
    },
  });

  assert.equal(adapter.formatRackNumber(1.236), 'formatted number');
  assert.deepEqual(calls, [{ value: 1.236, options: { format: { maximumFractionDigits: 2 } } }]);
});

test('power labels keep the local fallback and unit semantics', () => {
  const formatNumber = (value: number) => `localized ${value}`;

  assert.equal(formatRackPowerLabel(undefined, formatNumber), '-');
  assert.equal(formatRackPowerLabel(0, formatNumber), '-');
  assert.equal(formatRackPowerLabel(12.5, formatNumber), 'localized 12.5 kW');
});

test('formatter failures use the adapter fallback', () => {
  const adapter = createRackDetailFormatterAdapter({
    formatInstant: () => null,
    formatNumber: () => null,
  });

  assert.equal(adapter.formatDate(), '—');
  assert.equal(adapter.formatRackNumber(null), '—');
});
