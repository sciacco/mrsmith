// Unica fonte TS per gli stati della card di lavorazione v2 (KANBAN-V2-PLAN.md
// §5). Sostituisce le mappe stati/esiti duplicate. Le chiavi coincidono col
// backend (ma_types.go, migrazione 112) e coi token cromatici (styles/tokens.css,
// `--kanban-<stato>-{base,tint,text}`, direzione A "rampa termica" ratificata a G0).

export type MacrofaseKey = 'origination' | 'engagement' | 'followup' | 'esito';

export type CardStateKey =
  | 'approfondimento'
  | 'da_contattare'
  | 'primo_contatto'
  | 'primo_incontro'
  | 'visita'
  | 'nda'
  | 'loi'
  | 'ricontattare'
  | 'won'
  | 'ko_nostro'
  | 'ko_target'
  | 'rimossa';

export interface CardStateMeta {
  key: CardStateKey;
  label: string;
  /** Sigla della strip funnel: APP/DC/PC/PI/VI/NDA/LOI/R/W/KN/KT. */
  abbr: string;
  macrofase: MacrofaseKey;
  terminal: boolean;
}

/** Gli 11 stati operativi in ordine di funnel. `rimossa` è fuori board (errore di
 *  triage, mai un verdetto): ha solo una label, non compare qui. */
export const CARD_STATES: CardStateMeta[] = [
  { key: 'approfondimento', label: 'Approfondimento', abbr: 'APP', macrofase: 'origination', terminal: false },
  { key: 'da_contattare', label: 'Da contattare', abbr: 'DC', macrofase: 'origination', terminal: false },
  { key: 'primo_contatto', label: 'Primo contatto', abbr: 'PC', macrofase: 'origination', terminal: false },
  { key: 'primo_incontro', label: 'Primo incontro', abbr: 'PI', macrofase: 'engagement', terminal: false },
  { key: 'visita', label: 'Visita', abbr: 'VI', macrofase: 'engagement', terminal: false },
  { key: 'nda', label: 'NDA', abbr: 'NDA', macrofase: 'engagement', terminal: false },
  { key: 'loi', label: 'LOI', abbr: 'LOI', macrofase: 'engagement', terminal: false },
  { key: 'ricontattare', label: 'Ricontattare', abbr: 'R', macrofase: 'followup', terminal: false },
  { key: 'won', label: 'WON', abbr: 'W', macrofase: 'esito', terminal: true },
  { key: 'ko_nostro', label: 'KO nostro', abbr: 'KN', macrofase: 'esito', terminal: true },
  { key: 'ko_target', label: 'KO target', abbr: 'KT', macrofase: 'esito', terminal: true },
];

export const REMOVED_STATE_KEY = 'rimossa';
export const REMOVED_STATE_LABEL = 'Rimossa';

const STATE_BY_KEY = new Map<string, CardStateMeta>(CARD_STATES.map((s) => [s.key, s]));

/** Gli 8 stati non terminali, in ordine di funnel (colonne della board attiva). */
export const ACTIVE_STATES: CardStateMeta[] = CARD_STATES.filter((s) => !s.terminal);
/** I 3 stati terminali (Esito). Si raggiungono da /close, non da /state. */
export const TERMINAL_STATES: CardStateMeta[] = CARD_STATES.filter((s) => s.terminal);

export interface MacrofaseMeta {
  key: MacrofaseKey;
  label: string;
  /** Numerazione grafica: 01..04. */
  num: string;
  /** Stati figli in ordine. */
  states: CardStateKey[];
  /** Visibile in "Pipeline attiva" (Esito solo in "Vista completa"). */
  activeView: boolean;
}

export const MACROFASI: MacrofaseMeta[] = [
  { key: 'origination', label: 'Origination', num: '01', states: ['approfondimento', 'da_contattare', 'primo_contatto'], activeView: true },
  { key: 'engagement', label: 'Engagement', num: '02', states: ['primo_incontro', 'visita', 'nda', 'loi'], activeView: true },
  { key: 'followup', label: 'Follow-up', num: '03', states: ['ricontattare'], activeView: true },
  { key: 'esito', label: 'Esito', num: '04', states: ['won', 'ko_nostro', 'ko_target'], activeView: false },
];

export function isKnownState(key: string): key is CardStateKey {
  return key === REMOVED_STATE_KEY || STATE_BY_KEY.has(key);
}

export function isTerminalState(key: string): boolean {
  return STATE_BY_KEY.get(key)?.terminal ?? false;
}

export function stateMacrofase(key: string): MacrofaseKey | null {
  return STATE_BY_KEY.get(key)?.macrofase ?? null;
}

export function stateAbbr(key: string): string {
  return STATE_BY_KEY.get(key)?.abbr ?? key.toUpperCase();
}

