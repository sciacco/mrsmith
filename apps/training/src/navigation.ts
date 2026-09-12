// Gruppi di navigazione dell'app Training (pattern afc-tools). La slice 6.5
// (#156) aggiunge "Eventi"; la slice 6.6 (#157) aggiunge "Richieste" e
// "Regole"; la slice 6.7 aggiunge le proprie voci qui, senza toccare App.tsx.
// Il gruppo "Catalogo", scomposto dalle sottoviste di CatalogPage, ha quattro
// voci che aprono un menu a tendina in TabNavGroup invece di puntare a
// un'unica pagina con vista interna.

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
  { label: 'Pianificazione', items: [{ label: 'Pianificazione', path: '/pianificazione' }] },
  { label: 'Richieste', items: [{ label: 'Richieste', path: '/richieste' }] },
  { label: 'Regole', items: [{ label: 'Regole', path: '/regole' }] },
  { label: 'Eventi', items: [{ label: 'Eventi', path: '/eventi' }] },
  { label: 'Persone', items: [{ label: 'Persone', path: '/persone' }] },
  {
    label: 'Catalogo',
    items: [
      { label: 'Corsi', path: '/catalogo/corsi' },
      { label: 'Anagrafiche', path: '/catalogo/anagrafiche' },
      { label: 'Certificazioni', path: '/catalogo/certificazioni' },
      { label: 'Percorsi', path: '/catalogo/percorsi' },
    ],
  },
  { label: 'Report', items: [{ label: 'Report', path: '/report' }] },
  { label: 'Factorial', items: [{ label: 'Factorial', path: '/factorial' }] },
];
