const DEFAULT_LOCALE = 'it-IT';
const DEFAULT_INSTANT_TIME_ZONE = 'Europe/Rome';

const DEFAULT_DATE_FORMAT: Intl.DateTimeFormatOptions = {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
};
const DEFAULT_DATE_TIME_FORMAT: Intl.DateTimeFormatOptions = {
  ...DEFAULT_DATE_FORMAT,
  hour: '2-digit',
  minute: '2-digit',
};

export interface LocalDateFormatOptions {
  locale?: string;
  format?: Intl.DateTimeFormatOptions;
}

export interface LocalDateTimeFormatOptions {
  locale?: string;
  format?: Intl.DateTimeFormatOptions;
}

export interface InstantFormatOptions {
  locale?: string;
  format?: Intl.DateTimeFormatOptions;
}

type CalendarParts = {
  year: number;
  month: number;
  day: number;
};

type ClockParts = {
  hour: number;
  minute: number;
  second: number;
  milliseconds: number;
};

const DATE_PREFIX = /^(\d{4})-(\d{2})-(\d{2})(.*)$/;
const RFC3339_TAIL = /^T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$/;
const SQL_TAIL = /^ (\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?$/;
const LOCAL_DATE_TIME = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2})(?:\.(\d{1,9}))?)?$/;
const INSTANT = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$/;

function isLeapYear(year: number): boolean {
  return year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
}

function daysInMonth(year: number, month: number): number {
  if (month === 2) return isLeapYear(year) ? 29 : 28;
  return [4, 6, 9, 11].includes(month) ? 30 : 31;
}

function validCalendarDate(year: number, month: number, day: number): boolean {
  return month >= 1 && month <= 12 && day >= 1 && day <= daysInMonth(year, month);
}

function fractionToMilliseconds(fraction: string | undefined): number {
  if (!fraction) return 0;
  return Number(`${fraction}000`.slice(0, 3));
}

function validClockTime(hour: number, minute: number, second: number): boolean {
  return hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 && second >= 0 && second <= 59;
}

function parseClock(
  hourText: string,
  minuteText: string,
  secondText: string,
  fraction: string | undefined,
): ClockParts | null {
  const hour = Number(hourText);
  const minute = Number(minuteText);
  const second = Number(secondText);
  if (!validClockTime(hour, minute, second)) return null;
  return { hour, minute, second, milliseconds: fractionToMilliseconds(fraction) };
}

function validOffset(offset: string): boolean {
  if (offset === 'Z') return true;
  const hour = Number(offset.slice(1, 3));
  const minute = Number(offset.slice(4, 6));
  return hour <= 23 && minute <= 59;
}

function parseCalendarDate(yearText: string, monthText: string, dayText: string): CalendarParts | null {
  const year = Number(yearText);
  const month = Number(monthText);
  const day = Number(dayText);
  return validCalendarDate(year, month, day) ? { year, month, day } : null;
}

function parseLocalDate(value: string): CalendarParts | null {
  const match = DATE_PREFIX.exec(value);
  if (!match) return null;

  const date = parseCalendarDate(match[1]!, match[2]!, match[3]!);
  if (!date) return null;
  if (match[4] === '') return date;

  const rfcTail = RFC3339_TAIL.exec(match[4]!);
  if (rfcTail) {
    const clock = parseClock(rfcTail[1]!, rfcTail[2]!, rfcTail[3]!, rfcTail[4]);
    return clock && validOffset(rfcTail[5]!) ? date : null;
  }

  const sqlTail = SQL_TAIL.exec(match[4]!);
  if (sqlTail) return parseClock(sqlTail[1]!, sqlTail[2]!, sqlTail[3]!, sqlTail[4]) ? date : null;
  return null;
}

function parseLocalDateTime(value: string): (CalendarParts & ClockParts) | null {
  const match = LOCAL_DATE_TIME.exec(value);
  if (!match) return null;

  const date = parseCalendarDate(match[1]!, match[2]!, match[3]!);
  if (!date) return null;
  const clock = parseClock(match[4]!, match[5]!, match[6] ?? '00', match[7]);
  return clock ? { ...date, ...clock } : null;
}

function makeUtcCarrier(parts: CalendarParts & Partial<ClockParts>): Date {
  const carrier = new Date(0);
  carrier.setUTCFullYear(parts.year, parts.month - 1, parts.day);
  carrier.setUTCHours(parts.hour ?? 0, parts.minute ?? 0, parts.second ?? 0, parts.milliseconds ?? 0);
  return carrier;
}

function formatWithIntl(
  value: Date,
  locale: string | undefined,
  format: Intl.DateTimeFormatOptions | undefined,
  defaults: Intl.DateTimeFormatOptions,
  timeZone: string,
  allowTimeZone: boolean,
): string | null {
  if (format !== undefined && (format === null || typeof format !== 'object')) return null;
  if (!allowTimeZone && format !== undefined && 'timeZone' in format) return null;

  const intlOptions =
    format === undefined
      ? { ...defaults, timeZone }
      : { ...format, timeZone: allowTimeZone ? format.timeZone ?? timeZone : timeZone };
  return new Intl.DateTimeFormat(locale ?? DEFAULT_LOCALE, intlOptions).format(value);
}

export function formatLocalDate(
  value: string | null | undefined,
  options?: LocalDateFormatOptions,
): string | null {
  try {
    if (typeof value !== 'string') return null;
    const date = parseLocalDate(value);
    if (!date) return null;
    return formatWithIntl(
      makeUtcCarrier(date),
      options?.locale,
      options?.format,
      DEFAULT_DATE_FORMAT,
      'UTC',
      false,
    );
  } catch {
    return null;
  }
}

export function formatLocalDateTime(
  value: string | null | undefined,
  options?: LocalDateTimeFormatOptions,
): string | null {
  try {
    if (typeof value !== 'string') return null;
    const dateTime = parseLocalDateTime(value);
    if (!dateTime) return null;
    return formatWithIntl(
      makeUtcCarrier(dateTime),
      options?.locale,
      options?.format,
      DEFAULT_DATE_TIME_FORMAT,
      'UTC',
      false,
    );
  } catch {
    return null;
  }
}

export function formatInstant(
  value: string | null | undefined,
  options?: InstantFormatOptions,
): string | null {
  try {
    if (typeof value !== 'string') return null;
    const match = INSTANT.exec(value);
    if (!match) return null;

    const date = parseCalendarDate(match[1]!, match[2]!, match[3]!);
    const clock = parseClock(match[4]!, match[5]!, match[6]!, match[7]);
    if (!date || !clock || !validOffset(match[8]!)) return null;

    const instant = new Date(value);
    if (!Number.isFinite(instant.getTime())) return null;
    return formatWithIntl(
      instant,
      options?.locale,
      options?.format,
      DEFAULT_DATE_TIME_FORMAT,
      DEFAULT_INSTANT_TIME_ZONE,
      true,
    );
  } catch {
    return null;
  }
}
