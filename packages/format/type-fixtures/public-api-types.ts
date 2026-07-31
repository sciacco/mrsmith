import type {
  CurrencyFormatOptions,
  InstantFormatOptions,
  LocalDateFormatOptions,
  LocalDateTimeFormatOptions,
  NumberFormatOptions,
} from '../src/index.ts';

const numberOptions: NumberFormatOptions = {
  locale: 'en-US',
  format: { maximumFractionDigits: 2 },
};

const currencyOptions: CurrencyFormatOptions = {
  locale: 'en-US',
  format: { maximumFractionDigits: 2 },
};

const localDateOptions: LocalDateFormatOptions = {
  locale: 'en-GB',
  format: { day: '2-digit', month: '2-digit', year: 'numeric' },
};

const localDateTimeOptions: LocalDateTimeFormatOptions = {
  locale: 'en-GB',
  format: { hour: '2-digit', minute: '2-digit', second: '2-digit' },
};

const instantOptions: InstantFormatOptions = {
  locale: 'en-GB',
  format: { timeZone: 'UTC', dateStyle: 'short', timeStyle: 'short' },
};

void numberOptions;
void currencyOptions;
void localDateOptions;
void localDateTimeOptions;
void instantOptions;
