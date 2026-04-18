import { lazy } from 'react';
import { Navigate, type RouteObject } from 'react-router-dom';

const SituazioneRackPage = lazy(() =>
  import('./pages/SituazioneRackPage').then((module) => ({ default: module.SituazioneRackPage })),
);
const ConsumiKwPage = lazy(() =>
  import('./pages/ConsumiKwPage').then((module) => ({ default: module.ConsumiKwPage })),
);
const AddebitiPage = lazy(() =>
  import('./pages/AddebitiPage').then((module) => ({ default: module.AddebitiPage })),
);
const SenzaVariabilePage = lazy(() =>
  import('./pages/SenzaVariabilePage').then((module) => ({ default: module.SenzaVariabilePage })),
);
const ConsumiBassiPage = lazy(() =>
  import('./pages/ConsumiBassiPage').then((module) => ({ default: module.ConsumiBassiPage })),
);

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/situazione-rack" replace /> },
  { path: 'situazione-rack', element: <SituazioneRackPage /> },
  { path: 'consumi-kw', element: <ConsumiKwPage /> },
  { path: 'addebiti', element: <AddebitiPage /> },
  { path: 'senza-variabile', element: <SenzaVariabilePage /> },
  { path: 'consumi-bassi', element: <ConsumiBassiPage /> },
  { path: '*', element: <Navigate to="/situazione-rack" replace /> },
];
