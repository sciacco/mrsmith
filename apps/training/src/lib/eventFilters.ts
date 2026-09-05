// Filtri della lista /eventi: stato nell'URL, parser puri e testabili (#156).
// La lista non ha parametri server: il filtro e sempre client-side, applicato
// dopo il caricamento di GET /events.

import type { EventFlags, EventListRow } from '../api/types';

export const EVENT_CONDITIONS = [
  'all',
  'cancelled',
  'withoutSessions',
  'unassignedEnrollments',
  'inProgress',
  'needsReconciliation',
  'concluded',
] as const;

export type EventCondition = (typeof EVENT_CONDITIONS)[number];

export interface EventFiltersState {
  condition: EventCondition;
  courseId: string;
  q: string;
}

const CONDITION_PARAM = 'condizione';
const COURSE_PARAM = 'corso';
const QUERY_PARAM = 'q';

export const DEFAULT_EVENT_FILTERS: EventFiltersState = { condition: 'all', courseId: '', q: '' };

function isEventCondition(value: string): value is EventCondition {
  return (EVENT_CONDITIONS as readonly string[]).includes(value);
}

// parseEventFilters legge lo stato dei filtri dai search params correnti.
// Valori assenti o non riconosciuti tornano ai default: la funzione non
// solleva mai eccezioni su input arbitrario.
export function parseEventFilters(params: URLSearchParams): EventFiltersState {
  const rawCondition = params.get(CONDITION_PARAM) ?? '';
  return {
    condition: isEventCondition(rawCondition) ? rawCondition : 'all',
    courseId: params.get(COURSE_PARAM) ?? '',
    q: params.get(QUERY_PARAM) ?? '',
  };
}

// eventFiltersToParams produce i nuovi search params a partire da uno stato:
// i valori di default vengono omessi dall'URL invece di essere scritti
// esplicitamente, cosi l'URL resta pulito quando nessun filtro e attivo.
export function eventFiltersToParams(state: EventFiltersState, base: URLSearchParams): URLSearchParams {
  const next = new URLSearchParams(base);
  if (state.condition === 'all') next.delete(CONDITION_PARAM);
  else next.set(CONDITION_PARAM, state.condition);
  if (state.courseId === '') next.delete(COURSE_PARAM);
  else next.set(COURSE_PARAM, state.courseId);
  if (state.q === '') next.delete(QUERY_PARAM);
  else next.set(QUERY_PARAM, state.q);
  return next;
}

export function matchesEventCondition(flags: EventFlags, condition: EventCondition): boolean {
  if (condition === 'all') return true;
  return flags[condition];
}

export function matchesEventFilters(row: EventListRow, state: EventFiltersState): boolean {
  if (!matchesEventCondition(row.flags, state.condition)) return false;
  if (state.courseId !== '' && row.courseId !== state.courseId) return false;
  const q = state.q.trim().toLowerCase();
  if (q === '') return true;
  const haystack = `${row.title} ${row.courseTitle} ${row.vendorName ?? ''}`.toLowerCase();
  return haystack.includes(q);
}

export function filterEvents(rows: EventListRow[], state: EventFiltersState): EventListRow[] {
  return rows.filter((row) => matchesEventFilters(row, state));
}

export function isEventFiltersActive(state: EventFiltersState): boolean {
  return state.condition !== 'all' || state.courseId !== '' || state.q !== '';
}
