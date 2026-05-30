import { Navigate, type RouteObject } from 'react-router-dom';
import { StatoAziendePage } from '@mrsmith/features';
import { GestioneUtentiPage } from './views/GestioneUtenti/GestioneUtentiPage';
import { AccessiBiometricoPage } from './views/AccessiBiometrico/AccessiBiometricoPage';
import { SezioniCPPage } from './views/SezioniCP/SezioniCPPage';

export function createRoutes(
  canAccessFullBackoffice: boolean,
  canAccessBiometrics: boolean,
): RouteObject[] {
  if (!canAccessBiometrics) {
    if (canAccessFullBackoffice) {
      return [
        { index: true, element: <Navigate to="/stato-aziende" replace /> },
        { path: 'stato-aziende', element: <StatoAziendePage /> },
        { path: 'sezioni-cp', element: <SezioniCPPage /> },
        { path: 'gestione-utenti', element: <GestioneUtentiPage /> },
        { path: 'accessi-biometrico', element: <Navigate to="/stato-aziende" replace /> },
        { path: '*', element: <Navigate to="/stato-aziende" replace /> },
      ];
    }
    return [
      { path: '*', element: <Navigate to="/" replace /> },
    ];
  }

  if (!canAccessFullBackoffice) {
    return [
      { index: true, element: <Navigate to="/accessi-biometrico" replace /> },
      { path: 'stato-aziende', element: <Navigate to="/accessi-biometrico" replace /> },
      { path: 'sezioni-cp', element: <Navigate to="/accessi-biometrico" replace /> },
      { path: 'gestione-utenti', element: <Navigate to="/accessi-biometrico" replace /> },
      { path: 'accessi-biometrico', element: <AccessiBiometricoPage /> },
      { path: '*', element: <Navigate to="/accessi-biometrico" replace /> },
    ];
  }

  return [
    { index: true, element: <Navigate to="/stato-aziende" replace /> },
    { path: 'stato-aziende', element: <StatoAziendePage /> },
    { path: 'sezioni-cp', element: <SezioniCPPage /> },
    { path: 'gestione-utenti', element: <GestioneUtentiPage /> },
    { path: 'accessi-biometrico', element: <AccessiBiometricoPage /> },
    { path: '*', element: <Navigate to="/stato-aziende" replace /> },
  ];
}