/** Etichetta di stato. `rimossa` e le chiavi legacy del diario (payload storici,
 *  mai riscritti) risolvono qui; il testo sconosciuto passa invariato. */
export function stateLabel(key: string): string {
  if (key === REMOVED_STATE_KEY) return REMOVED_STATE_LABEL;
  return STATE_BY_KEY.get(key)?.label ?? LEGACY_STATE_LABELS[key] ?? key;
}

/** Nome-token di uno stato per l'uso in `var(--kanban-…)`: gli underscore delle
 *  chiavi diventano trattini (ko_nostro → ko-nostro). */
function cssStateKey(key: string): string {
  return key.replace(/_/g, '-');
}

/** Variabili CSS della rampa per uno stato, da applicare inline: il CSS del
 *  componente consuma poi `var(--c)`, `var(--tint)`, `var(--strong)`. Stati
 *  sconosciuti (o `rimossa`) ricadono su un grigio neutro. */
export function stateVars(key: string): Record<string, string> {
  if (!STATE_BY_KEY.has(key)) {
    return { '--c': 'var(--color-text-faint)', '--tint': 'var(--color-surface)', '--strong': 'var(--color-text-secondary)' };
  }
  const k = cssStateKey(key);
  return {
    '--c': `var(--kanban-${k}-base)`,
    '--tint': `var(--kanban-${k}-tint)`,
    '--strong': `var(--kanban-${k}-text)`,
  };
}

/** Variabili CSS di una macrofase (header colonne in vista Macrofasi): colore
 *  pieno `--mc` (token) + tinta di sfondo derivata via color-mix. */
export function macroVars(key: MacrofaseKey): Record<string, string> {
  return {
    '--mc': `var(--kanban-macro-${key})`,
    '--mtint': `color-mix(in srgb, var(--kanban-macro-${key}) 9%, var(--color-bg-elevated))`,
  };
}

export interface EsitoSuggestion {
  key: string;
  label: string;
  description: string;
}

/** Vocabolario suggerito per l'esito dei KO (combobox + testo libero ammesso).
 *  WON non porta esito. Per la calibrazione dello score le chiavi note mantengono
 *  il segnale; il testo libero vale neutro/da classificare (§1.1). */
export const ESITO_SUGGESTIONS: Record<'ko_nostro' | 'ko_target', EsitoSuggestion[]> = {
  ko_nostro: [
    { key: 'non_idonea', label: 'Non idonea', description: 'alla prova dei fatti non è in profilo per questa iniziativa' },
    { key: 'no_go_strategico', label: 'No-go strategico', description: 'idonea, ma si sceglie di non procedere' },
    { key: 'prezzo', label: 'Prezzo', description: 'condizioni economiche non sostenibili' },
  ],
  ko_target: [
    { key: 'non_vende', label: 'Non vende', description: 'il titolare non intende vendere' },
    { key: 'in_trattativa_altrui', label: 'In trattativa con altri', description: '' },
    { key: 'prezzo_richiesto', label: 'Prezzo richiesto', description: 'aspettative di prezzo fuori range' },
    { key: 'altro', label: 'Altro', description: '' },
  ],
};

/** Fatti tipizzati propagabili al registro azienda: solo per `ko_target` (§6). */
export const REGISTRY_BRIDGE_FACTS: Array<{ key: string; label: string; hint: string }> = [
  { key: 'non_vende', label: 'Non vende', hint: 'il titolare non intende vendere' },
  { key: 'in_trattativa_altrui', label: 'In trattativa con altri', hint: '' },
];

/** Label legacy per i renderer del diario/storico. I payload append-only del log
 *  NON si riscrivono (ground truth): le chiavi stato/esito pregresse risolvono qui.
 *  Consumata SOLO da timeline/diario, mai per gli stati vivi della board. */
export const LEGACY_STATE_LABELS: Record<string, string> = {
  // stati pregressi
  contattata: 'Contattata',
  in_dialogo: 'In dialogo',
  offerta: 'Offerta',
  chiusa: 'Chiusa',
  // esiti storici
  conclusa: 'Conclusa',
  no_go: 'No-go',
  non_idonea: 'Non idonea',
  sfumata: 'Sfumata',
  rimandata: 'Rimandata',
};

const ESITO_LABEL_BY_KEY: Record<string, string> = {
  ...Object.fromEntries([...ESITO_SUGGESTIONS.ko_nostro, ...ESITO_SUGGESTIONS.ko_target].map((s) => [s.key, s.label])),
  ...LEGACY_STATE_LABELS,
};

/** Etichetta di un esito: risolve il vocabolario suggerito e le chiavi storiche;
 *  il testo libero passa invariato. */
export function esitoLabel(esito: string): string {
  if (!esito) return '';
  return ESITO_LABEL_BY_KEY[esito] ?? esito;
}
