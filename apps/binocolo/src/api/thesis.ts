import type { MAThesis } from './types';

export type ThesisMeta = {
  label: string;
  description: string;
};

export const THESIS_META: Record<MAThesis, ThesisMeta> = {
  generico: {
    label: 'Generico',
    description: 'Bilancia aderenza, opportunità ed economia; l’anzianità dell’azienda non è valutata.',
  },
  successione: {
    label: 'Successione',
    description: 'Privilegia proprietà concentrata, azienda matura e titolare vicino al ricambio; penalizza il controllo di una holding.',
  },
  crescita: {
    label: 'Crescita',
    description: 'Premia i fondamentali economici — trend del fatturato, produttività e solidità — e l’azienda giovane.',
  },
  consolidamento: {
    label: 'Consolidamento',
    description: 'Dà massimo peso all’aderenza al perimetro; le situazioni di difficoltà economica restano azionabili.',
  },
  tuck_in: {
    label: 'Competenze',
    description: 'Privilegia l’aderenza di settore e competenze; il profilo economico pesa meno.',
  },
};

export const THESIS_OPTIONS = (Object.entries(THESIS_META) as Array<[MAThesis, ThesisMeta]>).map(
  ([value, meta]) => ({ value, ...meta }),
);

export function thesisLabel(value: MAThesis | 'unknown'): string {
  return value === 'unknown' ? 'Non definita' : THESIS_META[value].label;
}
