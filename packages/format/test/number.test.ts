import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, extname, resolve } from 'node:path';
import test from 'node:test';
import { fileURLToPath, pathToFileURL } from 'node:url';
import type {
  CurrencyFormatOptions,
  InstantFormatOptions,
  LocalDateFormatOptions,
  LocalDateTimeFormatOptions,
  NumberFormatOptions,
} from '../src/index.ts';
import { formatCurrency, formatNumber } from '../src/number.ts';

const numberOptions: NumberFormatOptions = {
  locale: 'it-IT',
  format: { maximumFractionDigits: 2 },
};
const currencyOptions: CurrencyFormatOptions = {
  locale: 'it-IT',
  format: { maximumFractionDigits: 2 },
};
const localDateOptions: LocalDateFormatOptions = { locale: 'it-IT' };
const localDateTimeOptions: LocalDateTimeFormatOptions = { locale: 'it-IT' };
const instantOptions: InstantFormatOptions = { locale: 'it-IT' };

void numberOptions;
void currencyOptions;
void localDateOptions;
void localDateTimeOptions;
void instantOptions;

test('formatNumber uses the Italian default and native grouping boundaries', () => {
  assert.equal(formatNumber(9999), '9999');
  assert.equal(formatNumber(10000), '10.000');
  assert.equal(formatNumber(1234.56), '1234,56');
  assert.equal(formatNumber(12345.67), '12.345,67');
});

test('formatNumber preserves zero and negative values', () => {
  assert.equal(formatNumber(0), '0');
  assert.equal(formatNumber(-1234.5), '-1234,5');
});

test('formatNumber accepts native precision options without domain rounding', () => {
  assert.equal(formatNumber(1.23456, { format: { maximumFractionDigits: 5 } }), '1,23456');
  assert.equal(formatNumber(1.23456, { format: { minimumFractionDigits: 3 } }), '1,235');
  assert.equal(formatNumber(1234.5, { locale: 'en-US', format: { minimumFractionDigits: 3 } }), '1,234.500');
});

test('formatNumber returns null for absent, non-finite, and malformed input', () => {
  assert.equal(formatNumber(null), null);
  assert.equal(formatNumber(undefined), null);
  assert.equal(formatNumber(Number.NaN), null);
  assert.equal(formatNumber(Number.POSITIVE_INFINITY), null);
  assert.equal(formatNumber(Number.NEGATIVE_INFINITY), null);
  assert.equal(formatNumber('12' as unknown as number), null);
  assert.equal(formatNumber(12, { locale: 'not_a_locale' }), null);
  assert.equal(formatNumber(12, { format: { minimumFractionDigits: 3, maximumFractionDigits: 2 } }), null);
  assert.equal(formatNumber(12, { format: null as unknown as Intl.NumberFormatOptions }), null);
});

test('formatCurrency uses requested currencies and standard currency precision', () => {
  assert.equal(formatCurrency(1234.56), '1234,56\u00a0€');
  assert.equal(formatCurrency(1234.56, 'USD'), '1234,56\u00a0USD');
  assert.equal(formatCurrency(1234.56, 'JPY'), '1235\u00a0JPY');
  assert.equal(formatCurrency(1234.56, 'USD', { locale: 'en-US' }), '$1,234.56');
});

test('formatCurrency rejects syntactically valid but unsupported currency codes', () => {
  assert.equal(formatCurrency(12, 'FOO'), null);
  assert.equal(formatCurrency(12, 'ZZZ'), null);
  assert.equal(formatCurrency(12, 'EUR'), '12,00\u00a0€');
  assert.equal(formatCurrency(12, 'USD'), '12,00\u00a0USD');
  assert.equal(formatCurrency(12, 'JPY'), '12\u00a0JPY');
});

