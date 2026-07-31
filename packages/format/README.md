# @mrsmith/format

Small, rendering-safe formatters for values whose meaning is already known. Choose a
formatter from field semantics, not by autodetecting a string: a date, local date-time,
instant, number, and currency are different kinds of data.

These functions distinguish semantic values from wire representations. Wire values are
validated and interpreted only by the temporal formatter that documents that wire format;
numeric formatters accept numbers, not numeric strings. Formatting does not mutate the
wire value, apply business rounding, or parse money strings.

## API

All functions return `string | null` and do not throw while rendering. `null`, `undefined`,
malformed values, non-finite numbers, and invalid `Intl` locales/options return `null`.
There is no UI fallback or placeholder added by this package. Callers decide how a null
result should be presented.

`locale` defaults to `it-IT`. Each `format` option is passed to the native `Intl` API;
when provided, it replaces that formatter's defaults rather than being merged with them.
Invalid native options return `null`.

### `formatLocalDate(value, options?)`

Formats a civil date without timezone conversion. Accepted wire representations are:

- `YYYY-MM-DD`
- the same date followed by `THH:mm:ss[.fraction](Z|±HH:mm)`
- the same date followed by a space and `HH:mm:ss[.fraction]`

Fractions contain one to nine digits. Calendar and clock values must be valid. The
formatting timezone is fixed to `UTC`; a `timeZone` in `format` is rejected.

```ts
formatLocalDate('2026-07-16'); // '16/07/2026'
formatLocalDate('2026-07-16', { locale: 'en-GB' }); // '16/07/2026'
```

### `formatLocalDateTime(value, options?)`

Formats a local civil date-time without timezone conversion. Accepted values are
`YYYY-MM-DD[T ]HH:mm`, optionally followed by `:ss` and one to nine fractional-second
digits. Timezone-bearing values are rejected. The formatting timezone is fixed to `UTC`.

```ts
formatLocalDateTime('2026-07-16T12:30'); // '16/07/2026, 12:30'
formatLocalDateTime('2026-07-16 12:30', { locale: 'en-GB' }); // '16/07/2026, 12:30'
```

### `formatInstant(value, options?)`

Formats an RFC 3339 instant: `YYYY-MM-DDTHH:mm:ss`, optionally with one to nine
fractional-second digits, followed by `Z` or an explicit `±HH:mm` offset. The default
timezone is `Europe/Rome`, including its daylight-saving rules. An instant may override
that timezone through `format.timeZone`.

```ts
formatInstant('2026-07-16T12:30:00Z'); // '16/07/2026, 14:30'
formatInstant('2026-07-16T12:30:00Z', { format: { timeZone: 'UTC' } }); // '16/07/2026, 12:30'
```

### `formatNumber(value, options?)`

Accepts only finite numbers and uses native `Intl.NumberFormat` behavior. It does not
pre-round or impose precision when no `format` is supplied.

```ts
formatNumber(12345.67); // '12.345,67'
formatNumber(12.5, { format: { minimumFractionDigits: 3 } }); // '12,500'
```

### `formatCurrency(value, currency?, options?)`

Accepts only finite numbers. `currency` defaults to `EUR` and is passed to native
`Intl.NumberFormat`; unsupported codes and other invalid currencies return `null`. No
currency aliases are normalized, and strings representing money are not parsed. Native
currency precision applies by default and can be overridden with `format`.

`style: 'currency'` and the selected `currency` are always forced at runtime. They are
also excluded from `CurrencyFormatOptions`, so `format` cannot replace them.

```ts
formatCurrency(1234.56); // '1234,56 €'
formatCurrency(1234.56, 'USD', { locale: 'en-US' }); // '$1,234.56'
formatCurrency(12.3456, 'EUR', { format: { maximumFractionDigits: 4 } }); // '12,3456 €'
```

The package only formats values; it does not select formatters, provide UI fallbacks, or
change application/domain data.
