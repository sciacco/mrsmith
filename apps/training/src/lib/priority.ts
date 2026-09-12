// Scala di priorità 1–5 condivisa dai selettori dei form richieste (e dalla
// visualizzazione). 1 = più importante: l'ordine va da "Molto Alta" (1) a
// "Molto bassa" (5). L'assenza (null/undefined) si legge "Nessuna".
export const PRIORITY_OPTIONS: { value: number; label: string }[] = [
  { value: 1, label: 'Molto Alta' },
  { value: 2, label: 'Alta' },
  { value: 3, label: 'Media' },
  { value: 4, label: 'Bassa' },
  { value: 5, label: 'Molto bassa' },
];

// Etichetta del valore selezionabile, per chi visualizza il numero grezzo
// (es. il dettaglio richiesta). Derivata da PRIORITY_OPTIONS: un'unica fonte.
export const PRIORITY_LABELS: Record<number, string> = Object.fromEntries(
  PRIORITY_OPTIONS.map((o) => [o.value, o.label]),
);

// Etichetta dello stato vuoto / assenza di priorità.
export const PRIORITY_NONE_LABEL = 'Nessuna';

// Etichetta con cui mostrare un valore di priorità: parola per 1–5, "—" per
// l'assenza, e il numero stesso come fallback per i valori fuori scala
// (dati legacy importati che non rientrano nella scala 1–5).
export function priorityDisplayLabel(priority: number | null | undefined): string {
  if (priority === null || priority === undefined) return '—';
  return PRIORITY_LABELS[priority] ?? String(priority);
}