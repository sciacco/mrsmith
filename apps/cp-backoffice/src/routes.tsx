import { Navigate, type RouteObject } from 'react-router-dom';
import { StatoAziendePage } from './views/StatoAziende/StatoAziendePage';
import { GestioneUtentiPage } from './views/GestioneUtenti/GestioneUtentiPage';
import { AccessiBiometricoPage } from './views/AccessiBiometrico/AccessiBiometricoPage';

export function createRoutes(canAccessFullBackoffice: boolean): RouteObject[] {
  if (!canAccessFullBackoffice) {
    return [
      { index: true, element: <Navigate to="/accessi-biometrico" replace /> },
      { path: 'stato-aziende', element: <Navigate to="/accessi-biometrico" replace /> },
      { path: 'gestione-utenti', element: <Navigate to="/accessi-biometrico" replace /> },
      { path: 'accessi-biometrico', element: <AccessiBiometricoPage /> },
    ];
  }

  return [
    { index: true, element: <Navigate to="/stato-aziende" replace /> },
    { path: 'stato-aziende', element: <StatoAziendePage /> },
    { path: 'gestione-utenti', element: <GestioneUtentiPage /> },
    { path: 'accessi-biometrico', element: <AccessiBiometricoPage /> },
  ];
}
