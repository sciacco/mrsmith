// Scala dei livelli 0–5 condivisa dai selettori dei form richieste e
// anagrafiche (#170.D). Distinta dalla scala descrittiva ASSESSMENT_LEVEL_LABELS
// (lib/labels.ts), che resta il copy presentazionale: qui servono solo i valori
// selezionabili, 0 distinto dall'assenza (undefined).
export const LEVEL_VALUES = [0, 1, 2, 3, 4, 5] as const;

export const LEVEL_OPTIONS = LEVEL_VALUES.map((value) => ({ value, label: String(value) }));
