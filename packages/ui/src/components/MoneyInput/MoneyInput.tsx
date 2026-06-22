import { useEffect, useState, type ChangeEvent } from 'react';
import styles from './MoneyInput.module.css';

// ── Public options ──

interface ParseOptions {
  locale?: string;
  fractionDigits?: number;
  allowNegative?: boolean;
}

interface FormatOptions {
  locale?: string;
  fractionDigits?: number;
}

// it-IT, de-DE, fr-FR, es-ES use "," as decimal and "." as thousands.
// en-US, en-GB, ja-JP use "." as decimal and "," as thousands.
// Used only to resolve the "exactly 3 trailing digits" ambiguity.
const COMMA_DECIMAL_LOCALES = ['it', 'de', 'fr', 'es', 'pt', 'nl', 'sv', 'pl', 'ro'];

function isCommaDecimalLocale(locale: string): boolean {
  const base = locale.toLowerCase().split('-')[0] ?? '';
  return COMMA_DECIMAL_LOCALES.includes(base);
}

/**
 * Parses a user-typed monetary string into the canonical wire format:
 * a decimal string with exactly `fractionDigits` fractional digits
 * (e.g. "1500.00"), or "" when unparseable.
 *
 * Tolerance rules (locale-aware):
 *   1. Strip whitespace, currency symbols (€/$/£/¥), currency codes, nbsp.
 *   2. If both "," and "." appear → rightmost is the decimal separator;
 *      the other (and any repeats) is a thousands separator and is removed.
 *   3. If only one separator appears:
 *        - more than once            → thousands separator (removed)
 *        - with exactly 3 trailing digits → ambiguous, resolved by locale
 *        - with 0, 1, 2, or ≥4 trailing digits → decimal separator
 *   4. Numbers without a separator are integers.
 *
 * Non-numeric residue (after stripping) yields "".
 */
export function parseToWire(text: string, opts: ParseOptions = {}): string {
  const { locale = 'it-IT', fractionDigits = 2, allowNegative = false } = opts;

  let s = text.trim();
  if (s === '') return '';

  // Strip currency symbols, ISO codes, and any whitespace (incl. nbsp).
  s = s.replace(/[€$£¥]/g, '');
  s = s.replace(/\b(EUR|USD|GBP|JPY|CHF)\b/g, '');
  s = s.replace(/[\s\u00A0]/g, '');
  if (s === '') return '';

  // Sign handling.
  let negative = false;
  if (s.startsWith('-')) {
    if (allowNegative) negative = true;
    s = s.slice(1);
  } else if (s.startsWith('+')) {
    s = s.slice(1);
  }
  // Parenthetical negatives like "(1500)" — common in accounting.
  if (s.startsWith('(') && s.endsWith(')')) {
    if (allowNegative) negative = true;
    s = s.slice(1, -1);
  }

  // Keep only digits and separators from here.
  s = s.replace(/[^0-9.,]/g, '');
  if (s === '') return '';

  const commaCount = (s.match(/,/g) || []).length;
  const dotCount = (s.match(/\./g) || []).length;
  const lastComma = s.lastIndexOf(',');
  const lastDot = s.lastIndexOf('.');

  let decimalSep: string | null = null; // the char acting as decimal
  let thousandsSep: string | null = null; // the char acting as thousands

  if (commaCount > 0 && dotCount > 0) {
    // Both separators present → rightmost wins as decimal.
    if (lastComma > lastDot) {
      decimalSep = ',';
      thousandsSep = '.';
    } else {
      decimalSep = '.';
      thousandsSep = ',';
    }
  } else if (commaCount > 0) {
    // Only commas.
    if (commaCount > 1) {
      thousandsSep = ','; // repeated → thousands grouping
    } else {
      const trailing = s.length - 1 - lastComma;
      decimalSep = resolveSingleSeparator(trailing, ',', locale);
      if (decimalSep === null) thousandsSep = ',';
    }
  } else if (dotCount > 0) {
    // Only dots.
    if (dotCount > 1) {
      thousandsSep = '.';
    } else {
      const trailing = s.length - 1 - lastDot;
      decimalSep = resolveSingleSeparator(trailing, '.', locale);
      if (decimalSep === null) thousandsSep = '.';
    }
  }

  // Apply: drop thousands seps, normalize decimal to ".".
  if (thousandsSep) {
    const re = new RegExp(`\\${thousandsSep}`, 'g');
    s = s.replace(re, '');
  }
  if (decimalSep === ',') {
    s = s.replace(/,/g, '.');
  }

  const n = Number(s);
  if (!Number.isFinite(n)) return '';
  const magnitude = Math.abs(n).toFixed(fractionDigits);
  return negative ? '-' + magnitude : magnitude;
}

