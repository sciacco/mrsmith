// Conversioni data/ora condivise dalle superfici evento (#156): il backend
// restituisce i timestamptz come testo Postgres ("2026-08-27 21:00:00+00") e
// le date self-paced sono scritte a mezzanotte UTC per convenzione di
// progetto (docs/knowledge/training.md). Nessuna logica di dominio qui, solo
// formattazione/parsing puri.

import { formatInstant, formatLocalDate } from '@mrsmith/format';

const ROME_TIME_ZONE = 'Europe/Rome';

// Normalizza il cast testuale Postgres di una colonna timestamptz in RFC
// 3339: separatore spazio -> "T", offset a due cifre -> offset a quattro
// cifre. "Z" e gli offset gia a quattro cifre restano invariati.
export function toRfc3339Instant(value: string): string {
  const withT = value.includes('T') ? value : value.replace(' ', 'T');
  const offsetMatch = /^(.*)([+-]\d{2})$/.exec(withT);
  return offsetMatch ? `${offsetMatch[1]}${offsetMatch[2]}:00` : withT;
}

export function formatInstantDateTime(value: string | undefined | null): string {
  if (!value) return '—';
  return formatInstant(toRfc3339Instant(value)) ?? value;
}

// Giorno civile nel fuso Europe/Rome di un timestamptz (es. la scadenza
// self-paced, scritta a mezzanotte UTC: in Rome cade sempre sullo stesso
// giorno, mai su quello precedente).
export function formatInstantDate(value: string | undefined | null): string {
  if (!value) return '—';
  return (
    formatInstant(toRfc3339Instant(value), { format: { day: '2-digit', month: '2-digit', year: 'numeric' } }) ??
    value
  );
}

export function formatDateOnly(value: string | undefined | null): string {
  if (!value) return '—';
  return formatLocalDate(value) ?? value;
}

// ── Input <input type="date"> per scadenze self-paced: mezzanotte UTC ──

export function dateValueToUtcMidnightInstant(value: string): string {
  return value === '' ? '' : `${value}T00:00:00Z`;
}

export function instantToUtcDateValue(value: string | undefined): string {
  if (!value) return '';
  const instant = new Date(toRfc3339Instant(value));
  if (Number.isNaN(instant.getTime())) return '';
  return instant.toISOString().slice(0, 10);
}

// ── Input <input type="datetime-local"> per sessioni con date: interpretato
// come ora civile di Europe/Rome, convertito nell'istante UTC per il filo.

function romeUtcOffsetMinutes(atUtcGuess: Date): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: ROME_TIME_ZONE,
    hour12: false,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).formatToParts(atUtcGuess);
  const at = Object.fromEntries(parts.map((p) => [p.type, p.value]));
  const asIfUtc = Date.UTC(
    Number(at.year),
    Number(at.month) - 1,
    Number(at.day),
    Number(at.hour) === 24 ? 0 : Number(at.hour),
    Number(at.minute),
    Number(at.second),
  );
  return Math.round((asIfUtc - atUtcGuess.getTime()) / 60000);
}

const LOCAL_DATE_TIME_INPUT = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/;

export function localDateTimeValueToInstant(value: string): string {
  const match = LOCAL_DATE_TIME_INPUT.exec(value);
  if (!match) return '';
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const hour = Number(match[4]);
  const minute = Number(match[5]);
  const guessUtc = new Date(Date.UTC(year, month - 1, day, hour, minute, 0));
  const offsetMinutes = romeUtcOffsetMinutes(guessUtc);
  const trueUtc = new Date(guessUtc.getTime() - offsetMinutes * 60000);
  return trueUtc.toISOString().replace(/\.\d{3}Z$/, 'Z');
}

export function instantToLocalDateTimeValue(value: string | undefined): string {
  if (!value) return '';
  const instant = new Date(toRfc3339Instant(value));
  if (Number.isNaN(instant.getTime())) return '';
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: ROME_TIME_ZONE,
    hour12: false,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).formatToParts(instant);
  const at = Object.fromEntries(parts.map((p) => [p.type, p.value]));
  const hour = at.hour === '24' ? '00' : at.hour;
  return `${at.year}-${at.month}-${at.day}T${hour}:${at.minute}`;
}
