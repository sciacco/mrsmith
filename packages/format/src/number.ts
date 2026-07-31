const DEFAULT_LOCALE = 'it-IT';
const DEFAULT_CURRENCY = 'EUR';
const SUPPORTED_CURRENCIES = new Set(Intl.supportedValuesOf('currency'));

function isSupportedCurrency(currency: string): boolean {
  return SUPPORTED_CURRENCIES.has(currency);
}

export interface NumberFormatOptions {
  locale?: string;
  format?: Intl.NumberFormatOptions;
}

export interface CurrencyFormatOptions {
  locale?: string;
  format?: Omit<Intl.NumberFormatOptions, 'style' | 'currency'>;
}

function validOptions(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object';
}

export function formatNumber(
  value: number | null | undefined,
  options?: NumberFormatOptions,
): string | null {
  try {
    if (typeof value !== 'number' || !Number.isFinite(value)) return null;
    if (options !== undefined && !validOptions(options)) return null;
    if (options?.locale !== undefined && typeof options.locale !== 'string') return null;
    if (options?.format !== undefined && !validOptions(options.format)) return null;

    return new Intl.NumberFormat(options?.locale ?? DEFAULT_LOCALE, options?.format).format(value);
  } catch {
    return null;
  }
}

export function formatCurrency(
  value: number | null | undefined,
  currency: string = DEFAULT_CURRENCY,
  options?: CurrencyFormatOptions,
): string | null {
  try {
    if (typeof value !== 'number' || !Number.isFinite(value)) return null;
    if (typeof currency !== 'string') return null;
    if (options !== undefined && !validOptions(options)) return null;
    if (options?.locale !== undefined && typeof options.locale !== 'string') return null;
    if (options?.format !== undefined && !validOptions(options.format)) return null;
    if (!isSupportedCurrency(currency)) return null;

    const format = options?.format === undefined ? {} : { ...options.format };
    return new Intl.NumberFormat(options?.locale ?? DEFAULT_LOCALE, {
      ...format,
      style: 'currency',
      currency,
    }).format(value);
  } catch {
    return null;
  }
}
