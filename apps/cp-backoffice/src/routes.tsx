import { Navigate, type RouteObject } from 'react-router-dom';
import { StatoAziendePage } from './views/StatoAziende/StatoAziendePage';
import { GestioneUtentiPage } from './views/GestioneUtenti/GestioneUtentiPage';
import { AccessiBiometricoPage } from './views/AccessiBiometrico/AccessiBiometricoPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/stato-aziende" replace /> },
  { path: 'stato-aziende', element: <StatoAziendePage /> },
  { path: 'gestione-utenti', element: <GestioneUtentiPage /> },
  { path: 'accessi-biometrico', element: <AccessiBiometricoPage /> },
];
