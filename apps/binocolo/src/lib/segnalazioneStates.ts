import type { Segnalazione, SegnalazioneState, SegnalazioneWrite } from '../api/types';

export interface SegnalazioneStateMeta {
  key: SegnalazioneState;
  label: string;
  /** Prefisso dei token `--kanban-<ramp>-*` riusati per la colonna; null = grigio neutro. */
  ramp: string | null;
  /** Stato terminale: la colonna cresce per sempre, sulla lavagna parte compressa
   *  e da espansa mostra solo le più recenti. */
  terminal: boolean;
}

/** Le quattro colonne della Kanban, nell'ordine di visualizzazione. */
export const SEGNALAZIONE_STATES: SegnalazioneStateMeta[] = [
  { key: 'da_gestire', label: 'Da gestire', ramp: 'approfondimento', terminal: false },
  { key: 'in_gestione', label: 'In gestione', ramp: 'primo-incontro', terminal: false },
  { key: 'chiusa', label: 'Chiusa', ramp: 'won', terminal: true },
  { key: 'annullata', label: 'Annullata', ramp: null, terminal: true },
];

/** Card mostrate in una colonna terminale espansa quando la ricerca è vuota. */
export const SEGNALAZIONE_TERMINAL_PREVIEW = 10;

export function isSegnalazioneTerminal(key: SegnalazioneState): boolean {
  return BY_KEY.get(key)?.terminal ?? false;
}

const BY_KEY = new Map(SEGNALAZIONE_STATES.map((s) => [s.key, s]));

export function segnalazioneStateLabel(key: string): string {
  return BY_KEY.get(key as SegnalazioneState)?.label ?? key;
}

export function isSegnalazioneState(value: string): value is SegnalazioneState {
  return BY_KEY.has(value as SegnalazioneState);
}

/** In `chiusa` i contenuti sono consultabili ma non modificabili; il cambio
 *  di stato resta sempre disponibile. */
export function segnalazioneEditable(state: SegnalazioneState): boolean {
  return state !== 'chiusa';
}

/** Variabili CSS `--c/--tint/--strong` della colonna, riusando la rampa
 *  Kanban esistente (nessun token nuovo). */
export function segnalazioneStateVars(key: SegnalazioneState): Record<string, string> {
  const ramp = BY_KEY.get(key)?.ramp;
  if (!ramp) {
    return { '--c': 'var(--color-text-faint)', '--tint': 'var(--color-surface)', '--strong': 'var(--color-text-secondary)' };
  }
  return {
    '--c': `var(--kanban-${ramp}-base)`,
    '--tint': `var(--kanban-${ramp}-tint)`,
    '--strong': `var(--kanban-${ramp}-text)`,
  };
}

/** Regola minima di compilazione, identica a quella del backend: almeno uno
 *  tra nome, sito web, identificativo fiscale e note dopo il taglio degli spazi. */
export function segnalazioneHasContent(value: SegnalazioneWrite): boolean {
  return [value.name, value.website, value.fiscalId, value.notes].some((field) => field.trim() !== '');
}

export const SEGNALAZIONE_EMPTY_RULE = 'Compila almeno uno tra nome, sito web, identificativo fiscale e note.';

/** Titolo della card quando il nome manca: primo non vuoto fra sito web,
 *  identificativo fiscale, località e prima riga delle note. */
export function segnalazioneTitle(value: Pick<Segnalazione, 'name' | 'website' | 'fiscalId' | 'location' | 'notes'>): string {
  const name = value.name.trim();
  if (name) return name;
  const website = value.website.trim();
  if (website) return website;
  const fiscalId = value.fiscalId.trim();
  if (fiscalId) return fiscalId;
  const location = value.location.trim();
  if (location) return location;
  const firstLine = value.notes.split(/\r?\n/).map((line) => line.trim()).find((line) => line !== '');
  return firstLine ?? 'Segnalazione senza titolo';
}

export function emptySegnalazioneWrite(): SegnalazioneWrite {
  return { name: '', website: '', location: '', fiscalId: '', contacts: '', notes: '' };
}

export function toSegnalazioneWrite(value: Segnalazione): SegnalazioneWrite {
  return { name: value.name, website: value.website, location: value.location, fiscalId: value.fiscalId, contacts: value.contacts, notes: value.notes };
}
