// Gruppi di navigazione dell'app Training (pattern afc-tools). La slice 6.5
// (#156) aggiunge "Eventi"; le slice 6.6–6.7 aggiungono le proprie voci qui,
// senza toccare App.tsx.

export interface TrainingNavItem {
  label: string;
  path: string;
}

export interface TrainingNavGroup {
  label: string;
  items: TrainingNavItem[];
}

export const trainingNavGroups: TrainingNavGroup[] = [
  { label: 'Coda', items: [{ label: 'Coda', path: '/' }] },
  { label: 'Eventi', items: [{ label: 'Eventi', path: '/eventi' }] },
  { label: 'Factorial', items: [{ label: 'Factorial', path: '/factorial' }] },
];
