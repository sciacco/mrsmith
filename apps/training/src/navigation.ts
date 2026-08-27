// Gruppi di navigazione dell'app Training (pattern afc-tools). La slice 6.5
// (#156) aggiunge "Eventi"; la slice 6.6 (#157) aggiunge "Richieste" e
// "Regole"; la slice 6.7 aggiunge le proprie voci qui, senza toccare App.tsx.

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
  { label: 'Richieste', items: [{ label: 'Richieste', path: '/richieste' }] },
  { label: 'Regole', items: [{ label: 'Regole', path: '/regole' }] },
  { label: 'Eventi', items: [{ label: 'Eventi', path: '/eventi' }] },
  { label: 'Factorial', items: [{ label: 'Factorial', path: '/factorial' }] },
];
