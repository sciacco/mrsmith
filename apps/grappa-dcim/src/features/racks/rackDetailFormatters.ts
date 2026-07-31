export interface InstantFormatterOptions {
  locale?: string;
  format?: Intl.DateTimeFormatOptions;
}

export interface NumberFormatterOptions {
  locale?: string;
  format?: Intl.NumberFormatOptions;
}

export type InstantFormatter = (value: string | null | undefined, options?: InstantFormatterOptions) => string | null;
export type NumberFormatter = (value: number | null | undefined, options?: NumberFormatterOptions) => string | null;

export interface RackDetailFormatterDependencies {
  formatInstant: InstantFormatter;
  formatNumber: NumberFormatter;
}

export interface RackDetailFormatterAdapter {
  formatDate(isoDate?: string): string;
  formatRackNumber(value?: number | null): string;
}

export function createRackDetailFormatterAdapter(
  dependencies: RackDetailFormatterDependencies,
): RackDetailFormatterAdapter {
  return {
    formatDate(isoDate?: string): string {
      return (
        dependencies.formatInstant(isoDate, {
          format: { day: 'numeric', month: 'short', year: 'numeric' },
        }) ?? '—'
      );
    },
    formatRackNumber(value?: number | null): string {
      return dependencies.formatNumber(value, { format: { maximumFractionDigits: 2 } }) ?? '—';
    },
  };
}
