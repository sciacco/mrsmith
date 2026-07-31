import assert from 'node:assert/strict';
import test from 'node:test';
import { formatInstant, formatLocalDate, formatLocalDateTime } from '../src/date.ts';

const defaultDate = '16/07/2026';
const defaultDateTime = '16/07/2026, 12:30';

function withTimeZone<T>(timeZone: string, callback: () => T): T {
  const previous = process.env.TZ;
  process.env.TZ = timeZone;
  try {
    return callback();
  } finally {
    if (previous === undefined) delete process.env.TZ;
    else process.env.TZ = previous;
  }
}

test('formatLocalDate preserves the civil date across supported wire tails', () => {
  for (const value of [
    '2026-07-16',
    '2026-07-16T00:00:00Z',
    '2026-07-16T23:59:59.123456789+02:00',
    '2026-07-16 00:00:00',
  ]) {
    assert.equal(formatLocalDate(value), defaultDate);
  }
});

test('formatLocalDate validates calendar dates and supported syntax', () => {
  assert.equal(formatLocalDate('2024-02-29'), '29/02/2024');
  assert.equal(formatLocalDate('2023-02-29'), null);
  assert.equal(formatLocalDate('2026-04-31'), null);
  assert.equal(formatLocalDate('2026-07-16T00:00:00'), null);
  assert.equal(formatLocalDate('2026/07/16'), null);
  assert.equal(formatLocalDate(null), null);
  assert.equal(formatLocalDate(undefined), null);
});

test('formatLocalDate supports custom locale and visual options without timezone conversion', () => {
  const options = { locale: 'de-DE', format: { day: '2-digit', month: '2-digit', year: 'numeric' } };
  assert.equal(formatLocalDate('2026-07-16', options), '16.07.2026');
  assert.equal(
    withTimeZone('America/Los_Angeles', () => formatLocalDate('2026-07-16')),
    defaultDate,
  );
  assert.equal(formatLocalDate('2026-07-16', { format: { timeZone: 'Europe/Rome' } }), null);
});

test('formatLocalDateTime accepts space/T separators and fractional seconds', () => {
  assert.equal(formatLocalDateTime('2026-07-16 12:30'), defaultDateTime);
  assert.equal(formatLocalDateTime('2026-07-16T12:30:00'), defaultDateTime);
  assert.equal(formatLocalDateTime('2026-07-16T12:30:00.123456'), defaultDateTime);
  assert.equal(
    formatLocalDateTime('2026-07-16T12:30:00.123456', {
      locale: 'en-GB',
      format: { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 },
    }),
    '12:30:00.123',
  );
});

test('formatLocalDateTime validates boundaries and rejects timezone-bearing values', () => {
  assert.equal(formatLocalDateTime('2024-02-29T00:00'), '29/02/2024, 00:00');
  assert.equal(formatLocalDateTime('2026-07-16T23:59:59.999999999'), '16/07/2026, 23:59');
  assert.equal(formatLocalDateTime('2023-02-29T12:30'), null);
  assert.equal(formatLocalDateTime('2026-07-16T24:00'), null);
  assert.equal(formatLocalDateTime('2026-07-16T12:60'), null);
  assert.equal(formatLocalDateTime('2026-07-16T12:30:00Z'), null);
  assert.equal(formatLocalDateTime('2026-07-16T12:30:00+02:00'), null);
  assert.equal(formatLocalDateTime('2026-07-16T12:30:00.123456789Z'), null);
  assert.equal(formatLocalDateTime('2026-07-16T12:30', { format: { timeZone: 'UTC' } }), null);
});

test('formatLocalDateTime is independent of the process timezone and DST rules', () => {
  assert.equal(
    withTimeZone('America/Los_Angeles', () => formatLocalDateTime('2026-03-29T02:30')),
    '29/03/2026, 02:30',
  );
});

test('formatInstant converts Z, positive and negative offsets to Rome time', () => {
  assert.equal(formatInstant('2026-07-16T12:30:00Z'), '16/07/2026, 14:30');
  assert.equal(formatInstant('2026-07-16T14:30:00+02:00'), '16/07/2026, 14:30');
  assert.equal(formatInstant('2026-07-16T12:30:00-04:00'), '16/07/2026, 18:30');
  assert.equal(formatInstant('2026-01-16T12:30:00Z'), '16/01/2026, 13:30');
  assert.equal(formatInstant('2026-07-16T23:30:00Z'), '17/07/2026, 01:30');
});

test('formatInstant validates explicit offsets, dates, times and high precision fractions', () => {
  assert.equal(formatInstant('2026-07-16T12:30:00.1Z'), '16/07/2026, 14:30');
  assert.equal(formatInstant('2026-07-16T12:30:00.123456789Z'), '16/07/2026, 14:30');
  assert.equal(formatInstant('2026-07-16T12:30:00'), null);
  assert.equal(formatInstant('2026-07-16T12:30:00.123456789'), null);
  assert.equal(formatInstant('2023-02-29T12:30:00Z'), null);
  assert.equal(formatInstant('2026-02-29T12:30:00Z'), null);
  assert.equal(formatInstant('2026-07-16T24:00:00Z'), null);
  assert.equal(formatInstant('2026-07-16T12:30:00+24:00'), null);
  assert.equal(formatInstant(null), null);
});

test('formatInstant accepts a timezone override and returns null for invalid Intl options', () => {
  assert.equal(
    formatInstant('2026-07-16T12:30:00Z', { locale: 'en-GB', format: { timeZone: 'UTC', dateStyle: 'short', timeStyle: 'short' } }),
    '16/07/2026, 12:30',
  );
  assert.equal(formatInstant('2026-07-16T12:30:00Z', { format: { timeZone: 'Not/AZone' } }), null);
  assert.equal(
    formatInstant('2026-07-16T12:30:00Z', { format: { dateStyle: 'full', hour: '2-digit' } }),
    null,
  );
  assert.equal(formatLocalDate('2026-07-16', { locale: 'not_a_locale' }), null);
});