test('formatCurrency allows native precision overrides and preserves signs', () => {
  assert.equal(formatCurrency(12.3456, 'EUR', { format: { minimumFractionDigits: 3 } }), '12,346\u00a0€');
  assert.equal(formatCurrency(0, 'EUR'), '0,00\u00a0€');
  assert.equal(formatCurrency(-12.5, 'USD'), '-12,50\u00a0USD');
  assert.equal(formatCurrency(12.3456, 'EUR', { format: { maximumFractionDigits: 4 } }), '12,3456\u00a0€');
});

test('formatCurrency forces style and currency over runtime option attempts', () => {
  const attemptedOverride = {
    style: 'decimal',
    currency: 'USD',
    maximumFractionDigits: 0,
  } as unknown as CurrencyFormatOptions['format'];
  assert.equal(formatCurrency(12.5, 'EUR', { format: attemptedOverride }), '13\u00a0€');
});

test('formatCurrency rejects bad values, currencies, locales, and options', () => {
  assert.equal(formatCurrency(null), null);
  assert.equal(formatCurrency(undefined), null);
  assert.equal(formatCurrency(Number.NaN), null);
  assert.equal(formatCurrency(Number.POSITIVE_INFINITY), null);
  assert.equal(formatCurrency(Number.NEGATIVE_INFINITY), null);
  assert.equal(formatCurrency('12' as unknown as number), null);
  assert.equal(formatCurrency(12, 'not-a-currency'), null);
  assert.equal(formatCurrency(12, '€'), null);
  assert.equal(formatCurrency(12, 'euro'), null);
  assert.equal(formatCurrency(12, 'EUR', { locale: 'not_a_locale' }), null);
  assert.equal(formatCurrency(12, 'EUR', { format: { minimumFractionDigits: 3, maximumFractionDigits: 2 } }), null);
  assert.equal(formatCurrency(12, 'EUR', { format: null as unknown as CurrencyFormatOptions['format'] }), null);
});

const barrelUrl = new URL('../src/index.ts', import.meta.url);

function absoluteTypeScriptSpecifier(specifier: string): string {
  const path = resolve(dirname(fileURLToPath(barrelUrl)), specifier);
  return pathToFileURL(extname(path) === '' ? `${path}.ts` : path).href;
}

async function loadPublicNamespace(): Promise<Record<string, unknown>> {
  const source = readFileSync(barrelUrl, 'utf8')
    // Type exports describe the compile-time API, but are not part of a runtime data URL.
    .replace(/export\s+type\s+\{[^}]*\}\s+from\s+['"][^'"]+['"]\s*;?/g, '')
    // Keep the package's extensionless production specifiers; resolve them only in this test.
    .replace(/\bfrom\s+(['"])(\.\.?\/[^'"]+)\1/g, (_, quote: string, specifier: string) =>
      `from ${quote}${absoluteTypeScriptSpecifier(specifier)}${quote}`,
    );

  return (await import(`data:text/javascript,${encodeURIComponent(source)}`)) as Record<string, unknown>;
}

test('the loaded public namespace has exactly five functions and invokes each one', async () => {
  const namespace = await loadPublicNamespace();
  const expectedExports = [
    'formatCurrency',
    'formatInstant',
    'formatLocalDate',
    'formatLocalDateTime',
    'formatNumber',
  ];

  assert.deepEqual(Object.keys(namespace).sort(), expectedExports.sort());

  const invoke = (name: string, ...args: unknown[]): unknown => {
    const implementation = namespace[name];
    assert.equal(typeof implementation, 'function', `${name} should be a function`);
    return (implementation as (...values: unknown[]) => unknown)(...args);
  };

  assert.equal(invoke('formatLocalDate', '2026-07-16'), '16/07/2026');
  assert.equal(invoke('formatLocalDateTime', '2026-07-16T12:30'), '16/07/2026, 12:30');
  assert.equal(invoke('formatInstant', '2026-07-16T12:30:00Z'), '16/07/2026, 14:30');
  assert.equal(invoke('formatNumber', 1234.56), '1234,56');
  assert.equal(invoke('formatCurrency', 12.5, 'EUR'), '12,50 €');
});