function resolveSingleSeparator(
  trailingDigits: number,
  sep: string,
  locale: string,
): string | null {
  // Exactly 3 trailing digits is genuinely ambiguous (1.500 = 1500 or 1.5?).
  // Resolve by locale convention; everything else with a single separator
  // and 0,1,2, or >=4 trailing digits is unambiguously a decimal point.
  if (trailingDigits === 3) {
    const decimalChar = isCommaDecimalLocale(locale) ? ',' : '.';
    return sep === decimalChar ? sep : null;
  }
  return sep;
}

/**
 * Formats a wire string for display using locale grouping, with exactly
 * `fractionDigits` fractional digits and NO currency symbol (the component's
 * adornment carries the symbol). Returns "" for non-numeric input.
 *
 * Presentation only — never used for state.
 */
export function formatForDisplay(wire: string, opts: FormatOptions = {}): string {
  const { locale = 'it-IT', fractionDigits = 2 } = opts;
  const n = Number(wire);
  if (!Number.isFinite(n)) return '';
  return new Intl.NumberFormat(locale, {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(n);
}

function currencySymbol(locale: string, currency: string): string {
  try {
    const parts = new Intl.NumberFormat(locale, {
      style: 'currency',
      currency,
    }).formatToParts(0);
    const lit = parts.find((p) => p.type === 'currency');
    return lit ? lit.value : currency;
  } catch {
    return currency;
  }
}

// ── Component ──

export interface MoneyInputProps {
  /** Canonical wire string (e.g. "1500.00"). "" means empty/cleared. */
  value: string;
  /** Always receives the canonical wire string (or ""). */
  onChange: (canonical: string) => void;
  label?: string;
  placeholder?: string;
  error?: string;
  disabled?: boolean;
  required?: boolean;
  id?: string;
  name?: string;
  autoFocus?: boolean;
  locale?: string;
  currency?: string;
  fractionDigits?: number;
  allowNegative?: boolean;
  showSymbol?: boolean;
}

export function MoneyInput({
  value,
  onChange,
  label,
  placeholder,
  error,
  disabled = false,
  required = false,
  id,
  name,
  autoFocus = false,
  locale = 'it-IT',
  currency = 'EUR',
  fractionDigits = 2,
  allowNegative = false,
  showSymbol = true,
}: MoneyInputProps) {
  const parseOpts = { locale, fractionDigits, allowNegative };
  const formatOpts = { locale, fractionDigits };

  const [displayText, setDisplayText] = useState(() =>
    value ? formatForDisplay(value, formatOpts) : '',
  );
  const [focused, setFocused] = useState(false);

  // Controlled sync: re-derive displayText from `value` ONLY when the field
  // is not focused. This lets the user type freely (even transiently
  // unparseable text) without the parent's canonical value clobbering their
  // in-progress input on every keystroke.
  useEffect(() => {
    if (focused) return;
    setDisplayText(value ? formatForDisplay(value, formatOpts) : '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, focused, locale, fractionDigits]);

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    const raw = e.target.value;
    setDisplayText(raw);
    onChange(parseToWire(raw, parseOpts));
  }

  function handleBlur() {
    setFocused(false);
    // Normalize the final text to its canonical display, or clear if empty.
    const wire = parseToWire(displayText, parseOpts);
    setDisplayText(wire ? formatForDisplay(wire, formatOpts) : '');
    if (wire !== value) onChange(wire);
  }

  const symbol = showSymbol ? currencySymbol(locale, currency) : '';

  return (
    <div className={`${styles.field} ${error ? styles.fieldError : ''}`}>
      {label && (
        <label htmlFor={id} className={styles.label}>
          {label}
          {required && <span className={styles.requiredMarker} aria-hidden="true" />}
        </label>
      )}
      <div
        className={`${styles.inputContainer} ${focused ? styles.focused : ''} ${
          disabled ? styles.disabled : ''
        }`}
      >
        {showSymbol && <span className={styles.symbol}>{symbol}</span>}
        {showSymbol && <span className={styles.divider} />}
        <input
          id={id}
          type="text"
          inputMode="decimal"
          className={styles.input}
          value={displayText}
          onChange={handleChange}
          onFocus={() => setFocused(true)}
          onBlur={handleBlur}
          disabled={disabled}
          required={required}
          autoFocus={autoFocus}
          placeholder={placeholder ?? defaultPlaceholder(locale, fractionDigits)}
          aria-invalid={error ? 'true' : undefined}
        />
      </div>
      {name && <input type="hidden" name={name} value={value} />}
      {error && <span className={styles.errorText}>{error}</span>}
    </div>
  );
}

function defaultPlaceholder(locale: string, fractionDigits: number): string {
  const zero = (0).toFixed(fractionDigits);
  // Render zero through the locale formatter to get the right separators,
  // then drop any leading "0" before the decimal for a cleaner placeholder
  // like "0,00" rather than "00,00".
  return formatForDisplay(zero, { locale, fractionDigits });
}
